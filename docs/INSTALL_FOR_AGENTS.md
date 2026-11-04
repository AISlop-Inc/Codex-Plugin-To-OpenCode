# Installation guide for AI agents

Install this repository as a local MCP server in OpenCode V2. Reuse the user's existing Codex ChatGPT login and connected services. This is an unofficial bridge, not a general Codex plugin installer.

## 1. Check the environment

- Read the repository's `README.md` and applicable local instructions.
- Check `node --version`, `git --version`, `opencode --version`, and `opencode mcp list`.
- Require Node.js 24 or newer. If `node` is an unresolved version-manager shim, activate an installed compatible version before continuing.
- Resolve the actual executable with `node -p 'process.execPath'`. Use this absolute path in the MCP command, not a shim.
- Check the intended clone directory and existing `codex_apps` server before writing anything. Reuse an existing installation only after confirming its remote and working-tree state. Do not overwrite another server with this name.
- Check whether `${CODEX_HOME:-$HOME/.codex}/auth.json` exists and is readable. If necessary, validate only that `tokens.access_token` and `tokens.account_id` are nonempty strings, returning booleans only. Never display, copy, log, or commit their values.
- If file-backed ChatGPT credentials are absent, ask the user to complete Codex login with file-backed credentials. This bridge cannot read keychain-only credentials or API-key-only authentication. Do not extract browser cookies as a fallback.

## 2. Clone and inspect

For a fresh installation on macOS/Linux:

```sh
INSTALL_DIR="$HOME/.local/share/opencode/codex-plugin-to-opencode"
git clone https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git "$INSTALL_DIR"
NODE_BIN="$(node -p 'process.execPath')"
```

Read `bridge.ts` before registering it. No runtime dependencies or build step are needed. Do not clone over an existing directory or remove local modifications.

For development validation, run these commands inside the clone:

```sh
npm ci --ignore-scripts
npm run typecheck
npm test
```

These tests use mocked requests, not the user's account.

## 3. Register in OpenCode

Prefer the CLI, which preserves unrelated configuration:

```sh
opencode mcp add codex_apps --global -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"
```

Use `--global` for a personal installation. Omit it only if the user requests a project-local installation, and run the command in that project.

If the user uses a non-default `CODEX_HOME`, pass its absolute path explicitly:

```sh
opencode mcp add codex_apps --global \
  --env "CODEX_HOME=$CODEX_HOME" \
  -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"
```

Only use this second form when `CODEX_HOME` is actually set. Do not put access tokens, account IDs, or refresh tokens in the configuration. If manual editing is necessary, merge `mcp.servers.codex_apps` using the V2 shape in the README and preserve other entries.

## 4. Verify through OpenCode

1. Run `opencode mcp list` and require `codex_apps connected` before reporting the server as connected.
2. Discover tools through the current OpenCode catalog. Search for Google Drive tools; do not assume all accounts expose the same tool names or count.
3. If the user authorized a read check, use the discovered recent-documents tool with `top_k: 1`. Retrieve metadata only; do not create, edit, share, or delete a file for testing.
4. Distinguish connection success, catalog availability, and successful service calls. A connected transport alone does not prove Google Drive access.
5. Report the installation path, config scope, verified service, and any remaining blocker. Do not include credentials or unnecessary private file metadata.

OpenCode V2 watches configuration changes. A full application restart is not normally required. Use `/mcps` to inspect or reconnect the server. Do not invoke an unrelated MCP OAuth flow: this bridge uses Codex's file-backed ChatGPT session.

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
- To uninstall, remove only this server's entry from the configuration where it was registered. Delete the clone only if requested. Preserve Codex credentials and the user's ChatGPT connections.
