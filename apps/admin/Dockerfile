# syntax=docker/dockerfile:1.7

FROM node:24-alpine  AS deps

WORKDIR /app/apps/admin

COPY apps/admin/package.json apps/admin/package-lock.json ./
RUN npm ci

FROM node:24-alpine  AS build

WORKDIR /app/apps/admin

ARG VITE_API_PREFIX=/api
ENV VITE_API_PREFIX=${VITE_API_PREFIX}

COPY --from=deps /app/apps/admin/node_modules ./node_modules
COPY apps/admin ./
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine AS runtime

COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/apps/admin/dist /usr/share/nginx/html

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/healthz || exit 1
