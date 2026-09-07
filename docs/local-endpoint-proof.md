# Local HTTPS endpoint proof

For local development only, the server optionally reads the operator-owned
`.local/endpoint-proof.json` (directory mode 0700, file mode 0600):

```json
{
  "environment": "development",
  "origin": "https://app.emisell.test",
  "certificatePem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
}
```

Use the public leaf certificate, never its private key. The certificate must
cover the configured hostname and be unexpired. Exactly one `.test` origin is
accepted, without path or explicit port. Requests dial only 127.0.0.1:443 and
verify the TLS chain, hostname, expiry and exact certificate fingerprint.
DNS, proxy environment variables and redirects cannot change the destination.
The existing bounded challenge path, JSON validation and timeouts still apply.

Without this file, public-only endpoint verification is unchanged. With the
file, other domains are rejected; invalid configuration stops startup.
Either NODE_ENV=production or EMISELL_ENV=production prohibits this option.
Never copy `.local` configuration into deployment images.

This only enables endpoint ownership proof. It does not approve a UI release,
create a seller installation, grant scopes, or bypass app review. The local
app still needs to serve its exact proof challenge and complete the normal flow.
After certificate rotation, update this file and restart the local server.
