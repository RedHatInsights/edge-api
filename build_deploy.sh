#!/bin/bash

set -exv

# Determine Git commit hash (7 hex characters)
declare IMAGE_TAG
IMAGE_TAG="$(git rev-parse --short=7 HEAD)"
readonly IMAGE_TAG

if [[ -z "${QUAY_USER}" || -z "${QUAY_TOKEN}" ]]; then
    echo "QUAY_USER and QUAY_TOKEN must be set"
    exit 1
fi

podman login -u="${QUAY_USER}" -p="${QUAY_TOKEN}" quay.io

#if [[ -z "${RH_REGISTRY_USER}" || -z "${RH_REGISTRY_TOKEN}" ]]; then
#    echo "RH_REGISTRY_USER and RH_REGISTRY_TOKEN  must be set"
#    exit 1
#fi
#podman login -u="${RH_REGISTRY_USER}" -p="${RH_REGISTRY_TOKEN}" registry.redhat.io

declare AUTH_CONF_DIR
AUTH_CONF_DIR="$(pwd)/.podman"
readonly AUTH_CONF_DIR

mkdir -p "${AUTH_CONF_DIR}"
export REGISTRY_AUTH_FILE="${AUTH_CONF_DIR}/auth.json"
readonly REGISTRY_AUTH_FILE

declare CLOUDSERVICES_IMAGE
CLOUDSERVICES_IMAGE="quay.io/cloudservices/edge-api"
readonly CLOUDSERVICES_IMAGE

declare CLOUDSERVICES_CONTAINERFILE
CLOUDSERVICES_CONTAINERFILE="Dockerfile"
readonly CLOUDSERVICES_CONTAINERFILE

# Build Cloudservices image
podman build \
        --file "${CLOUDSERVICES_CONTAINERFILE}" \
        --no-cache \
        --tag "${CLOUDSERVICES_IMAGE}:${IMAGE_TAG}" \
        .

# Push image to remote repository
podman push \
        "${CLOUDSERVICES_IMAGE}:${IMAGE_TAG}"

declare ADDITIONAL_TAG_LIST
ADDITIONAL_TAG_LIST="latest main qa"

# if cmd/kafka directory contents are changed, add kafka tag
#num_files=$(git log --raw -n 1 --no-merges | egrep "^:.*" | wc -l)
# shellcheck disable=SC2126
NUM_KAFKA_FILES=$(git log --raw -n 1 --no-merges | grep -E "^:.*cmd/kafka" | wc -l)
# if all changes are under cmd/kafka then only tag kafka
if [[ ${NUM_KAFKA_FILES} -gt 0 ]];
then
    #[[ num_files -eq num_kafka_files ]] && TAGS="kafka" || TAGS="$TAGS kafka"
    ADDITIONAL_TAG_LIST="${ADDITIONAL_TAG_LIST} kafka"
fi

for TAG in ${ADDITIONAL_TAG_LIST};
do
    podman tag "${CLOUDSERVICES_IMAGE}:${IMAGE_TAG}" "${CLOUDSERVICES_IMAGE}:${TAG}"
    podman push "${CLOUDSERVICES_IMAGE}:${TAG}"
done

declare FLEET_MANAGEMENT_LIBFDO_CONTAINERFILE
FLEET_MANAGEMENT_LIBFDO_CONTAINERFILE="test-container/Dockerfile"
# shellcheck disable=SC2034
readonly FLEET_MANAGEMENT_LIBFDO_CONTAINERFILE

declare FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE
FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE="Dockerfile"
# shellcheck disable=SC2034
readonly FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE

# Travis before_install
#if [ "${GITHUB_PR_NUMBER}" == false ]; # FIXME - ${IMAGE_TAG}?
#then
#   TAG="latest"
#    FLEET_MANAGEMENT_EDGE_API_IMAGE="quay.io/fleet-management/edge-api:${TAG}"
#    FLEET_MANAGEMENT_LIBFDO_IMAGE="quay.io/fleet-management/libfdo-data:${TAG}"
#else
#   TAG="pr-${GITHUB_PR_NUMBER}" # FIXME
#    FLEET_MANAGEMENT_EDGE_API_IMAGE="quay.io/fleet-management/edge-api:pr-checks:${TAG}-edge-api"
#    FLEET_MANAGEMENT_LIBFDO_IMAGE="quay.io/fleet-management/libfdo-data:pr-checks:${TAG}-libfdo"
#   echo "LABEL quay.expires-after=2d" >> "${FLEET_MANAGEMENT_CONTAINERFILE}"
#   echo "LABEL quay.expires-after=2d" >> "${"./Dockerfile
#fi

# Travis build libfdo-data
#podman build \
#       --file "${FLEET_MANAGEMENT_CONTAINERFILE}" \
#       --no-cache \
#       --tag "${FLEET_MANAGEMENT_LIBFDO_IMAGE}" \
#       .
#
#podman push \
#        "${FLEET_MANAGEMENT_LIBFDO_IMAGE}"

# Travus build and test edge-api
#sed -i 's|registry.access.redhat.com/ubi8/ubi|quay.io/centos/centos:stream8|' "${FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE}"
#sed -i 's|.*ubi-micro-build.*ubi.repo||' "${FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE}"
#sed -i "s|${FLEET_MANAGEMENT_ORG}/libfdo-data|${LIBFDO_IMAGE}|" "${FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE}"
#
#podman build \
#           --file "${FLEET_MANAGEMENT_EDGE_API_CONTAINERFILE}" \
#           --no-cache \
#           --tag ${FLEET_MANAGEMENT_EDGE_API_IMAGE} \
#           .
#
#podman push \
#       "${FLEET_MANAGEMENT_EDGE_API_IMAGE}""
