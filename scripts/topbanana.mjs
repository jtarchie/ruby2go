#!/usr/bin/env node
// Deploys a dir of text files to a Top Banana site over MCP, its only upload API; the first run signs in through the browser (OAuth PKCE), then reuses the refresh token in ~/.config/rb2go/topbanana.json.
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import { chmodSync, mkdirSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFile } from 'node:child_process';

const BASE = process.env.TOPBANANA_URL || 'https://apps.topbanana.dev';
const REDIRECT = 'http://127.0.0.1:8976/callback'; // fixed: a registered client allows only the redirect it registered with
const CONFIG = join(homedir(), '.config', 'rb2go', 'topbanana.json');

const b64url = buf => buf.toString('base64url');
const load = () => { try { return JSON.parse(readFileSync(CONFIG, 'utf8')); } catch { return {}; } };
// The config holds a refresh token. `mode` only applies when a file/dir is created, so chmod explicitly (a pre-existing one may be looser), and rename a 0600 temp file into place so the token is never half-written or under the old mode.
export function save(cfg, file = CONFIG) {
  const dir = dirname(file);
  mkdirSync(dir, { recursive: true, mode: 0o700 });
  chmodSync(dir, 0o700);
  const tmp = `${file}.${process.pid}.tmp`;
  writeFileSync(tmp, JSON.stringify(cfg, null, 2), { mode: 0o600 });
  chmodSync(tmp, 0o600);
  renameSync(tmp, file);
}
// Refresh tokens rotate, but a server that does not rotate omits the field: keep the stored one rather than writing undefined.
export const nextRefreshToken = (cfg, tok) => tok.refresh_token ?? cfg.refresh_token;
// The platform's own files, which a deploy never deletes.
const platform = path => path.startsWith('.') || path === 'app.css' || path.startsWith('functions/');
// A deploy is the whole site, so it must at least be a site: an empty or index-less set would otherwise delete everything. Returns the remote files to delete; throws before any write or delete.
export function planDeploy(names, remoteFiles) {
  if (!names.length) throw new Error('refusing to deploy: no files to upload (the deploy would delete the whole site)');
  if (!names.includes('index.html')) throw new Error('refusing to deploy: no index.html in the deploy set');
  return remoteFiles.filter(path => !names.includes(path) && !platform(path));
}
// The files of the deploy dir, sorted; fails clearly (and before any login or network) when it is missing, empty or has no index.html.
export function deployNames(dir) {
  let ents;
  try { ents = readdirSync(dir, { withFileTypes: true }); } catch (e) { throw new Error(`refusing to deploy: cannot read ${dir}: ${e.code ?? e.message}`); }
  const names = ents.filter(e => e.isFile()).map(e => e.name).sort();
  planDeploy(names, []);
  return names;
}
const LOGIN_TIMEOUT_MS = 5 * 60 * 1000;

async function form(path, fields) {
  const r = await fetch(BASE + path, { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: new URLSearchParams(fields) });
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(`${path}: ${r.status} ${JSON.stringify(body)}`);
  return body;
}

async function login(cfg) {
  if (!cfg.client_id) {
    const r = await fetch(BASE + '/oauth/register', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ redirect_uris: [REDIRECT], client_name: 'rb2go deploy' }) });
    if (!r.ok) throw new Error(`register: ${r.status} ${await r.text()}`);
    cfg.client_id = (await r.json()).client_id;
    save(cfg);
  }
  const verifier = b64url(randomBytes(32)), state = b64url(randomBytes(16));
  const challenge = b64url(createHash('sha256').update(verifier).digest());
  const url = `${BASE}/oauth/authorize?` + new URLSearchParams({ response_type: 'code', client_id: cfg.client_id, redirect_uri: REDIRECT, code_challenge: challenge, code_challenge_method: 'S256', scope: 'mcp', state });
  const code = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => { srv.close(); reject(new Error(`authorize: no sign-in within ${LOGIN_TIMEOUT_MS / 60000} minutes`)); }, LOGIN_TIMEOUT_MS);
    const srv = createServer((req, res) => {
      const q = new URL(req.url, REDIRECT).searchParams;
      if (!req.url.startsWith('/callback')) { res.writeHead(404).end(); return; }
      // a stray local request with the wrong state must not abort the login: refuse it and keep waiting
      if (q.get('state') !== state) { res.writeHead(400).end('Bad state.'); return; }
      res.end(q.get('code') ? 'Signed in; you can close this tab.' : 'Sign-in failed.');
      clearTimeout(timer);
      srv.close();
      if (!q.get('code')) reject(new Error(`authorize: ${q.get('error') ?? 'no code'}`)); else resolve(q.get('code'));
    }).listen(8976, '127.0.0.1');
    console.error(`Opening ${url}`);
    execFile(process.platform === 'darwin' ? 'open' : 'xdg-open', [url], () => {});
  });
  return form('/oauth/token', { grant_type: 'authorization_code', code, client_id: cfg.client_id, redirect_uri: REDIRECT, code_verifier: verifier });
}

