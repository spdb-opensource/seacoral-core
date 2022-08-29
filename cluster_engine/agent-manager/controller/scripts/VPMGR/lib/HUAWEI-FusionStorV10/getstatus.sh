#!/bin/bash

set -o nounset

INPUT="${1}"

SCRIPTS_DIR="$(readlink -f "$0")"
SCRIPTS_BASE_DIR="$(dirname "${SCRIPTS_DIR}")"
declare -r SCRIPTS_BASE_DIR
LIB_BASE_DIR="${SCRIPTS_BASE_DIR%/*}"
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=./lib/function.sh
source "${LIB_BASE_DIR}/_function.sh"

installed jq || die 100 "jq not installed!"

MOUNT_DIR="$( getValueNotNull ".mount_dir" "${INPUT}" )" || die $? "MOUNT_DIR ${MOUNT_DIR}"
MOUNT_DIR_A=${MOUNT_DIR%/*}
MOUNT_DIR_B=${MOUNT_DIR##*/}
MOUNT_DIR="${MOUNT_DIR_A}/${MOUNT_DIR_B//-/_}"
LV_NAME="$( getValueNotNull ".lv.name" "${INPUT}" )" || die $? "LV_NAME ${LV_NAME}"
VG_NAME="$( getValueNotNull ".vg.name" "${INPUT}")" || die $? "VG_NAME ${VG_NAME}"
LV_NAME="$( sed 's/-/_/g' <<< "${LV_NAME}_lv" )"
VG_NAME="$( sed 's/-/_/g' <<< "${VG_NAME}_vg" )"
LV_PATH="/dev/${VG_NAME}/${LV_NAME}"

main(){
    local lv_dm_path
    local status

    lv_dm_path="$(lvdisplay -C -o "lv_dm_path" --noheadings "${LV_PATH}" 2> /dev/null)" || die 6 "get lv_dm_path failed!"
    lv_dm_path="$(sed 's/ //g' <<< "${lv_dm_path}")"

    if grep "${lv_dm_path}" "/proc/mounts" | grep "${MOUNT_DIR}" &> /dev/null; then
        status=true
    else
        status=false
    fi

    jq . <<< "{\"mounted\": ${status}}"
}

main
