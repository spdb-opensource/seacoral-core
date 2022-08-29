#!/bin/bash
set -o nounset

die() { echo "$*" 1>&2 ; exit 1; }

which packr2 || die "cannot find packr2 please run go get -u github.com/gobuffalo/packr/v2/packr2 to install firstly"
cd ./api && packr2 clean && packr2
sed -i 's!^import.*api/packrd"$!import _ "github.com/upmio/dbscale-kube/cluster_manager/apiserver/api/packrd"!g' api-packr.go


cd ../

VERSION=`git rev-parse --short HEAD`
BUILDTIME=`date '+%Y%m%d %H:%M %z'`

GOOS=linux GOARCH=amd64 go build -v -ldflags "-X github.com/upmio/dbscale-kube/pkg/vars.GITCOMMIT=$VERSION -X \"github.com/upmio/dbscale-kube/pkg/vars.BUILDTIME=${BUILDTIME}\"" || {
    echo "build failed!!!!!!!!!"
    exit 2
}
