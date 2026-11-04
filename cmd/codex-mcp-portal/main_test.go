package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHelpWithoutCredentials(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	var output, diagnostics bytes.Buffer
	if code := run([]string{"--help"}, nil, &output, &diagnostics); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(output.String(), "Codex MCP Portal") || diagnostics.Len() != 0 {
		t.Fatal("unexpected help output")
	}
}

func TestUnknownArguments(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if code := run([]string{"--http"}, nil, &output, &diagnostics); code != 2 {
		t.Fatalf("exit code %d", code)
	}
	if output.Len() != 0 || !strings.Contains(diagnostics.String(), "Unknown arguments") {
		t.Fatal("unexpected argument error")
	}
}

func TestStdioInvalidInput(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if code := run(nil, strings.NewReader("not-json\n"), &output, &diagnostics); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	var response struct {
		JSONRPC string             `json:"jsonrpc"`
		Error   struct{ Code int } `json:"error"`
	}
	if json.Unmarshal(output.Bytes(), &response) != nil || response.JSONRPC != "2.0" || response.Error.Code != -32600 {
		t.Fatal("stdout must contain only the protocol error")
	}
	if diagnostics.Len() != 0 {
		t.Fatal("unexpected diagnostic output")
	}
}
