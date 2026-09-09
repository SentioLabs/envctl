#!/usr/bin/env bash
# Helpers below override functions called by the sourced production scripts.
# shellcheck disable=SC2329
# shellcheck source-path=SCRIPTDIR
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for tag in v1.2.3-rc.1 v1.2.3-rc10 v1.2.3-alpha.1 v1.2.3-beta2 v1.2.4-nightly.20260909; do
    bash "$script_dir/validate-prerelease.sh" "refs/tags/$tag"
done
for ref in refs/heads/main refs/tags/v1.2.3 refs/tags/v1.2.4-nightly.202609091200 refs/heads/v1.2.3-rc.1; do
    if bash "$script_dir/validate-prerelease.sh" "$ref" 2>/dev/null; then exit 1; fi
done

# shellcheck source=nightly.sh
source "$script_dir/nightly.sh"
export GITHUB_REPOSITORY=sentiolabs/envctl
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export work
mock_status=missing
gh() {
    if [[ "$1" == api ]]; then
        case "$mock_status" in
            error) printf 'HTTP/2.0 500 Server Error\r\n\r\n{}\n'; return 1 ;;
            missing) printf 'HTTP/2.0 404 Not Found\r\n\r\n{}\n'; return 1 ;;
            *)
                printf 'HTTP/2.0 200 OK\r\n\r\n'
                cat "$work/release.json" ;;
        esac
    else
        printf '%s\n' "$*" >> "$work/dispatches"
    fi
}
date() { echo 20260909; }
# All git writes are confined to disposable repositories.
git init -q --bare "$work/remote.git"
git init -q -b main "$work/repo"
cd "$work/repo"
git config user.name Test
git config user.email test@example.invalid
printf '{".":"1.2.3"}\n' > .release-please-manifest.json
git add .release-please-manifest.json
git commit -qm initial
git remote add origin "$work/remote.git"
main > /dev/null
tag=v1.2.4-nightly.20260909
original=$(git rev-parse "$tag^{commit}")
grep -q "workflow run prerelease.yml --ref $tag" "$work/dispatches"
# A missing release must retry even after main advances, without moving the tag.
git commit -qm later --allow-empty
main > /dev/null
[[ "$(git rev-parse "$tag^{commit}")" == "$original" ]]
[[ "$(wc -l < "$work/dispatches" | tr -d ' ')" == 2 ]]

jq -n --arg version "${tag#v}" '{draft:false, assets: (["linux_amd64","linux_arm64","darwin_amd64","darwin_arm64"] | map({name:("envctl_"+$version+"_"+.+".tar.gz")})) + [{name:"checksums.txt"}]}' > "$work/release.json"
mock_status=complete
release_complete "$tag"
main > /dev/null
[[ "$(wc -l < "$work/dispatches" | tr -d ' ')" == 2 ]]
# Partial publication is retried.
printf '{"draft":false,"assets":[]}\n' > "$work/release.json"
main > /dev/null
[[ "$(wc -l < "$work/dispatches" | tr -d ' ')" == 3 ]]
mock_status=error
if release_complete "$tag"; then exit 1; else [[ "$?" == 2 ]]; fi
# Next day's unchanged revision produces no additional tag or dispatch.
git checkout -q "$original"
date() { echo 20260910; }
main > /dev/null
[[ "$(wc -l < "$work/dispatches" | tr -d ' ')" == 3 ]]
[[ "$(git tag | wc -l | tr -d ' ')" == 1 ]]
printf 'Release automation tests passed\n'
