#!/usr/bin/env bash
set -e

if ! command -v envguard &> /dev/null; then
  echo "envguard CLI is not installed or not in PATH." >&2
  echo "Install it with: go install github.com/RINKESH2497/envguard/cmd/envguard@latest" >&2
  exit 1
fi

exec envguard "$@"
