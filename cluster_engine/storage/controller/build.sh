#!/bin/bash
set -o nounset

GITCOMMIT="$(git rev-parse --short HEAD)"
BUILDTIME="$(date +%FT%T%z)"
KEY=${1:-default}
PKG="github.com/upmio/dbscale-kube/pkg/vars"

GOOS=linux GOARCH=amd64 go build -v -ldflags "-w -X ${PKG}.GITCOMMIT=${GITCOMMIT} -X ${PKG}.BUILDTIME=${BUILDTIME} -X ${PKG}.SeCretAESKey=${KEY}"

if [[ $? -ne 0 ]]; then
    echo "build failed!!!!!!!!!"
    exit 2
fi
