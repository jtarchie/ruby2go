// Run: node scripts/topbanana.check.mjs  (offline: config modes save() leaves behind, and the pure deploy decisions that guard against data loss)
import nodeAssert from 'node:assert/strict';
let checks = 0; // a check run that asserted nothing must fail, not pass
const assert = new Proxy(nodeAssert, { get: (t, k) => typeof t[k] === 'function' ? (...a) => { checks++; return t[k](...a); } : t[k], apply: (t, _, a) => { checks++; return t(...a); } });
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const { save, planDeploy, deployNames, nextRefreshToken } = await import('./topbanana.mjs');
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

// deploy guard: never plan a delete from an empty or index-less set
assert.throws(() => planDeploy([], ['index.html', 'old.html']), /no files/, 'empty set aborts');
assert.throws(() => planDeploy(['a.html'], ['index.html']), /no index\.html/, 'set without index.html aborts');
assert.deepEqual(
  planDeploy(['index.html', 'new.js'], ['index.html', 'stale.html', 'new.js', 'app.css', '.well-known/x', 'functions/f.js', 'sub/stale.css']),
  ['stale.html', 'sub/stale.css'], 'only stale non-platform files are deleted');
assert.deepEqual(planDeploy(['index.html'], []), [], 'nothing remote, nothing to delete');
// deploy dir: missing, empty, index-less all abort before any network
assert.throws(() => deployNames(join(root, 'nope')), /cannot read/, 'missing dir aborts');
const empty = join(root, 'empty'); mkdirSync(empty);
assert.throws(() => deployNames(empty), /no files/, 'empty dir aborts');
mkdirSync(join(empty, 'subdir')); // a subdirectory is not a file to upload
assert.throws(() => deployNames(empty), /no files/, 'dir with only a subdir aborts');
writeFileSync(join(empty, 'x.html'), 'x');
assert.throws(() => deployNames(empty), /no index\.html/, 'dir without index.html aborts');
writeFileSync(join(empty, 'index.html'), 'x');
assert.deepEqual(deployNames(empty), ['index.html', 'x.html'], 'good dir lists its files, sorted');
// refresh token: keep the stored one when the server does not rotate
assert.equal(nextRefreshToken({ refresh_token: 'old' }, { access_token: 'a' }), 'old', 'kept when omitted');
assert.equal(nextRefreshToken({ refresh_token: 'old' }, { refresh_token: 'new' }), 'new', 'rotated token wins');
assert.equal(nextRefreshToken({}, { refresh_token: 'new' }), 'new', 'first login');
if (!checks) { console.error('no assertions ran'); process.exit(1); }
console.log(`ok (${checks} assertions)`);
