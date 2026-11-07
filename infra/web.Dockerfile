# syntax=docker/dockerfile:1
#
# Next.js web app (apps/web) — standalone output.
# Build context: apps/web   (compose: context: apps/web, dockerfile: ../../infra/web.Dockerfile)
#
# Runtime environment (all read at request time, not baked into the image):
#   API_INTERNAL_URL           Go API base URL as seen from this container (default http://localhost:8080)
#   NEXT_PUBLIC_SITE_URL       public origin for canonical URLs / sitemap (default http://localhost:3000)
#   NEXT_PUBLIC_ALLOW_INDEXING "true" only in production; anything else => robots disallow + noindex
#   NEXT_PUBLIC_PRIVACY_EMAIL  privacy contact shown on legal pages
#
# The browser talks to same-origin /api/* and /healthz; those are forwarded by a
# runtime route handler (src/app/api/[...path]/route.ts), so API_INTERNAL_URL can
# change without rebuilding.

ARG NODE_IMAGE=node:24-alpine

FROM ${NODE_IMAGE} AS deps
WORKDIR /app
RUN apk add --no-cache libc6-compat
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

FROM ${NODE_IMAGE} AS builder
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# The build never contacts the API: every data-backed page renders on demand.
RUN npm run build

FROM ${NODE_IMAGE} AS runner
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    API_INTERNAL_URL=http://localhost:8080 \
    NEXT_PUBLIC_SITE_URL=http://localhost:3000 \
    NEXT_PUBLIC_ALLOW_INDEXING=false

# The official node image ships an unprivileged "node" user (uid 1000).
COPY --from=builder --chown=node:node /app/public ./public
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static

USER node
EXPOSE 3000

# Liveness of the web process itself (does not depend on the API).
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3000/robots.txt >/dev/null || exit 1

CMD ["node", "server.js"]
