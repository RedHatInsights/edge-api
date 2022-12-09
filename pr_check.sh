#!/bin/bash

#export GOROOT="/opt/go/1.17.7" # Force Jenkins to use Go 1.17.7 since we don't have 1.18 yet
#export PATH="${GOROOT}/bin:${PATH}"

export PR_CHECK="true" # Only used when doing a PR check from Github.

# Generate coverate report for sonarqube
CONTAINER_NAME="edge-pr-check-$ghprbPullId"

# Run coverage using same version of Go as the App
podman run --rm -i \
    --name "$CONTAINER_NAME" \
    -v "$PWD:/usr/src:z" \
    registry.access.redhat.com/ubi8/go-toolset:1.18.4-8 \
    cd /usr/src && make coverage-no-fdo

# Generate sonarqube reports
#make scan_project
