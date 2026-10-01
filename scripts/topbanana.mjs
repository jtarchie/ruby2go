#!/usr/bin/env node
// Deploys a dir of text files to a Top Banana site over MCP, its only upload API; the first run signs in through the browser (OAuth PKCE), then reuses the refresh token in ~/.config/rb2go/topbanana.json.
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { execFile } from 'node:child_process';

const BASE = process.env.TOPBANANA_URL || 'https://apps.topbanana.dev';
const REDIRECT = 'http://127.0.0.1:8976/callback'; // fixed: a registered client allows only the redirect it registered with
const CONFIG = join(homedir(), '.config', 'rb2go', 'topbanana.json');

const b64url = buf => buf.toString('base64url');
const load = () => { try { return JSON.parse(readFileSync(CONFIG, 'utf8')); } catch { return {}; } };
const save = cfg => { mkdirSync(join(homedir(), '.config', 'rb2go'), { recursive: true }); writeFileSync(CONFIG, JSON.stringify(cfg, null, 2), { mode: 0o600 }); };

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
    const srv = createServer((req, res) => {
      const q = new URL(req.url, REDIRECT).searchParams;
      if (!req.url.startsWith('/callback')) { res.writeHead(404).end(); return; }
      res.end(q.get('state') === state && q.get('code') ? 'Signed in; you can close this tab.' : 'Sign-in failed.');
      srv.close();
      if (q.get('state') !== state || !q.get('code')) reject(new Error('authorize: bad state or no code')); else resolve(q.get('code'));
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
  save({ ...cfg, refresh_token: tok.refresh_token }); // refresh tokens rotate: the old one is spent
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
  const tok = await token();
  const { sites } = await call(tok, 'list_sites', {});
  if (!sites.some(s => s.slug === slug)) {
    const made = await call(tok, 'create_site', { slug, title: 'rb2go playground', description: 'Write typed Ruby and see the Go that rb2go compiles it to, in your browser.' });
    console.log(`created ${made.url ?? slug}`);
  }
  const names = readdirSync(dir).sort();
  for (const name of names) {
    const content = readFileSync(join(dir, name), 'utf8');
    const res = await call(tok, 'write_file', { slug, path: name, content, expect_sha256: createHash('sha256').update(content).digest('hex') });
    console.log(`wrote ${name} (${res.bytes ?? content.length} bytes)`);
  }
  // a deploy is the whole site: drop what an earlier one left, except the platform's own files
  for (const path of (await call(tok, 'list_files', { slug })).files) {
    if (names.includes(path) || path.startsWith('.') || path === 'app.css' || path.startsWith('functions/')) continue;
    await call(tok, 'delete_file', { slug, path });
    console.log(`deleted ${path}`);
  }
  console.log('lint:', JSON.stringify(await call(tok, 'lint_site', { slug }), null, 1));
  const site = (await call(tok, 'list_sites', {})).sites.find(s => s.slug === slug);
  console.log(`live: ${site?.url}`);
}

const [cmd, slug, dir] = process.argv.slice(2);
if (cmd !== 'deploy' || !slug || !dir) {
  console.error('usage: node scripts/topbanana.mjs deploy <slug> <dir>');
  process.exit(2);
}
deploy(slug, dir).catch(e => { console.error(e.message); process.exit(1); });
