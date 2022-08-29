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
    LUNGROUP_NAME="$( getValueNotNull ".data.name" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="LUNGROUP_NAME ${LUNGROUP_NAME}"
        return "${ERROR_CODE}"
    }
    VSTORENAME="$( getValueNull ".auth_info.vstorename" "${input}" )"
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

getLunInfoByLunGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lungroup_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lun/associate?ASSOCIATEOBJTYPE=256&ASSOCIATEOBJID=${lungroup_id}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

deleteLunGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lungroup_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup/${lungroup_id}"
    local response

    response="$( curlDelete "${ibasetoken}" "${url}" )"

    echo "${response}"
}

deleteLun () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lun_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lun/${lun_id}"
    local response

    response="$(curlDelete "${ibasetoken}" "${url}")"

    echo "${response}"
}

deleteLunGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lun_id="${1}"
    local lungroup_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup/associate?ID=${lungroup_id}&ASSOCIATEOBJTYPE=11&ASSOCIATEOBJID=${lun_id}"
    local response

    response="$(curlDelete "${ibasetoken}" "${url}")"

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

    #check lungroup . If existed , delete lungroup
    local lungroup_id
    response="$( getLunGroupInfo "${LUNGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getLunGroupInfo : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        lungroup_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "LUNGROUP_ID ${lungroup_id}"
        }
    else
        sanLogout
        die 0 "lungroup ${LUNGROUP_NAME} is not existed , no need to delete"
    fi

    #check lungroup is associated or not . If is associated , get lun id array and delete Lungroup Associate
    response="$( getLunInfoByLunGroup "${lungroup_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getLunInfoByLunGroup : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        local i=0
        while getValueNotNull ".data[${i}]" "${response}" &> /dev/null; do
            local lun_id
            lun_id="$( getValueNotNull ".data[${i}].ID" "${response}" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "LUN_ID ${lun_id}"
            }

            response="$( deleteLunGroupAssociate "${lun_id}" "${lungroup_id}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "deleteLunGroupAssociate : ${ERROR_DESC}"
            }
            echo "delete lun ${lun_id} to lungroup ${LUNGROUP_NAME} success!"

            #delete lun
            response="$( deleteLun "${lun_id}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "deleteLun : ${ERROR_DESC}"
            }
            echo "delete lun ${lun_id} success!"
            ((i++))
        done
    else
        echo "no lun associated to lungroup ${LUNGROUP_NAME}"
    fi

    response="$( deleteLunGroup "${lungroup_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "deleteLun : ${ERROR_DESC}"
    }
    echo "delete lungroup ${LUNGROUP_NAME} success!"

    sanLogout
    echo "delete lungroup done"
}

main
