import { mkdir, open, readFile, lstat, rename, unlink } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { randomUUID } from 'node:crypto';

export function origin(value) {
  const u = new URL(value);
  if (u.username || u.password || u.search || u.hash || u.pathname !== '/' ||
      !(u.protocol === 'https:' || (u.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(u.hostname)))) {
    throw new Error('URL harus origin HTTPS, atau HTTP localhost untuk development (tanpa path/credential).');
  }
  return u.origin;
}

// CLI sessions are deliberately outside the project and never included in npm packs.
export class SessionStore {
  constructor(directory = join(homedir(), '.emisell-cli')) { this.directory = directory; }
  async prepare() {
    await mkdir(this.directory, { recursive: true, mode: 0o700 });
    const stat = await lstat(this.directory);
    if (!stat.isDirectory() || stat.isSymbolicLink() || (process.platform !== 'win32' && (stat.mode & 0o077))) {
      throw new Error('Direktori sesi harus privat (0700), bukan symlink.');
    }
  }
  async load() {
    await this.prepare();
    let handle;
    try {
      handle = await open(join(this.directory, 'session.json'), constants.O_RDONLY | constants.O_NOFOLLOW);
      const stat = await handle.stat();
      if (!stat.isFile() || (process.platform !== 'win32' && (stat.mode & 0o077))) throw new Error('File sesi harus privat (0600).');
      const session = JSON.parse(await handle.readFile('utf8'));
      if (origin(session.origin) !== session.origin || !/^emisell_portal_session=[A-Za-z0-9_-]+$/.test(session.cookie) || !(session.expiresAt > Date.now())) {
        throw new Error('Sesi tidak valid atau kedaluwarsa. Jalankan login kembali.');
      }
      return session;
    } catch (error) {
      if (error.code === 'ENOENT') throw new Error('Belum login. Jalankan emisell login --url URL --email EMAIL.');
      throw error;
    } finally { await handle?.close(); }
  }
  async save(session) {
    await this.prepare();
    const file = join(this.directory, `session-${randomUUID()}.tmp`);
    const handle = await open(file, 'wx', 0o600);
    try { await handle.writeFile(JSON.stringify(session)); }
    finally { await handle.close(); }
    try { await rename(file, join(this.directory, 'session.json')); }
    finally { await unlink(file).catch(e => { if (e.code !== 'ENOENT') throw e; }); }
  }
  async clear() {
    await this.prepare();
    await unlink(join(this.directory, 'session.json')).catch(e => { if (e.code !== 'ENOENT') throw e; });
  }
}

export class Client {
  constructor(base, cookie = '', fetcher = fetch) {
    this.base = origin(base); this.cookie = cookie; this.fetcher = fetcher;
  }
  async request(path, { method = 'GET', body, key } = {}) {
    if (!/^\/api\/v1\/(developer|portal)\/[A-Za-z0-9_/?=&.-]+$/.test(path) || path.includes('..')) throw new Error('Path CLI tidak valid.');
    const headers = { Accept: 'application/json', Origin: this.base };
    if (this.cookie) headers.Cookie = this.cookie;
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (key) headers['Idempotency-Key'] = key;
    let response;
    try {
      response = await this.fetcher(this.base + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), redirect: 'error', signal: AbortSignal.timeout(15000) });
    } catch {
      throw new Error('Koneksi gagal/timeout/redirect. Untuk mutasi, periksa status sebelum mencoba ulang dengan request-key yang sama.');
    }
    // Do not print remote error bodies: proxies may include credentials or HTML.
    if (!response.ok) {
      await response.body?.cancel();
      const hints = { 401: 'Sesi habis; login kembali.', 403: 'Akses ditolak; periksa akun developer dan URL dashboard.', 409: 'Konflik revisi/request-key; baca ulang data sebelum mengirim.', 429: 'Terlalu banyak permintaan; coba lagi nanti.' };
      throw new Error(`HTTP ${response.status}. ${hints[response.status] || 'Permintaan ditolak server; periksa dokumen dan izin.'}`);
    }
    if (!response.headers.get('content-type')?.includes('application/json')) {
      await response.body?.cancel(); throw new Error('Respons bukan JSON. Gunakan URL dashboard Apps Platform.');
    }
    const chunks = []; let size = 0;
    for await (const chunk of response.body) {
      size += chunk.length;
      if (size > 4 * 1024 * 1024) throw new Error('Respons terlalu besar.');
      chunks.push(chunk);
    }
    let data;
    try { data = JSON.parse(Buffer.concat(chunks).toString('utf8')); }
    catch { throw new Error('Respons JSON tidak valid.'); }
    return { data, headers: response.headers };
  }
  async login(email, password) {
    const { data, headers } = await this.request('/api/v1/portal/login', { method: 'POST', body: { email, password } });
    const raw = headers.getSetCookie().find(c => c.startsWith('emisell_portal_session='));
    const cookie = raw?.split(';')[0];
    if (!cookie || !/^emisell_portal_session=[A-Za-z0-9_-]+$/.test(cookie)) throw new Error('Server tidak memberikan sesi yang valid.');
    this.cookie = cookie;
    if (data.user?.surface !== 'developer') {
      this.cookie = '';
      throw new Error('Gunakan akun developer, bukan akun admin. Sesi tidak disimpan.');
    }
    const age = Number(raw.match(/Max-Age=(\d+)/i)?.[1]);
    if (!age) throw new Error('Sesi tidak memiliki masa berlaku.');
    return { origin: this.base, cookie, expiresAt: Date.now() + Math.min(age, 28800) * 1000 };
  }
}

export async function documentFile(file) {
  const raw = await readFile(file, 'utf8');
  if (Buffer.byteLength(raw) > 1024 * 1024) throw new Error('Dokumen terlalu besar.');
  const document = JSON.parse(raw);
  const allowed = ['name', 'summary', 'description', 'version', 'capability', 'scopes', 'endpoint', 'accessScopes', 'webhooks'];
  if (!document || Array.isArray(document) || typeof document !== 'object' || Object.keys(document).some(k => !allowed.includes(k))) throw new Error('File harus AppDocument, tanpa ID, credential, atau revision.');
  return document;
}
