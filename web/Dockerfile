# Build context is the monorepo root (not web/) — see docker-compose.yml
# — so the next-intl config (web/i18n/request.ts) can import the shared
# locales/ translation source one level above web/ at build time. See
# backend/Dockerfile for the same reasoning on the other image.
FROM node:22-alpine AS base
RUN corepack enable

FROM base AS deps
# Deliberately /build/web, not /app: next.config.ts's outputFileTracingRoot
# points one level above this directory so the standalone trace can reach
# locales/, and Next's standalone output mirrors that relative path — it
# nests this app's own files under .next/standalone/<this dir's name>/.
# Naming the directory "web" here makes that nested path predictable
# (.next/standalone/web/) and matches what a local (non-Docker) build
# produces, since the real web/ directory is also one level under the
# repo root there.
WORKDIR /build/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS build
WORKDIR /build/web
COPY --from=deps /build/web/node_modules ./node_modules
COPY web/ .
# /build/locales, not /locales: web/i18n/request.ts imports
# "../../locales/{locale}.json", two levels up from web/i18n/ — matching
# that means locales/ must sit next to web/ under /build here, the same
# shape as the real repo root next to the real web/ directory.
COPY locales/ /build/locales/
RUN pnpm build

FROM base AS run
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001

COPY --from=build /build/web/public ./public
COPY --from=build --chown=nextjs:nodejs /build/web/.next/standalone/web ./
COPY --from=build --chown=nextjs:nodejs /build/web/.next/static ./.next/static
COPY --from=build /build/locales /locales

USER nextjs
EXPOSE 3000
ENV PORT=3000 HOSTNAME=0.0.0.0
CMD ["node", "server.js"]
