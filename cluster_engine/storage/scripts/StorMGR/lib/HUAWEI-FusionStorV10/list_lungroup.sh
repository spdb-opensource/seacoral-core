#!/bin/bash

set -o nounset

INPUT="$1"

SCRIPTS_DIR="$(readlink -f "$0")"
SCRIPTS_BASE_DIR=$(dirname "${SCRIPTS_DIR}")
declare -r SCRIPTS_BASE_DIR
LIB_BASE_DIR=${SCRIPTS_BASE_DIR%/*}
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=./function.sh
source "${LIB_BASE_DIR}"/_function.sh

installed jq || die 100 "jq not installed!"

BASIC_URI="dsware/service"

SAN_IP="$(getValueNotNull ".auth_info.ip" "${INPUT}")" || die $? "SAN_IP is null!"
SAN_PORT="$(getValueNotNull ".auth_info.port" "${INPUT}")" || die $? "SAN_PORT is null!"
SAN_USER="$(getValueNotNull ".auth_info.username" "${INPUT}")" || die $? "SAN_USER is null!"
SAN_PWD="$(getValueNotNull ".auth_info.password" "${INPUT}")" || die $? "SAN_PWD is null!"

LUNGROUP_NAME="$(getValueNotNull ".data.name" "${INPUT}")" || die $? "LUNGROUP_NAME is null!"
PAGE_SIZE=65000
PAGE_NUM=1

getVersion () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"

    local url="https://${san_ip}:${san_port}/${basic_uri}/rest/version"
    local response
    response="$(curl --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --referer "https://${san_ip}" --request GET --url "${url}")"
    echo "${response}"
}

sanLogin () {
    local san_ip="$1"
    local san_port="$2"
    local san_user="$3"
    local san_pwd="$4"
    local version="$5"
    local basic_uri="$6"

    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/sec/login"
    local request_json
    request_json="$(createTemplateJson "userName=${san_user}" "password=${san_pwd}")"
    local response
    response="$(curl  --silent --write-out "HTTPSTATUS:%{http_code}" -X HEAD -i --insecure --header "Content-Type: application/json;charset=UTF-8" --request POST --data "${request_json}" --url "${url}")"
    echo "${response}" | tr -d '\r'
}

sanLogout () {
    local san_ip="$1"
    local san_port="$2"
    local x_auth_token="$3"
    local version="$4"
    local basic_uri="$5"

    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/sec/logout"

    local response
    response="$(curl --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request POST --header "Content-Type: application/json;charset=UTF-8" --header 'X-Auth-Token: '"${x_auth_token}"'' --url "${url}")"
    local status
    status="$(echo "${response}" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')"
    local body
    body="$(sed -e 's/HTTPSTATUS\:.*//g'  <<< "${response}")"
    if [[ "${status}" == "200" ]]; then
        local code
        code=$(getValueNotNull ".result" "${body}")
        if [[ "${code}" -ne 0 ]]; then
            die 102 "logout failed : error code=${code}"
        fi
    else
        die 101 "http failed ${status}"
    fi
}

getStoragePoolInfoAll (){
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/storagePool"
    local response
    response="$(curlGetFS "${x_auth_token}" "${url}")"
    echo "${response}"
}

getLunInfoByLunGroup () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local storagepool_id="$6"
    local page_num="$7"
    local page_size="$8"
    local lungroup_name="$9"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/list"
    local request_json="{\"poolId\": ${storagepool_id},\"pageNum\": ${page_num},\"pageSize\": ${page_size},\"filters\": {\"volumeName\": \"${lungroup_name}\"}}"
    local response
    response="$(curlPostFS "${x_auth_token}" "${request_json}" "${url}")"
    echo "${response}"
}

getNodeInfoByLun () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local lun_name="$6"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/attachNodes?volName=${lun_name}"
    local response
    response="$(curlGetFS "${x_auth_token}" "${url}")"
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

appendLunOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".luns += [\$append_string]" <<< "${input_string}"
}

appendHostGroupOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".mapping_host += \$append_string" <<< "${input_string}"
}

appendHostOutput(){
    local input_string="${1}"
    local append_string="${2}"

    jq --argjson append_string "${append_string}" ".hosts += [\$append_string]" <<< "${input_string}"
}

