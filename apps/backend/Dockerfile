# syntax=docker/dockerfile:1.7

FROM golang:1.26.6-bookworm  AS build

WORKDIR /src/apps/backend

COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/arenda-api ./cmd/api

# The offline GeoIP city database (DB-IP City Lite, CC BY 4.0) is baked into
# the image at build time and refreshed by the monthly image rebuild
# (ticket #728). CI passes DBIP_MONTH=$(date +%Y-%m); a local build falls
# back to the current month, then to the previous one (the file for the
# running month may not be published yet in its first days).
ARG DBIP_MONTH=""
# DB-IP отдаёт lite-файлы только в gzip (с 09.2026 голые .mmdb-URL отдают 404),
# поэтому качаем .mmdb.gz и распаковываем; gunzip есть в bookworm-образе.
RUN set -eux; \
    month="${DBIP_MONTH:-$(date +%Y-%m)}"; \
    prev="$(date -d "$month-01 -1 month" +%Y-%m)"; \
    mkdir -p /out/geoip; \
    curl --max-time 600 --retry 2 -fsSL -o /tmp/dbip-city-lite.mmdb.gz \
      "https://download.db-ip.com/free/dbip-city-lite-${month}.mmdb.gz" \
    || curl --max-time 600 --retry 2 -fsSL -o /tmp/dbip-city-lite.mmdb.gz \
      "https://download.db-ip.com/free/dbip-city-lite-${prev}.mmdb.gz"; \
    gunzip -c /tmp/dbip-city-lite.mmdb.gz > /out/geoip/dbip-city-lite.mmdb; \
    rm -f /tmp/dbip-city-lite.mmdb.gz

FROM debian:bookworm-slim AS runtime

ARG APP_VERSION=dev
ARG VCS_REF=unknown
ENV APP_VERSION=$APP_VERSION

LABEL org.opencontainers.image.source="https://github.com/devnumbers/arenda-platform" \
      org.opencontainers.image.revision="$VCS_REF"

# ca-certificates/curl не пинятся: версии меняются при обновлении пакетов
# в базовом образе, и жёсткий пин ломал бы rebuild.
# hadolint ignore=DL3008
# Bump APT_REFRESH to re-pull security fixes from the Debian repos: the value
# is echoed into the layer below, so a change invalidates the build cache —
# otherwise the layer stays cached with packages frozen at the old build
# date (perl CVEs 2026-10, same drift as the admin image's APK_REFRESH).
ARG APT_REFRESH=2026-10-06

RUN echo "apt refresh: ${APT_REFRESH}" \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 arenda \
    && useradd --system --uid 10001 --gid arenda --home-dir /app --shell /usr/sbin/nologin arenda

COPY apps/backend/certs/tbank/*.crt /usr/local/share/ca-certificates/tbank/
RUN update-ca-certificates

WORKDIR /app

COPY --from=build /out/arenda-api /app/arenda-api
COPY --from=build /src/apps/backend/db/migrations /app/db/migrations
COPY --from=build /src/apps/backend/templates/email /app/templates/email
COPY --from=build /out/geoip/dbip-city-lite.mmdb /app/geoip/dbip-city-lite.mmdb

# The baked-in GeoIP base enables city resolution in the image; a host-run
# local build without GEOIP_DB_PATH simply skips geolocation.
ENV GEOIP_DB_PATH=/app/geoip/dbip-city-lite.mmdb

USER arenda

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/healthz >/dev/null || exit 1

CMD ["/app/arenda-api"]
