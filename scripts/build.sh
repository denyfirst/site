#!/bin/bash
#
# Builds the release artifact into a directory.
#
# One copy, called by build-release.yml, by reproduce.yml and by the
# maintainer's release.ps1 -Compare. Two copies of a build command drift, and
# when they do the hashes disagree and the failure looks exactly like a
# tampered release. A reproduction is only worth running if a mismatch means
# something.
#
# The toolchain is not arranged here: the go directive in go.mod names the
# version and Go fetches it into the module cache, where -trimpath reaches its
# paths. Installing one elsewhere gives a different GOROOT in the binary.
#
# Usage:
#   bash scripts/build.sh v0.1.0 dist

set -euo pipefail

tag=${1:?usage: build.sh TAG OUTPUT_DIR}
out=${2:?usage: build.sh TAG OUTPUT_DIR}

# A tag goes into the binary and into a file name, so it is held to the shape
# of one before it reaches either.
if ! [[ "${tag}" =~ ^v[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}$ ]]; then
    echo "build.sh: ${tag} is not a release tag such as v0.1.0" >&2
    exit 2
fi

mkdir -p "${out}"

# One target. The site runs on one server, a Linux machine on amd64, and a
# build nobody runs is a build nobody checks.
#
# -trimpath removes the local paths, CGO_ENABLED=0 the host C toolchain,
# -buildvcs=false the version-control stamp that varies with how the tree was
# fetched, and -s -w the symbol and debugging tables. -X main.version stamps
# the tag, so the binary can say which release it is after it has been
# renamed; both callers pass the same tag, so two builds of it still agree.
#
# The output then depends on the source, the compiler and the tag, and nothing
# else.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -buildvcs=false \
        -ldflags "-s -w -X main.version=${tag}" \
        -o "${out}/denyfirstd_${tag}_linux_amd64" ./cmd/denyfirstd

ls -la "${out}"
