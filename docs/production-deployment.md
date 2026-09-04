# Production Docker deployment

The production stack is deliberately separate from `compose.yaml`. Development keeps hot reload, source mounts, sandbox tokens, host PostgreSQL access, and raw Swagger UI. `compose.production.yaml` contains none of those development conveniences.

## What the production stack runs

- `frontend`: a Vinext standalone Node.js bundle built in multiple stages. The final image contains the runtime bundle only and runs as the unprivileged `node` user.
- `app-gateway`: static Go binaries in a distroless, non-root image.
- `migrate`: the same immutable App Gateway image with the migration binary as a one-shot entrypoint.
- `postgres`: PostgreSQL 17 with a private Docker network and persistent production volume. It has no host port.

Both application containers use a read-only filesystem, a small temporary filesystem, dropped Linux capabilities, `no-new-privileges`, and native health checks. Frontend and gateway ports bind to `127.0.0.1` by default so a TLS reverse proxy can be the only public entrypoint.

## Prepare configuration

```sh
cp deploy/production.env.example deploy/production.env
```

Edit `deploy/production.env`. Docker Compose refuses to render if any required setting is absent. The copied file is ignored by Git; do not commit it or paste its values into tickets or chat.

Generate the App Platform credential-encryption key:

```sh
openssl rand -base64 32
```

`AUTH_JWT_PUBLIC_KEY_BASE64` is the public RSA key for the issuer that signs developer/API bearer tokens. `EMISELL_BACKEND_JWT_PUBLIC_KEY_BASE64` is the public RSA key for merchant identity assertions signed by Emisell Backend. Encode a PEM public key without line wrapping:

```sh
openssl base64 -A -in public-key.pem
```

Only public verification keys belong in App Platform. Private signing keys remain in their respective issuers. In a managed deployment, inject every secret from the deployment secret manager instead of keeping a long-lived environment file.

`FRONTEND_URL` and `PUBLIC_GATEWAY_URL` must be HTTPS origins without paths. Configure the reverse proxy to forward them to the loopback frontend and gateway ports. For browser cookies, prefer same-site hostnames under the same Emisell parent domain.

## Validate, build, and start

```sh
npm run docker:prod:config
npm run docker:prod:build
npm run docker:prod:up
```

The startup order is PostgreSQL health check, one-shot migration, App Gateway readiness, then frontend readiness. Inspect or stop it with:

```sh
npm run docker:prod:logs
npm run docker:prod:down
```

`down` does not delete the database volume. Do not add `--volumes` during normal operations.

## Create the first Admin account

Core migrations include the Admin login schema. Create accounts manually after the stack is healthy; there is no public registration or seeded password:

```sh
docker compose --env-file deploy/production.env -f compose.production.yaml run --rm \
  --entrypoint /app-gateway-admin-user app-gateway create \
  --email admin@emisell.com --name "Emisell Admin" \
  --organization-id YOUR_INTERNAL_PLATFORM_ORGANIZATION_UUID
```

The CLI prompts twice for a hidden password and requires at least 15 characters in production. For automation, pipe the value from a secret manager and append `--password-stdin`; never pass a password as a command-line argument.

## Safe MVP defaults

The production Compose path permanently disables development login, sandbox browser tokens, live billing, and the incomplete Emisell Resource pilot. OIDC, managed extension connections, the billing foundation, and the API Kurir rate bridge remain explicit feature gates. Enabling a gate still requires its documented counterpart, credentials, integration test, and rollback plan; enabling OIDC also requires every `AUTH_OIDC_*` setting accepted by App Gateway.

Raw Swagger UI is intentionally omitted because it has no application login. API documentation remains available through the authenticated Admin and Developer dashboards.

Before an internet-facing launch, add the deployment platform's TLS proxy, backup/restore policy, centralized logs and metrics, secret rotation, image registry/digest pinning, and an external smoke test. Docker readiness proves the processes and database are healthy; it does not replace those operational controls.
