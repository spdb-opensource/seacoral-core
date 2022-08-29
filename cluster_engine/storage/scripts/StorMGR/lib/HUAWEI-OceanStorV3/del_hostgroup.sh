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
    local host_name

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
    HOSTGROUP_NAME="$( getValueNotNull ".data.name" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="HOSTGROUP_NAME ${HOSTGROUP_NAME}"
        return "${ERROR_CODE}"
    }
    VSTORENAME="$( getValueNull ".auth_info.vstorename" "${input}" )"
    return "${ERROR_CODE}"
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

getHostInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local host_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/host?filter=NAME:${host_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getHostGroupInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/hostgroup?filter=NAME:${hostgroup_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getHostInfoByHostGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/host/associate?ASSOCIATEOBJTYPE=14&ASSOCIATEOBJID=${hostgroup_id}"
    local response

    response="$(curlGet "${ibasetoken}" "${url}")"

    echo "${response}"
}

deleteHostGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/hostgroup/${hostgroup_id}"
    local response

    response="$( curlDelete "${ibasetoken}" "${url}" )"

    echo "${response}"
}

deleteHostGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local host_id="$1"
    local hostgroup_id="$2"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/host/associate?ID=${hostgroup_id}&ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=${host_id}"
    local response

    response="$( curlDelete "${ibasetoken}" "${url}" )"

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

    # check hostgroup . If existed , get hostgroup id
    local hostgroup_id
    response="$( getHostGroupInfo "${HOSTGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostGroupInfo : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        hostgroup_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "HOSTGROUP_ID ${hostgroup_id}"
        }
    else
        sanLogout
        die 0 "hostgroup ${HOSTGROUP_NAME} is not existed , no need to delete!"
    fi

    #check Hostgroup is associated or not . If is associated , get host id array and delete Hostgroup Associate
    response="$( getHostInfoByHostGroup "${hostgroup_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostInfoByHostGroup : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        local i=0
        while getValueNotNull ".data[${i}]" "${response}" &> /dev/null; do
            local host_id
            host_id="$( getValueNotNull ".data[${i}].ID" "${response}" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "HOST_ID ${host_id}"
            }

            response="$( deleteHostGroupAssociate "${host_id}" "${hostgroup_id}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "deleteHostGroupAssociate : ${ERROR_DESC}"
            }
            echo "delete host ${host_id} to hostgroup ${HOSTGROUP_NAME} success!"
            ((i++))
        done
    else
        echo "no host associated to hostgroup ${HOSTGROUP_NAME}"
    fi

    #delete Hostgroup
    response="$( deleteHostGroup "${hostgroup_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "deleteHostGroup : ${ERROR_DESC}"
    }
    echo "delete hostgroup ${HOSTGROUP_NAME} success!"

    sanLogout
    echo "delete hostgroup done"
}

main
