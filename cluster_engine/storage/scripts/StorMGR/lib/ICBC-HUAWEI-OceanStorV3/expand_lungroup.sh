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

parseLunAllocType () {
    local input="${1}"
    local code
    case "${input}" in
        "thick") code=0 ;;
        "thin") code=1 ;;
        *) return 100 ;;
    esac

    echo "${code}"
}

checkInput () {
    local input="${INPUT}"
    local lun_name
    local lun_capacity
    local storagepool_name

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
    LUNGROUP_NAME="$( getValueNotNull ".data.name" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="LUNGROUP_NAME ${LUNGROUP_NAME}"
        return "${ERROR_CODE}"
    }
    ALLOC_TYPE="$( getValueNotNull ".data.alloc_type" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="ALLOC_TYPE ${ALLOC_TYPE}"
        return "${ERROR_CODE}"
    }
    ALLOC_TYPE="$( parseLunAllocType "${ALLOC_TYPE}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="parse ALLOC_TYPE failed: only support thick and thin"
        return "${ERROR_CODE}"
    }
    local i=0
    while getValueNotNull ".data.luns[${i}]" "${input}" &> /dev/null; do
        lun_name="$( getValueNotNull ".data.luns[${i}].name" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="LUN_NAME ${lun_name}"
            break
        }
        lun_capacity="$( getValueNotNull ".data.luns[${i}].capacity_MB" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="LUN_CAPACITY ${lun_capacity}"
            break
        }
        if [[ "${lun_capacity}" -le 0 ]]; then
            ERROR_CODE=101
            ERROR_DESC="lun capacity can't lower equal than 0"
            break
        fi
        storagepool_name="$( getValueNotNull ".data.luns[${i}].storagepool_name" "${input}" )"|| {
            ERROR_CODE=$?
            ERROR_DESC="STORAGEPOOL_NAME ${storagepool_name}"
            break
        }
        ((i++))
    done
    return "${ERROR_CODE}"
}

sanLogin () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local san_user="${SAN_USER}"
    local san_pwd="${SAN_PWD}"

    local request_json="{\"username\": \"${san_user}\",\"password\": \"${san_pwd}\",\"scope\": 0}"
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

createLun () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lun_name="${1}"
    local storage_pool_id="${2}"
    local lun_capacity="${3}"
    local alloc_type="${4}"
    local lun_description="${5}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lun"
    local request_json="{\"NAME\": \"${lun_name}\",\"PARENTID\": \"${storage_pool_id}\",\"CAPACITY\": \"${lun_capacity}\",\"ALLOCTYPE\": \"${alloc_type}\",\"DESCRIPTION\": \"${lun_description}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

createLunGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lungroup_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup"
    local request_json="{\"NAME\": \"${lungroup_name}\",\"APPTYPE\": 0}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

getLunInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lun_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lun?filter=NAME:${lun_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getLunGroupInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lungroup_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup?filter=NAME:${lungroup_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getStoragePoolInfo() {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local storage_pool_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/storagepool?filter=NAME:${storage_pool_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

createLunGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lun_id="${1}"
    local lungroup_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup/associate"
    local request_json="{\"ID\": \"${lungroup_id}\",\"ASSOCIATEOBJTYPE\": 11,\"ASSOCIATEOBJID\": \"${lun_id}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

main () {
    local response

    # check install curl
    installed curl || die 200 "not found curl"

    # check input
    if ! checkInput ; then
        die "${ERROR_CODE}" "checkInput failed : ${ERROR_DESC}"
    fi

    # login
    if ! sanLogin ; then
        die "${ERROR_CODE}" "login failed : ${ERROR_DESC}"
    else
        echo "login success!"
    fi

    #check lungroup . If not existed , create lungroup
    response="$( getLunGroupInfo "${LUNGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getLunGroupInfo : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        echo "lungroup ${LUNGROUP_NAME} is existed , no need to create!"
    else
        response="$( createLunGroup "${LUNGROUP_NAME}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "createLunGroup : ${ERROR_DESC}"
        }
        echo "add lungroup ${LUNGROUP_NAME} success!"
    fi

    # get lungroup id
    local lungroup_id
    response="$( getLunGroupInfo "${LUNGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getLunGroupInfo : ${ERROR_DESC}"
    }
    lungroup_id="$(getValueNotNull ".data[0].ID" "${response}")" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "LUNGROUP_ID ${lungroup_id}"
    }

    local i=0
    while getValueNotNull ".data.luns[${i}]" "${INPUT}" &> /dev/null; do
        # parse input to get lun info
        local lun_name
        local lun_capacity
        local storagepool_name
        local lun_description

        lun_name="$( getValueNotNull ".data.luns[${i}].name" "${INPUT}" )"
        lun_capacity="$( getValueNotNull ".data.luns[${i}].capacity_MB" "${INPUT}" )"
        lun_capacity=$( transMBToSector "${lun_capacity}" )
        storagepool_name="$( getValueNotNull ".data.luns[${i}].storagepool_name" "${INPUT}" )"
        lun_description="$( getValueNull ".data.luns[${i}].description" "${INPUT}" )"
        lun_description="${lun_description:-"default"}"

        #check storagepool is existed or not . If existed , get storagepool id
        response="$( getStoragePoolInfo "${storagepool_name}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getStoragePoolInfo : ${ERROR_DESC}"
        }
        if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
            local storagepool_id
            storagepool_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "STORAGEPOOL_ID ${storagepool_id}"
            }
        else
            sanLogout
            die 201 "STORAGEPOOL ${storagepool_name} is not existed, please check input is correct or not"
        fi

        #check lun and create lun
        response="$( getLunInfo "${lun_name}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getLunInfo : ${ERROR_DESC}"
        }
        if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
            echo "lun ${lun_name} is existed , no need to create"
        else
            response="$( createLun "${lun_name}" "${storagepool_id}" "${lun_capacity}" "${ALLOC_TYPE}" "${lun_description}" )"
            ERROR_DESC="$(checkResponse "${response}" ".error.code" ".error.description")" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "createHost : ${ERROR_DESC}"
            }
            echo "add lun ${lun_name} success!"
        fi

        # get lun id and is_associate
        local lun_id
        local is_associate
        response="$( getLunInfo "${lun_name}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getHostInfo : ${ERROR_DESC}"
        }
        lun_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "LUN_ID ${lun_id}"
        }
        is_associate="$( getValueNotNull ".data[0].ISADD2LUNGROUP" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "IS_ASSOCIATE ${is_associate}"
        }

        # check and create LunGroup Associate
        if [[ "${is_associate}" == "true" ]]; then
            echo "lun ${lun_name} is associated！"
        elif [[ "${is_associate}" == "false" ]]; then
            response="$( createLunGroupAssociate "${lun_id}" "${lungroup_id}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "createLunGroupAssociate : ${ERROR_DESC}"
            }
            echo "add lun ${lun_name} to lungroup ${LUNGROUP_NAME} success!"
        else
            sanLogout
            die 204 "getHostInfo failed: is_associate type is not boolean!"
        fi

        ((i++))
    done

    # finish and logout
    sanLogout
    echo "add lungroup done"
}

main
