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
        else
            echo "Logout success!"
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

deleteLun () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local lun_name="$6"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/delete"
    local request_json="{\"volNames\": [\"${lun_name}\"]}"
    local response
    response="$(curlPostFS "${x_auth_token}" "${request_json}" "${url}")"
    echo "${response}"
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
    echo "Login success!"

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

    local count=0
    local not_exist="true"
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
            not_exist="false"
            local i=0
            while getValueNotNull ".volumeList[${i}]" "${response}" &> /dev/null; do
                local lun_names
                lun_names[${count}]="$(getValueNotNull ".volumeList[${i}].volName" "${response}")"
                ((i++))
                ((count++))
            done
        fi
    done

    if [[ "${not_exist}" == "true" ]]; then
        echo "lungroup ${LUNGROUP_NAME} is not existed, no need to delete!"
        sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
        exit 0
    fi

    for lun_name in ${lun_names[*]}; do
        local response
        response="$(deleteLun "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${lun_name}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            die ${ret} "${output}"
        fi
        echo "delete lun ${lun_name} success!"
    done

    sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
}

main
