#!/usr/bin/env bash
# Disposable PostgreSQL + real binaries; no application databases are reused.
set -euo pipefail
cd "$(dirname "$0")/.."

GOOSE_BIN="${GOOSE_BIN:-goose}"
NEWMAN_BIN="${NEWMAN_BIN:-newman}"
export PATH="$(pg_config --bindir):$PATH"
for tool in initdb pg_ctl psql go curl "$GOOSE_BIN" "$NEWMAN_BIN"; do
  command -v "$tool" >/dev/null || { echo "Required tool not found: $tool" >&2; exit 1; }
done
if [[ $(id -u) == 0 ]]; then
  echo "Run as a regular user: initdb cannot run as root." >&2
  exit 1
fi

work=$(mktemp -d /tmp/lab2-rsoi-system.XXXXXX)
pids=()
pg_started=false
cleanup() {
  result=$?
  trap - EXIT
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
  if $pg_started; then pg_ctl -D "$work/data" -m fast -w stop >/dev/null; fi
  if [[ $result != 0 ]]; then
    for log in "$work"/*.log; do [[ ! -f $log ]] || { echo "$log"; tail -50 "$log"; }; done
    echo "Failed run retained at $work" >&2
  else
    rm -rf "$work"
  fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

export PGHOST=127.0.0.1 PGPORT="${TEST_PG_PORT:-55432}" PGUSER=postgres PGDATABASE=postgres
mkdir "$work/socket" "$work/bin"
initdb -D "$work/data" -U postgres -A trust --encoding=UTF8 --no-locale >"$work/initdb.log"
pg_ctl -D "$work/data" -l "$work/postgres.log" -o "-h 127.0.0.1 -p $PGPORT -k $work/socket" -w start
pg_started=true
psql -v ON_ERROR_STOP=1 -f postgres/10-create-user.sql
SCRIPT_PATH="$PWD/postgres" bash postgres/20-create-databases.sh

for service in reservation payment loyalty; do
  psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${service}_test OWNER ${service}_service"
  for database in "$service" "${service}_test"; do
    dsn="postgres://${service}_service:test@127.0.0.1:$PGPORT/$database?sslmode=disable"
    "$GOOSE_BIN" -dir "migrations/$service" postgres "$dsn" up
  done
  # Verify reverse migrations and reapplication on isolated test databases.
  "$GOOSE_BIN" -dir "migrations/$service" postgres "$dsn" down-to 0
  "$GOOSE_BIN" -dir "migrations/$service" postgres "$dsn" up
  go build -o "$work/bin/$service" "./cmd/$service"
done
go build -o "$work/bin/gateway" ./cmd/gateway

# Service credentials must not allow connecting to a neighboring database.
if PGUSER=payment_service PGDATABASE=reservation psql -c 'SELECT 1' >"$work/isolation.log" 2>&1; then
  echo "Database isolation failed" >&2
  exit 1
fi

export RESERVATION_TEST_DATABASE_URL="postgres://reservation_service:test@127.0.0.1:$PGPORT/reservation_test?sslmode=disable"
export PAYMENT_TEST_DATABASE_URL="postgres://payment_service:test@127.0.0.1:$PGPORT/payment_test?sslmode=disable"
export LOYALTY_TEST_DATABASE_URL="postgres://loyalty_service:test@127.0.0.1:$PGPORT/loyalty_test?sslmode=disable"
go vet ./...
SYSTEM_TEST_URL= go test -race -count=1 ./...

for pair in reservation:8070 payment:8060 loyalty:8050; do
  service=${pair%:*}
  port=${pair#*:}
  HTTP_ADDR="127.0.0.1:$port" DATABASE_URL="postgres://${service}_service:test@127.0.0.1:$PGPORT/$service?sslmode=disable" \
    "$work/bin/$service" >"$work/$service.log" 2>&1 &
  pids+=("$!")
done
GATEWAY_ADDR=127.0.0.1:8080 RESERVATION_URL=http://127.0.0.1:8070 PAYMENT_URL=http://127.0.0.1:8060 LOYALTY_URL=http://127.0.0.1:8050 \
  "$work/bin/gateway" >"$work/gateway.log" 2>&1 &
pids+=("$!")

for port in 8070 8060 8050 8080; do
  ready=false
  for attempt in {1..50}; do
    for pid in "${pids[@]}"; do kill -0 "$pid" || { echo "Service exited" >&2; exit 1; }; done
    if curl --max-time 1 -fsS "http://127.0.0.1:$port/manage/health" >/dev/null 2>&1; then ready=true; break; fi
    sleep 0.2
  done
  $ready || { echo "Service on $port did not become ready" >&2; exit 1; }
done

"$NEWMAN_BIN" run v2/postman/collection.json -e v2/postman/environment.json
SYSTEM_TEST_URL=http://127.0.0.1:8080 go test -race -count=1 -v ./tests/system

# Liveness of each actual binary's container healthcheck path.
for pair in reservation:8070 payment:8060 loyalty:8050; do
  service=${pair%:*}
  HTTP_ADDR="127.0.0.1:${pair#*:}" DATABASE_URL="postgres://${service}_service:test@127.0.0.1:$PGPORT/$service?sslmode=disable" \
    "$work/bin/$service" -healthcheck
done
GATEWAY_ADDR=127.0.0.1:8080 "$work/bin/gateway" -healthcheck

# Readiness must become unavailable when the database goes away.
pg_ctl -D "$work/data" -m fast -w stop
pg_started=false
for port in 8070 8060 8050; do
  status=$(curl --max-time 8 -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/manage/health")
  [[ $status == 503 ]] || { echo "Expected 503 after DB stop on $port, got $status" >&2; exit 1; }
done
echo "All services, migrations, database isolation, race tests, Postman and system checks passed."
