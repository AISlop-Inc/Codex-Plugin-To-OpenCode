package portal

import (
	"encoding/json"
	"testing"
)

func TestInvalidRequests(t *testing.T) {
	for _, wire := range []string{
		`null`, `[]`, `{"jsonrpc":"2.0","id":{},"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":true,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","method":"initialize","params":null}`,
	} {
		if _, err := parseRequest([]byte(wire)); err == nil {
			t.Fatalf("accepted invalid request %s", wire)
		}
	}
}

func TestResponseValidation(t *testing.T) {
	for _, wire := range []string{
		`null`,
		`{"jsonrpc":"2.0","id":1,"result":"invalid"}`,
		`{"jsonrpc":"2.0","id":1,"result":null}`,
		`{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":1,"message":"x"}}`,
		`{"jsonrpc":"2.0","id":1,"error":{"message":"x"}}`,
		`{"jsonrpc":"2.0","id":2,"result":{}}`,
	} {
		if _, err := validateResponse([]byte(wire), json.RawMessage(`1`)); err == nil {
			t.Fatalf("accepted invalid response %s", wire)
		}
	}
	for _, id := range []string{`9007199254740993`, `"string-id"`, `null`} {
		wire := `{"jsonrpc":"2.0","id":` + id + `,"result":{}}`
		if _, err := validateResponse([]byte(wire), json.RawMessage(id)); err != nil {
			t.Fatal(err)
		}
	}
	if !sameID(json.RawMessage(`"a\u0062"`), json.RawMessage(`"ab"`)) {
		t.Fatal("escaped string IDs must match")
	}
	if sameID(json.RawMessage(`9007199254740993`), json.RawMessage(`9007199254740992`)) {
		t.Fatal("large IDs lost precision")
	}
}
