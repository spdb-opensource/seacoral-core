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
LV_NAME="$( getValueNotNull ".lv.name" "${INPUT}" )" || die $? "LV_NAME ${LV_NAME}"
VG_NAME="$( getValueNotNull ".vg.name" "${INPUT}")" || die $? "VG_NAME ${VG_NAME}"
LV_PATH="/dev/${VG_NAME}/${LV_NAME}"
VG_TYPE="$( getValueNotNull ".vg.type" "${INPUT}" )" || die $? "VG_TYPE ${VG_TYPE}"

mountDir() {
    local lv_path="${1}"
    local mount_dir="${2}"

    local lv_dm_path
    lv_dm_path="$(lvdisplay -C -o "lv_dm_path" --noheadings "${lv_path}" 2> /dev/null)" || die 6 "get lv_dm_path failed!"
    lv_dm_path="$(sed 's/ //g' <<< "${lv_dm_path}")"

    test -d "${mount_dir}" || {
        mkdir -p "${mount_dir}" > /dev/null || {
            die 8 "mkdir failed!"
        }
    }

    grep "${lv_dm_path}" "/proc/mounts" | grep "${mount_dir}" &> /dev/null || {
        mount "${lv_dm_path}" "${mount_dir}" > /dev/null || {
            die 9 "mount failed!"
        }
    }
}

main() {
    if [[ "${VG_TYPE}" == "remote" ]]; then
        installed upadm || {
            die 3 "upadm not installed!"
        }

        upadm start hotscan &> /dev/null

        local interval=10
        local time=3
        for i in $(seq ${interval}); do
            vgdisplay "${VG_NAME}" &> /dev/null || {
                upadm start hotscan &> /dev/null || {
                    sleep "${time}"
                }

                ((i++))

                if [[ ${i} -gt ${interval} ]]; then
                    die 5 "scan timeout!"
                fi
                continue
            }
            break
        done

        vgdisplay "${VG_NAME}" -C -o "vg_exported" --noheadings | grep -w "exported" && {
            vgimport "${VG_NAME}" > /dev/null || {
                die 6 "vg import failed!"
            }
        }

        lvdisplay "${LV_PATH}" -C -o "lv_active" --noheadings | grep -w "active" &> /dev/null || {
            lvchange -ay "${LV_PATH}" > /dev/null || {
                die 7 "lv active failed!"
            }
        }

        mountDir "${LV_PATH}" "${MOUNT_DIR}"

        die 0 "active successful"
    else
        die 13 "VG_TYPE error , active action only support remote"
    fi
}
main
