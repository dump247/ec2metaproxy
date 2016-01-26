#!/usr/bin/env bash

set -o errexit
set -o nounset

SCRIPT_NAME="${0}"

function run_docker {
  local image="${1}"
  local container_name="${2}"
  local default_iam_role="${3}"
  local default_iam_policy="${4}"
  local proxy_iface="${5}"
  local proxy_port="${6}"

  local proxy_ip=$(ifconfig "${proxy_iface}" | grep "inet addr" | awk -F: '{print $2}' | awk '{print $1}')

  docker run                                                \
    -d                                                      \
    --net=host                                              \
    -v /var/run/docker.sock:/var/run/docker.sock            \
    --name="${container_name}"                              \
    --restart=always                                        \
    "${image}"                                              \
    --default-iam-role "${default_iam_role}"                \
    --default-iam-policy "${default_iam_policy}"            \
    --server "${proxy_ip}:${proxy_port}"                    \
    docker
}

function error {
  echo "${@:-}" 1>&2
}

function print_help {
  error "${SCRIPT_NAME} [options]"
  error
  error "Options:"
  error "  --image: metadata proxy docker image (default: dump247/ec2metaproxy)"
  error "  --container-name: name for the local metadata proxy container (default: ec2metaproxy)"
  error "  --proxy-iface: interface to bind the metadata proxy service to (default: docker0)"
  error "  --proxy-port: port to bind the metadata proxy service to (default: 18000)"
  error "  --default-iam-role: ARN of default role to apply to a container if the container does"
  error "                      not specify a role"
  error "  --default-iam-policy: default policy to apply to a container if the container"
  error "                        does not specify a role or policy (default: none)"
}

function main {
  local image="dump247/ec2metaproxy"
  local container_name="ec2metaproxy"
  local default_iam_role=""
  local default_iam_policy=""
  local proxy_port="18000"
  local proxy_iface="docker0"

  while [[ ${#} -gt 0 ]]; do
    case "${1}" in
      --image) image="${2}"; shift;;
      --container-name) container_name="${2}"; shift;;
      --default-iam-role) default_iam_role="${2}"; shift;;
      --default-iam-policy) default_iam_policy="${2}"; shift;;
      --proxy-iface) proxy_iface="${2}"; shift;;
      --proxy-port) proxy_port="${2}"; shift;;
      -h|--help)
        print_help
        exit 0;;
      *)
        if [[ -n "${1}" ]]; then
          error "Unknown option: ${1}"
          print_help
          exit 1
        fi
        ;;
    esac

    shift
  done

  run_docker                \
    "${image}"              \
    "${container_name}"     \
    "${default_iam_role}"   \
    "${default_iam_policy}" \
    "${proxy_iface}"        \
    "${proxy_port}"
}

main "${@:-}"
