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

FS_TYPE="$( getValueNotNull ".fs_type" "${INPUT}" )" || die $? "FS_TYPE ${FS_TYPE}"
MOUNT_DIR="$( getValueNotNull ".mount_dir" "${INPUT}" )" || die $? "MOUNT_DIR ${MOUNT_DIR}"
LV_NAME="$( getValueNotNull ".lv.name" "${INPUT}" )" || die $? "LV_NAME ${LV_NAME}"
VG_NAME="$( getValueNotNull ".vg.name" "${INPUT}")" || die $? "VG_NAME ${VG_NAME}"
LV_PATH="/dev/${VG_NAME}/${LV_NAME}"
VG_TYPE="$( getValueNotNull ".vg.type" "${INPUT}" )" || die $? "VG_TYPE ${VG_TYPE}"
IFS=" " read -r -a LUN_IDS <<< "$( getJsonArrNotNull ".vg.LUN_ID" "${INPUT}" )" || die $? "LUN_IDS ${LUN_IDS[*]}"

makeFS() {
    local lv_path="${1}"
    local fs_type="${2}"

    if blkid "${lv_path}" &> /dev/null; then
        if [[ $(blkid "${lv_path}" -o export | awk -F '=' '/TYPE=/{print $2}') != "${fs_type}" ]]; then
            die 10 "lv filesystem type is not (${fs_type})!"
        fi
    else
        case "${fs_type}" in
            ext4)
                mkfs.ext4 "${lv_path}" &> /dev/null || {
                    die 10 "initialize failed!"
                }
                ;;

            xfs)
                mkfs.xfs "${lv_path}" &> /dev/null || {
                    die 10 "initialize failed!"
                }
                ;;

            *)
                die 10 "filesystem type(${fs_type}) not support!"
                ;;

        esac
    fi
}

mountDir() {
    local lv_path="${1}"
    local mount_dir="${2}"

    local lv_dm_path
    lv_dm_path="$(lvdisplay -C -o "lv_dm_path" --noheadings "${lv_path}" 2> /dev/null)" || die 6 "get lv_dm_path failed!"
    lv_dm_path="$(sed 's/ //g' <<< "${lv_dm_path}")"

    test -d "${mount_dir}" || {
        mkdir -p "${mount_dir}" > /dev/null || {
            die 11 "mkdir failed!"
        }
    }

    grep "${lv_dm_path}" "/proc/mounts" | grep "${mount_dir}" &> /dev/null || {
        mount "${lv_dm_path}" "${mount_dir}" > /dev/null || {
            die 12 "mount failed!"
        }
    }
}

main() {
    if [[ "${VG_TYPE}" == "remote" ]]; then
        installed upadm || {
            die 5 "upadm not installed!"
        }

        installed upadmin || {
            die 5 "upadmin not installed!"
        }

        upadm start hotscan &> /dev/null

        local pv_list=""
        for lun_id in ${LUN_IDS[*]}; do
            local disk_name
            # TODO use lun WWN replace DEV lun id
            disk_name="$(upadmin show vlun | awk "\$9 == ${lun_id} {print \$2}" 2> /dev/null)"

            local interval=10
            local time=3
            for i in $(seq ${interval}); do
                test -z "${disk_name}" && {
                    upadm start hotscan &> /dev/null || {
                        sleep "${time}"
                    }

                    disk_name="$(upadmin show vlun | awk "\$9 == ${lun_id} {print \$2}" 2> /dev/null)"

                    ((i++))

                    if [[ ${i} -gt ${interval} ]]; then
                        die 7 "scan timeout!"
                    fi
                    continue
                }
            done

            local pv_name="/dev/${disk_name}"
            grep -w "${pv_name}" <<< "$( vgdisplay -C -o "pv_name" --noheadings "${VG_NAME}" 2> /dev/null )" &> /dev/null || {
                pv_list="${pv_list} ${pv_name}"
            }
        done

        test -n "${pv_list}" && {
            eval "vgcreate -y ${VG_NAME} ${pv_list}" > /dev/null || {
                die 8 "vgcreate failed!"
            }
        }

        lvdisplay "${LV_PATH}" &> /dev/null || {
            lvcreate -W y -y -l 100%FREE -n "${LV_NAME}" "${VG_NAME}" > /dev/null || {
                die 9 "lvcreate failed!"
            }
        }

        makeFS "${LV_PATH}" "${FS_TYPE}"

        mountDir "${LV_PATH}" "${MOUNT_DIR}"

        local json_output="{\"mounter\": \"${MOUNT_DIR}\",\"device\": \"${VG_NAME}/${LV_NAME}\"}"

        jq . <<< "${json_output}"

    else
        die 13 "VG_TYPE error , only support local and remote"
    fi
}

main
