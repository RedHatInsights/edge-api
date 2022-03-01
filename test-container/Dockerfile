###################################
###### Build the FDO package ######
###################################
FROM registry.access.redhat.com/ubi8/ubi AS builder
RUN curl -L https://github.com/dominikh/go-tools/releases/download/2021.1.2/staticcheck_linux_amd64.tar.gz | tar xz
RUN curl -L https://github.com/securego/gosec/releases/download/v2.9.5/gosec_2.9.5_linux_amd64.tar.gz | tar xz

RUN yum install --installroot /mnt/rootfs \
    coreutils-single glibc-minimal-langpack \
    make go git-core \
    --releasever 8 --setopt \
    install_weak_deps=false --nodocs -y; \
    yum --installroot /mnt/rootfs clean all
RUN rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.*
RUN cp ./staticcheck/staticcheck /mnt/rootfs/usr/bin/
RUN cp ./gosec /mnt/rootfs/usr/bin/

##############################################
###### Build libfdo-data test container ######
##############################################
FROM scratch
LABEL maintainer="Red Hat, Inc."
LABEL com.redhat.component="ubi8-micro-container"

# label for EULA
LABEL com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI"

# labels for container catalog
LABEL summary="edge-api micro test image"
LABEL description="The edge-api project is an API server for fleet edge management capabilities."
LABEL io.k8s.display-name="edge-api-micro-testing"

COPY --from=builder /mnt/rootfs/ /
COPY --from=builder /etc/yum.repos.d/ubi.repo /etc/yum.repos.d/ubi.repo

RUN echo -e 'if [[ ! -d "edge-api" ]];then \n\
    git clone -b "${GIT_UPSTREAM_BRANCH}" "${GIT_UPSTREAM_REMOTE}"; fi\n \ 
    make -C edge-api coverage' > test.sh
RUN chmod +x ./test.sh

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
RUN mkdir -p /usr/local/include/libfdo-data
COPY --from=quay.io/fleet-management/libfdo-data ${LD_LIBRARY_PATH}/ ${LD_LIBRARY_PATH}/
COPY --from=quay.io/fleet-management/libfdo-data /usr/local/include/libfdo-data/fdo_data.h /usr/local/include/libfdo-data/fdo_data.h
COPY --from=quay.io/fleet-management/libfdo-data /testdevice1.ov /testdevice1.ov

ARG GIT_BRANCH="main"
ARG GIT_REMOTE="https://github.com/RedHatInsights/edge-api.git"
ENV GIT_UPSTREAM_REMOTE ${GIT_REMOTE}
ENV GIT_UPSTREAM_BRANCH ${GIT_BRANCH}

CMD [ "sh", "./test.sh" ]
