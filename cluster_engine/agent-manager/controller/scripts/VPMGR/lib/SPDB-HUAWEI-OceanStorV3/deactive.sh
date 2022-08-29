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

MOUNT_DIR="$(getValueNotNull ".mount_dir" "${INPUT}")" || die $? "${MOUNT_DIR}"
LV_NAME="$(getValueNotNull ".lv.name" "${INPUT}")" || die $? "${LV_NAME}"
VG_NAME="$(getValueNotNull ".vg.name" "${INPUT}")" || die $? "${VG_NAME}"
LV_PATH="/dev/${VG_NAME}/${LV_NAME}"
VG_TYPE="$(getValueNotNull ".vg.type" "${INPUT}")" || die $? "${VG_TYPE}"

IFS=" " read -r -a WWN_IDS <<< "$( getJsonArrNotNull ".vg.LUN_ID" "${INPUT}" )" || die $? "WWN_IDS ${WWN_IDS[*]}"

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
            lvdisplay -C -o "lv_active" --noheading "${LV_PATH}" | grep -w "active" &> /dev/null && {
                blockdev --flushbufs "${LV_PATH}" > /dev/null || {
                    die 5 "blockdev --flushbufs failed!"
                }

                lvchange -an "${LV_PATH}" > /dev/null || {
                    die 6 "lv deactive failed!"
                }
            }

            sleep 2

            vgdisplay "${VG_NAME}" -C -o "vg_exported" --noheadings | grep -w "exported" &> /dev/null || {
                vgexport -y "${VG_NAME}" > /dev/null || {
                    die 7 "vg export failed!"
                }
            }

            #remove scsi
            for wwn in ${WWN_IDS[*]}; do
                local multipath
                multipath="$( lsscsi -i | grep HUAWEI | grep "${wwn}" | awk '${print $NF}' | uniq )"
                if [[ -n "${multipath}" ]]; then
                    while read -r name; do
                        echo "scsi remove-single-device ${name}" > /proc/scsi/scsi || return 10
                    done <<< "$( lsscsi -i | grep HUAWEI | grep "${wwn}" | awk '{print $1}' | tr -d [] | sed 's/\:/\ /g' )"
                    multipath -f "${wwn}" || return 11
                fi
            done

            # flush vg info
            vgimport "${VG_NAME}" &> /dev/null
        }

        die 0 "deactive successful"
    else
        die 8 "VG_TYPE error , deactive action only support remote"
    fi
}
main
