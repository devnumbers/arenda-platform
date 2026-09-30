# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS deps

WORKDIR /app/apps/landing

COPY apps/landing/package.json apps/landing/package-lock.json apps/landing/.npmrc ./
RUN npm ci

FROM node:24-alpine AS build

WORKDIR /app/apps/landing

ENV NEXT_TELEMETRY_DISABLED=1
# Ассеты лендинга в образе публикуются под /landing/_next/*: кабинет тоже
# Next.js и владеет общим /_next/* (контракт Caddy, карта #649, ADR 0063);
# префикс снимает Caddy (deploy/caddy/rentlee.caddy). В dev-прогонах
# (make landing-dev) переменной нет — пути остаются дефолтными /_next/*.
ENV LANDING_ASSET_PREFIX=/landing

COPY --from=deps /app/apps/landing/node_modules ./node_modules
COPY apps/landing ./
RUN npm run build

FROM node:24-alpine AS runtime

WORKDIR /app

ENV HOSTNAME=0.0.0.0
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production
ENV PORT=8080

RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx \
    && addgroup -S nextjs \
    && adduser -S nextjs -G nextjs

COPY --from=build --chown=nextjs:nextjs /app/apps/landing/public ./public
COPY --from=build --chown=nextjs:nextjs /app/apps/landing/.next/standalone ./
COPY --from=build --chown=nextjs:nextjs /app/apps/landing/.next/static ./.next/static

USER nextjs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/healthz || exit 1

CMD ["node", "server.js"]
