#!/usr/bin/env bash
set -euo pipefail
# Run from the repository root so `go run ./cmd/falafel` resolves.
cd "$(dirname "$0")/../.."
PORT="${PORT:-8091}"
if [[ -d .cache/gopath ]]; then
  export GOPATH="$PWD/.cache/gopath"
  export GOMODCACHE="$PWD/.cache/gopath/pkg/mod"
  export GOCACHE="$PWD/.cache/gobuild"
fi
rm -rf .cache/e2e
mkdir -p .cache/e2e
export FALAFEL_ADDR="127.0.0.1:${PORT}"
export FALAFEL_DB_PATH=".cache/e2e/falafel.db"
export FALAFEL_BASE_URL="http://127.0.0.1:${PORT}"
exec go run ./cmd/falafel
