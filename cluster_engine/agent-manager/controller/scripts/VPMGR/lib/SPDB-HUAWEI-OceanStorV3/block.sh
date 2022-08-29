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

scanDevice() {
    #shellcheck disable=SC2045
    for fc_host in $( ls /sys/class/fc_host/ ); do
        #echo 1 > /sys/class/fc_host/${fc_host}/issue_lip
        echo '- - -' > "/sys/class/scsi_host/${fc_host}/scan"
    done
}

main() {
    if [[ "${VG_TYPE}" == "remote" ]]; then
        vgdisplay "${VG_NAME}" &> /dev/null && {
            vgdisplay "${VG_NAME}" -C -o "vg_exported" --noheading | grep -w "exported" &> /dev/null || {
                vgexport -y "${VG_NAME}" > /dev/null || die 3 "vg export failed"
            }

            scanDevice &> /dev/null
            local interval=10
            local time=3
            for i in $(seq ${interval}); do
                vgdisplay "${VG_NAME}" &> /dev/null && {
                    scanDevice &> /dev/null
                    sleep "${time}"

                    ((i++))
                    # flush vg info
                    vgimport "${VG_NAME}" &> /dev/null

                    if [[ ${i} -gt ${interval} ]]; then
                        die 5 "scan timeout!"
                    fi
                    continue
                }
                break
            done
        }

        die 0 "block successful"
    else
        die 6 "VG_TYPE error , block action only support remote"
    fi
}
main
