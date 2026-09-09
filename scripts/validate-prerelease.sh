#!/usr/bin/env bash
set -euo pipefail
# Keep nightly tags compatible with selfupdate-go's default channel matcher.
ref=${1:-}
if [[ ! "$ref" =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+-((rc|alpha|beta)\.?[0-9]+|nightly\.[0-9]{8})$ ]]; then
    printf 'Expected an rc, alpha, beta, or nightly tag ref; got %s\n' "$ref" >&2
    exit 1
fi
