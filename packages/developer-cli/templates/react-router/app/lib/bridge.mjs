// Universal browser protocol. No cookie, persistent storage or provider secrets.
export const protocol = 'emisell.embedded/v1';

function exactOrigin(value) {
  const url = new URL(value);
  if (url.origin !== value || !['https:', 'http:'].includes(url.protocol)) throw Error('Invalid origin');
  if (url.protocol === 'http:' && !['127.0.0.1', 'localhost'].includes(url.hostname)) throw Error('HTTPS required');
  return value;
}

// Parent shell: getSession calls its authenticated, CSRF-protected Core BFF.
// frame must remain the same dedicated app frame for this controller lifetime.
export function connectFrame({host = window, frame, appOrigin, parentOrigin, launchURL, getSession}) {
  exactOrigin(appOrigin); exactOrigin(parentOrigin);
  if (host.location.origin !== parentOrigin || new URL(launchURL).origin !== appOrigin || appOrigin === parentOrigin) throw Error('Origin mismatch');
  let disposed = false, busy = false, generation = 0;
  const seen = new Set();
  const onLoad = () => { generation++; seen.clear(); };
  frame.addEventListener('load', onLoad);
  const receive = async (event) => {
    const data = event.data;
    if (disposed || busy || event.source !== frame.contentWindow || event.origin !== appOrigin ||
        data?.protocol !== protocol || data.type !== 'identity.request' || typeof data.nonce !== 'string' ||
        !/^[a-zA-Z0-9_-]{24,64}$/.test(data.nonce) || seen.has(data.nonce) || seen.size >= 120) return;
    seen.add(data.nonce); busy = true;
    const current = generation;
    const target = frame.contentWindow;
    try {
      const session = await getSession();
      if ((session.launch.mode && session.launch.mode !== 'embedded') || session.launch.url !== launchURL || session.launch.parentOrigin !== parentOrigin ||
          typeof session.identityToken !== 'string' || session.identityToken.length > 4096 || session.expiresIn !== 60) throw Error('Invalid session');
      if (!disposed && current === generation && target === frame.contentWindow) target.postMessage({protocol, type:'identity.response', nonce:data.nonce, token:session.identityToken, expiresIn:60}, appOrigin);
    } catch {
      if (!disposed && current === generation && target === frame.contentWindow) target.postMessage({protocol, type:'identity.error', nonce:data.nonce}, appOrigin);
    } finally { busy = false; }
  };
  host.addEventListener('message', receive);
  return () => { disposed = true; generation++; host.removeEventListener('message', receive); frame.removeEventListener('load', onLoad); seen.clear(); };
}

// App frame: request a fresh identity token for backend authentication.
export async function getIdentity({host = window, parentOrigin, timeoutMs = 5000}) {
  exactOrigin(parentOrigin);
  if (host.parent === host || !Number.isFinite(timeoutMs) || timeoutMs < 1 || timeoutMs > 10000) return Promise.reject(Error('Embedded parent required'));
  if (host.document && host.document.readyState !== 'complete') {
    await new Promise(resolve => host.addEventListener('load', resolve, {once:true}));
  }
  const bytes = new Uint8Array(24); host.crypto.getRandomValues(bytes);
  const nonce = Array.from(bytes, b => b.toString(16).padStart(2,'0')).join('');
  return new Promise((resolve, reject) => {
    const finish = (error, value) => { host.clearTimeout(timer); host.removeEventListener('message', receive); error ? reject(error) : resolve(value); };
    const receive = event => {
      const d = event.data;
      if (event.source !== host.parent || event.origin !== parentOrigin || d?.protocol !== protocol || d.nonce !== nonce) return;
      if (d.type === 'identity.error') finish(Error('Embedded access unavailable'));
      else if (d.type === 'identity.response' && typeof d.token === 'string' && d.token.length <= 4096 && d.expiresIn === 60) finish(null, {token:d.token, expiresIn:d.expiresIn});
    };
    const timer = host.setTimeout(() => finish(Error('Embedded request timed out')), timeoutMs);
    host.addEventListener('message', receive);
    host.parent.postMessage({protocol, type:'identity.request', nonce}, parentOrigin);
  });
}

// Minimal embedded shell for an already authenticated parent surface.
// getSession must call the real same-origin BFF, never a browser API key.
export async function mountEmbeddedApp({container, getSession, host = window}) {
  const session = await getSession();
  const launch = session.launch;
  if (launch.mode && launch.mode !== 'embedded') throw Error('Embedded launch required');
  const url = new URL(launch.url);
  exactOrigin(url.origin);
  if (launch.parentOrigin !== host.location.origin || url.origin === host.location.origin || url.username || url.password || url.search || url.hash) throw Error('Invalid launch binding');
  const frame = host.document.createElement('iframe');
  frame.title = 'Embedded application';
  frame.setAttribute('sandbox', 'allow-scripts allow-forms allow-same-origin');
  frame.referrerPolicy = 'no-referrer';
  frame.style.width = '100%'; frame.style.minHeight = '640px'; frame.style.border = '0';
  const disconnect = connectFrame({host,frame,appOrigin:url.origin,parentOrigin:launch.parentOrigin,launchURL:launch.url,getSession});
  frame.src = launch.url;
  container.appendChild(frame);
  return () => { disconnect(); frame.remove(); };
}
