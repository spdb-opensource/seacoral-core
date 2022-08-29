#!/bin/bash

set -o nounset
# ##############################################################################
# Globals, settings
# ##############################################################################
LANG=C

FILE_NAME="local"
LOG_FILE="/var/log/cluster_agent/VPMGR.log"
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

    echo "[${timestamp}] ERR[${function_name}]: $* ;" | tee -a "${LOG_FILE}"
}

info () {
    local function_name="${1}"
    shift
    local timestamp
    timestamp="$( date +"%Y-%m-%d %T %N" )"

    echo "[${timestamp}] INFO[${function_name}]: $* ;" | tee -a "${LOG_FILE}"
}

installed () {
    command -v "$1" >/dev/null 2>&1
}

add_field () {
    local func_name="${FILE_NAME}.add_field"

    local value_type="${1}"
    local key="${2}"
    local value="${3}"
    local input="${4}"

    case "${value_type}" in
        "json_string")
            jq  ". +{\"${key}\": ${value}}" <<< "${input}" 2> /dev/null
            ;;
        "string")
            jq  ". +{\"${key}\": \"${value}\"}" <<< "${input}" 2> /dev/null
            ;;
        "int64")
            jq  ". +{\"${key}\": ${value}}" <<< "${input}" 2> /dev/null
            ;;
        "float64")
            # check int,if value is int , convert type to float64
            [[ "${value}" =~ ^[0-9]*$ ]] && value="${value}.0"

            jq  ". +{\"${key}\": ${value}}" <<< "${input}" 2> /dev/null
            ;;
        *)
            error "${func_name}" "value_type nonsupport"
            return 2
            ;;
    esac
}

check_value_is_exist () {
    local key="${1}"
    local input="${2}"
    local output

    output="$( jq --raw-output -c "${key}" 2> /dev/null <<< "${input}" )"

    [[ "${output}" != "null" ]] || return 2
}

get_value_not_null () {
    local func_name="${FILE_NAME}.get_value_not_null"

    local key="${1}"
    local input="${2}"

    local output

    check_value_is_exist "${key}" "${input}" || {
        error "${func_name}" "check value is not exist"
        return 2
    }

    output="$( jq --raw-output -c "${key}" <<< "${input}" 2> /dev/null )"

    [[ -n "${output}" ]] || {
        error "${func_name}" "the length of value is zero"
        return 2
    }

    [[ "${output}" != "null" ]] || {
        error "${func_name}" "value equal to \"null\""
        return 2
    }

    echo "${output}"
}
# ##############################################################################
# action function
# action function can use function( die ) and exit
# ##############################################################################
create_lv () {
    local func_name="${FILE_NAME}.create_lv"

    local vg_name="${1}"
    local lv_name="${2}"
    local lvm_path="${3}"
    local size="${4}"

    vgdisplay "${vg_name}" &> /dev/null || {
        die 21 "vg ${vg_name} is not existed!"
    }

    lvdisplay "${lvm_path}" &> /dev/null || {
        lvcreate -W y -y -L "${size}m" -n "${lv_name}" "${vg_name}" > /dev/null || {
            die 22 "${func_name}" "lvcreate ${lv_name} failed!"
        }
    }

    if [[ $(lvdisplay "${lvm_path}" -C -o "lv_size" --noheadings --units "m" | sed 's/ //g' | awk -F. '{print $1}') -ne "${size}" ]]; then
        die 23 "lv size not match!"
    fi
}

expand_lv () {
    local func_name="${FILE_NAME}.expand_lv"

    local vg_name="${1}"
    local lv_name="${2}"
    local lv_dm_path="${3}"
    local size="${4}"

    vgdisplay "${vg_name}" &> /dev/null || {
        die 31 "vg ${vg_name} is not existed!"
    }

    lvdisplay "${lv_dm_path}" &> /dev/null || {
        die 32 "lv ${lv_dm_path} is not existed"
    }

    if [[ $(lvdisplay "${lv_dm_path}" -C -o "lv_size" --noheadings --units "m" | sed 's/ //g' | awk -F. '{print $1}') -lt "${size}" ]]; then
        lvextend -L "${size}m" -n "${lv_dm_path}" > /dev/null || {
            die 33 "lvextend ${lv_dm_path} to ${size} failed!"
        }
    fi
}

delete_lv () {
    local func_name="${FILE_NAME}.delete_lv"

    local lv_dm_path="${1}"

    lvdisplay "${lv_dm_path}" &> /dev/null && {
        blockdev --flushbufs "${lv_dm_path}" > /dev/null || {
            die 41 "blockdev --flushbufs failed!"
        }

        dmsetup remove "${lv_dm_path}" > /dev/null || {
            die 42 "dmsetup remove failed!"
        }

        lvremove -f "${lv_dm_path}" > /dev/null || {
            die 43 "lvremove failed!"
        }
    }
}

