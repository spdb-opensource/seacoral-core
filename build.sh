#!/bin/sh

die() { echo "$*" 1>&2 ; exit 1; }

export GOPROXY=goproxy.cn
CWD=`pwd`

which packr2 &> /dev/null
if [ $? -ne 0 ];then
    go get -u github.com/gobuffalo/packr/v2/packr2
fi

VERSION=`git rev-parse --short HEAD`
BUILDTIME=`date '+%Y%m%d %H:%M %z'`

cd cluster_manager/apiserver/api && packr2 clean && packr2 || die " packr2 failed"
sed -i 's!^import.*api/packrd"$!import _ "github.com/upmio/dbscale-kube/cluster_manager/apiserver/api/packrd"!g' api-packr.go
cd ${CWD}
cd cluster_manager/apiserver && go get . && go build -ldflags "-X github.com/upmio/dbscale-kube/pkg/vars.GITCOMMIT=$VERSION -X \"github.com/upmio/dbscale-kube/pkg/vars.BUILDTIME=${BUILDTIME}\"" || die "!!! build cluster_manager/apiserver failed"
cd ${CWD}
cd cluster_engine/controller-manager && go get . && go build -ldflags "-X github.com/upmio/dbscale-kube/pkg/vars.GITCOMMIT=$VERSION -X \"github.com/upmio/dbscale-kube/pkg/vars.BUILDTIME=${BUILDTIME}\"" || die "!!! build cluster_engine/controller-manager failed"
cd ${CWD}
cd cluster_engine/agent-manager && go get . && go build -ldflags "-X github.com/upmio/dbscale-kube/pkg/vars.GITCOMMIT=$VERSION -X \"github.com/upmio/dbscale-kube/pkg/vars.BUILDTIME=${BUILDTIME}\"" || die "!!! build cluster_engine/agent-manager failed"
cd ${CWD}

\rm -f CM-apiserver CE-controller-manager CE-controller-agent

mv cluster_manager/apiserver/apiserver CM-apiserver
# mv cluster_engine/controller-manager/controller-manager CE-controller-manager
# mv cluster_engine/agent-manager/agent-manager CE-controller-agent

strip C[EM]*
rm -rf a.zip
# zip -9 a.zip CM-apiserver CE-controller-manager CE-controller-agent
zip -9 a.zip CM-apiserver
