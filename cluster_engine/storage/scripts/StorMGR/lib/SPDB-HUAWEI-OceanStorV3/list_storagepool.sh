#!/bin/bash

set -o nounset

INPUT="${1}"

SCRIPTS_DIR="$( readlink -f "$0" )"
SCRIPTS_BASE_DIR="$( dirname "${SCRIPTS_DIR}" )"
declare -r SCRIPTS_BASE_DIR
LIB_BASE_DIR="${SCRIPTS_BASE_DIR%/*}"
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=./function.sh
source "${LIB_BASE_DIR}/_function.sh"

installed jq || die 100 "jq not installed!"

ERROR_DESC=""
ERROR_CODE=0

checkInput () {
    local input="${INPUT}"

    SAN_IP="$( getValueNotNull ".auth_info.ip" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SAN_IP ${SAN_IP}"
        return "${ERROR_CODE}"
    }
    SAN_PORT="$( getValueNotNull ".auth_info.port" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SAN_PORT ${SAN_PORT}"
        return "${ERROR_CODE}"
    }
    SAN_USER="$( getValueNotNull ".auth_info.username" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SAN_USER ${SAN_USER}"
        return "${ERROR_CODE}"
    }
    SAN_PWD="$( getValueNotNull ".auth_info.password" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SAN_PWD ${SAN_PWD}"
        return "${ERROR_CODE}"
    }
    VSTORENAME="$( getValueNull ".auth_info.vstorename" "${input}" )"
    STORAGEPOOL_NAME="$( getValueNull ".data.name" "${input}" )"
    STORAGEPOOL_NAME="${STORAGEPOOL_NAME:-all}"
}

sanLogin () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local san_user="${SAN_USER}"
    local san_pwd="${SAN_PWD}"
    local vstorename="${VSTORENAME}"

    if [[ -n "${vstorename}" ]]; then
        local request_json="{\"username\": \"${san_user}\",\"password\": \"${san_pwd}\",\"vstorename\":\"${vstorename}\",\"scope\": 0}"
    elif [[ -z "${vstorename}" || "${vstorename}" == "null" ]]; then
        local request_json="{\"username\": \"${san_user}\",\"password\": \"${san_pwd}\",\"scope\": 0}"
    fi
    local url="https://${san_ip}:${san_port}/deviceManager/rest/xxxxx/sessions"
    local response
    local cookie_file_temp

    cookie_file_temp="${COOKIE_DIR}/$( mktemp -u cookie.XXXXXX )"
    response="$( curl --cookie-jar "${cookie_file_temp}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request POST --data "${request_json}" --url "${url}" )"

    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        rm -f "${cookie_file_temp}"
        return "${ERROR_CODE}"
    }

    SESSION_DEVICE_ID="$( getValueNotNull ".data.deviceid" "${response}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SESSION_DEVICE_ID ${SESSION_DEVICE_ID}"
        rm -f "${cookie_file_temp}"
        return "${ERROR_CODE}"
    }
    SESSION_IBASETOKEN="$( getValueNotNull ".data.iBaseToken" "${response}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="SESSION_IBASETOKEN ${SESSION_IBASETOKEN}"
        rm -f "${cookie_file_temp}"
        return "${ERROR_CODE}"
    }
    mv "${cookie_file_temp}" "${COOKIE_DIR}/cookie.${SESSION_IBASETOKEN}" || {
        rm -f "${cookie_file_temp}"
        ERROR_CODE=102
        ERROR_DESC="not found cookie file"
        return "${ERROR_CODE}"
    }
}

sanLogout () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/sessions"
    local cookie_file="${COOKIE_DIR}/cookie.${ibasetoken}"
    local response

    response="$(curl --cookie "${cookie_file}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request DELETE --header 'iBaseToken: '"${ibasetoken}"'' --url "${url}")"
    rm -rf "${cookie_file}"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        die "${ERROR_CODE}" "Logout failed : ${ERROR_DESC}"
    }
}

getStoragePoolInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local storagepool_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/storagepool?filter=NAME:${storagepool_name}"
    local response

    response=$( curlGet "${ibasetoken}" "${url}" )

    echo "${response}"
}

getStoragePoolInfoAll () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/storagepool?"
    local response

    response=$( curlGet "${ibasetoken}" "${url}" )

    echo "${response}"
}

formatStoragepoolJson() {
    local storagepool_id="$1"
    local storagepool_name="$2"
    local storagepool_total_capacity_MB="$3"
    local storagepool_free_capacity_MB="$4"
    local storagepool_disk_type="$5"
    local storagepool_health_status="$6"
    local storagepool_running_status="$7"
    local storagepool_description="$8"
    local output="{\"id\": \"${storagepool_id}\",\"name\": \"${storagepool_name}\",\"total_capacity_MB\": ${storagepool_total_capacity_MB},\"free_capacity_MB\": ${storagepool_free_capacity_MB},\"disk_type\": \"${storagepool_disk_type}\",\"health_status\": \"${storagepool_health_status}\",\"running_status\": \"${storagepool_running_status}\",\"description\": \"${storagepool_description}\"}"

    echo "${output}"
}

appendStorageOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ". += [\$append_string]" <<< "${input_string}"
}

