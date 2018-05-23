#!/usr/bin/env bash

# This script builds binaries

set -o errexit
set -o nounset
set -o pipefail


if ! which go > /dev/null; then
	echo "golang needs to be installed"
	exit 1
fi

GO_BUILD_FLAGS="$@"

bin_dir="$(pwd)/hack/build/_output/bin"
mkdir -p ${bin_dir} || true
rm -f ${bin_dir}/*

git_sha=`git rev-parse --short HEAD || echo "GitNotFound"`
git_hash="github.com/camilocot/cassandra-cmd/version.GitSHA=${git_sha}"
go_ldflags="-X ${git_hash}"


echo "building ..."
# We’re disabling cgo which gives us a static binary.
# This is needed for building minimal container based on alpine image.
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $GO_BUILD_FLAGS -o ${bin_dir}/cassandra-crd -installsuffix cgo -ldflags "$go_ldflags" ./cmd/
