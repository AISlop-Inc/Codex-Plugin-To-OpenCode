#!/usr/bin/env node
const { spawnSync } = require('node:child_process');
const { join } = require('node:path');

function binaryPath(platform, arch) {
  if (!['darwin', 'linux', 'win32'].includes(platform) || !['x64', 'arm64'].includes(arch)) {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }
  return join(__dirname, '..', 'binaries', `${platform}-${arch}`, platform === 'win32' ? 'codex-mcp-portal.exe' : 'codex-mcp-portal');
}

module.exports = { binaryPath };
if (require.main === module) {
  const child = spawnSync(binaryPath(process.platform, process.arch), process.argv.slice(2), { stdio: 'inherit' });
  if (child.error) { console.error(child.error.message); process.exit(1); }
  if (child.signal) { process.kill(process.pid, child.signal); }
  else { process.exit(child.status ?? 1); }
}
