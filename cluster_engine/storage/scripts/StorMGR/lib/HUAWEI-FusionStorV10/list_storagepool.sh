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

STORAGEPOOL_NAME="$(getValueNull ".data.name" "${INPUT}")"

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

getStoragePoolInfo (){
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local storagepool_id="$6"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/storagePool?poolId=${storagepool_id}"
    local response
    response="$(curlGetFS "${x_auth_token}" "${url}")"
    echo "${response}"
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

    if [[ -n "${STORAGEPOOL_NAME}" ]] && [[ "${STORAGEPOOL_NAME}" != "null" ]]; then
        local response
        response="$(getStoragePoolInfo "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${STORAGEPOOL_NAME}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            local json_output="{}"
            jq . <<< "${json_output}"
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            exit 0
        fi
    else
        local response
        response="$(getStoragePoolInfoAll "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            local json_output="{}"
            jq . <<< "${json_output}"
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            exit 0
        fi
    fi

    local json_output
    json_output="[]"

    local i=0
    while getValueNotNull ".storagePools[${i}]" "${response}" &> /dev/null; do
        local storagepool_name
        storagepool_name="$(getValueNotNull ".storagePools[${i}].poolId" "${response}")"
        local storagepool_id
        storagepool_id="$(getValueNotNull ".storagePools[${i}].poolId" "${response}")"
        local storagepool_total_capacity_MB
        storagepool_total_capacity_MB="$(getValueNotNull ".storagePools[${i}].totalCapacity" "${response}")"
        local storagepool_allocated_capacity_MB
        storagepool_allocated_capacity_MB="$(getValueNotNull ".storagePools[${i}].allocatedCapacity" "${response}")"
        local storagepool_free_capacity_MB
        storagepool_free_capacity_MB=$((storagepool_total_capacity_MB - storagepool_allocated_capacity_MB))

        local storagepool_health_status
        local storagepool_health_status_code
        storagepool_health_status_code="$(getValueNotNull ".storagePools[${i}].poolStatus" "${response}")"
        case ${storagepool_health_status_code} in
            0)  storagepool_health_status="normal" ;;
            1)  storagepool_health_status="fault" ;;
            2)  storagepool_health_status="write-protected" ;;
            3)  storagepool_health_status="stopped" ;;
            4)  storagepool_health_status="fault and write-protected" ;;
            5)  storagepool_health_status="rebalance not 100" ;;
            *)  storagepool_health_status="null" ;;
        esac
        local storagepool_running_status="${storagepool_health_status}"
        local storagepool_description_output
        storagepool_description_output="$(getValueNull ".storagePools[${i}].DESCRIPTION" "${response}")"
        local storagepool_description=${storagepool_description_output:-"null"}

        local storagepool_disk_type
        storagepool_disk_type="$(getValueNull ".storagePools[${i}].mediaType" "${response}")"

        local storagepool_output
        storagepool_output="$(formatStoragepoolJson "${storagepool_id}" "${STORAGEPOOL_NAME:-${storagepool_name}}" "${storagepool_total_capacity_MB}" "${storagepool_free_capacity_MB}" "${storagepool_disk_type}" "${storagepool_health_status}" "${storagepool_running_status}" "${storagepool_description}")"

        json_output="$(appendStorageOutput "${json_output}" "${storagepool_output}")"
        ((i++))

    done

    jq . <<< "${json_output}"

    sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
}

main