make_filesystem () {
    local func_name="${FILE_NAME}.make_filesystem"

    local lvm_path="${1}"
    local fs_type="${2}"

    if blkid "${lvm_path}" &> /dev/null; then
        if [[ $(blkid "${lvm_path}" -o export | awk -F '=' '/TYPE=/{print $2}') != "${fs_type}" ]]; then
            die 51 "${func_name}" "lvm ${lvm_path} filesystem type is not (${fs_type})!"
        fi
    else
        case "${fs_type}" in
            "ext4")
                mkfs.ext4 -E nodiscard "${lvm_path}" &> /dev/null || {
                    die 52 "mkfs.ext4 ${lvm_path} failed!"
                }
                ;;

            "xfs")
                mkfs.xfs -K "${lvm_path}" &> /dev/null || {
                    die 52 "mkfs.xfs ${lvm_path} failed!"
                }
                ;;
            *)
                die 53 "filesystem type(${fs_type}) not support!"
                ;;
        esac
    fi
}

expand_filesystem  () {
    local func_name="${FILE_NAME}.expand_filesystem"

    local lvm_path="${1}"
    local fs_type="${2}"

    case ${fs_type} in
        "ext4")
            resize2fs "${lvm_path}" > /dev/null || {
                die 61 "resize2fs ${lvm_path} failed!"
            }
            ;;
        "xfs")
            xfs_growfs "${lvm_path}" > /dev/null || {
                die 61 "xfs_growfs ${lvm_path} failed!"
            }
            ;;
        *)
            die 62 "filesystem type(${fs_type}) not support!"
            ;;
    esac
}
# ##############################################################################
# The main() function is called at the action function.
# ##############################################################################
main(){
    local func_name="${FILE_NAME}.main"

    installed jq || die 101 "${func_name}" "not found jq!"

    local option="${1}"
    local input="${2}"

    local vg_type
    vg_type="$( get_value_not_null ".vg.type" "${input}" )" || {
        die 102 "${func_name}" "get .vg.type failed!"
    }

    if [[ "${vg_type}" != "local" ]]; then
        die 103 "${func_name}" "vg type is not local!"
    fi

    local vg_name
    vg_name="$( get_value_not_null ".vg.name" "${input}" )" || {
        die 104 "${func_name}" "get .vg.name failed!"
    }

    local lv_name
    lv_name="$( get_value_not_null ".lv.name" "${input}" )" || {
        die 105 "${func_name}" "get .lv.name failed!"
    }

    local size
    size="$( get_value_not_null ".size_MB" "${input}" )" || {
        die 106 "${func_name}" "get .size failed!"
    }

    local fs_type
    fs_type="$( get_value_not_null ".fs_type" "${input}" )" || {
        die 107 "${func_name}" "get .fs_type failed!"
    }

    local lvm_path="/dev/${vg_name}/${lv_name}"

    case "${option}" in
        "add")
            create_lv "${vg_name}" "${lv_name}" "${lvm_path}" "${size}"

            local lv_dm_path
            lv_dm_path="$(lvdisplay -C -o "lv_dm_path" --noheadings "${lvm_path}" 2> /dev/null)" || {
                die 108 "get lv_dm_path failed!"
            }
            lv_dm_path="$(sed 's/ //g' <<< "${lv_dm_path}")"

            make_filesystem "${lvm_path}" "${fs_type}"

            jq . <<< "{\"mounter\":\"\",\"device\": \"${lv_dm_path}\"}"
            ;;
        "delete")
            delete_lv "${lvm_path}"
            ;;
        "expand")
            expand_lv "${vg_name}" "${lv_name}" "${lvm_path}" "${size}"

            expand_filesystem "${lvm_path}" "${fs_type}"
            ;;
        *)
            die 106 "${func_name}" "option(${option}) nonsupport"
            ;;
    esac

    exit 0
}

main "${@:-""}"

# ##############################################################################
# Documentation
# ##############################################################################
:<<'DOCUMENTATION'
================================================================================
local.sh add {{json_string}}
input json examle:
{
  "fs_type": "xfs",
  "size_MB": 100,
  "lv": {
    "name": "lvdata1"
  },
  "vg": {
    "name": "vgdata1",
    "type": "local",
    "vendor": "local",
    "initiator_type": "",
    "LUN_ID": [
      ""
    ]
  }
}
output json example:
{
  "device": "vgdata1/vgdata1"
}
================================================================================
local.sh delete {{json_string}}
input json examle:
{
  "fs_type": "xfs",
  "size_MB": 100,
  "lv": {
    "name": "lvdata1"
  },
  "vg": {
    "name": "vgdata1",
    "type": "local",
    "vendor": "local",
    "initiator_type": "",
    "LUN_ID": [
      ""
    ]
  }
}
================================================================================
local.sh expand {{json_string}}
input json examle:
{
  "fs_type": "xfs",
  "size_MB": 200,
  "lv": {
    "name": "lvdata1"
  },
  "vg": {
    "name": "vgdata1",
    "type": "local",
    "vendor": "local",
    "initiator_type": "",
    "LUN_ID": [
      ""
    ]
  }
}
DOCUMENTATION
