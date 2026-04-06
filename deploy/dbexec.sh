#!/usr/bin/env bash
# dbexec.sh - build, ship, run, and delete the dbexec binary on the server.
#
# 1. Write the SQL in the query constant in cmd/dbexec/main.go
# 2. Run: ./deploy/dbexec.sh

set -euo pipefail

SSH_HOST="${SSH_HOST:-merylmoe}"
DB_PATH="${DB_PATH:-/opt/meryl.moe/data/meryl.db}"
REMOTE_BIN="/tmp/dbexec_$$"
LOCAL_BIN="$(mktemp)"

cleanup() {
	rm -f "$LOCAL_BIN"
	ssh "$SSH_HOST" "rm -f '$REMOTE_BIN'" 2>/dev/null || true
}
trap cleanup EXIT

read -r -p "run dbexec against production ($SSH_HOST)? [y/N] " confirm < /dev/tty
if [[ "${confirm,,}" != "y" ]]; then
	echo "aborted"
	exit 0
fi

echo "--- building"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o "$LOCAL_BIN" ./cmd/dbexec/main.go

echo "--- uploading"
scp -q "$LOCAL_BIN" "$SSH_HOST:$REMOTE_BIN"
ssh "$SSH_HOST" "chmod +x '$REMOTE_BIN'"

echo "--- executing"
ssh "$SSH_HOST" "'$REMOTE_BIN' -db '$DB_PATH'"

echo "--- done"

