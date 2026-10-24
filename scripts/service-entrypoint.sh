#!/bin/sh
set -eu

/goose -dir /migrations postgres "$DATABASE_URL" up
exec /service "$@"
