#!/bin/bash
set -o nounset

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
