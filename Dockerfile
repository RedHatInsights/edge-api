############################################
# STEP 1: build executable edge-api binaries
############################################
FROM registry.access.redhat.com/ubi8/go-toolset:1.15.14 AS edge-builder
WORKDIR $GOPATH/src/mypackage/myapp/
COPY . .
# Use go mod
ENV GO111MODULE=on
# Fetch dependencies.
# Using go get requires root.
USER root
RUN go get -d -v

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
COPY --from=quay.io/cloudservices/edge-api:libfdo-data ${LD_LIBRARY_PATH}/libfdo_data.so.0 ${LD_LIBRARY_PATH}/libfdo_data.so.0
COPY --from=quay.io/cloudservices/edge-api:libfdo-data /usr/local/include/fdo_data.h /usr/local/include/fdo_data.h

# Build the binary.
RUN go build -o /go/bin/edge-api

# Build the migration binary.
RUN go build -o /go/bin/edge-api-migrate cmd/migrate/migrate.go

######################################
# STEP 2: build the dependencies image
######################################
FROM registry.access.redhat.com/ubi8/ubi AS ubi-micro-build
RUN mkdir -p /mnt/rootfs
# This step is needed because of subscription-manager issue. 
RUN yum install coreutils-single -y
RUN yum install --installroot /mnt/rootfs \
    coreutils-single glibc-minimal-langpack \
    pykickstart mtools xorriso genisoimage \
    syslinux isomd5sum file ostree \
    --releasever 8 --setopt \
    install_weak_deps=false --nodocs -y; \
    yum --installroot /mnt/rootfs clean all
RUN rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.*

####################################
# STEP 3: build edge-api micro image
####################################
FROM scratch
LABEL maintainer="Red Hat, Inc."
LABEL com.redhat.component="ubi8-micro-container"

# label for EULA
LABEL com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI"

# labels for container catalog
LABEL summary="edge-api micro image"
LABEL description="The edge-api project is an API server for fleet edge management capabilities."
LABEL io.k8s.display-name="edge-api-micro"

COPY --from=ubi-micro-build /mnt/rootfs/ /
COPY --from=ubi-micro-build /etc/yum.repos.d/ubi.repo /etc/yum.repos.d/ubi.repo

ENV MTOOLS_SKIP_CHECK=1

# Copy the edge-api binaries into the image.
COPY --from=edge-builder /go/bin/edge-api /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate /usr/bin
COPY --from=edge-builder /src/mypackage/myapp/cmd/spec/openapi.json /var/tmp

# kickstart inject requirements
COPY --from=edge-builder /src/mypackage/myapp/pkg/services/fleetkick.sh /usr/local/bin
RUN chmod +x /usr/local/bin/fleetkick.sh
COPY --from=edge-builder /src/mypackage/myapp/pkg/services/templateKickstart.ks /usr/local/etc

# template to playbook dispatcher
COPY --from=edge-builder /src/mypackage/myapp/pkg/services/template_playbook/template_playbook_dispatcher_ostree_upgrade_payload.yml /usr/local/etc

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
COPY --from=quay.io/cloudservices/edge-api:libfdo-data ${LD_LIBRARY_PATH}/libfdo_data.so.0 ${LD_LIBRARY_PATH}/libfdo_data.so.0
COPY --from=quay.io/cloudservices/edge-api:libfdo-data /usr/local/include/fdo_data.h /usr/local/include/fdo_data.h

USER 1001
CMD ["edge-api"]
