import { spawn } from 'node:child_process';
import { setTimeout as sleep } from 'node:timers/promises';
import { origin } from './client.mjs';

export async function openBrowser(url) {
  const command = process.platform === 'darwin' ? 'open' : process.platform === 'win32' ? 'rundll32' : 'xdg-open';
  const args = process.platform === 'win32' ? ['url.dll,FileProtocolHandler', url] : [url];
  return new Promise(resolve => {
    const child = spawn(command, args, { stdio: 'ignore', shell: false });
    child.once('error', () => resolve(false)); child.once('exit', code => resolve(code === 0));
  });
}

export async function browserLogin(client, { output, open = openBrowser, wait = sleep, noOpen = false } = {}) {
  const { data } = await client.request('/api/v1/developer-login/cli/start', { method: 'POST', body: {} });
  const valid = value => /^[A-Z2-7]{52}$/.test(value || '');
  let authorize;
  try {
    authorize = new URL(data.authorizeUrl);
    origin(authorize.origin);
    if (authorize.username || authorize.password || authorize.hash || authorize.pathname !== '/auth/developer' ||
        authorize.searchParams.get('request') !== data.request || [...authorize.searchParams].length !== 1 ||
        !valid(data.request) || !valid(data.verifier) || data.expiresIn !== 300 || data.interval !== 3) throw Error();
  } catch { throw Error('Respons login browser tidak valid.'); }
  output(`Login dengan akun merchant Emisell, lalu konfirmasi login CLI:\n${authorize.href}`);
  if (!noOpen) await open(authorize.href);
  const end = Date.now() + 300000;
  for (let attempt = 0; attempt < 100 && Date.now() < end; attempt++) {
    await wait(3000);
    const result = await client.request('/api/v1/developer-login/cli/poll', { method: 'POST', body: { request: data.request, verifier: data.verifier } });
    if (result.data.status === 'pending') continue;
    if (result.data.status !== 'authorized') throw Error('Respons login browser tidak valid.');
    const session = client.sessionFromHeaders(result.headers);
    const identity = await client.request('/api/v1/developer/session');
    if (identity.data.user?.surface !== 'developer' || identity.data.user?.role !== 'developer') throw Error('Sesi bukan akun developer merchant.');
    return { ...session, accountId: identity.data.user.id };
  }
  throw Error('Login kedaluwarsa atau belum dikonfirmasi. Jalankan emisell auth login kembali.');
}