main () {
    local response
    local json_output
    # check install curl
    installed curl || die 200 "not found curl"

    # check input
    if ! checkInput ; then
        die "${ERROR_CODE}" "checkInput failed : ${ERROR_DESC}"
    fi

    # login
    if ! sanLogin ; then
        die "${ERROR_CODE}" "login failed : ${ERROR_DESC}"
    fi

    if [[ "${STORAGEPOOL_NAME}" == "all" ]]; then
        response="$( getStoragePoolInfoAll )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getStoragePoolInfoAll : ${ERROR_DESC}"
        }
    else
        response="$( getStoragePoolInfo "${STORAGEPOOL_NAME}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getStoragePoolInfo : ${ERROR_DESC}"
        }
    fi

    json_output="[]"
    local i=0
    while getValueNotNull ".data[${i}]" "${response}" &> /dev/null; do
        local storagepool_name
        local storagepool_id
        local storagepool_lunconfigedcapacity_sector
        local storagepool_free_capacity_MB_sector
        local storagepool_free_capacity_MB
        local storagepool_total_capacity_MB_sector
        local storagepool_total_capacity_MB
        local storagepool_health_status
        local storagepool_running_status
        local storagepool_description
        local smarttier_enable
        local tier0_disk_type
        local tier1_disk_type
        local tier2_disk_type
        local storagepool_disk_type
        local storagepool_output

        storagepool_name="$( getValueNotNull ".data[${i}].NAME" "${response}" )" || {
            storagepool_name=""
        }
        storagepool_id="$( getValueNotNull ".data[${i}].ID" "${response}" )" || {
            storagepool_id=""
        }
        storagepool_lunconfigedcapacity_sector="$( getValueNotNull ".data[${i}].LUNCONFIGEDCAPACITY" "${response}" )" || {
            storagepool_lunconfigedcapacity_sector=0
        }
        storagepool_free_capacity_MB_sector="$( getValueNotNull ".data[${i}].DATASPACE" "${response}" )" || {
            storagepool_free_capacity_MB_sector=0
        }
        storagepool_free_capacity_MB=$( transSectorToMB "${storagepool_free_capacity_MB_sector}" )
        storagepool_total_capacity_MB_sector=$(( storagepool_lunconfigedcapacity_sector + storagepool_free_capacity_MB_sector ))
        storagepool_total_capacity_MB=$( transSectorToMB "${storagepool_total_capacity_MB_sector}" )

        storagepool_health_status="$( getValueNotNull ".data[${i}].HEALTHSTATUS" "${response}" )" || {
            storagepool_id=""
        }
        case ${storagepool_health_status} in
            1) storagepool_health_status="normal" ;;
            2) storagepool_health_status="fault" ;;
            5) storagepool_health_status="leveldown" ;;
            *) storagepool_health_status="" ;;
        esac

        storagepool_running_status="$( getValueNotNull ".data[${i}].RUNNINGSTATUS" "${response}" )" || {
            storagepool_running_status=""
        }
        case ${storagepool_running_status} in
            14) storagepool_running_status="pre-copy" ;;
            16) storagepool_running_status="rebuild" ;;
            27) storagepool_running_status="online" ;;
            28) storagepool_running_status="offline" ;;
            32) storagepool_running_status="balancing" ;;
            53) storagepool_running_status="initialized" ;;
            *) storagepool_running_status="" ;;
        esac

        storagepool_description="$( getValueNotNull ".data[${i}].DESCRIPTION" "${response}" )" || {
            storagepool_description=""
        }

        smarttier_enable="$( getValueNotNull ".data[${i}].ISSMARTTIERENABLE" "${response}" )" || {
            smarttier_enable=""
        }
        tier0_disk_type="$( getValueNotNull ".data[${i}].TIER0DISKTYPE" "${response}" )" || {
            tier0_disk_type=""
        }
        tier1_disk_type="$( getValueNotNull ".data[${i}].TIER1DISKTYPE" "${response}" )" || {
            tier1_disk_type=""
        }
        tier2_disk_type="$( getValueNotNull ".data[${i}].TIER2DISKTYPE" "${response}" )" || {
            tier2_disk_type=""
        }
        if [[ "${smarttier_enable}" == "false" ]]; then
            if [[ "${tier0_disk_type}" -eq 0 && "${tier1_disk_type}" -eq 0 ]]; then
                storagepool_disk_type="sata"
            elif [[ "${tier1_disk_type}" -eq 0 && "${tier2_disk_type}" -eq 0 ]]; then
                storagepool_disk_type="ssd"
            else
                storagepool_disk_type="sas"
            fi
        elif [[ "${smarttier_enable}" == "false" ]]; then
            storagepool_disk_type="mix"
        else
            storagepool_disk_type=""
        fi

        storagepool_output="$(formatStoragepoolJson "${storagepool_id}" "${storagepool_name}" "${storagepool_total_capacity_MB}" "${storagepool_free_capacity_MB}" "${storagepool_disk_type}" "${storagepool_health_status}" "${storagepool_running_status}" "${storagepool_description}")"

        json_output="$(appendStorageOutput "${json_output}" "${storagepool_output}")"
        ((i++))
    done

    jq . <<< "${json_output}"

    sanLogout
}

main
