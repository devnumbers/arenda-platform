# syntax=docker/dockerfile:1.7

FROM node:24-alpine  AS deps

WORKDIR /app/apps/landing

COPY apps/landing/package.json apps/landing/package-lock.json ./
RUN npm ci

FROM node:24-alpine  AS build

WORKDIR /app/apps/landing

COPY --from=deps /app/apps/landing/node_modules ./node_modules
COPY apps/landing ./
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.31.3-alpine@sha256:59ccf0943b0b8e8d9e6ea9039a39555730f544701a655c596f7df7d096c593f5 AS runtime

USER root

RUN apk update \
    && apk upgrade --no-cache \
    && rm -rf /var/cache/apk/*

USER nginx

COPY apps/landing/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/apps/landing/dist /usr/share/nginx/html

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/healthz || exit 1
