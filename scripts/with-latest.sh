#!/usr/bin/env bash
# Run the repro tests against the latest nats.go / nats-server releases, in a
# temporary copy of the module, leaving the pinned go.mod untouched.
# Usage: scripts/with-latest.sh [go test args...]   e.g. -race -v ./micro_race/
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp -r "$root/go.mod" "$root/go.sum" "$root/micro_race" "$root/filestore_lock" "$tmp/"
cd "$tmp"
go get github.com/nats-io/nats.go@latest github.com/nats-io/nats-server/v2@latest
go mod tidy
grep 'nats-io/nats' go.mod
go test -count=1 "$@"
