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

main() {
    if [[ "${VG_TYPE}" == "remote" ]]; then
        grep "${MOUNT_DIR}" "/proc/mounts" &> /dev/null && {
            umount "${MOUNT_DIR}" > /dev/null || {
                die 3 "umount mount_dir failed!"
            }
        }

        test -d "${MOUNT_DIR}" && {
            rm -rf "${MOUNT_DIR}" > /dev/null || {
                die 4 "remove mount_dir failed!"
            }
        }

        vgdisplay "${VG_NAME}" &> /dev/null && {
            lvdisplay -C -o "lv_active" --noheadings "${LV_PATH}" | grep -w "active" &> /dev/null && {
                lvchange -an "${LV_PATH}" > /dev/null || {
                    die 5 "lv deactive failed!"
                }
            }

            sleep 2

            vgdisplay "${VG_NAME}" -C -o "vg_exported" --noheadings | grep -w "exported" &> /dev/null || {
                vgexport -y "${VG_NAME}" > /dev/null || {
                    die 6 "vg export failed!"
                }
            }
        }

        die 0 "deactive successful"
    else
        die 7 "VG_TYPE error , deactive action only support remote"
    fi
}
main
