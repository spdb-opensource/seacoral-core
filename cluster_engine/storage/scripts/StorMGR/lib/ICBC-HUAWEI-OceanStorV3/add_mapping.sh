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
    MAPPINGVIEW_NAME=${LUNGROUP_NAME}
    HOSTGROUP_NAME="$( getValueNotNull ".data.hostgroup_name" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="HOSTGROUP_NAME ${HOSTGROUP_NAME}"
        return "${ERROR_CODE}"
    }
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

createMappingView () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local mappingview_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/mappingview"
    local request_json="{\"NAME\": \"${mappingview_name}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

getHostGroupInfoByMappingView () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local mappingview_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/hostgroup/associate?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=${mappingview_id}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getLunGroupInfoByMappingView() {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local mappingview_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/lungroup/associate?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=${mappingview_id}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getMappingViewInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local mappingview_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/mappingview?filter=NAME:${mappingview_name}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

createMappingViewHostGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_id="${1}"
    local mappingview_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/mappingview/create_associate"
    local request_json="{\"ID\": \"${mappingview_id}\",\"ASSOCIATEOBJTYPE\": 14,\"ASSOCIATEOBJID\": \"${hostgroup_id}\"}"
    local response

    response="$( curlPut "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

createMappingViewLunGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local lungroup_id="${1}"
    local mappingview_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/mappingview/create_associate"
    local request_json="{\"ID\": \"${mappingview_id}\",\"ASSOCIATEOBJTYPE\": 256,\"ASSOCIATEOBJID\": \"${lungroup_id}\"}"
    local response

    response="$( curlPut "${ibasetoken}" "${request_json}" "${url}" )"

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

    #check mappingview is existed or not . If not existed , create mappingview then get mappingview id
    local mappingview_id
    response="$( getMappingViewInfo "${MAPPINGVIEW_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getMappingViewInfo : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        mappingview_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "MAPPINGVIEW_ID ${mappingview_id}"
        }
        echo "mappingview ${MAPPINGVIEW_NAME} is existed , no need to create!"
    else
        response="$( createMappingView "${MAPPINGVIEW_NAME}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "createMappingView : ${ERROR_DESC}"
        }
        mappingview_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "MAPPINGVIEW_ID ${mappingview_id}"
        }
        echo "add mappingview ${MAPPINGVIEW_NAME} success!"
    fi

    #check hostgroup is existed or not . get hostgroup id
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
        die 201 "hostgroup ${HOSTGROUP_NAME} is not existed"
    fi

    #check hostgroup is associated to mappingview or not . match hostgroup id
    local hostgroup_is_mapping
    local hostgroup_id_match
    response="$( getHostGroupInfoByMappingView "${mappingview_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostGroupInfoByMappingView : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        hostgroup_id_match="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "HOSTGROUP_ID_MATCH ${hostgroup_id_match}"
        }
        if [[ "${hostgroup_id}" == "${hostgroup_id_match}" ]]; then
            hostgroup_is_mapping=true
        else
            sanLogout
            die 202 "mappingview associate other hostgroup!"
        fi
    else
        hostgroup_is_mapping=false
    fi

    #check lungroup is existed or not . get lungroup id
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
        die 203 "lungroup ${LUNGROUP_NAME} is not existed"
    fi

    #check lungroup is associate to mappingview or not . match lungroup id
    local lungroup_is_mapping
    local lungroup_id_match
    response="$( getLunGroupInfoByMappingView "${mappingview_id}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getLunGroupInfoByMappingView : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        lungroup_id_match="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "LUNGROUP_ID_MATCH ${lungroup_id_match}"
        }
        if [[ ${lungroup_id} == "${lungroup_id_match}" ]]; then
            lungroup_is_mapping=true
        else
            sanLogout
            die 204 "mappingview associate other lungroup!"
        fi
    else
        lungroup_is_mapping=false
    fi

    #create hostgroup association to mappingview
    if [[ "${hostgroup_is_mapping}" == false ]]; then
        response="$( createMappingViewHostGroupAssociate "${hostgroup_id}" "${mappingview_id}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "createMappingViewHostGroupAssociate : ${ERROR_DESC}"
        }
        echo "add hostgroup ${HOSTGROUP_NAME} to mappingview ${MAPPINGVIEW_NAME} success!"
    else
        echo "hostgroup ${HOSTGROUP_NAME} is mapped!"
    fi

    #create lungroup association to mappinview
    if [[ "${lungroup_is_mapping}" == false ]]; then
        response="$( createMappingViewLunGroupAssociate "${lungroup_id}" "${mappingview_id}" )"
        ERROR_DESC="$(checkResponse "${response}" ".error.code" ".error.description")" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "createMappingViewLunGroupAssociate : ${ERROR_DESC}"
        }
        echo "add lungroup ${LUNGROUP_NAME} to mappingview ${MAPPINGVIEW_NAME} success!"
    else
        echo "lungroup ${LUNGROUP_NAME} is mapped!"
    fi

    sanLogout
    echo "add mapping done"
}

main
