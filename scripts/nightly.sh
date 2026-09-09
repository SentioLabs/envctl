#!/usr/bin/env bash
set -euo pipefail

# A release is complete only when all native updater assets are published.
# API errors are fatal; they must not be confused with a missing release.
release_complete() {
    local tag=$1 response status
    response=$(mktemp)
    if gh api --include "repos/$GITHUB_REPOSITORY/releases/tags/$tag" > "$response"; then
        # Drop HTTP headers before parsing the JSON body.
        awk 'body { print } /^\r?$/ { body=1 }' "$response" | jq -e '
            .draft == false and
            ([.assets[].name] as $names | all(
                ["linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"][];
                ("envctl_" + $version + "_" + . + ".tar.gz") as $asset |
                ($names | index($asset) != null)
            )) and any(.assets[]; .name == "checksums.txt")
        ' --arg version "${tag#v}" >/dev/null && status=0 || status=$?
        [[ "$status" -le 1 ]] || status=2
    elif head -1 "$response" | grep -qE '^HTTP/[^ ]+ 404'; then
        status=1
    else
        status=2
    fi
    rm -f "$response"
    return "$status"
}

main() {
    local released major minor patch tag previous status
    released=$(jq -er '.["."]' .release-please-manifest.json)
    [[ "$released" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid release manifest version' >&2; exit 1; }
    IFS=. read -r major minor patch <<< "$released"
    tag="v${major}.${minor}.$((patch + 1))-nightly.$(date -u +%Y%m%d)"
    if git rev-parse --verify "refs/tags/$tag" >/dev/null 2>&1; then
        if release_complete "$tag"; then
            echo "$tag is already published; skipping"
            return
        else
            status=$?
            [[ "$status" == 1 ]] || { echo 'Unable to check existing release' >&2; exit 1; }
        fi
        echo "Retrying $tag at its original commit"
    else
        previous=$(git tag -l 'v*-nightly.*' --sort=-v:refname | sed -n '1p')
        if [[ -n "$previous" ]] && [[ "$(git rev-list "$previous..HEAD" --count)" == 0 ]]; then
            echo 'No new commits since the last nightly; skipping'
            return
        fi
        git config user.name 'github-actions[bot]'
        git config user.email 'github-actions[bot]@users.noreply.github.com'
        git tag -a "$tag" -m "Nightly build ${tag##*.}"
        git push origin "refs/tags/$tag"
    fi
    # GITHUB_TOKEN pushes do not trigger another push workflow.
    gh workflow run prerelease.yml --ref "$tag"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
