package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/internal/auth"
	"github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/internal/portal"
)

const help = `Codex MCP Portal

Usage: codex-mcp-portal [--help]

Expose existing ChatGPT connector tools as a stdio MCP server.
Configure this command in any stdio-compatible MCP client.

Credentials: $CODEX_HOME/auth.json (default: ~/.codex/auth.json)
Token renewal is managed by Codex. No HTTP listener is started.
`

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(output, help)
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(diagnostics, "Unknown arguments. Use codex-mcp-portal --help.")
		return 2
	}

	client := portal.NewClient(auth.Load)
	if err := portal.Serve(context.Background(), input, output, diagnostics, client); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
