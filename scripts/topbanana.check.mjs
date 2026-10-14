// Run: node scripts/topbanana.check.mjs  (offline: checks the config dir/file modes save() leaves behind)
import assert from 'node:assert/strict';
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const { save } = await import('./topbanana.mjs');
const root = mkdtempSync(join(tmpdir(), 'tbcheck-'));
const dir = join(root, '.config', 'rb2go'), file = join(dir, 'topbanana.json');
mkdirSync(dir, { recursive: true }); chmodSync(dir, 0o755);
writeFileSync(file, '{"old":1}'); chmodSync(file, 0o644);
save({ refresh_token: 'x' }, file);
const mode = p => statSync(p).mode & 0o777;
assert.equal(mode(file), 0o600, 'file mode');
assert.equal(mode(dir), 0o700, 'dir mode');
assert.deepEqual(JSON.parse(readFileSync(file, 'utf8')), { refresh_token: 'x' });
assert.deepEqual(readdirSync(dir), ['topbanana.json'], 'no temp left');
const f2 = join(root, 'a', 'b', 'c.json'); save({}, f2);
assert.equal(mode(f2), 0o600, 'fresh file mode'); assert.equal(mode(join(root, 'a', 'b')), 0o700, 'fresh dir mode');
console.log('ok');