main () {
    installed curl || die 100 "not found curl"

    local version_output
    version_output="$(getVersion "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}")"
    local version
    version="$(getValueNotNull ".currentVersion" "${version_output}")"

    local output
    output="$(sanLogin "${SAN_IP}" "${SAN_PORT}" "${SAN_USER}" "${SAN_PWD}" "${version}" "${BASIC_URI}")"
    local session
    session="$(tail -n1 <<< "${output}")"
    local session_output
    session_output="$(checkResponse "${session}" ".result" ".description")"
    local ret=$?
    if [[ ${ret} -ne 0 ]]; then
        die ${ret} "${session_output}"
    fi

    local x_auth_token
    x_auth_token="$(awk -F ': ' '/X-Auth-Token/{print $2}' <<< "${output}")"

    local response
    response="$(getStoragePoolInfoAll "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}")"
    local output
    output="$(checkResponse "${response}" ".result" ".description")"
    local ret=$?
    if [[ ${ret} -ne 0 ]]; then
        sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
        die ${ret} "${output}"
    fi
    local i=0
    while getValueNotNull ".storagePools[${i}]" "${response}" &> /dev/null; do
        local storagepool_ids
        storagepool_ids[${i}]="$(getValueNotNull ".storagePools[${i}].poolId" "${response}")"
        ((i++))
    done

    local not_exist=true

    for storagepool_id in ${storagepool_ids[*]}; do
        local response
        response="$(getLunInfoByLunGroup "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${storagepool_id}" "${PAGE_NUM}" "${PAGE_SIZE}" "${LUNGROUP_NAME}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            die ${ret} "${output}"
        fi

        if getValueNotNull ".volumeList[0]" "${response}" &> /dev/null; then
            local lungroup_type_output
            lungroup_type_output="$(getValueNotNull ".volumeList[0].volType" "${response}")"
            not_exist=false
            local lungroup_type="null"

            local json_output="{\"name\": \"${LUNGROUP_NAME}\",\"not_exist\": ${not_exist},\"alloc_type\": \"${lungroup_type}\",\"luns\": [],\"mapping_host\": {}}"

            local i=0
            while getValueNotNull ".volumeList[${i}]" "${response}" &> /dev/null; do
                local lun_name_output
                lun_name_output="$(getValueNotNull ".volumeList[${i}].volName" "${response}")"
                local lun_name
                lun_name=${lun_name_output##*-}
                local lun_id
                lun_id="$(getValueNotNull ".volumeList[${i}].wwn" "${response}")"
                local lun_capacity
                lun_capacity="$(getValueNotNull ".volumeList[${i}].volSize" "${response}")"
                local storagepool_name
                storagepool_name="$(getValueNotNull ".volumeList[${i}].poolId" "${response}")"
                local lun_health_status_output
                lun_health_status_output="$(getValueNotNull ".volumeList[${i}].status" "${response}")"
                case ${lun_health_status_output} in
                    0)
                        local lun_health_status="normal"
                        ;;
                    *)
                        local lun_health_status="unknown"
                        ;;
                esac
                local lun_running_status="${lun_health_status}"
                local lun_description="null"

                local lun_output
                lun_output="$(formatLunJSon "${lun_id}" "${lun_name}" "${lun_capacity}" "${storagepool_name}" "${lun_health_status}" "${lun_running_status}" "${lun_description}")"

                json_output="$(appendLunOutput "${json_output}" "${lun_output}")"
                ((i++))
            done
        fi
    done

    if [[ "${not_exist}" == "true" ]]; then
        local json_output="{\"not_exist\": ${not_exist}}"
        jq . <<< "${json_output}"
        sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
        exit 0
    fi

    if [[ "${lungroup_type_output}" -eq 2 ]]; then
        jq . <<< "${json_output}"
        sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
        exit 0
    else
        local host_response
        host_response="$(getNodeInfoByLun "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${lun_name_output[0]}")"
        local output
        output="$(checkResponse "${host_response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            jq . <<< "${json_output}"
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            exit 0
        fi

        local i=0
        local hostgroup_name
        hostgroup_name="$(getValueNotNull ".volAttachNodeInfo[0].nodeIp" "${host_response}")"
        local hostgroup_output="{\"name\": \"${hostgroup_name}\",\"hosts\": []}"

        while getValueNotNull ".volAttachNodeInfo[${i}].nodeIp" "${host_response}" &> /dev/null; do
            local host_ip
            host_ip="$(getValueNotNull ".volAttachNodeInfo[${i}].nodeIp" "${host_response}")"
            local host_name="${host_ip}"
            local host_os_type="Linux"
            local host_health_status="normal"
            local host_running_status="online"
            local host_location="null"
            local host_description="null"
            local host_network_name="null"
            local host_model="null"

            local host_output
            host_output="$(formatHostJson "${host_name}" "${host_ip}" "${host_os_type}" "${host_health_status}" "${host_running_status}" "${host_location}" "${host_description}" "${host_network_name}" "${host_model}")"

            hostgroup_output="$(appendHostOutput "${hostgroup_output}" "${host_output}")"
            ((i++))
        done
        json_output="$(appendHostGroupOutput "${json_output}" "${hostgroup_output}")"

        jq . <<< "${json_output}"
    fi

    sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
}

main
