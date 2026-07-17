#!/usr/bin/env bash
# Arenda Platform: HTTP healthcheck for stage/prod targets.
#
# On every failed check a JSON line is appended to
# /var/log/arenda/healthcheck.log; Vector ships it to Uptrace where the
# HealthcheckFailed monitor fires (see observability/README.md).
#
# Always exits 0: this is a monitoring probe, it must never fail cron.
set -u

LOG_FILE=/var/log/arenda/healthcheck.log

check() {
  local env="$1" name="$2" url="$3"
  if ! curl -fsS --max-time 5 -o /dev/null "$url" 2>/dev/null; then
    mkdir -p "$(dirname "$LOG_FILE")"
    printf '{"level":"error","source":"healthcheck","env":"%s","target":"%s","msg":"healthcheck failed: %s","time":"%s"}\n' \
      "$env" "$name" "$url" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG_FILE"
  fi
}

check stage backend  http://127.0.0.1:28080/healthz
check prod  backend  http://127.0.0.1:18080/healthz
check stage frontend http://127.0.0.1:23000/
check stage admin    http://127.0.0.1:23001/
check stage landing  http://127.0.0.1:23002/
check prod  frontend http://127.0.0.1:13000/
check prod  admin    http://127.0.0.1:13001/
check prod  landing  http://127.0.0.1:13002/

exit 0
