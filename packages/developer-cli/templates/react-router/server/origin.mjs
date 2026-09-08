export function origin(value) {
  const url = new URL(value);
  if (url.username || url.password || url.search || url.hash || url.pathname !== '/' ||
      !(url.protocol === 'https:' || (url.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)))) {
    throw Error('URL harus origin HTTPS, atau HTTP localhost untuk development (tanpa path/credential).');
  }
  return url.origin;
}
