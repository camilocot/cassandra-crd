#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

CODEGEN_PKG=./../../../../../../../..${GOPATH}/src/k8s.io/code-generator

${CODEGEN_PKG}/generate-groups.sh "deepcopy,client" \
  github.com/camilocot/cassandra-crd/pkg/client github.com/camilocot/cassandra-crd/pkg/apis \
  cassandra:v1alpha1
