const { spawnSync } = require('node:child_process');
const { mkdirSync, chmodSync } = require('node:fs');
const { dirname } = require('node:path');
const { binaryPath } = require('./cli.cjs');

for (const platform of ['darwin', 'linux', 'win32']) {
  for (const arch of ['x64', 'arm64']) {
    const output = binaryPath(platform, arch);
    mkdirSync(dirname(output), { recursive: true });
    const result = spawnSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', output, './cmd/codex-mcp-portal'], {
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '0', GOOS: platform === 'win32' ? 'windows' : platform, GOARCH: arch === 'x64' ? 'amd64' : arch },
    });
    if (result.error) throw result.error;
    if (result.status !== 0) process.exit(result.status ?? 1);
    chmodSync(output, 0o755);
  }
}
