#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


if ! output=$(./hack/k8s/codegen/update-codegen.sh --verify-only 2>&1); then
  echo "FAILURE: verification of codegen failed:"
  echo "${output}"
  exit 1
fi

echo "Verified generated code ==="
