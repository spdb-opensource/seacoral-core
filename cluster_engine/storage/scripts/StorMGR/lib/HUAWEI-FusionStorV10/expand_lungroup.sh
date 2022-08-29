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

createLun () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local lun_name="$6"
    local storagepool_id="$7"
    local lun_capacity="$8"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/create"
    local request_json="{\"volName\": \"${lun_name}\",\"poolId\": ${storagepool_id},\"volSize\": ${lun_capacity}}"
    local response
    response="$(curlPostFS "${x_auth_token}" "${request_json}" "${url}")"
    echo "${response}"
}

getLunInfo() {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local lun_name="$6"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/queryByName?volName=${lun_name}"
    local response
    response="$(curlGetFS "${x_auth_token}" "${url}")"
    echo "${response}"
}

getMappingViewInfo() {
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

createMappingView () {
    local san_ip="$1"
    local san_port="$2"
    local basic_uri="$3"
    local version="$4"
    local x_auth_token="$5"
    local hostgroup_name="$6"
    local lun_name_list="$7"
    local url="https://${san_ip}:${san_port}/${basic_uri}/${version}/volume/attach"
    local request_json="{\"ipList\": [\"${hostgroup_name}\"],\"volName\": [${lun_name_list}]}"
    local response
    response="$(curlPostFS "${x_auth_token}" "${request_json}" "${url}")"
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

    local i=0
    while getValueNotNull ".data.luns[${i}]" "${INPUT}" &> /dev/null; do
        LUN_NAME="$(getValueNotNull ".data.luns[${i}].name" "${INPUT}")" || die $? "LUN_NAME is null!"
        LUN_CAPACITY="$(getValueNotNull ".data.luns[${i}].capacity_MB" "${INPUT}")" || die $? "LUN_CAPACITY is null!"
        STORAGEPOOL_NAME="$(getValueNotNull ".data.luns[${i}].storagepool_name" "${INPUT}")" || die $? "STORAGEPOOL_NAME is null!"
        STORAGEPOOL_ID="${STORAGEPOOL_NAME}"

        local response
        response="$(createLun "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${LUNGROUP_NAME}-${LUN_NAME}" "${STORAGEPOOL_ID}" "${LUN_CAPACITY}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            local err_code
            err_code="$(getValueNotNull ".errorCode" "${response}")"
            if [[ "${ret}" -eq 2 && "${err_code}" -eq 32150007 ]]; then
                echo "lun ${LUN_NAME} is existed , no need to create!"
            else
                sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
                die ${ret} "${output}"
            fi
        else
            echo "add lun ${LUN_NAME} success!"
        fi

        ((i++))
    done

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
    for ((i=0;i<${#storagepool_ids[@]};i++)); do
        local response
        response="$(getLunInfoByLunGroup "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${storagepool_ids[${i}]}" "${PAGE_NUM}" "${PAGE_SIZE}" "${LUNGROUP_NAME}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            die ${ret} "${output}"
        fi

        if getValueNotNull ".volumeList[0]" "${response}" &> /dev/null; then
            local j=0
            while getValueNotNull ".volumeList[${j}]" "${response}" &> /dev/null; do
                local lun_names
                lun_names[${count}]="$(getValueNotNull ".volumeList[${j}].volName" "${response}")"
                ((j++))
                ((count++))
            done
        fi
    done

    local lun_name_list=""
    local hostgroup_name
    local count=0
    local match="false"
    for lun_name in ${lun_names[*]}; do
        local response
        response="$(getMappingViewInfo "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${lun_name}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            die ${ret} "${output}"
        fi

        if getValueNotNull ".volAttachNodeInfo[0].nodeIp" "${response}" &> /dev/null; then
            hostgroup_name="$(getValueNotNull ".volAttachNodeInfo[0].nodeIp" "${response}")"
        else
            local lun_names_match
            lun_names_match[${count}]=${lun_name}
            match="true"
            ((count++))
        fi

    done

    if [[ "${match}" == "true" ]]; then
        for((i=0;i<${#lun_names_match[*]};i++)); do
            if [[ $((i + 1)) -eq ${#lun_names_match[*]} ]]; then
                lun_name_list=${lun_name_list}"\"${lun_names_match[${i}]}\""
            else
                lun_name_list=${lun_name_list}"\"${lun_names_match[${i}]}\", "
            fi
        done

        local response
        response="$(createMappingView "${SAN_IP}" "${SAN_PORT}" "${BASIC_URI}" "${version}" "${x_auth_token}" "${hostgroup_name}" "${lun_name_list}")"
        local output
        output="$(checkResponse "${response}" ".result" ".description")"
        local ret=$?
        if [[ ${ret} -ne 0 ]]; then
            sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
            die ${ret} "${output}"
        fi
        echo "add lun ${lun_name_list} to hostgroup ${hostgroup_name} success!"
    fi
    sanLogout "${SAN_IP}" "${SAN_PORT}" "${x_auth_token}" "${version}" "${BASIC_URI}"
}

main
