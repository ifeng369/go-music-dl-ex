#!/usr/bin/env bash
# Update the upstream go-music-dl checkout in source/ to the latest main (or a
# specific branch, tag or commit). Local changes in source/ are discarded.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REF="${1:-main}"
REPO="${UPSTREAM_REPO:-https://github.com/guohuiyuan/go-music-dl.git}"

if [ -d "${ROOT}/source/.git" ]; then
    echo "Fetching ${REF} into source/ ..."
    git -C "${ROOT}/source" fetch --depth 1 origin "${REF}"
    git -C "${ROOT}/source" reset --hard FETCH_HEAD
else
    echo "Cloning ${REPO} (${REF}) into source/ ..."
    rm -rf "${ROOT}/source"
    git clone --depth 1 --branch "${REF}" "${REPO}" "${ROOT}/source"
fi

git -C "${ROOT}/source" --no-pager log -1 --oneline
