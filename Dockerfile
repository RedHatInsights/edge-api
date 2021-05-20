############################
# STEP 1 build executable binary
############################
FROM registry.redhat.io/rhel8/go-toolset:latest AS builder
WORKDIR $GOPATH/src/mypackage/myapp/
COPY . .
# Use go mod
ENV GO111MODULE=on
# Fetch dependencies.
# Using go get requires root.
USER root
RUN go get -d -v
# Build the binary.
RUN CGO_ENABLED=0 go build -o /go/bin/edge-api
############################
# STEP 2 build a small image
############################
FROM registry.redhat.io/ubi8-minimal:latest

COPY --from=builder /go/bin/edge-api /usr/bin

USER 1001

CMD ["edge-api"]
