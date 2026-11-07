package portal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/internal/auth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func fixtureAuth() (auth.Credentials, error) {
	return auth.Credentials{AccessToken: "fixture-token", AccountID: "account-a"}, nil
}

func response(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func mustRequest(t *testing.T, wire string) request {
	t.Helper()
	message, err := parseRequest([]byte(wire))
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func TestAuthRotationAndSessionHeaders(t *testing.T) {
	credentials, _ := fixtureAuth()
	client := NewClient(func() (auth.Credentials, error) { return credentials, nil })
	calls := 0
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != endpoint || req.Header.Get("Authorization") != "Bearer "+credentials.AccessToken || req.Header.Get("ChatGPT-Account-ID") != credentials.AccountID {
			t.Error("unexpected authentication or endpoint")
		}
		if req.Header.Get("X-OpenAI-Product-Sku") != "codex" {
			t.Error("missing product SKU")
		}
		if calls == 1 {
			var body struct {
				Params struct {
					Capabilities map[string]any `json:"capabilities"`
					ClientInfo   map[string]any `json:"clientInfo"`
				} `json:"params"`
			}
			if json.NewDecoder(req.Body).Decode(&body) != nil || len(body.Params.Capabilities) != 0 || body.Params.ClientInfo["name"] != "test" {
				t.Error("initialization must preserve client info but clear unsupported capabilities")
			}
			result := response(200, "application/json", `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
			result.Header.Set("Mcp-Session-Id", "fixture-session")
			return result, nil
		}
		if req.Header.Get("Mcp-Session-Id") != "fixture-session" || req.Header.Get("MCP-Protocol-Version") != "2025-03-26" {
			t.Error("missing negotiated headers")
		}
		return response(200, "application/json", `{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}`), nil
	})
	initialize := mustRequest(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{"sampling":{}},"clientInfo":{"name":"test"}}}`)
	if _, err := client.relay(context.Background(), initialize); err != nil {
		t.Fatal(err)
	}
	credentials.AccessToken = "refreshed-token"
	list := mustRequest(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if _, err := client.relay(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	credentials.AccountID = "account-b"
	if _, err := client.relay(context.Background(), list); err == nil || !strings.Contains(err.Error(), "account changed") {
		t.Fatal("account switch must fail")
	}
	if calls != 2 {
		t.Fatal("account switch reached upstream")
	}
}

func TestRedirectAndFailedWriteAreNotReplayed(t *testing.T) {
	for _, status := range []int{302, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := NewClient(fixtureAuth)
			calls := 0
			client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.String() != endpoint {
					t.Error("credentials followed a redirect")
				}
				result := response(status, "text/html", "private upstream body")
				result.Header.Set("Location", "https://other.invalid/mcp")
				return result, nil
			})
			message := mustRequest(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
			_, err := client.relay(context.Background(), message)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", status)) || strings.Contains(err.Error(), "private") {
				t.Fatal("unexpected error disclosure")
			}
			if calls != 1 {
				t.Fatal("failed operation was replayed")
			}
		})
	}
}

func TestSSEAndLargeJSONResponses(t *testing.T) {
	sse := "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\r\n\r\n" +
		"data: {\"jsonrpc\":\"2.0\",\r\ndata: \"id\":8,\"result\":{\"tools\":[]}}\r\n\r\n"
	result, err := readResponse(response(200, "text/event-stream", sse), json.RawMessage(`8`))
	if err != nil || string(result) != `{"jsonrpc":"2.0","id":8,"result":{"tools":[]}}` {
		t.Fatalf("SSE failed: %s, %v", result, err)
	}
	large := `{"jsonrpc":"2.0","id":8,"result":{"text":"` + strings.Repeat("x", 100000) + `"}}`
	for _, contentType := range []string{"application/json", "text/event-stream"} {
		body := large
		if contentType == "text/event-stream" {
			body = "data: " + large + "\n\n"
		}
		if _, err := readResponse(response(200, contentType, body), json.RawMessage(`8`)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestToolResultStructuredContent(t *testing.T) {
	for _, test := range []struct {
		name   string
		method string
		result string
		want   string
	}{
		{
			name:   "null",
			method: "tools/call",
			result: `{"content":[{"type":"text","text":"Slack result"}],"structuredContent":null,"_meta":{"source":"slack"}}`,
			want:   `{"content":[{"type":"text","text":"Slack result"}],"_meta":{"source":"slack"}}`,
		},
		{
			name:   "tool error",
			method: "tools/call",
			result: `{"content":[{"type":"text","text":"Search failed"}],"structuredContent":null,"isError":true}`,
			want:   `{"content":[{"type":"text","text":"Search failed"}],"isError":true}`,
		},
		{
			name:   "object",
			method: "tools/call",
			result: `{"content":[],"structuredContent":{"matches":[],"optional":null}}`,
			want:   `{"content":[],"structuredContent":{"matches":[],"optional":null}}`,
		},
		{
			name:   "absent",
			method: "tools/call",
			result: `{"content":[]}`,
			want:   `{"content":[]}`,
		},
		{
			name:   "invalid non-null remains invalid",
			method: "tools/call",
			result: `{"content":[],"structuredContent":[]}`,
			want:   `{"content":[],"structuredContent":[]}`,
		},
		{
			name:   "other methods unchanged",
			method: "tools/list",
			result: `{"tools":[],"structuredContent":null}`,
			want:   `{"tools":[],"structuredContent":null}`,
		},
	} {
		for _, contentType := range []string{"application/json", "text/event-stream"} {
			t.Run(test.name+"/"+contentType, func(t *testing.T) {
				client := NewClient(fixtureAuth)
				wire := `{"jsonrpc":"2.0","id":9007199254740993,"result":` + test.result + `,"extension":{"keep":true}}`
				client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
					body := wire
					if contentType == "text/event-stream" {
						body = "data: " + wire + "\n\n"
					}
					return response(200, contentType, body), nil
				})
				message := mustRequest(t, `{"jsonrpc":"2.0","id":9007199254740993,"method":"`+test.method+`"}`)
				result, err := client.relay(context.Background(), message)
				if err != nil {
					t.Fatal(err)
				}
				var envelope map[string]json.RawMessage
				if err := json.Unmarshal(result, &envelope); err != nil {
					t.Fatal(err)
				}
				if string(envelope["id"]) != "9007199254740993" || string(envelope["extension"]) != `{"keep":true}` {
					t.Fatal("response identity or extensions changed")
				}
				var got, want any
				if err := json.Unmarshal(envelope["result"], &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(test.want), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("result = %s, want %s", envelope["result"], test.want)
				}
			})
		}
	}
}

func TestToolRPCErrorIsPreserved(t *testing.T) {
	client := NewClient(fixtureAuth)
	wire := `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Invalid arguments","data":{"field":"keywords"}}}`
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(200, "application/json", wire), nil
	})
	message := mustRequest(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	result, err := client.relay(context.Background(), message)
	if err != nil || string(result) != wire {
		t.Fatalf("RPC error changed: %s, %v", result, err)
	}
}

func TestNotificationsHaveNoResponse(t *testing.T) {
	client := NewClient(fixtureAuth)
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(204, "", ""), nil
	})
	message := mustRequest(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if result, err := client.relay(context.Background(), message); err != nil || result != nil {
		t.Fatal("notification must not generate a response")
	}
	request := mustRequest(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if _, err := client.relay(context.Background(), request); err == nil {
		t.Fatal("empty response to a request must fail")
	}
}

func TestConcurrentStdioResponsesAndMetadata(t *testing.T) {
	client := NewClient(fixtureAuth)
	var calls atomic.Int32
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		var message struct {
			ID     json.RawMessage `json:"id"`
			Params struct {
				Meta map[string]string `json:"_meta"`
			} `json:"params"`
		}
		if err := json.NewDecoder(req.Body).Decode(&message); err != nil {
			return nil, err
		}
		if message.Params.Meta["client"] != "test" {
			t.Error("request metadata was lost")
		}
		return response(200, "application/json", fmt.Sprintf("{\n\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{}}", message.ID)), nil
	})
	var input, output, diagnostics bytes.Buffer
	for i := range 30 {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/list","params":{"_meta":{"client":"test"}}}`+"\n", i)
	}
	if err := Serve(context.Background(), &input, &output, &diagnostics, client); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	seen := map[int]bool{}
	for _, line := range lines {
		var message struct{ ID int }
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatal("interleaved stdout responses")
		}
		seen[message.ID] = true
	}
	if len(seen) != 30 || calls.Load() != 30 || diagnostics.Len() != 0 {
		t.Fatal("missing or duplicate responses")
	}
}
