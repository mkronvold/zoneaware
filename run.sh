#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd -- "$script_dir"

if ! command -v go >/dev/null 2>&1; then
    printf 'zoneaware: go is required to run the TUI\n' >&2
    exit 1
fi

bin_dir="${script_dir}/.bin"
bin_path="${bin_dir}/za"

needs_build=false
if [[ ! -x "$bin_path" ]]; then
    needs_build=true
elif [[ go.mod -nt "$bin_path" || go.sum -nt "$bin_path" || run.sh -nt "$bin_path" ]]; then
    needs_build=true
elif find cmd internal -type f -name '*.go' -newer "$bin_path" -print -quit | grep -q .; then
    needs_build=true
fi

if [[ "$needs_build" == true ]]; then
    mkdir -p -- "$bin_dir"
    printf 'zoneaware: building local binary...\n' >&2
    go build -o "$bin_path" ./cmd/za
fi

default_config="${HOME}/.config/zoneaware.yaml"
if [[ $# -eq 0 && ! -f "$default_config" ]]; then
    printf 'zoneaware: config not found at %s\n' "$default_config" >&2
    printf 'copy examples/zoneaware.yaml there, or run ./run.sh -config examples/zoneaware.yaml\n' >&2
    exit 1
fi

exec "$bin_path" "$@"
