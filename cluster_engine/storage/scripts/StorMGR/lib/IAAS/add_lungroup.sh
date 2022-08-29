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
    LUNGROUP_NAME="$( getValueNotNull ".data.name" "${input}" )" || {
        ERROR_CODE=$?
        ERROR_DESC="LUNGROUP_NAME ${LUNGROUP_NAME}"
        return "${ERROR_CODE}"
    }
}

main () {
    local response

    # check install curl
    installed curl || die 200 "not found curl"

    # check input
    if ! checkInput ; then
        die "${ERROR_CODE}" "checkInput failed : ${ERROR_DESC}"
    fi

    local url="http://${SAN_IP}:${SAN_PORT}/lungroup/add"
    response="$( curl  --silent --write-out "HTTPSTATUS:%{http_code}" --insecure --header "Content-type:application/json" --request POST --data "${INPUT}" --url "${url}" )"
    ERROR_DESC="$( checkResponse "${response}" ".error.code" ".error.description" )" || {
        ERROR_CODE=$?
    }

    if [[ "${ERROR_CODE}" -eq 5 ]]; then
        echo "lungroup ${LUNGROUP_NAME} is existed, no need to create"
    elif [[ "${ERROR_CODE}" -eq 0 ]]; then
        echo "add lungroup ${LUNGROUP_NAME} success!"
    else
        die "${ERROR_CODE}" "${ERROR_DESC}"
    fi
}

main
