#!/bin/bash

set -o nounset

TEST_DIR="$(readlink -f "$0")"
HUAWEI_TEST_BASE_DIR=$(dirname "${TEST_DIR}")
declare -r HUAWEI_TEST_BASE_DIR
TEST_BASE_DIR=${HUAWEI_TEST_BASE_DIR%/*}
declare -r TEST_BASE_DIR
VPMGR_BASE_DIR=${TEST_BASE_DIR%/*}
declare -r VPMGR_BASE_DIR
LIB_BASE_DIR=${VPMGR_BASE_DIR}"/lib"
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=../lib/function.sh
source "${LIB_BASE_DIR}"/function.sh

OPTION="$1"
OUTPUT="/tmp/VPMGR_test.txt"

testAdd() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/add.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "add" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

testExpand() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/expand.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "expand" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

testDelete() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/delete.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "delete" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

testActive() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/active.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "active" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

testBlock() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/block.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "block" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

testDeactive() {
    local input="$1"
    local input_data
    input_data="$(cat "${input}")"
    local lv_name
    lv_name="$(${JQ} ".lv.name" "${input}" | sed 's/\"//g')"
    local vg_name
    vg_name="$(${JQ} ".vg.name" "${input}" | sed 's/\"//g')"
    local vg_type
    vg_type="$(${JQ} ".vg.type" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".vg.vendor" "${input_data}")" || die $? "VENDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/deactive.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "METHOD" "LV" "VG" "TYPE" "Status"
    printf "%-20s %-20s %-20s %-20s %-20s\\n" "deactive" "${lv_name}" "${vg_name}" "${vg_type}" "${status}"
    printf "\\n"
}

main() {
    [ -e ${OUTPUT} ] && rm -rf ${OUTPUT}
    [ -e /tmp/fd1 ] || mkfifo /tmp/fd1
    exec 3<>/tmp/fd1
    rm -rf /tmp/fd1
    for ((i=1;i<=3;i++)); do
        echo >&3
    done

    case ${OPTION} in
        add)
            for input in "${HUAWEI_TEST_BASE_DIR}"/add/*; do
                read -r -u3
                {
                    testAdd "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        expand)
            for input in "${HUAWEI_TEST_BASE_DIR}"/expand/*; do
                read -r -u3
                {
                    testExpand "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        delete)
            for input in "${HUAWEI_TEST_BASE_DIR}"/delete/*; do
                read -r -u3
                {
                    testDelete "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        active)
            for input in "${HUAWEI_TEST_BASE_DIR}"/active/*; do
                read -r -u3
                {
                    testActive "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        deactive)
            for input in "${HUAWEI_TEST_BASE_DIR}"/deactive/*; do
                read -r -u3
                {
                    testDeactive "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
    esac
    exec 3<&-
    exec 3>&-
}

main
