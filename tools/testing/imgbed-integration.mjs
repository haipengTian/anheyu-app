// Runs real ImgBed source with synthetic credentials and a disposable database.
// Never use a production endpoint or credentials with this fixture.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { cp, mkdir, mkdtemp, readFile, realpath, rm, symlink, unlink, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join, resolve, basename } from 'node:path';
import { fileURLToPath } from 'node:url';

const source = resolve(process.argv[2] || '../cloud-flare-image');
const blog = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const tempRoot = await realpath(tmpdir());
const fixture = await mkdtemp(join(tempRoot, 'codex-blog-imgbed-'));
let child;
let log = '';
const run = (exe, args, options) => new Promise((resolveRun, reject) => {
  const proc = spawn(exe, args, { windowsHide: true, ...options });
  proc.on('error', reject);
  proc.on('exit', code => code === 0 ? resolveRun() : reject(new Error(`fixture check failed: exit ${code}`)));
});
try {
  assert.equal(JSON.parse(await readFile(join(source, 'package.json'), 'utf8')).name, 'cloudflare-imgbed');
  for (const dir of ['functions', 'database', 'deploy/server']) await cp(join(source, dir), join(fixture, dir), { recursive: true });
  await cp(join(source, 'package.json'), join(fixture, 'package.json'));
  await symlink(join(source, 'node_modules'), join(fixture, 'node_modules'), process.platform === 'win32' ? 'junction' : 'dir');
  await mkdir(join(fixture, 'frontend-dist'));
  await writeFile(join(fixture, 'frontend-dist/index.html'), '<!doctype html><title>Isolated fixture</title>');
  const socket = createServer();
  await new Promise(resolveListen => socket.listen(0, '127.0.0.1', resolveListen));
  const port = socket.address().port;
  await new Promise(resolveClose => socket.close(resolveClose));
  const endpoint = `http://127.0.0.1:${port}`;
  child = spawn(process.execPath, ['--import', './deploy/server/register.mjs', 'deploy/server/index.js'], {
    cwd: fixture, windowsHide: true, env: { ...process.env, PORT: String(port), BASIC_USER: 'synthetic-admin', BASIC_PASS: 'synthetic-fixture-password', AUTH_CODE: 'synthetic-upload-password', AI_CONFIG_SECRET: 'synthetic-fixture-ai-secret' }, stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.on('data', data => { log = (log + data).slice(-8000); });
  child.stderr.on('data', data => { log = (log + data).slice(-8000); });
  const request = (path, options = {}) => fetch(endpoint + path, { ...options, signal: AbortSignal.timeout(15000) });
  let ready = false;
  for (let i=0; i<60; i++) {
    if (child.exitCode !== null) throw new Error('isolated ImgBed process exited before readiness');
    try { const r = await request('/'); await r.body?.cancel(); if (r.ok) { ready = true; break; } } catch {}
    await new Promise(resolveDelay => setTimeout(resolveDelay, 250));
  }
  assert.ok(ready, 'isolated ImgBed did not start');
  const login = await request('/api/auth/adminLogin', { method: 'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({username:'synthetic-admin', password:'synthetic-fixture-password'}) });
  assert.equal(login.status,200,'fixture login');
  const cookie = login.headers.get('set-cookie')?.split(';')[0];
  assert.ok(cookie); await login.body?.cancel();
  const tokenResponse = await request('/api/manage/apiTokens', { method:'POST', headers:{'Content-Type':'application/json', Cookie:cookie}, body:JSON.stringify({name:'blog-fixture', permissions:['upload','delete','list'],owner:'synthetic-test'}) });
  assert.equal(tokenResponse.status,200,'fixture token creation');
  const {token} = await tokenResponse.json(); assert.ok(token);
  await run('go', ['test','./internal/infra/storage','-run','^TestImgBedLocalServerIntegration$','-count=1','-v'], {cwd:blog, env:{...process.env,IMGBED_INTEGRATION_URL:endpoint,IMGBED_INTEGRATION_TOKEN:token},stdio:'inherit'});
  console.log('PASS: isolated real ImgBed API integration; production data untouched.');
} catch (error) {
  // Fixture output may contain synthetic auth material; retain it locally only.
  const diagnostics = join(blog, '.imgbed-fixture-diagnostics.log');
  await writeFile(diagnostics, log, {mode:0o600});
  console.error(`${error.message}; fixture diagnostics saved locally.`);
  process.exitCode = 1;
} finally {
  if (child && child.exitCode === null) {
    await new Promise(resolveExit => { child.once('exit',resolveExit); child.kill(); });
  }
  await unlink(join(fixture, 'node_modules')).catch(()=>{});
  const resolved = await realpath(fixture);
  assert.equal(dirname(resolved), tempRoot);
  assert.ok(basename(resolved).startsWith('codex-blog-imgbed-'));
  await rm(resolved, {recursive:true,force:true});
}
