# Installation guide for AI agents

Install **Codex MCP Portal** as a standard stdio MCP server in the user's chosen client. OpenCode is one supported configuration target, not a runtime dependency. Reuse the user's existing Codex ChatGPT login and connected services. This is an unofficial bridge for hosted connector tools, not a loader for arbitrary plugin Skills, Hooks, or local MCP manifests.

## 1. Check the environment

- Read the repository's `README.md` and applicable local instructions.
- Identify the target MCP client from the user's request or existing context; ask if unclear. Do not configure a different client silently.
- Check `node --version` and `git --version`. For OpenCode, also check `opencode --version` and `opencode mcp list`.
- Require Node.js 24 or newer. If `node` is an unresolved version-manager shim, activate an installed compatible version before continuing.
- Resolve the actual executable with `node -p 'process.execPath'`. Use this absolute path in the MCP command, not a shim.
- Inspect the target client's MCP configuration before writing. Look for both `codex-mcp-portal` and the earlier `codex_apps` name. Avoid duplicate connections and never overwrite an unrelated server.
- Check the intended clone directory. Reuse an existing installation only after confirming its remote and working-tree state.
- Check whether `${CODEX_HOME:-$HOME/.codex}/auth.json` exists and is readable. If necessary, validate only that `tokens.access_token` and `tokens.account_id` are nonempty strings, returning booleans only. Never display, copy, log, or commit their values.
- If file-backed ChatGPT credentials are absent, ask the user to complete Codex login with file-backed credentials. This bridge cannot read keychain-only credentials or API-key-only authentication. Do not extract browser cookies as a fallback.

## 2. Install

Read `bridge.ts` and `package.json` before installing. Pick one of these methods.

### Packaged CLI

```sh
npm install -g git+https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git
codex-mcp-portal --help
```

Git installation runs `prepare` to compile TypeScript into `dist/bridge.js`. The CLI runs this JavaScript because Node.js does not strip TypeScript under `node_modules`. No npm registry package is published; use the full Git URL. Do not assume `npx codex-mcp-portal` resolves to this repository.

### Direct clone

For a fresh installation on macOS/Linux:

```sh
INSTALL_DIR="$HOME/.local/share/codex-mcp-portal"
git clone https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git "$INSTALL_DIR"
NODE_BIN="$(node -p 'process.execPath')"
```

No runtime dependencies or build step are needed for the direct-clone method. Do not clone over an existing directory or remove local modifications.

For development validation, run these commands inside the clone:

```sh
npm ci --ignore-scripts
npm run typecheck
npm test
```

These tests use mocked requests, not the user's account.

## 3. Register in the selected client

For a client using `mcpServers`, merge an entry such as:

```json
{
  "mcpServers": {
    "codex-mcp-portal": {
      "command": "codex-mcp-portal",
      "args": []
    }
  }
}
```

For direct-clone installations, use the resolved absolute Node path as `command` and the absolute `bridge.ts` path as the single argument. If needed, supply `env.CODEX_HOME` with an absolute path. Use the selected client's documented config location and reload mechanism; the JSON above is not a universal config-file format.

GUI applications may not inherit a shell's PATH. Check executable resolution and use absolute paths rather than claiming that installation alone makes the command available. The installed CLI's shebang also needs `node` on PATH; the direct-clone configuration can avoid both lookups.

### OpenCode V2

Prefer the CLI, which preserves unrelated configuration:

```sh
opencode mcp add codex_apps --global -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"
```

Use `--global` for a personal installation. Omit it only if the user requests a project-local installation, and run the command in that project.

For a packaged installation, replace the Node/source command with `codex-mcp-portal` (or the verified absolute executable path). Use only one registration method.

If the user uses a non-default `CODEX_HOME`, pass its absolute path explicitly:

```sh
opencode mcp add codex_apps --global \
  --env "CODEX_HOME=$CODEX_HOME" \
  -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"
```

Only use this second form when `CODEX_HOME` is actually set. Do not put access tokens, account IDs, or refresh tokens in the configuration. If manual editing is necessary, merge `mcp.servers.codex_apps` using the V2 shape in the README and preserve other entries.

## 4. Verify through the client

1. Check the client's MCP connection status. In OpenCode, run `opencode mcp list` and require `codex_apps connected` before reporting the server as connected.
2. Discover tools through the client's catalog. Search for Google Drive tools; do not assume all accounts expose the same tool names or count. OpenCode normalizes names; other clients may retain names such as `google_drive.recent_documents`.
3. If the user authorized a read check, use the discovered recent-documents tool with `top_k: 1`. Retrieve metadata only; do not create, edit, share, or delete a file for testing.
4. Distinguish connection success, catalog availability, and successful service calls. A connected transport alone does not prove Google Drive access.
5. Report the installation path, config scope, verified service, and any remaining blocker. Do not include credentials or unnecessary private file metadata.

OpenCode V2 watches configuration changes; use `/mcps` to inspect or reconnect. Other clients have their own reload mechanisms. Do not invoke an unrelated MCP OAuth flow: this bridge uses Codex's file-backed ChatGPT session. Do not register an HTTP URL; the portal's client-facing transport is stdio only.

## Failure handling

- **401 / 403:** Have the user update their Codex ChatGPT login and reconnect. A 403 can also indicate an endpoint or access change; do not promise login will resolve every case.
- **Account changed:** Reconnect before making further service calls.
- **Missing tools:** Check the user's ChatGPT plugin availability and service connection. Do not install an unrelated replacement MCP without asking.
- **Timeout or failed write:** Never blindly replay. The remote action may already have completed.
- **Unsupported flow:** This bridge does not implement custom UI, server-initiated elicitation/sampling, browser automation, or plugin Skills installation. Report the limitation rather than claiming full plugin compatibility.

The bridge rereads credentials on each request but never uses the refresh token itself. Ongoing token renewal depends on Codex updating the credentials file.

## Updating or removing

- Check `git status --short` and the remote before updates. Use `git pull --ff-only` on a clean checkout; never reset user changes.
- Reconnect the MCP server after updating source files so the running process uses the new code.
- For the packaged CLI, rerun installation from the same Git URL to update; use `npm uninstall -g codex-mcp-portal` to remove it when requested.
- To uninstall, remove only this server's entry from the configuration where it was registered. Delete the clone only if requested. Preserve Codex credentials and the user's ChatGPT connections.
