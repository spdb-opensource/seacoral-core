#!/usr/bin/env bash

# This program is part of DBScale.

set -o nounset
# ##############################################################################
# Globals, settings
# ##############################################################################
FILE_NAME="pack_host_init"
VERSION="1.0.0"

DIR="$(readlink -f "${0}")"
BASE_DIR="$(dirname "${DIR}")"

ARCH="${1:-amd64}"

IMAGE_DEP_PATH="${GOPATH}/src/git.bsgchina.com/dbscale-kube/dbscale-kube-dependency"
RPM_DIR="${IMAGE_DEP_PATH}/${ARCH}/rpm"
CE_PATH="${BASE_DIR}/../../../"

JQ_VERSION="1.5"
FIO_VERSION="3.7"
# ##############################################################################
# common function package
# ##############################################################################
die () {
    local status="${1}"
    shift
    local function_name="${1}"
    shift
    error "${function_name}" "$*"
    exit "$status"
}

error () {
    local function_name="${1}"
    shift
    local timestamp
    timestamp="$( date +"%Y-%m-%d %T %N" )"
    local log_file="/tmp/${FILE_NAME}-${VERSION}.log"

    [[ -v LOG_MOUNT ]] && {
        log_file="${LOG_MOUNT}/${FILE_NAME}.log"
    }

    if [[ -z "${log_file}" ]]; then
        echo "[${timestamp}] ERR[${function_name}]: $* ;"
    else
        echo "[${timestamp}] ERR[${function_name}]: $* ;" | tee -a "${log_file}"
    fi
}

info () {
    local function_name="${1}"
    shift
    local timestamp
    timestamp="$( date +"%Y-%m-%d %T %N" )"
    local log_file="/tmp/${FILE_NAME}-${VERSION}.log"

    [[ -v LOG_MOUNT ]] && {
        log_file="${LOG_MOUNT}/${FILE_NAME}.log"
    }

    if [[ -z "${log_file}" ]]; then
        echo "[${timestamp}] INFO[${function_name}]: $* ;"
    else
        echo "[${timestamp}] INFO[${function_name}]: $* ;" | tee -a "${log_file}"
    fi
}

installed () {
    command -v "$1" >/dev/null 2>&1
}
# ##############################################################################
# The pack() function is called at the main function.
# ##############################################################################
pack_amd64(){
    local func_name="${FILE_NAME}.pack_amd64"

    [[ -d "${BASE_DIR}/packages" ]] && {
        rm -rf "${BASE_DIR}/packages"
    }

    mkdir -p "${BASE_DIR}/packages/x86_64/bin" || die 20 "mkdir bin failed!"

    mkdir -p "${BASE_DIR}/packages/x86_64/script" || die 20 "mkdir script failed!"

    mkdir -p "${BASE_DIR}/packages/x86_64/rpm" || die 20 "mkdir rpm failed!"

    # prepare rpm
    cp -r "${RPM_DIR}/jq-${JQ_VERSION}" "${BASE_DIR}/packages/x86_64/rpm/jq" || die 21  "${func_name}" "copy jq failed!"
    cp -r "${RPM_DIR}/fio-${FIO_VERSION}" "${BASE_DIR}/packages/x86_64/rpm/fio" || die 21 "${func_name}" "copy fio failed!"

    # prepare cluster_engine binary
    cd "${CE_PATH}/agent-manager" || die 22 "cluster_agent directory not exists!"
    sh build.sh || die 22 "build cluster_agent failed!"
    mv agent-manager "${BASE_DIR}/packages/x86_64/bin/cluster_agent" || die 22 "update cluster_agent failed!"

    # prepare netdev-plugin binary
    cd "${CE_PATH}/network/plugin" || die 23 "cd netdev-plugin directory failed!"
    sh build.sh || die 23 "build netdev-plugin failed!"
    mv plugin "${BASE_DIR}/packages/x86_64/bin/netdev-plugin" || die 23 "update netdev-plugin failed!"

    # prepare VPMGR scripts
    cd "${CE_PATH}/agent-manager/controller/scripts" || die 24 "cd VPMGR directory failed!"
    tar -cf VPMGR.tar VPMGR || die 24 "tar VPMGR failed!"
    mv VPMGR.tar "${BASE_DIR}/packages/x86_64/script/" || die 24 "update VPMGR failed!"

    # prepare hostMGR scripts
    tar -cf hostMGR.tar hostMGR || die 25 "tar hostMGR failed!"
    mv hostMGR.tar "${BASE_DIR}/packages/x86_64/script/" || die 25 "update hostMGR failed!"

    # prepare netdevMGR scripts
    cd "${CE_PATH}/network/plugin/scripts/macvlan" || die 26 "cd sriovMGR directory failed!"
    tar -cf "macvlan.tar" netdevMGR || die 26 "tar macvlan scripts failed!"
    mv macvlan.tar "${BASE_DIR}/packages/x86_64/script/" || die 26 "update macvlan scripts failed!"
    cd "${CE_PATH}/network/plugin/scripts/sriov" || die 26 "cd sriovMGR directory failed!"
    tar -cf "sriov.tar" netdevMGR || die 26 "tar sriov scripts failed!"
    mv sriov.tar "${BASE_DIR}/packages/x86_64/script/" || die 26 "update sriov scripts failed!"
}
# ##############################################################################
# The main() function is called at the action function.
# ##############################################################################
main () {
    case "${ARCH}" in
        "amd64")
            pack_amd64 || exit $?
            ;;
    esac
}

main "${@:-""}"
