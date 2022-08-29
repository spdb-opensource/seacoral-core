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
}

main () {
    local response
    local json_output

    # check install curl
    installed curl || die 200 "not found curl"

    # check input
    if ! checkInput ; then
        die "${ERROR_CODE}" "checkInput failed : ${ERROR_DESC}"
    fi

    local url="http://${SAN_IP}:${SAN_PORT}/hostgroup/info"
    response="$( curl  --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --header "Content-type:application/json" --request POST --data "${INPUT}" --url "${url}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
    }

    if [[ "${ERROR_CODE}" -eq 0 ]]; then
        json_output="$( getValueNotNull ".data" "${response}")" || {
            ERROR_CODE=$?
            ERROR_DESC="JSONOUTPUT ${json_output}"
            die "${ERROR_CODE}" "${ERROR_DESC}"
        }
    elif [[ "${ERROR_CODE}" -eq 5 ]]; then
        json_output="{}"
    else
        die "${ERROR_CODE}" "${ERROR_DESC}"
    fi

    jq . <<< "${json_output}"
}

main
