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

parseHostOSType () {
    local input="${1}"
    local code
    case "${input}" in
        "Linux") code=0 ;;
        "Windows") code=1 ;;
        "Solaris") code=2 ;;
        "HP-UX") code=3 ;;
        "AIX") code=4 ;;
        "XenServer") code=5 ;;
        "Mac OS") code=6 ;;
        "VMware ESX") code=7 ;;
        "LINUX_VIS") code=8 ;;
        "Windows Server 2012") code=9 ;;
        "Oracle VM") code=10 ;;
        "OpenVMS") code=11 ;;
        *) return 100 ;;
    esac

    echo "${code}"
}

checkInput () {
    local input="${INPUT}"
    local host_name
    local host_ip
    local os_type
    local initiator_type
    local initiator_id

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
    local i=0
    while getValueNotNull ".data.hosts[${i}]" "${input}" &> /dev/null; do
        host_name="$( getValueNotNull ".data.hosts[${i}].name" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="HOST_NAME ${host_name}"
            break
        }
        host_ip="$( getValueNotNull ".data.hosts[${i}].ip" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="HOST_IP ${host_ip}"
            break
        }
        os_type="$( getValueNotNull ".data.hosts[${i}].os_type" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="HOST_OS_TYPE ${os_type}"
            break
        }
        os_type="$( parseHostOSType "${os_type}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="parse HOST_OS_TYPE failed!"
            break
        }
        initiator_type="$( getValueNotNull ".data.hosts[${i}].initiator.type" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="INITIATOR_TYPE ${initiator_type}"
            break
        }
        if [[ ${initiator_type} != "FC" ]]; then
            ERROR_CODE=101
            ERROR_DESC="parse INITIATOR_TYPE failed: only support FC"
            break
        fi
        host_initiator_ids="$( getJsonArrNotNull ".data.hosts[$i].initiator.id" "${input}" )" || {
            ERROR_CODE=$?
            ERROR_DESC="INITIATOR_ID ${host_initiator_ids}"
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

createHost () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local host_name="${1}"
    local host_description="${2}"
    local host_location="${3}"
    local host_network_name="${4}"
    local host_ip="${5}"
    local host_os_type="${6}"
    local host_model="${7}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/host"
    local request_json="{\"NAME\": \"${host_name}\",\"DESCRIPTION\": \"${host_description}\",\"LOCATION\": \"${host_location}\",\"OPERATIONSYSTEM\": \"${host_os_type}\",\"NETWORKNAME\": \"${host_network_name}\",\"IP\": \"${host_ip}\",\"MODEL\": \"${host_model}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

createInitiatorAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local initiator_id="${1}"
    local host_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/fc_initiator/${initiator_id}"
    local request_json="{\"ID\": \"${initiator_id}\",\"PARENTTYPE\": 21,\"PARENTID\": \"${host_id}\"}"
    local response

    response="$( curlPut "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

createHostGroup () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_name="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/hostgroup"
    local request_json="{\"NAME\": \"${hostgroup_name}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
}

createHostGroupAssociate () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local host_id="${1}"
    local hostgroup_id="${2}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/hostgroup/associate"
    local request_json="{\"ID\": \"${hostgroup_id}\",\"ASSOCIATEOBJTYPE\": 21,\"ASSOCIATEOBJID\": \"${host_id}\"}"
    local response

    response="$( curlPost "${ibasetoken}" "${request_json}" "${url}" )"

    echo "${response}"
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

