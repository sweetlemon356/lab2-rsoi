#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${SCRIPT_PATH:-/docker-entrypoint-initdb.d}"
psql -v ON_ERROR_STOP=1 -f "$SCRIPT_PATH/scripts/db-v2.sql"
