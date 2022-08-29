#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x

export GOPATH=/root/go

# SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
# CODEGEN_PKG="/root/go/pkg/mod/k8s.io/code-generator@v0.18.8"
CODEGEN_PKG=" /root/Dev/golang/pkg/mod/k8s.io/code-generator@v0.18.8"

# san
${CODEGEN_PKG}/generate-groups.sh all github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1  github.com/upmio/dbscale-kube/pkg/apis san:v1alpha1 --go-header-file boilerplate.go.txt

# volumepath
${CODEGEN_PKG}/generate-groups.sh all github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1  github.com/upmio/dbscale-kube/pkg/apis volumepath:v1alpha1 --go-header-file boilerplate.go.txt

# network
${CODEGEN_PKG}/generate-groups.sh all github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1  github.com/upmio/dbscale-kube/pkg/apis networking:v1alpha1 --go-header-file boilerplate.go.txt
#
# unit v4
${CODEGEN_PKG}/generate-groups.sh all github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4  github.com/upmio/dbscale-kube/pkg/apis unit:v1alpha4 --go-header-file boilerplate.go.txt


# host
${CODEGEN_PKG}/generate-groups.sh all github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1  github.com/upmio/dbscale-kube/pkg/apis host:v1alpha1 --go-header-file boilerplate.go.txt


echo "please copy pkg/client from go directory to pkg"
