#!/bin/bash

set -o nounset
# ##############################################################################
# Globals, settings
# ##############################################################################
FILE_NAME="build"
VERSION="1.0.0"

DIR="$(readlink -f "${0}")"
BASE_DIR="$(dirname "${DIR}")"

ARCH="${1:-amd64}"

CE_PATH="${BASE_DIR}/../"
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
# ##############################################################################
# pack function package
# ##############################################################################
pack_amd64(){
    local func_name="${FILE_NAME}.pack_amd64"

    [[ -d "${BASE_DIR}/bin" ]] && {
        rm -rf "${BASE_DIR:?}/bin"
    }

    [[ -d "${BASE_DIR}/script" ]] && {
        rm -rf "${BASE_DIR}/script"
    }

    mkdir "${BASE_DIR}/bin" || die 20 "mkdir bin failed!"

    mkdir "${BASE_DIR}/script"  || die 20 "mkdir script failed!"

    cd "${CE_PATH}/controller-manager" || die 21 "${func_name}" "cd controller-manager directory failed!"

    sh build.sh || die 22 "${func_name}" "build cluster_engine binary failed!"

    mv controller-manager "${BASE_DIR}/bin/cluster_engine" || die 23 "${func_name}" "update cluster_engine binary failed!"

    cd "${BASE_DIR}/../storage/scripts" || die 24 "${func_name}" "cd storage scripts directory failed!"

    cp -r StorMGR "${BASE_DIR}"/script || die 25 "${func_name}" "update StorMGR failed!"

    cd "${BASE_DIR}/../image/scripts" || die 26 "${func_name}" "cd image scripts directory failed!"

    cp -r imageMGR "${BASE_DIR}"/script || die 27 "${func_name }" "update imageMGR failed!"

    cd "${BASE_DIR}/../host/scripts" || die 28 "${func_name}" "cd host scripts directory failed!"

    sh host-init/pack.sh || die 29 "${func_name}" "pack host-init failed!"

    cp -r host-init "${BASE_DIR}"/script  || die 30 "${func_name}" "update host-init failed!"
}
# ##############################################################################
# The main() function is called at the action function.
# ##############################################################################
main () {
    name=dbscale/cluster_engine
    version="$(git symbolic-ref --short -q HEAD)-$(git rev-parse --short HEAD)-${ARCH}"
    info "main" "Starting build image"

    case "${ARCH}" in
        "amd64")
            pack_amd64 || exit $?
            ;;
    esac

    cd "${BASE_DIR}" || exit $?

    if printenv http_proxy; then
        docker build --rm=true --network=host -t "${name}:${version}"  --build-arg http_proxy="$( printenv http_proxy )" --build-arg https_proxy="$( printenv https_proxy )" . || {
            die 11 "build docker" "build docker image(${name}:${version}}) files failed!"
        }
    else
        docker build --rm=true --network=host -t "${name}:${version}" . || {
            die 11 "build docker" "build docker image(${name}:${version}}) files failed!"
        }
    fi

    info "main" "Build image(${name}:${version}}) done !!!"
}

main "${@:-""}"
