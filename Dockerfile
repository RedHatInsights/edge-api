############################################
# STEP 1: build executable edge-api binaries
############################################
FROM registry.access.redhat.com/ubi8/go-toolset:1.20 AS edge-builder
WORKDIR $GOPATH/src/github.com/RedHatInsights/edge-api/
COPY . .
# Use go mod
ENV GO111MODULE=on
# Fetch dependencies.
# Using go get requires root.
USER root
RUN go get -d -v

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
RUN mkdir -p /usr/local/include/libfdo-data
COPY --from=quay.io/fleet-management/libfdo-data ${LD_LIBRARY_PATH}/ ${LD_LIBRARY_PATH}/
COPY --from=quay.io/fleet-management/libfdo-data /usr/local/include/libfdo-data/fdo_data.h /usr/local/include/libfdo-data/fdo_data.h

# Build the binary.
RUN go build -tags=fdo -o /go/bin/edge-api

# Build the migration binary.
RUN go build -o /go/bin/edge-api-migrate cmd/migrate/main.go
RUN go build -o /go/bin/edge-api-wipe cmd/db/wipe.go
RUN go build -o /go/bin/edge-api-migrate-device cmd/db/updDb/set_account_on_device.go
RUN go build -o /go/bin/edge-api-migrate-repositories cmd/migraterepos/main.go
RUN go build -o /go/bin/edge-api-migrate-groups cmd/migrategroups/main.go

# Run the doc binary
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN ~/go/bin/swag init --generalInfo api.go --o ./cmd/spec/ --dir pkg/models,pkg/routes --parseDependency
RUN go run cmd/swagger2openapi/main.go  cmd/spec/swagger.json cmd/spec/openapi.json

# Build the microservice binaries
RUN go build -o /go/bin/edge-api-ibvents cmd/kafka/main.go
RUN go build -o /go/bin/edge-api-images-build pkg/services/images_build/main.go
RUN go build -o /go/bin/edge-api-isos-build pkg/services/images_iso/main.go

# Build utilities binaries
RUN go build -o /go/bin/edge-api-cleanup cmd/cleanup/main.go

######################################
# STEP 2: build the dependencies image
######################################
FROM registry.access.redhat.com/ubi8/ubi AS ubi-micro-build
RUN mkdir -p /mnt/rootfs
# This step is needed for subscription-manager refresh.
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
ENV EDGE_API_WORKSPACE /src/github.com/RedHatInsights/edge-api

# Copy the edge-api binaries into the image.
COPY --from=edge-builder /go/bin/edge-api /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate /usr/bin
COPY --from=edge-builder /go/bin/edge-api-wipe /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-device /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-repositories /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-groups /usr/bin
COPY --from=edge-builder /go/bin/edge-api-ibvents /usr/bin
COPY --from=edge-builder /go/bin/edge-api-images-build /usr/bin
COPY --from=edge-builder /go/bin/edge-api-isos-build /usr/bin
COPY --from=edge-builder /go/bin/edge-api-cleanup /usr/bin
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/cmd/spec/openapi.json /var/tmp

# kickstart inject requirements
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/scripts/fleetkick.sh /usr/local/bin
RUN chmod +x /usr/local/bin/fleetkick.sh
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/templates/templateKickstart.ks /usr/local/etc

# template to playbook dispatcher
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/templates/template_playbook_dispatcher_ostree_upgrade_payload.yml /usr/local/etc

# interim FDO requirements
ENV LD_LIBRARY_PATH /usr/local/lib
RUN mkdir -p /usr/local/include/libfdo-data
COPY --from=edge-builder ${LD_LIBRARY_PATH}/ ${LD_LIBRARY_PATH}/
COPY --from=edge-builder /usr/local/include/libfdo-data/fdo_data.h /usr/local/include/libfdo-data/fdo_data.h

USER 1001
CMD ["edge-api"]
