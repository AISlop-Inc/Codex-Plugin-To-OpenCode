import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

const entrypoint = fileURLToPath(new URL('./dist/bridge.js', import.meta.url));

test('packaged CLI works through an npm-style symlink without credentials', () => {
    const directory = mkdtempSync(join(tmpdir(), 'codex-mcp-portal-'));
    try {
        const executable = join(directory, 'codex-mcp-portal');
        symlinkSync(entrypoint, executable);
        const result = spawnSync(process.execPath, [executable, '--help'], {
            encoding: 'utf8',
            env: { ...process.env, CODEX_HOME: directory },
            timeout: 5000
        });
        assert.equal(result.status, 0, result.stderr);
        assert.match(result.stdout, /Codex MCP Portal/);
        assert.match(result.stdout, /stdio/);
        assert.equal(result.stderr, '');
    } finally {
        rmSync(directory, { recursive: true, force: true });
    }
});

test('stdio mode emits protocol JSON only and rejects invalid input', () => {
    const result = spawnSync(process.execPath, [entrypoint], {
        input: 'not-json\n', encoding: 'utf8', timeout: 5000
    });
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(JSON.parse(result.stdout), {
        jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Invalid MCP request.' }
    });
    assert.equal(result.stderr, '');
});

test('unsupported arguments fail without starting MCP', () => {
    const result = spawnSync(process.execPath, [entrypoint, '--http'], {
        encoding: 'utf8', timeout: 5000
    });
    assert.equal(result.status, 2);
    assert.equal(result.stdout, '');
    assert.match(result.stderr, /Unknown arguments/);
});