async function token() {
  const cfg = load();
  let tok = null;
  if (cfg.client_id && cfg.refresh_token) {
    tok = await form('/oauth/token', { grant_type: 'refresh_token', refresh_token: cfg.refresh_token, client_id: cfg.client_id }).catch(() => null);
  }
  tok ??= await login(cfg);
  save({ ...cfg, refresh_token: nextRefreshToken(cfg, tok) });
  return tok.access_token;
}

let id = 0;
async function call(tok, name, args) {
  const r = await fetch(BASE + '/mcp', {
    method: 'POST',
    headers: { Authorization: `Bearer ${tok}`, 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' },
    body: JSON.stringify({ jsonrpc: '2.0', id: ++id, method: 'tools/call', params: { name, arguments: args } }),
  });
  const text = await r.text();
  if (!r.ok) throw new Error(`${name}: ${r.status} ${text}`);
  const json = text.trimStart().startsWith('{') ? text : text.split('\n').filter(l => l.startsWith('data:')).map(l => l.slice(5)).join(''); // plain JSON or one SSE event
  const msg = JSON.parse(json);
  if (msg.error) throw new Error(`${name}: ${JSON.stringify(msg.error)}`);
  const out = msg.result.content?.map(c => c.text).join('\n') ?? '';
  if (msg.result.isError) throw new Error(`${name}: ${out}`);
  try { return JSON.parse(out); } catch { return out; }
}

async function deploy(slug, dir) {
  const names = deployNames(dir); // before sign-in: a bad dir must not reach the network
  const tok = await token();
  const { sites } = await call(tok, 'list_sites', {});
  if (!sites.some(s => s.slug === slug)) {
    const made = await call(tok, 'create_site', { slug, title: 'rb2go playground', description: 'Write typed Ruby and see the Go that rb2go compiles it to, in your browser.' });
    console.log(`created ${made.url ?? slug}`);
  }
  for (const name of names) {
    const content = readFileSync(join(dir, name), 'utf8');
    const res = await call(tok, 'write_file', { slug, path: name, content, expect_sha256: createHash('sha256').update(content).digest('hex') });
    console.log(`wrote ${name} (${res.bytes ?? content.length} bytes)`);
  }
  // a deploy is the whole site: drop what an earlier one left, except the platform's own files
  for (const path of planDeploy(names, (await call(tok, 'list_files', { slug })).files)) {
    await call(tok, 'delete_file', { slug, path });
    console.log(`deleted ${path}`);
  }
  console.log('lint:', JSON.stringify(await call(tok, 'lint_site', { slug }), null, 1));
  const site = (await call(tok, 'list_sites', {})).sites.find(s => s.slug === slug);
  console.log(`live: ${site?.url}`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) { // not when imported by the check
  const [cmd, slug, dir] = process.argv.slice(2);
  if (cmd !== 'deploy' || !slug || !dir) {
    console.error('usage: node scripts/topbanana.mjs deploy <slug> <dir>');
    process.exit(2);
  }
  deploy(slug, dir).catch(e => { console.error(e.message); process.exit(1); });
}
