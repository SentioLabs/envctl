#!/usr/bin/env bash
# Bootstrap envctl. Subsequent updates use: envctl self update
# curl -fsSL https://raw.githubusercontent.com/sentiolabs/envctl/main/scripts/install.sh | bash
set -euo pipefail

REPO="sentiolabs/envctl"
TAG="${TAG:-}"
# --force and FORCE are accepted for older envctl self updaters. Bootstrap
# always installs the requested release; version selection belongs to the CLI.

fail() { printf 'envctl: %s\n' "$*" >&2; exit 1; }

usage() {
    printf '%s\n' 'Usage: install.sh [--tag TAG|--tag=TAG] [--force]' \
        'Install the latest stable release, or the given tag.' \
        'Use envctl self update for subsequent updates.'
}

# Write to a file so a failed download cannot feed partial data into extraction.
download() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 30 --retry 2 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --timeout=30 -O "$2" "$1"
    else
        fail 'curl or wget is required'
    fi
}

install_directory() {
    if [[ -w /usr/local/bin ]]; then
        printf '%s\n' /usr/local/bin
    else
        printf '%s\n' "$HOME/.local/bin"
    fi
}

main() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --tag)
                [[ $# -ge 2 && -n "$2" ]] || fail '--tag requires a value'
                TAG="$2"; shift 2 ;;
            --tag=*) TAG="${1#*=}"; [[ -n "$TAG" ]] || fail '--tag requires a value'; shift ;;
            --force|-f) shift ;;
            --help|-h) usage; return ;;
            *) fail "unknown option: $1" ;;
        esac
    done

    local os arch
    case "$(uname -s)" in
        Linux) os=linux ;;
        Darwin) os=darwin ;;
        *) fail 'supported operating systems are Linux and macOS' ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) fail 'supported architectures are amd64 and arm64' ;;
    esac

    bootstrap_staged=''
    bootstrap_tmp_dir=$(mktemp -d)
    # Cleanup paths stay available after Bash unwinds a failed function.
    trap 'rm -rf "$bootstrap_tmp_dir"; if [[ -n "$bootstrap_staged" ]]; then rm -f "$bootstrap_staged"; fi' EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM

    if [[ -z "$TAG" ]]; then
        download "https://api.github.com/repos/${REPO}/releases/latest" "$bootstrap_tmp_dir/release.json"
        TAG=$(sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$bootstrap_tmp_dir/release.json")
    fi
    [[ "$TAG" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[[:alnum:].-]+)?$ ]] || fail "invalid release tag: $TAG"
    local archive="envctl_${TAG#v}_${os}_${arch}.tar.gz"
    local base="https://github.com/${REPO}/releases/download/${TAG}"
    printf 'Installing envctl %s (%s/%s)...\n' "$TAG" "$os" "$arch"
    download "$base/$archive" "$bootstrap_tmp_dir/$archive"
    download "$base/checksums.txt" "$bootstrap_tmp_dir/checksums.txt"

    local expected actual
    expected=$(awk -v name="$archive" '$2 == name || $2 == "*" name {print $1}' "$bootstrap_tmp_dir/checksums.txt")
    [[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail "missing or invalid checksum for $archive"
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$bootstrap_tmp_dir/$archive")
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$bootstrap_tmp_dir/$archive")
    else
        fail 'sha256sum or shasum is required'
    fi
    [[ "${actual%% *}" == "$expected" ]] || fail "checksum mismatch for $archive"
    # Extract only the binary, never unrelated archive entries or paths.
    tar -xzOf "$bootstrap_tmp_dir/$archive" envctl > "$bootstrap_tmp_dir/envctl"
    [[ -s "$bootstrap_tmp_dir/envctl" ]] || fail 'archive contains an empty binary'

    local install_dir
    install_dir=$(install_directory)
    mkdir -p "$install_dir"
    bootstrap_staged=$(mktemp "$install_dir/.envctl.XXXXXX")
    cp "$bootstrap_tmp_dir/envctl" "$bootstrap_staged"
    chmod 755 "$bootstrap_staged"
    if [[ "$os" == darwin ]] && command -v codesign >/dev/null 2>&1; then
        codesign --force --sign - "$bootstrap_staged" 2>/dev/null || printf 'Warning: could not ad-hoc sign envctl\n' >&2
    fi
    mv -f "$bootstrap_staged" "$install_dir/envctl"
    bootstrap_staged=''
    printf 'Installed envctl %s to %s/envctl\n' "$TAG" "$install_dir"
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        printf 'Add %s to your PATH.\n' "$install_dir"
    fi
    printf 'For updates, run: envctl self update\n'
    rm -rf "$bootstrap_tmp_dir"
    trap - EXIT INT TERM
}

# BASH_SOURCE is unset for curl ... | bash. Keep sourced helpers inert,
# while allowing both file execution and the documented stdin entry point.
if [[ "${BASH_SOURCE[0]:-$0}" == "$0" ]]; then
    main "$@"
fi
