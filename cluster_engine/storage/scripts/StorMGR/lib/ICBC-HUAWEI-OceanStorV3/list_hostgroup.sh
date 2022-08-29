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
    HOSTGROUP_NAME="$( getValueNotNull ".data.name" "${INPUT}" )" || {
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

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getInitiatorInfoByHost () {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local host_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/fc_initiator?PARENTID=${host_id}"
    local response

    response="$( curlGet "${ibasetoken}" "${url}" )"

    echo "${response}"
}

getMappingViewInfoByHostGroup() {
    local san_ip="${SAN_IP}"
    local san_port="${SAN_PORT}"
    local device_id="${SESSION_DEVICE_ID}"
    local ibasetoken="${SESSION_IBASETOKEN}"
    local hostgroup_id="${1}"

    local url="https://${san_ip}:${san_port}/deviceManager/rest/${device_id}/mappingview/associate?ASSOCIATEOBJTYPE=14&ASSOCIATEOBJID=${hostgroup_id}"
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

formatLunJSon () {
    local lun_id="$1"
    local lun_name="$2"
    local lun_capacity="$3"
    local storagepool_name="$4"
    local lun_health_status="$5"
    local lun_running_status="$6"
    local lun_description="$7"
    local output="{\"id\": \"${lun_id}\",\"name\": \"${lun_name}\",\"capacity_MB\": ${lun_capacity},\"storagepool_name\": \"${storagepool_name}\",\"health_status\": \"${lun_health_status}\",\"running_status\": \"${lun_running_status}\",\"description\": \"${lun_description}\"}"

    echo "${output}"
}

formatHostJson () {
    local host_name="$1"
    local host_ip="$2"
    local host_os_type="$3"
    local host_health_status="$4"
    local host_running_status="$5"
    local host_location="$6"
    local host_description="$7"
    local host_network_name="$8"
    local host_model="$9"
    local output="{\"name\": \"${host_name}\",\"ip\": \"${host_ip}\",\"os_type\": \"${host_os_type}\",\"health_status\": \"${host_health_status}\",\"running_status\": \"${host_running_status}\",\"initiator\": {},\"location\": \"${host_location}\",\"description\": \"${host_description}\",\"network_name\": \"${host_network_name}\",\"model\": \"${host_model}\"}"

    echo "${output}"
}

appendInitiatorOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".initiator += \$append_string" <<< "${input_string}"
}

appendInitiatorIdOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".id += [\$append_string]" <<< "${input_string}"
}

appendHostOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".hosts += [\$append_string]" <<< "${input_string}"
}

appendLunOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".luns += [\$append_string]" <<< "${input_string}"
}

appendLunGroupOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".mapping_lungroup += [\$append_string]" <<< "${input_string}"
}

