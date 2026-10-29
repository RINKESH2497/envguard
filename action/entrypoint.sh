#!/usr/bin/env bash
# entrypoint.sh — Simple wrapper to run envguard with arguments.
# This script is provided for compatibility; the GitHub Action uses
# composite steps and does not require this entrypoint.

set -euo pipefail

if ! command -v envguard &> /dev/null; then
  echo "Error: envguard is not installed." >&2
  echo "Install it with: go install github.com/envguard/envguard/cmd/envguard@latest" >&2
  exit 1
fi

# Pass all arguments directly to envguard
exec envguard "$@"
