# syntax=docker/dockerfile:1
# Refresh the Node base regularly, and rebuild/re-test before releasing an image.
FROM node:22-bookworm-slim@sha256:83f487e0a63425e5b4d146fb5e5be574bcbe1b7b843d3ebafdd95eaf7767a7e5 AS base
WORKDIR /app

FROM base AS dependencies
COPY package.json package-lock.json ./
RUN npm ci --no-fund

FROM dependencies AS build
COPY . .
RUN npm run typecheck && npm run build

FROM base AS production-dependencies
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-fund

FROM base AS runtime
ENV NODE_ENV=production HOST=0.0.0.0 PORT=3000 EMISELL_BACKEND_MODE=disabled
COPY --from=production-dependencies --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/build ./build
COPY --from=build --chown=node:node /app/server ./server
COPY --from=build --chown=node:node /app/package.json /app/emisell.app.json ./
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD ["node", "server/healthcheck.mjs"]
CMD ["node", "server/standalone.mjs", "--built"]
