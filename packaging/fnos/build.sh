#!/usr/bin/env bash
# Build a fnOS FPK for one architecture.
#
# Usage:
#   FNPACK=/path/to/fnpack ./packaging/fnos/build.sh <version> <amd64|arm64> <path-to-linux-binary> [output-dir]
#
# Must run on Linux (fnpack is a Linux binary). The web UI is already embedded
# in the Go binary by go:embed, so the payload is just the binary plus metadata.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${HERE}/../.." && pwd)"

VERSION="${1:?usage: build.sh <version> <amd64|arm64> <binary> [output-dir]}"
VERSION="${VERSION#v}"
ARCH="${2:?architecture is required}"
BINARY="${3:?linux binary path is required}"
OUT="$(mkdir -p "${4:-${ROOT}/dist/fnos}" && cd "${4:-${ROOT}/dist/fnos}" && pwd)"
FNPACK="${FNPACK:-fnpack}"

case "${ARCH}" in
    amd64|x86_64|x86) ARCH="amd64"; PLATFORM="x86" ;;
    arm64|aarch64|arm) ARCH="arm64"; PLATFORM="arm" ;;
    *) echo "unsupported architecture: ${ARCH}" >&2; exit 2 ;;
esac

BINARY="$(cd "$(dirname "${BINARY}")" && pwd)/$(basename "${BINARY}")"
[ -f "${BINARY}" ] || { echo "binary not found: ${BINARY}" >&2; exit 2; }
[ -x "${FNPACK}" ] || command -v "${FNPACK}" >/dev/null || { echo "fnpack not found: ${FNPACK}" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT
STAGE="${WORK}/stage"
mkdir -p "${STAGE}"
cp -a "${HERE}/." "${STAGE}/"
rm -f "${STAGE}/build.sh"

cp "${BINARY}" "${STAGE}/app/musicdl-server"
chmod 755 "${STAGE}/app/musicdl-server"
chmod 755 "${STAGE}"/cmd/*

sed -i "s/^version[[:space:]]*=.*/version         = ${VERSION}/" "${STAGE}/manifest"
sed -i "s/^platform[[:space:]]*=.*/platform        = ${PLATFORM}/" "${STAGE}/manifest"
sed -i "s/^checksum[[:space:]]*=.*/checksum        =/" "${STAGE}/manifest"

echo "Building ${PLATFORM} package for version ${VERSION}..."
( cd "${STAGE}" && "${FNPACK}" build --directory "${STAGE}" )

FPK="$(find "${STAGE}" -maxdepth 1 -type f -name '*.fpk' | head -n 1)"
[ -n "${FPK}" ] || { echo "fnpack did not produce an .fpk" >&2; exit 1; }

TARGET="${OUT}/musicdl-ex-${VERSION}-fnos-${ARCH}.fpk"
mv "${FPK}" "${TARGET}"
( cd "${OUT}" && sha256sum "$(basename "${TARGET}")" > "$(basename "${TARGET}").sha256" )

echo "Created ${TARGET}"
cat "${TARGET}.sha256"
