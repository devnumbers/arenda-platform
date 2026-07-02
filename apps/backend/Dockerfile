# syntax=docker/dockerfile:1.7

FROM golang:1.26-bookworm  AS build

WORKDIR /src/apps/backend

COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/arenda-api ./cmd/api

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 arenda \
    && useradd --system --uid 10001 --gid arenda --home-dir /app --shell /usr/sbin/nologin arenda

COPY apps/backend/certs/tbank/*.crt /usr/local/share/ca-certificates/tbank/
RUN update-ca-certificates

WORKDIR /app

COPY --from=build /out/arenda-api /app/arenda-api
COPY --from=build /src/apps/backend/db/migrations /app/db/migrations
COPY --from=build /src/apps/backend/templates/email /app/templates/email

USER arenda

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/healthz >/dev/null || exit 1

CMD ["/app/arenda-api"]
