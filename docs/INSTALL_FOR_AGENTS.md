# Installation guide for AI agents

Install **Codex MCP Portal**, a dependency-free Go executable exposing hosted Codex/ChatGPT connector tools over standard stdio MCP. OpenCode is one configuration target, not a runtime dependency. This is not a loader for arbitrary plugin Skills, Hooks, or local MCP manifests.

## 1. Establish the target

- Read `README.md` and applicable local instructions.
- Identify the user's MCP client; ask if it is unclear. Use that client's documented configuration location and reload mechanism.
- Inspect existing `codex-mcp-portal` and `codex_apps` entries before writing. Preserve unrelated configuration and avoid duplicate connections.
- Check `go version`; building requires Go 1.24 or newer. The resulting executable does not need Go or Node.js installed to run.
- Check for readable `${CODEX_HOME:-$HOME/.codex}/auth.json`. If validation is needed, report only whether `tokens.access_token` and `tokens.account_id` are nonempty strings. Never display, copy, log, or commit credentials.
- File-backed ChatGPT credentials are required. If absent, ask the user to complete Codex login using file-backed storage. Keychain-only and API-key-only logins are unsupported; do not extract browser cookies as a fallback.

## 2. Install one way

### Go installation

```sh
go install github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/cmd/codex-mcp-portal@latest
```

Resolve the actual executable under `go env GOBIN`, or the first `go env GOPATH` entry's `bin` directory when GOBIN is unset. Do not assume a GUI client inherits the shell's PATH. Run the executable with `--help` to verify the path.

### Build from a clone

For a fresh installation on macOS/Linux:

```sh
INSTALL_DIR="$HOME/.local/share/codex-mcp-portal"
git clone https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git "$INSTALL_DIR"
cd "$INSTALL_DIR"
go test -race ./...
go vet ./...
go build -o bin/codex-mcp-portal ./cmd/codex-mcp-portal
"$INSTALL_DIR/bin/codex-mcp-portal" --help
```

Before reusing an existing clone, check its remote and working-tree state. Never overwrite or reset local changes. Review `cmd/codex-mcp-portal`, `internal/auth`, and `internal/portal` as needed. Tests use mocked HTTP and temporary credentials, not the user's real account.

No npm installation, TypeScript compilation, external Go modules, or generated configuration is needed. Prebuilt release downloads are not currently provided.

## 3. Register the executable

For clients using `mcpServers`, merge this entry with a verified absolute executable path:

```json
{
  "mcpServers": {
    "codex-mcp-portal": {
      "command": "/absolute/path/to/codex-mcp-portal",
      "args": []
    }
  }
}
```

Use `env.CODEX_HOME` for a non-default absolute Codex directory. Do not include tokens, account IDs, or refresh tokens. This JSON shape is not universal: follow the selected client's schema. The portal provides stdio, not an HTTP listener.

### OpenCode V2

For a new personal installation:

```sh
opencode mcp add codex_apps --global -- /absolute/path/to/codex-mcp-portal
opencode mcp list
```

Omit `--global` only for an explicitly project-local setup. When needed, add `--env "CODEX_HOME=$CODEX_HOME"` before `--`, using a nonempty absolute directory. OpenCode V2 configuration uses `mcp.servers.codex_apps`; a local server has `type: "local"` and a `command` array containing the executable path.

When migrating the previous TypeScript version, keep the server name and replace its entire `node …/bridge.ts` command with the Go executable. Preserve its environment, permissions, and unrelated settings. Do not register a second server. Do not uninstall the old runtime until the Go connection has been verified.

## 4. Verify in the client

1. Require successful MCP connection before reporting it as connected. In OpenCode, check `opencode mcp list` for `codex_apps connected`.
2. Discover the current tool catalog. Do not assume every account exposes the same tools or count. Names may be normalized by the client.
3. If a read check is authorized, discover the Google Drive recent-documents tool and call it with `top_k: 1`. Read metadata only; never create, edit, share, or delete data to test installation.
4. Distinguish executable startup, MCP connection, catalog discovery, and successful service calls. Connection alone does not prove Drive access.
5. Report the binary path, config scope, verified service, and blockers. Avoid unnecessary private metadata.

OpenCode watches config changes; `/mcps` can inspect or reconnect the server. Other clients have their own reload mechanisms. After replacing a binary, reconnect to launch the updated executable. Do not invoke an unrelated MCP OAuth flow: the portal uses Codex's file-backed ChatGPT session.

## Failures and limitations

- **401 / 403:** Update the Codex login and reconnect. A 403 can also reflect access or endpoint changes; login may not fix it.
- **Account changed:** Reconnect before further calls.
- **Missing tools:** Check the ChatGPT plugin and service connection. Do not silently replace it with an unrelated MCP server.
- **Timeout / failed write:** Verify the outcome before retrying; the operation may already have completed.
- **Large messages:** JSON-RPC messages and SSE events have a 32 MiB limit.
- **Unsupported flows:** No custom UI, server-initiated elicitation/sampling, browser automation, or Skills installation.

Credentials are reread on every request; the portal does not use refresh tokens or update the credential file. Codex owns token renewal. Write tools are exposed when upstream provides them; permissions belong to the MCP client.

## Updates and removal

Update with the same `go install` command, or fast-forward a clean checkout and rebuild. Reconnect the MCP server afterward. If removing the portal, remove only its server entry and the executable when requested; preserve Codex credentials and ChatGPT service connections.
