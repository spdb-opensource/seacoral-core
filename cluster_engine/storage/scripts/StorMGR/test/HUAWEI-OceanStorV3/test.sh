#!/bin/bash

set -o nounset

TEST_DIR="$(readlink -f "$0")"
HUAWEI_TEST_BASE_DIR=$(dirname "${TEST_DIR}")
declare -r HUAWEI_TEST_BASE_DIR
TEST_BASE_DIR=${HUAWEI_TEST_BASE_DIR%/*}
declare -r TEST_BASE_DIR
STORMGR_BASE_DIR=${TEST_BASE_DIR%/*}
declare -r STORMGR_BASE_DIR
LIB_BASE_DIR=${STORMGR_BASE_DIR}"/lib"
declare -r LIB_BASE_DIR

# shellcheck disable=SC1091
# shellcheck source=../lib/function.sh
source "${LIB_BASE_DIR}"/function.sh

OPTION="$1"
OUTPUT="/tmp/StorMGR_test.txt"

testAddHostGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local host_name
    host_name="$(${JQ} ".data.hosts[0].name" "${input}" | sed 's/\"//g')"
    local hostgroup_name
    hostgroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/add_hostgroup.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "Option" "Method" "Host" "HostGroup" "Status"
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "hostgroup" "add" "${host_name}" "${hostgroup_name}" "${status}"
    printf "\\n"
}

testDelHostGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local host_name
    host_name="$(${JQ} ".data.hosts_name[0]" "${input}" | sed 's/\"//g')"
    local hostgroup_name
    hostgroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/del_hostgroup.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "Option" "Method" "Host" "HostGroup" "Status"
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "hostgroup" "del" "${host_name}" "${hostgroup_name}" "${status}"
}

testListHostGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local hostgroup_name
    hostgroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    local output
    output="$(sh "${scripts_base_dir}/list_hostgroup.sh" "${input}" | sed 's/\"//g')"
    printf "%-20s %-20s %-25s\\n" "Option" "Method" "HostGroup"
    printf "%-20s %-20s %-25s\\n" "hostgroup" "list" "${hostgroup_name}"
    echo "Output:"
    echo "${output}"
    printf "\\n"
}

testListStoragepool(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local storagepool_name
    storagepool_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    local output
    output="$(sh "${scripts_base_dir}/list_storagepool.sh" "${input}")"
    printf "%-20s %-20s %-25s\\n" "Option" "Method" "StoragePool"
    printf "%-20s %-20s %-25s\\n" "storagepool" "list" "${storagepool_name}"
    echo "Output:"
    echo "${output}"
    printf "\\n"
}

testAddLunGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local lun_name
    lun_name="$(${JQ} ".data.luns[0].name" "${input}" | sed 's/\"//g')"
    local lungroup_name
    lungroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/add_lungroup.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "Option" "Method" "Lun" "LunGroup" "Status"
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "lungroup" "add" "${lun_name}" "${lungroup_name}" "${status}"
}

testDelLunGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local lun_id
    lun_id="$(${JQ} ".data.luns_id[0]" "${input}" | sed 's/\"//g')"
    local lungroup_name
    lungroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/del_lungroup.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "Option" "Method" "Lun" "LunGroup" "Status"
    printf "%-20s %-20s %-25s %-25s %-20s\\n" "lungroup" "del" "${lun_id}" "${lungroup_name}" "${status}"
}

testListLunGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local lungroup_name
    lungroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    local output
    output="$(sh "${scripts_base_dir}/list_lungroup.sh" "${input}")"
    printf "%-20s %-20s %-25s\\n" "Option" "Method" "LunGroup"
    printf "%-20s %-20s %-25s\\n" "lungroup" "list" "${lungroup_name}"
    echo "Output:"
    echo "${output}"
    printf "\\n"
}

testAddMapping(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local lungroup_name
    lungroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local hostgroup_name
    hostgroup_name="$(${JQ} ".data.hostgroup_name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/add_mapping.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-10s %-10s %-15s %-15s %-100s\\n" "mapping" "add" "${lungroup_name}" "${hostgroup_name}" "${status}"
}

testDelLunGroup(){
    local input="$1"
    local input_data="$(cat "${input}")"
    local lungroup_name
    lungroup_name="$(${JQ} ".data.name" "${input}" | sed 's/\"//g')"
    local hostgroup_name
    hostgroup_name="$(${JQ} ".data.hostgroup_name" "${input}" | sed 's/\"//g')"
    local vendor
    vendor="$(getValue ".auth_info.vendor" "${input_data}")" || die $? "VERDOR is null!"
    local scripts_base_dir=${LIB_BASE_DIR}"/"${vendor}
    sh "${scripts_base_dir}/del_mapping.sh" "${input}" &> /dev/null
    local ret=$?
    if [[ "${ret}" -eq 0 ]]; then
        local status="passing"
    else
        local status="failed"
    fi
    printf "%-10s %-10s %-15s %-15s %-100s\\n" "mapping" "del" "${lungroup_name}" "${hostgroup_name}" "${status}"
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
        add_hostgroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/add_hostgroup/*; do
                read -r -u3
                {
                    testAddHostGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        del_hostgroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/del_hostgroup/*; do
                read -r -u3
                {
                    testDelHostGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        list_hostgroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/list_hostgroup/*; do
                read -r -u3
                {
                    testListHostGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        list_storagepool)
            for input in "${HUAWEI_TEST_BASE_DIR}"/list_storagepool/*; do
                read -r -u3
                {
                    testListStoragepool "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        add_lungroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/add_lungroup/*; do
                read -r -u3
                {
                    testAddLunGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        del_lungroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/del_lungroup/*; do
                read -r -u3
                {
                    testDelLunGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        list_lungroup)
            for input in "${HUAWEI_TEST_BASE_DIR}"/list_lungroup/*; do
                read -r -u3
                {
                    testListLunGroup "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        add_mapping)
            for input in "${HUAWEI_TEST_BASE_DIR}"/add_mapping/*; do
                read -r -u3
                {
                    testAddMapping "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
        del_mapping)
            for input in "${HUAWEI_TEST_BASE_DIR}"/del_mapping/*; do
                read -r -u3
                {
                    testDelMapping "${input}" >> "${OUTPUT}"
                    echo >&3
                }&
            done
            wait ;;
    esac
    exec 3<&-
    exec 3>&-
}

main
