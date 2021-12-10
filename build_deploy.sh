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

# Update LATEST tag on remote repository to this hash
podman tag "${IMAGE}:${IMAGE_TAG}" "${IMAGE}:latest"
podman push "${IMAGE}:latest"

# Update MAIN tag on remote repository to this hash
podman tag "${IMAGE}:${IMAGE_TAG}" "${IMAGE}:main"
podman push "${IMAGE}:main"

# Update QA tag on remote repository to this hash
podman tag "${IMAGE}:${IMAGE_TAG}" "${IMAGE}:qa"
podman push "${IMAGE}:qa"