main () {
    local response
    local hostgroup_response
    local host_response
    local initiator_response
    local mappingview_response
    local lungroup_response
    local lun_response
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

    #get hostgroup info
    local hostgroup_id
    local is_mapping
    hostgroup_response="$( getHostGroupInfo "${HOSTGROUP_NAME}" )"
    ERROR_DESC="$( checkResponse "${hostgroup_response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostGroupInfo : ${ERROR_DESC}"
    }
    getValueNotNull ".data[0]" "${hostgroup_response}" &> /dev/null || {
        json_output="{}"
        jq . <<< "${json_output}"
        sanLogout
        exit 0
    }
    hostgroup_id="$( getValueNotNull ".data[0].ID" "${hostgroup_response}" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "HOSTGROUP_ID ${hostgroup_id}"
    }
    is_mapping="$( getValueNotNull ".data[0].ISADD2MAPPINGVIEW" "${hostgroup_response}" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "IS_MAPPING ${is_mapping}"
    }
    # build hostgroup base body
    json_output="{\"name\": \"${HOSTGROUP_NAME}\",\"hosts\": [],\"mapping_lungroup\": []}"

    #get host info
    host_response="$( getHostInfoByHostGroup "${hostgroup_id}" )"
    ERROR_DESC="$( checkResponse "${host_response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
        sanLogout
        die "${ERROR_CODE}" "getHostInfoByHostGroup : ${ERROR_DESC}"
    }
    local i=0
    while getValueNotNull ".data[${i}]" "${host_response}" &> /dev/null; do
        local host_name
        local host_ip
        local host_location
        local host_os_type
        local host_health_status
        local host_running_status
        local host_network_name
        local host_model
        local host_description
        local host_id
        local host_output

        host_name="$( getValueNotNull ".data[${i}].NAME" "${host_response}" )" || {
            host_name=""
        }
        host_ip="$( getValueNotNull ".data[${i}].IP" "${host_response}" )" || {
            host_ip=""
        }
        host_location="$( getValueNotNull ".data[${i}].LOCATION" "${host_response}" )" || {
            host_location=""
        }
        host_os_type="$( getValueNotNull ".data[${i}].OPERATIONSYSTEM" "${host_response}" )" || {
            host_os_type=""
        }
        case "${host_os_type}" in
            0) host_os_type="Linux" ;;
            1) host_os_type="Windows" ;;
            2) host_os_type="Solaris" ;;
            3) host_os_type="HP-UX" ;;
            4) host_os_type="AIX" ;;
            5) host_os_type="XenServer" ;;
            6) host_os_type="Mac OS" ;;
            7) host_os_type="VMware ESX" ;;
            8) host_os_type="LINUX_VIS" ;;
            9) host_os_type="Windows Server 2012" ;;
            10) host_os_type="Oracle VM" ;;
            11) host_os_type="OpenVMS" ;;
        esac

        host_health_status="$( getValueNotNull ".data[${i}].HEALTHSTATUS" "${host_response}" )" || {
            host_health_status=""
        }
        case "${host_health_status}" in
            1) host_health_status="normal" ;;
            *) host_health_status="abnormal" ;;
        esac

        host_running_status="$( getValueNotNull ".data[${i}].RUNNINGSTATUS" "${host_response}" )" || {
            host_running_status=""
        }
        case "${host_running_status}" in
            1) host_running_status="online" ;;
            *) host_running_status="offline" ;;
        esac

        host_network_name="$( getValueNotNull ".data[${i}].NETWORKNAME" "${host_response}" )" || {
            host_network_name=""
        }
        host_model="$( getValueNotNull ".data[${i}].MODEL" "${host_response}" )" || {
            host_model=""
        }
        host_description="$( getValueNotNull ".data[${i}].DESCRIPTION" "${host_response}" )" || {
            host_description=""
        }
        host_id="$( getValueNotNull ".data[${i}].ID" "${host_response}" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "HOST_ID ${host_id}"
        }
        host_output="$( formatHostJson "${host_name}" "${host_ip}" "${host_os_type}" "${host_health_status}" "${host_running_status}" "${host_location}" "${host_description}" "${host_network_name}" "${host_model}" )"

        #get initiator info
        initiator_response="$( getInitiatorInfoByHost "${host_id}" )"
        ERROR_DESC="$( checkResponse "${initiator_response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getInitiatorInfoByHost : ${ERROR_DESC}"
        }
        if getValueNotNull ".data[0]" "${initiator_response}" &> /dev/null; then
            local initiator_type
            local initiator_output
            initiator_type="$( getValueNotNull ".data[0].TYPE" "${initiator_response}" )" || {
                initiator_type=""
            }
            if [[ "${initiator_type}" -eq 223 ]]; then
                initiator_type="FC"
                initiator_output="{\"type\": \"${initiator_type}\",\"id\": []}"
                local j=0
                while getValueNotNull ".data[${j}]" "${initiator_response}" &> /dev/null; do
                    local initiator_id
                    local initiator_id_output
                    initiator_id="$( getValueNotNull ".data[${j}].ID" "${initiator_response}" )" || {
                        initiator_id=""
                    }
                    initiator_id_output="\"${initiator_id}\""
                    initiator_output="$( appendInitiatorIdOutput "${initiator_output}" "${initiator_id_output}" )"
                    ((j++))
                done
                host_output="$( appendInitiatorOutput "${host_output}" "${initiator_output}" )"
            fi
        fi
        json_output="$( appendHostOutput "${json_output}" "${host_output}" )"
        ((i++))
    done

    #get mappingview info
    if [[ "${is_mapping}" == "true" ]]; then
        mappingview_response="$( getMappingViewInfoByHostGroup "${hostgroup_id}" )"
        ERROR_DESC="$( checkResponse "${mappingview_response}" ".error.code" ".error.description" )" || {
            ERROR_CODE=$?
            sanLogout
            die "${ERROR_CODE}" "getMappingViewInfoByHostGroup : ${ERROR_DESC}"
        }
        local i=0
        while getValueNotNull ".data[${i}]" "${mappingview_response}" &> /dev/null; do
            local mappingview_id
            mappingview_id="$( getValueNotNull ".data[${i}].ID" "${mappingview_response}" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "MAPPINGVIEW_ID ${mappingview_id}"
            }

            #get lungroup info
            lungroup_response="$( getLunGroupInfoByMappingView "${mappingview_id}" )"
            ERROR_DESC="$( checkResponse "${lungroup_response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "getLunGroupInfoByMappingView : ${ERROR_DESC}"
            }
            local lungroup_id
            local lungroup_name
            local lungroup_output

            lungroup_id="$( getValueNotNull ".data[0].ID" "${lungroup_response}" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "LUNGROUP_ID ${lungroup_id}"
            }
            lungroup_name="$( getValueNotNull ".data[0].NAME" "${lungroup_response}" )" || {
                lungroup_name=""
            }
            # build lungroup base body
            lungroup_output="{\"name\": \"${lungroup_name}\",\"luns\": []}"

            #get lun info
            lun_response="$( getLunInfoByLunGroup "${lungroup_id}" )"
            ERROR_DESC="$( checkResponse "${lun_response}" ".error.code" ".error.description" )" || {
                ERROR_CODE=$?
                sanLogout
                die "${ERROR_CODE}" "getLunInfoByLunGroup : ${ERROR_DESC}"
            }
            local j=0
            while getValueNotNull ".data[${j}]" "${lun_response}" &> /dev/null; do
                local lun_name
                local lun_id
                local lun_capacity
                local storagepool_name
                local lun_health_status
                local lun_running_status
                local lun_description
                local lun_output

                lun_name="$( getValueNotNull ".data[${j}].NAME" "${lun_response}" )" || {
                    lun_name=""
                }
                lun_id="$( getValueNotNull ".data[${j}].ID" "${lun_response}" )" || {
                    lun_id=""
                }
                lun_capacity="$( getValueNotNull ".data[${j}].CAPACITY" "${lun_response}" )" || {
                    lun_capacity=0
                }
                lun_capacity=$( transSectorToMB "${lun_capacity}" )
                storagepool_name="$( getValueNotNull ".data[${j}].PARENTNAME" "${lun_response}" )" || {
                    storagepool_name=""
                }
                lun_health_status="$( getValueNotNull ".data[${j}].HEALTHSTATUS" "${lun_response}" )" || {
                    lun_health_status=""
                }
                case "${lun_health_status}" in
                    1) lun_health_status="normal" ;;
                    2) lun_health_status="abnormal" ;;
                esac
                lun_running_status="$( getValueNotNull ".data[${j}].RUNNINGSTATUS" "${lun_response}" )" || {
                    lun_running_status=""
                }
                case "${lun_running_status}" in
                    27) lun_running_status="online" ;;
                    28) lun_running_status="offline" ;;
                    53) lun_running_status="initialized" ;;
                esac
                lun_description="$( getValueNotNull ".data[${j}].DESCRIPTION" "${lun_response}" )" || {
                    lun_description=""
                }

                lun_output="$( formatLunJSon "${lun_id}" "${lun_name}" "${lun_capacity}" "${storagepool_name}" "${lun_health_status}" "${lun_running_status}" "${lun_description}" )"
                lungroup_output="$( appendLunOutput "${lungroup_output}" "${lun_output}" )"
                ((j++))
            done
            json_output="$( appendLunGroupOutput "${json_output}" "${lungroup_output}" )"
            ((i++))
        done
    fi

    jq . <<< "${json_output}"

    sanLogout
}

main
