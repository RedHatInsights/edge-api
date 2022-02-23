#!/bin/bash

set -exv

IMAGE="quay.io/cloudservices/edge-api"

# Determine Git commit hash (7 hex characters)
IMAGE_TAG="$(git rev-parse --short=7 HEAD)"

if [[ -z "${QUAY_USER}" || -z "${QUAY_TOKEN}" ]]; then
    echo "QUAY_USER and QUAY_TOKEN must be set"
    exit 1
fi

if [[ -z "${RH_REGISTRY_USER}" || -z "${RH_REGISTRY_TOKEN}" ]]; then
    echo "RH_REGISTRY_USER and RH_REGISTRY_TOKEN  must be set"
    exit 1
fi

AUTH_CONF_DIR="$(pwd)/.podman"
mkdir -p "${AUTH_CONF_DIR}"
export REGISTRY_AUTH_FILE="${AUTH_CONF_DIR}/auth.json"

podman login -u="${QUAY_USER}" -p="${QUAY_TOKEN}" quay.io
podman login -u="${RH_REGISTRY_USER}" -p="${RH_REGISTRY_TOKEN}" registry.redhat.io

# Build image
podman build -f Dockerfile -t "${IMAGE}:${IMAGE_TAG}" .

# Push image to remote repository
podman push "${IMAGE}:${IMAGE_TAG}"

TAGS="latest main qa"
# check if a change is under cmd/kafka directory and tag accordingly
#num_files=$(git log --raw -n 1 --no-merges | egrep "^:.*" | wc -l)
num_kafka_files=$(git log --raw -n 1 --no-merges | egrep "^:.*cmd/kafka" | wc -l)
# if all changes are under cmd/kafka then only tag kafka
if [[ $num_kafka_files -gt 0 ]]; then
    #[[ num_files -eq num_kafka_files ]] && TAGS="kafka" || TAGS="$TAGS kafka"
    TAGS="$TAGS kafka"
fi

for tag in $(echo $TAGS); do
    podman tag "${IMAGE}:${IMAGE_TAG}" "${IMAGE}:${tag}"
    podman push "${IMAGE}:${tag}"
done
