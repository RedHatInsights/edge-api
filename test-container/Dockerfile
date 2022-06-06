# Multistage build Dockerfile
#
############################################
###### Stage 1: Build the FDO package ######
############################################
# Use full ubi8 as builder environment
FROM registry.access.redhat.com/ubi8/ubi AS builder
# Download specific version of staticcheck
RUN curl -L https://github.com/dominikh/go-tools/releases/download/2021.1.2/staticcheck_linux_amd64.tar.gz | tar xz
# Download specific version of gosec
RUN curl -L https://github.com/securego/gosec/releases/download/v2.9.5/gosec_2.9.5_linux_amd64.tar.gz | tar xz

# Install RHEL 8 utilities to /mnt/rootfs
RUN yum install --installroot /mnt/rootfs \
    coreutils-single glibc-minimal-langpack \
    make go git-core \
    --releasever 8 --setopt \
    install_weak_deps=false --nodocs -y; \
    yum --installroot /mnt/rootfs clean all
# Cleanup cache files and logfiles
RUN rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.*
# Move the downloaded version of staticcheck binary so Stage 2 can find it
RUN cp ./staticcheck/staticcheck /mnt/rootfs/usr/bin/
# Similarly, move the downloaded version of gosec binary so Stage 2 can find it
RUN cp ./gosec /mnt/rootfs/usr/bin/

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
LABEL com.redhat.component="ubi8-micro-container"

# label for EULA
LABEL com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI"

# label for container catalog (short description of the Stage 2 container)
LABEL summary="edge-api micro test image"
# Label for full description of Stage 2 container
LABEL description="The edge-api project is an API server for fleet edge management capabilities."
# Label for container name displayed in Kubernetes/OpenShift for Stage 2 container
LABEL io.k8s.display-name="edge-api-micro-testing"

# Copy /mnt/rootfs contents over from Stage 1 to Stage 2 container
COPY --from=builder /mnt/rootfs/ /
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
# Location of required dynamic shared libraries used by FDO
ENV LD_LIBRARY_PATH /usr/local/lib
# Create directory for required FDO include file in Stage 2 container
RUN mkdir -p /usr/local/include/libfdo-data
# Copy over required dynamic shared libraries to Stage 2 container
COPY --from=quay.io/fleet-management/libfdo-data ${LD_LIBRARY_PATH}/ ${LD_LIBRARY_PATH}/
# Copy over required FDO include file to Stage 2 container
COPY --from=quay.io/fleet-management/libfdo-data /usr/local/include/libfdo-data/fdo_data.h /usr/local/include/libfdo-data/fdo_data.h
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
