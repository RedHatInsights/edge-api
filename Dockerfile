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
FROM quay.io/centos/centos:8 AS ubi-micro-build
RUN mkdir -p /mnt/rootfs
RUN yum install --installroot /mnt/rootfs \
    pykickstart mtools xorriso genisoimage \
    syslinux isomd5sum file ostree \
    --releasever 8 --setopt \
    install_weak_deps=false --nodocs -y; \
    yum --installroot /mnt/rootfs clean all
RUN rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.*

####################################
# STEP 3: build edge-api micro image
####################################
FROM registry.access.redhat.com/ubi8/ubi-micro
ENV MTOOLS_SKIP_CHECK=1
ENV PATH /usr/bin:/usr/local/bin:/mnt/rootfs/usr/bin:/mnt/rootfs/usr/local/bin
ENV LD_LIBRARY_PATH /usr/local/lib:/usr/local/lib64:/mnt/rootfs/usr/local/lib:/mnt/rootfs/usr/local/lib64

# Copy the edge-api dependencies into the image.
COPY --from=ubi-micro-build /mnt/rootfs/ /

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
COPY --from=quay.io/cloudservices/edge-api:libfdo-data /usr/local/lib/libfdo_data.so.0 /usr/local/lib/libfdo_data.so.0
COPY --from=quay.io/cloudservices/edge-api:libfdo-data /usr/local/include/fdo_data.h /usr/local/include/fdo_data.h

USER 1001
CMD ["edge-api"]
