package portal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/internal/auth"
)

const endpoint = "https://chatgpt.com/backend-api/ps/mcp"

type Client struct {
	loadAuth func() (auth.Credentials, error)
	http     *http.Client
	mu       sync.Mutex
	account  string
	session  string
	protocol string
}

func NewClient(loadAuth func() (auth.Credentials, error)) *Client {
	return &Client{
		loadAuth: loadAuth,
		http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

func (client *Client) headers() (http.Header, error) {
	credentials, err := client.loadAuth()
	if err != nil {
		return nil, err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.account != "" && client.account != credentials.AccountID {
		return nil, errors.New("Codex account changed. Reconnect this MCP server.")
	}
	client.account = credentials.AccountID
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+credentials.AccessToken)
	headers.Set("ChatGPT-Account-ID", client.account)
	headers.Set("X-OpenAI-Product-Sku", "codex")
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json, text/event-stream")
	if client.session != "" {
		headers.Set("Mcp-Session-Id", client.session)
	}
	if client.protocol != "" {
		headers.Set("MCP-Protocol-Version", client.protocol)
	}
	return headers, nil
}

func (client *Client) relay(ctx context.Context, message request) (json.RawMessage, error) {
	headers, err := client.headers()
	if err != nil {
		return nil, err
	}
	wire, err := prepareRequest(message)
	if err != nil {
		return nil, err
	}
	timeout := time.Minute
	if message.Method == "tools/call" {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(wire))
	if err != nil {
		return nil, errors.New("Could not create MCP request.")
	}
	req.Header = headers
	// Never replay failed tool calls: they may already have changed remote data.
	response, err := client.http.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("MCP request timed out; outcome unknown.")
		}
		return nil, errors.New("MCP transport failed.")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		hint := ""
		if response.StatusCode == 401 || response.StatusCode == 403 {
			hint = " Refresh the ChatGPT login in Codex, then reconnect."
		}
		return nil, fmt.Errorf("ChatGPT MCP returned HTTP %d.%s", response.StatusCode, hint)
	}
	client.mu.Lock()
	if session := response.Header.Get("Mcp-Session-Id"); session != "" {
		client.session = session
	}
	client.mu.Unlock()
	if message.ID == nil {
		return nil, nil
	}
	if response.StatusCode == 202 || response.StatusCode == 204 {
		return nil, errors.New("MCP request returned no result.")
	}
	result, err := readResponse(response, message.ID)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("MCP request timed out; outcome unknown.")
		}
		return nil, err
	}
	if message.Method == "initialize" {
		var initialization struct {
			Result struct {
				Protocol string `json:"protocolVersion"`
			} `json:"result"`
		}
		if json.Unmarshal(result, &initialization) == nil && initialization.Result.Protocol != "" {
			client.mu.Lock()
			client.protocol = initialization.Result.Protocol
			client.mu.Unlock()
		}
	}
	return result, nil
}

func readResponse(response *http.Response, id json.RawMessage) (json.RawMessage, error) {
	contentType := response.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		wire, err := io.ReadAll(io.LimitReader(response.Body, maxMessageSize+1))
		if err != nil || len(wire) > maxMessageSize {
			return nil, errors.New("MCP response unreadable or exceeds 32 MiB.")
		}
		return validateResponse(wire, id)
	}
	if !strings.Contains(contentType, "text/event-stream") {
		return nil, errors.New("Unexpected MCP response format.")
	}
	return readEvents(response.Body, id)
}

func readEvents(body io.Reader, id json.RawMessage) (json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), maxMessageSize)
	var event bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if event.Len()+len(data)+1 > maxMessageSize {
				return nil, errors.New("MCP event exceeds 32 MiB.")
			}
			event.WriteString(data)
			event.WriteByte('\n')
		} else if line == "" && event.Len() != 0 {
			var message struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(event.Bytes(), &message) != nil {
				return nil, errors.New("Invalid MCP event.")
			}
			if sameID(message.ID, id) {
				return validateResponse(event.Bytes(), id)
			}
			event.Reset()
		}
	}
	if scanner.Err() != nil {
		return nil, errors.New("MCP event stream unreadable or exceeds 32 MiB.")
	}
	return nil, errors.New("MCP event stream ended without a response.")
}
