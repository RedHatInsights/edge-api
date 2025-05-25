############################################
# STEP 1: build executable edge-api binaries
############################################
FROM registry.access.redhat.com/ubi9/go-toolset:9.6-1747333074 AS edge-builder
USER root
WORKDIR $GOPATH/src/github.com/RedHatInsights/edge-api/
COPY . .

# Download dependencies
RUN go get -d -v

# Build the binary.
RUN go build -o /go/bin/edge-api

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

# Build utilities binaries
RUN go build -o /go/bin/edge-api-cleanup cmd/cleanup/main.go

####################################
# STEP 2: build edge-api minimal image
####################################
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
LABEL maintainer="Red Hat, Inc."

# label for EULA
LABEL com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI"

# labels for container catalog
LABEL summary="edge-api minimal image"
LABEL description="The edge-api project is an API server for fleet edge management capabilities."
LABEL io.k8s.display-name="edge-api-minimal"

ENV EDGE_API_WORKSPACE /src/github.com/RedHatInsights/edge-api

# Copy the edge-api binaries into the image.
COPY --from=edge-builder /go/bin/edge-api /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate /usr/bin
COPY --from=edge-builder /go/bin/edge-api-wipe /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-device /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-repositories /usr/bin
COPY --from=edge-builder /go/bin/edge-api-migrate-groups /usr/bin
COPY --from=edge-builder /go/bin/edge-api-ibvents /usr/bin
COPY --from=edge-builder /go/bin/edge-api-cleanup /usr/bin
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/cmd/spec/openapi.json /var/tmp

RUN microdnf install -y coreutils-single glibc-minimal-langpack ostree && microdnf clean all

# template to playbook dispatcher
COPY --from=edge-builder ${EDGE_API_WORKSPACE}/templates/template_playbook_dispatcher_ostree_upgrade_payload.yml /usr/local/etc

USER 1001
CMD ["edge-api"]
