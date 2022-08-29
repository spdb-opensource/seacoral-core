#!/bin/bash

set -o nounset

STOR_BASE_DIR=${LIB_BASE_DIR%/*}
declare -r STOR_BASE_DIR
declare -r COOKIE_DIR="${STOR_BASE_DIR}/cookie"

[[ -d "${COOKIE_DIR}" ]] || mkdir "${COOKIE_DIR}"

installed () {
    command -v "$1" >/dev/null 2>&1
}

_warn () {
    echo "$@" >&2
}

die () {
    local status="$1"
    shift
    _warn "$@"
    exit "$status"
}

_error () {
    local status="$1"
    shift
    echo "ERROR: $*"
    return "$status"
}

transMBToSector () {
    local value_MB="$1"
    local value_sector=$(( value_MB * 1024 * 1024 / 512 ))
    echo "${value_sector}"
}

transSectorToMB () {
    local value_sector="$1"
    local value_MB=$(( value_sector * 512 / 1024 /1024 ))
    echo "${value_MB}"
}

createTemplateJson () {
    local cmd="{"
    local count=1
    for i in "$@"; do
        local key="${i%%=*}"
        local value="${i##*=}"
        if [[ "${count}" -eq "$#" ]]; then
            cmd=${cmd}"\"${key}\": \"${value}\""
            break
        fi
        cmd=${cmd}"\"${key}\": \"${value}\","
        (( count++ ))
    done
    cmd=${cmd}"}"
    jq . <<< "${cmd}"
}

getValueNotNull () {
    local key="$1"
    local input="$2"
    local body
    local output
    local ret

    # get json body
    body="$(sed -e 's/HTTPSTATUS\:.*//g' <<< "${input}")"

    # check json
    if jq -e . &> /dev/null <<< "${body}" ; then
        output=$(jq --raw-output -c "${key}" <<<"${body}")
        ret=$?
        if [[ ${ret} -ne 0 ]]; then
            _error 2 "get value failed"
        elif [[ -z "${output}" || "${output}" == "null" ]]; then
            _error 3 "value is null"
        else
            echo "${output}"
        fi
    else
        _error 4 "input json body syntax error"
    fi
}

getValueNull () {
    local key="$1"
    local input="$2"
    local body
    local output
    local ret

    # get json body
    body="$(sed -e 's/HTTPSTATUS\:.*//g' <<< "${input}")"

    # check json
    if jq -e . &> /dev/null <<< "${body}" ; then
        output=$(jq --raw-output -c "${key}" <<< "${body}")
        ret=$?
        if [[ ${ret} -ne 0 ]]; then
            _error 2 "get value failed"
        else
            echo "${output}"
        fi
    else
        _error 4 "input json body syntax error"
    fi
}

getJsonArrNotNull() {
    local key="$1"
    local input="$2"
    local i=0
    local array
    local output
    local ret
    output="$( getValueNotNull "${key}[0]" "${input}" )" || {
        ret=$?
        echo "${output}"
        return "${ret}"
    }
    while getValueNotNull "${key}[${i}]" "${input}" &> /dev/null; do
        array[${i}]="$(getValueNotNull "${key}[${i}]" "${input}")"
        ((i++))
    done
    echo "${array[*]}"
}

checkResponse () {
    local response="$1"
    local errcode="$2"
    local errdesc="$3"
    local status
    local body
    local code
    local desc

    status="$(echo "${response}" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')"
    body="$(sed -e 's/HTTPSTATUS\:.*//g' <<< "${response}")"
    if [[ -z "${response}" ]] ; then
        _error 5 "response is null"
    elif [[ "${status}" -eq "200" ]] ; then
        code=$(getValueNotNull "${errcode}" "${body}")
        desc=$(getValueNotNull "${errdesc}" "${body}")
        if [[ "${code}" -ne 0 ]]; then
            _error 6 "response message: code=${code}, description=${desc}"
        fi
    else
        _error 7 "http response code is ${status}"
    fi
}

curlPost () {
    local ibasetoken="$1"
    local request_json="$2"
    local url="$3"

    local cookie_file="${COOKIE_DIR}/cookie.${ibasetoken}"

    local response
    response="$(curl --cookie "${cookie_file}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request POST --header 'iBaseToken: '"${ibasetoken}"'' --data "${request_json}" --url "${url}")"
    echo "${response}"
}

curlPut () {
    local ibasetoken="$1"
    local request_json="$2"
    local url="$3"

    local cookie_file="${COOKIE_DIR}/cookie.${ibasetoken}"

    local response
    response="$(curl --cookie "${cookie_file}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request PUT --header 'iBaseToken: '"${ibasetoken}"'' --data "${request_json}" --url "${url}")"
    echo "${response}"
}

curlGet () {
    local ibasetoken="$1"
    local url="$2"

    local cookie_file="${COOKIE_DIR}/cookie.${ibasetoken}"

    local response
    response="$(curl --cookie "${cookie_file}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request GET --header 'iBaseToken: '"${ibasetoken}"'' --url "${url}")"
    echo "${response}"
}

curlDelete () {
    local ibasetoken="$1"
    local url="$2"

    local cookie_file="${COOKIE_DIR}/cookie.${ibasetoken}"

    local response
    response="$(curl --cookie "${cookie_file}" --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request DELETE --header 'iBaseToken: '"${ibasetoken}"'' --url "${url}")"
    echo "${response}"
}

curlPostFS () {
    local x_auth_token="$1"
    local request_json="$2"
    local url="$3"

    local response
    response="$(curl  --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request POST --header "Content-Type: application/json;charset=UTF-8" --header 'X-Auth-Token: '"${x_auth_token}"'' --data "${request_json}" --url "${url}")"
    echo "${response}"
}

curlGetFS () {
    local x_auth_token="$1"
    local url="$2"

    local response
    response="$(curl  --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --request GET --header "Content-Type: application/json;charset=UTF-8" --header 'X-Auth-Token: '"${x_auth_token}"'' --url "${url}")"
    echo "${response}"
}
