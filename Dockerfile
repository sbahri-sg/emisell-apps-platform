FROM node:22-bookworm-slim AS dependencies
WORKDIR /workspace
COPY package.json package-lock.json ./
RUN npm ci

FROM dependencies AS build
WORKDIR /workspace
COPY . .

ARG NEXT_PUBLIC_SITE_URL
ARG NEXT_PUBLIC_APP_GATEWAY_URL
ARG NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME=emisell_csrf
ARG NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME=emisell_merchant_csrf

ENV NODE_ENV=production \
    APP_GATEWAY_INTERNAL_URL=http://app-gateway:8080 \
    NEXT_PUBLIC_SITE_URL=${NEXT_PUBLIC_SITE_URL} \
    NEXT_PUBLIC_APP_GATEWAY_URL=${NEXT_PUBLIC_APP_GATEWAY_URL} \
    NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME=${NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME} \
    NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME=${NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME} \
    NEXT_PUBLIC_ENABLE_DEVELOPMENT_LOGIN=false \
    NEXT_PUBLIC_APP_GATEWAY_TOKEN= \
    NEXT_PUBLIC_ORGANIZATION_ID=

RUN test -n "$NEXT_PUBLIC_SITE_URL" \
    && test -n "$NEXT_PUBLIC_APP_GATEWAY_URL" \
    && npm run build

FROM node:22-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    PORT=3003 \
    HOST=0.0.0.0 \
    APP_GATEWAY_INTERNAL_URL=http://app-gateway:8080
COPY --from=build --chown=node:node /workspace/dist/standalone ./
# Vinext 1.0.0-beta.3 does not yet trace its React peer dependencies into the
# standalone directory. Copy the small, pinned peer runtime explicitly; all
# remaining build tooling stays behind in the build stages.
COPY --from=dependencies --chown=node:node /workspace/node_modules/react ./node_modules/react
COPY --from=dependencies --chown=node:node /workspace/node_modules/react-dom ./node_modules/react-dom
COPY --from=dependencies --chown=node:node /workspace/node_modules/react-server-dom-webpack ./node_modules/react-server-dom-webpack
COPY --from=dependencies --chown=node:node /workspace/node_modules/scheduler ./node_modules/scheduler
COPY --from=dependencies --chown=node:node /workspace/node_modules/acorn-loose ./node_modules/acorn-loose
COPY --from=dependencies --chown=node:node /workspace/node_modules/neo-async ./node_modules/neo-async
COPY --from=dependencies --chown=node:node /workspace/node_modules/webpack-sources ./node_modules/webpack-sources
USER node
EXPOSE 3003
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=5 \
  CMD ["node", "-e", "fetch('http://127.0.0.1:3003/admin/login').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
CMD ["node", "server.js"]
