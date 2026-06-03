#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd -- "$script_dir"

if ! command -v go >/dev/null 2>&1; then
    printf 'zoneaware: go is required to run the TUI\n' >&2
    exit 1
fi

default_config="${HOME}/.config/zoneaware.yaml"
if [[ $# -eq 0 && ! -f "$default_config" ]]; then
    printf 'zoneaware: config not found at %s\n' "$default_config" >&2
    printf 'copy examples/zoneaware.yaml there, or run ./run.sh -config examples/zoneaware.yaml\n' >&2
    exit 1
fi

exec go run ./cmd/za "$@"
