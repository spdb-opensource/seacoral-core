#!/bin/bash

set -o nounset

INPUT="${1}"

SCRIPTS_DIR="$(readlink -f "$0")"
SCRIPTS_BASE_DIR="$(dirname "${SCRIPTS_DIR}")"
declare -r SCRIPTS_BASE_DIR
LIB_BASE_DIR="${SCRIPTS_BASE_DIR%/*}"
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=./lib/_function.sh
source "${LIB_BASE_DIR}/_function.sh"

installed jq || die 100 "jq not installed!"

MOUNT_DIR="$( getValueNotNull ".mount_dir" "${INPUT}" )" || die $? "MOUNT_DIR ${MOUNT_DIR}"
LV_NAME="$( getValueNotNull ".lv.name" "${INPUT}" )" || die $? "LV_NAME ${LV_NAME}"
VG_NAME="$( getValueNotNull ".vg.name" "${INPUT}")" || die $? "VG_NAME ${VG_NAME}"
VG_TYPE="$( getValueNotNull ".vg.type" "${INPUT}" )" || die $? "VG_TYPE ${VG_TYPE}"

main() {
    if [[ "${VG_TYPE}" == "remote" ]]; then
        sh "${SCRIPTS_BASE_DIR}/block.sh" "${INPUT}"
    else
        die 6 "VG_TYPE error , only support local and remote"
    fi
}

main
