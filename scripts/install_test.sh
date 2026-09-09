#!/usr/bin/env bash
# Helpers below override functions called by the sourced production scripts.
# shellcheck disable=SC2329
# shellcheck source-path=SCRIPTDIR
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
# Exercise the documented pipe-to-bash entry point, not just sourced helpers.
# Help must work both as a file and through stdin without network or installation.
bash "$script_dir/install.sh" --help > "$work/file-help"
# The pipeline is the entry point under test.
# shellcheck disable=SC2002
cat "$script_dir/install.sh" | bash -s -- --help > "$work/pipe-help"
cmp "$work/file-help" "$work/pipe-help"
grep -q 'Usage: install.sh' "$work/pipe-help"

# No-argument piping must enter main too. Stop at platform detection so this
# regression test cannot install into the host's /usr/local/bin.
mkdir -p "$work/unsupported-platform"
printf '#!/bin/sh\nprintf "UnsupportedTestOS\\n"\n' > "$work/unsupported-platform/uname"
chmod 755 "$work/unsupported-platform/uname"
# Match curl ... | bash with no script arguments.
# shellcheck disable=SC2002
if cat "$script_dir/install.sh" | PATH="$work/unsupported-platform:$PATH" bash > "$work/piped-error" 2>&1; then
    echo 'Piped installer unexpectedly accepted an unsupported platform' >&2
    exit 1
fi
grep -q 'supported operating systems are Linux and macOS' "$work/piped-error"

mkdir -p "$work/fixture"
printf '#!/bin/sh\nprintf "envctl v9.0.0\\n"\n' > "$work/fixture/envctl"
chmod 755 "$work/fixture/envctl"
fixture_archive=envctl_9.0.0_linux_amd64.tar.gz
tar -czf "$work/fixture/$fixture_archive" -C "$work/fixture" envctl
(cd "$work/fixture" && shasum -a 256 "$fixture_archive" > checksums.txt)
printf '{"tag_name":"v9.0.0"}\n' > "$work/fixture/release.json"

run_install() (
    # shellcheck source=install.sh
    source "$script_dir/install.sh"
    install_directory() { printf '%s\n' "$work/case/bin"; }
    uname() { if [[ "$1" == -s ]]; then printf '%s\n' "${TEST_OS:-Linux}"; else echo x86_64; fi; }
    download() {
        printf '%s\n' "$1" >> "$work/downloads"
        [[ "${FAIL_DOWNLOAD:-false}" == false ]] || return 1
        local name=${1##*/}
        [[ "$name" != latest ]] || name=release.json
        cp "$work/fixture/$name" "$2"
    }
    export TMPDIR="$work/case/tmp"
    mkdir -p "$TMPDIR"
    main "$@"
)

assert_clean() {
    [[ -z "$(ls -A "$work/case/tmp")" ]] || { echo 'Temporary downloads leaked' >&2; exit 1; }
    [[ -z "$(find "$work/case/bin" -name '.envctl.*' -print)" ]] || exit 1
}

run_install > "$work/output"
[[ "$("$work/case/bin/envctl")" == 'envctl v9.0.0' ]]
grep -q 'envctl self update' "$work/output"
assert_clean
for args in '--force --tag=v9.0.0' '--tag v9.0.0'; do
    # Intentional splitting exercises both historical flag forms.
    # shellcheck disable=SC2086
    run_install $args > /dev/null
    assert_clean
done
TAG=v9.0.0 FORCE=true run_install > /dev/null
assert_clean

printf 'original' > "$work/case/bin/envctl"
printf '%064d  %s\n' 0 "$fixture_archive" > "$work/fixture/checksums.txt"
# Launch failure cases as standalone shells: calling main in an if condition
# would disable errexit inside the entire function being tested.
export work script_dir
export -f run_install
if bash -c 'run_install --tag=v9.0.0' > "$work/output" 2>&1; then exit 1; fi
grep -q 'checksum mismatch' "$work/output"
[[ "$(cat "$work/case/bin/envctl")" == original ]]
assert_clean
if TEST_OS=FreeBSD bash -c 'run_install' > "$work/output" 2>&1; then exit 1; fi
grep -q 'supported operating systems' "$work/output"
if FAIL_DOWNLOAD=true bash -c 'run_install' > "$work/output" 2>&1; then exit 1; fi
assert_clean
if bash -c 'run_install --tag' > "$work/output" 2>&1; then exit 1; fi
grep -q 'requires a value' "$work/output"

# Exercise real download dispatch with one mock downloader on PATH at a time.
for tool in curl wget; do
    mkdir -p "$work/$tool"
    cat > "$work/$tool/$tool" <<'MOCK'
#!/bin/bash
while [[ $# -gt 0 ]]; do
    case "$1" in
        -o|-O) output=$2; shift 2 ;;
        *) shift ;;
    esac
done
exec /bin/cp "$work/fixture/envctl" "$output"
MOCK
    chmod 755 "$work/$tool/$tool"
    PATH="$work/$tool" /bin/bash -c 'source "$script_dir/install.sh"; download https://example.invalid/archive "$work/downloaded"'
    cmp "$work/fixture/envctl" "$work/downloaded"
done
printf 'Bootstrap tests passed\n'
