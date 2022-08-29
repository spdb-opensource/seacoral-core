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

FS_TYPE="$(getValueNotNull ".fs_type" "${INPUT}")" || die $? "${FS_TYPE}"
MOUNT_DIR="$(getValueNotNull ".mount_dir" "${INPUT}")" || die $? "${MOUNT_DIR}"
LV_NAME="$(getValueNotNull ".lv.name" "${INPUT}")" || die $? "${LV_NAME}"
VG_NAME="$(getValueNotNull ".vg.name" "${INPUT}")" || die $? "${VG_NAME}"
LV_PATH="/dev/${VG_NAME}/${LV_NAME}"
VG_TYPE="$(getValueNotNull ".vg.type" "${INPUT}")" || die $? "${VG_TYPE}"
SIZE_MB="$(getValueNotNull ".size_MB" "${INPUT}")" || die $? "${SIZE_MB}"
UNITS="m"
IFS=" " read -r -a LUN_IDS <<< "$(getJsonArrNotNull ".vg.add_LUN_ID" "${INPUT}")"

expandFS() {
    local lv_path="${1}"
    local fs_type="${2}"

    case ${fs_type} in
        ext4)
            resize2fs "${lv_path}" > /dev/null || {
                die 8 "${fs_type} extend failed!"
            }
            ;;
        xfs)
            xfs_growfs "${lv_path}" > /dev/null || {
                die 8 "${fs_type} extend failed!"
            }
            ;;
        *)
            die 8 "filesystem type(${fs_type}) not support!"
            ;;
    esac
}

scanDevice() {
    #shellcheck disable=SC2045
    for fc_host in $( ls /sys/class/fc_host/ ); do
        #echo 1 > /sys/class/fc_host/${fc_host}/issue_lip
        echo '- - -' > "/sys/class/scsi_host/${fc_host}/scan"
    done
}

main() {
    vgdisplay "${VG_NAME}" &> /dev/null || {
        die 3 "vg ${VG_NAME} is not existed"
    }

    lvdisplay "${LV_PATH}" &> /dev/null || {
        die 4 "lv ${LV_PATH} is not existed"
    }

    if [[ "${VG_TYPE}" == "remote" ]]; then
        installed lsscsi || {
            die 4 "lsscsi not installed!"
        }

        scanDevice &> /dev/null

        local pv_list=""
        for lun_id in ${LUN_IDS[*]}; do
            local disk_name
            # TODO use lun WWN replace DEV lun id
            disk_name="$(lsscsi -i | grep HUAWEI | grep "${lun_id}" | awk '{print $NF}' | uniq 2> /dev/null)"

            local interval=10
            local time=3
            for i in $(seq ${interval}); do
                test -z "${disk_name}" && {
                    scanDevice &> /dev/null || {
                        sleep "${time}"
                    }

                    disk_name="$(lsscsi -i | grep HUAWEI | grep "${lun_id}" | awk '{print $NF}' | uniq 2> /dev/null)"

                    ((i++))

                    if [[ ${i} -gt ${interval} ]]; then
                        die 101 "scan timeout!"
                    fi
                    continue
                }
                break
            done

            local pv_name="/dev/mapper/${disk_name}"
            grep -w "${pv_name}" <<< "$( vgdisplay -C -o "pv_name" --noheadings "${VG_NAME}" 2> /dev/null )" &> /dev/null || {
                # clean lun meta info
                pvremove -y -ff "${pv_name}" &> /dev/null
                dd if=/dev/zero of="${pv_name}" bs=1024 count=1024 &> /dev/null
                pv_list="${pv_list} ${pv_name}"
            }
        done

        test -n "${pv_list}" && {
            eval "vgextend ${VG_NAME} ${pv_list}" > /dev/null || {
                die 6 "vgextend failed!"
            }
        }

        sleep 2

        if [[ $(lvdisplay "${LV_PATH}" -C -o "lv_size" --noheadings --units "${UNITS}" | sed 's/ //g' | awk -F. '{print $1}') -lt "${SIZE_MB}" ]]; then
            lvextend -l 100%VG "${LV_PATH}" > /dev/null || {
                die 7 "lvextend failed!"
            }
        fi

        expandFS "${LV_PATH}" "${FS_TYPE}"

        die 0 "expand successful"

    else
        die 10 "VG_TYPE error, only support local and remote"
    fi
}

main
