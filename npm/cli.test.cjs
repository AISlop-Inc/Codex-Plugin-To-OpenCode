const { test } = require('node:test');
const assert = require('node:assert/strict');
const { binaryPath } = require('./cli.cjs');

test('resolves all six platform binaries and rejects unsupported targets', () => {
  for (const platform of ['darwin', 'linux', 'win32']) {
    for (const arch of ['x64', 'arm64']) {
      assert.ok(binaryPath(platform, arch).includes(`${platform}-${arch}`));
      assert.equal(binaryPath(platform, arch).endsWith('.exe'), platform === 'win32');
    }
  }
  assert.throws(() => binaryPath('freebsd', 'x64'), /Unsupported platform/);
  assert.throws(() => binaryPath('linux', 'ia32'), /Unsupported platform/);
});