getInitiatorInfo () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local initiator_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/fc_initiator?filter=ID:${initiator_id}"
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

    # check hostgroup . If not existed , create hostgroup
    response="$( getHostGroupInfo "${HOSTGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostGroupInfo : ${ERROR_DESC}"
    }
    if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
        echo "hostgroup ${HOSTGROUP_NAME} is existed , no need to create!"
    else
        response="$( createHostGroup "${HOSTGROUP_NAME}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "createHostGroup : ${ERROR_DESC}"
        }
        echo "add hostgroup ${HOSTGROUP_NAME} success!"
    fi

    # get hostgroup id
    local hostgroup_id
    response="$( getHostGroupInfo "${HOSTGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostGroupInfo : ${ERROR_DESC}"
    }
    hostgroup_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "HOSTGROUP_ID ${hostgroup_id}"
    }

    local i=0
    while getValueNotNull ".data.hosts[${i}]" "${INPUT}" &> /dev/null; do
        # parse input to get host info
        local host_name
        local host_description
        local host_location
        local host_network_name
        local host_ip
        local host_os_type
        local host_model
        local host_initiator_type

        host_name="$( getValueNotNull ".data.hosts[${i}].name" "${INPUT}" )"
        host_description="$(getValueNull ".data.hosts[${i}].description" "${INPUT}")"
        host_description="${host_description:-"default"}"
        host_location="$(getValueNull ".data.hosts[${i}].location" "${INPUT}")"
        host_location="${host_location:-"default"}"
        host_network_name="$(getValueNull ".data.hosts[${i}].network_name" "${INPUT}")"
        host_network_name="${host_network_name:-"default"}"
        host_ip="$( getValueNotNull ".data.hosts[${i}].ip" "${INPUT}" )"
        host_os_type="$( getValueNotNull ".data.hosts[${i}].os_type" "${INPUT}" )"
        host_os_type="$( parseHostOSType "${host_os_type}" )"
        host_model="$( getValueNull ".data.hosts[${i}].model" "${INPUT}" )"
        host_model="${host_model:-"default"}"
        host_initiator_type="$( getValueNotNull ".data.hosts[${i}].initiator.type" "${INPUT}" )"
        IFS=" " read -r -a host_initiator_ids <<< "$( getJsonArrNotNull ".data.hosts[${i}].initiator.id" "${INPUT}" )"

        #check host and create host
        response="$( getHostInfo "${host_name}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getHostInfo : ${ERROR_DESC}"
        }
        if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
            echo "host ${host_name} exits, no need to create"
        else
            response="$( createHost "${host_name}" "${host_description}" "${host_location}" "${host_network_name}" "${host_ip}" "${host_os_type}" "${host_model}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "createHost : ${ERROR_DESC}"
            }
            echo "add host ${host_name} success!"
        fi

        # get host id and is_associate
        local host_id
        local is_associate
        response="$( getHostInfo "${host_name}" )"
        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getHostInfo : ${ERROR_DESC}"
        }
        host_id="$( getValueNotNull ".data[0].ID" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "HOST_ID ${host_id}"
        }
        is_associate="$( getValueNotNull ".data[0].ISADD2HOSTGROUP" "${response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "IS_ASSOCIATE ${is_associate}"
        }

        # check and create Initiator Associate
        if [[ "${host_initiator_type}" == "FC" ]]; then
            for initiator_id in ${host_initiator_ids[*]}; do
                local is_free
                local parent_id
                response="$( getInitiatorInfo "${initiator_id}" )"
                ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                    ERROR_CODE=$?
                    sanLogout
                    die "${ERROR_CODE}" "getInitiatorInfo : ${ERROR_DESC}"
                }
                if getValueNotNull ".data[0]" "${response}" &> /dev/null; then
                    is_free="$( getValueNotNull ".data[0].ISFREE" "${response}" )" || {
                        ERROR_CODE=$?
                        sanLogout
                        die "${ERROR_CODE}" "IS_FREE ${is_free}"
                    }
                    if [[ "${is_free}" == "true" ]]; then
                        response="$( createInitiatorAssociate "${initiator_id}" "${host_id}" )"
                        ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                            ERROR_CODE=$?
                            sanLogout
                            die "${ERROR_CODE}" "createInitiatorAssociate : ${ERROR_DESC}"
                        }
                        echo "add fc_initiator ${initiator_id} to ${host_name}success! "
                    elif [[ "${is_free}" == "false" ]]; then
                        parent_id="$( getValueNotNull ".data[0].PARENTID" "${response}" )" || {
                            ERROR_CODE=$?
                            sanLogout
                            die "${ERROR_CODE}" "PARENT_ID ${parent_id}"
                        }
                        if [[ "${parent_id}" -eq "${host_id}" ]]; then
                            echo "fc_initiator ${initiator_id} is already created to host ${host_name}!"
                        else
                            sanLogout
                            die 201 "fc_initiator ${initiator_id} is not free!"
                        fi
                    else
                        sanLogout
                        die 202 "getInitiatorInfo failed: is_free type is not boolean!"
                    fi
                else
                    sanLogout
                    die 203 "fc_initiator ${initiator_id} is not existed!"
                fi
            done
        fi

        # check and create HostGroup Associate
        if [[ "${is_associate}" == "true" ]]; then
            echo "host ${host_name} is associated"
        elif [[ "${is_associate}" == "false" ]]; then
            response="$( createHostGroupAssociate "${host_id}" "${hostgroup_id}" )"
            ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "createHostGroupAssociate : ${ERROR_DESC}"
            }
            echo "add host ${host_name} to hostgroup ${HOSTGROUP_NAME} success!"
        else
            sanLogout
            die 204 "getHostInfo failed: is_associate type is not boolean!"
        fi

        ((i++))
    done

    # finish and logout
    sanLogout
    echo "add hostgroup done"
}

main
