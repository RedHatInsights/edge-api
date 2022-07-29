# Multistage build Dockerfile
#
# Build-time argument(s):
#   RHEL_VERSION RHEL UBI version to use (default = 8)

############################################
###### Stage 1: Build the FDO package ######
############################################

# Use full ubi as builder environment
ARG RHEL_VERSION=8
FROM registry.access.redhat.com/ubi${RHEL_VERSION}/ubi AS builder

# Download latest version of staticcheck
RUN curl --location --url https://github.com/dominikh/go-tools/releases/latest/download/staticcheck_linux_amd64.tar.gz | tar xz
# staticcheck_linux_amd64.tar.gz.sha256

# Download latest version of gosec
RUN curl --location --silent --url $(curl --location --silent --url "https://api.github.com/repos/securego/gosec/releases/latest" | grep '"browser_download_url":' | grep '/gosec_[0-9.]*_linux_amd64.tar.gz"$' | cut -d\" -f4 ) | tar xz
# _linux_amd64.tar.gz.sig

# Install minimum RHEL utilities needed by test container into install root
ENV INSTALL_ROOT=/mnt/rootfs
ARG RHEL_VERSION=8
RUN yum install --installroot ${INSTALL_ROOT} \
    coreutils-single glibc-minimal-langpack \
    make go git-core \
    --releasever ${RHEL_VERSION} --setopt \
    install_weak_deps=false --nodocs -y; \
    yum --installroot ${INSTALL_ROOT} clean all
# Cleanup cache files and logfiles
RUN rm -rf ${INSTALL_ROOT}/var/cache/* ${INSTALL_ROOT}/var/log/dnf* ${INSTALL_ROOT}/var/log/yum.*
# Move the downloaded version of staticcheck binary so Stage 2 can find it
RUN cp ./staticcheck/staticcheck ${INSTALL_ROOT}/usr/bin/
# Similarly, move the downloaded version of gosec binary so Stage 2 can find it
RUN cp ./gosec ${INSTALL_ROOT}/usr/bin/

#######################################################
###### Stage 2: Build libfdo-data test container ######
#######################################################
# Super-minimal image for building base image (Stage 2 container)
# It is a no-op
FROM scratch

# Add desired Stage 2 container labels
# Author field for generated container images
LABEL maintainer="Red Hat, Inc."
# RedHat Bugzilla component name to file bugs against this container
ARG RHEL_VERSION=8
LABEL com.redhat.component="ubi${RHEL_VERSION}-micro-container"

# label for EULA
LABEL com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI"

# label for container catalog (short description of the Stage 2 container)
LABEL summary="edge-api micro test image"
# Label for full description of Stage 2 container
LABEL description="The edge-api project is an API server for fleet edge management capabilities."
# Label for container name displayed in Kubernetes/OpenShift for Stage 2 container
LABEL io.k8s.display-name="edge-api-micro-testing"

# Copy install root contents over from Stage 1 to Stage 2 container
COPY --from=builder /mnt/rootfs /
# Copy ubi repository over from Stage 1 to Stage 2 container
COPY --from=builder /etc/yum.repos.d/ubi.repo /etc/yum.repos.d/ubi.repo

# If edge-api directory doesn't exist, clone it to Stage 2 container
# and setup test.sh script
RUN echo -e 'if [[ ! -d "edge-api" ]];then \n\
    git clone -b "${GIT_UPSTREAM_BRANCH}" "${GIT_UPSTREAM_REMOTE}"; fi\n \
    make -C edge-api coverage' > test.sh
# Make test.sh script executable in Stage 2 container
RUN chmod +x ./test.sh

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
ENV FDO_INCLUDE_DIR /usr/local/include/libfdo-data
# Create directory for required FDO include file in Stage 2 container
RUN mkdir -p ${FDO_INCLUDE_DIR}
# Copy over required dynamic shared libraries to Stage 2 container
COPY --from=quay.io/fleet-management/libfdo-data ${LD_LIBRARY_PATH}/ ${LD_LIBRARY_PATH}/
# Copy over required FDO include file to Stage 2 container
COPY --from=quay.io/fleet-management/libfdo-data ${FDO_INCLUDE_DIR}/fdo_data.h ${FDO_INCLUDE_DIR}/fdo_data.h
# Copy over FDO test device for ownership vouchers to Stage 2 container
COPY --from=quay.io/fleet-management/libfdo-data /testdevice1.ov /testdevice1.ov

# Set desired git branch to test against
ARG GIT_BRANCH="main"
# Set location of edge-api repository URL
ARG GIT_REMOTE="https://github.com/RedHatInsights/edge-api.git"
# Set location of upstream URL
ENV GIT_UPSTREAM_REMOTE ${GIT_REMOTE}
# Set upstream branch
ENV GIT_UPSTREAM_BRANCH ${GIT_BRANCH}

# Default entrypoint is to run test.sh script within the existing shell
CMD [ "sh", "./test.sh" ]
