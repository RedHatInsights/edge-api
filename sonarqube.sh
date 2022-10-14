#!/bin/bash

set -o nounset

COMMIT_SHORT=$(git rev-parse --short=7 HEAD)

SONARSCANNER_CONTAINER_IMAGE="images.paas.redhat.com/alm/sonar-scanner:latest"

podman pull "${SONARSCANNER_CONTAINER_IMAGE}"

podman run \
    --volume "${PWD}":/home/jboss:z \
    "${SONARSCANNER_CONTAINER_IMAGE}" \
    sonar-scanner -X \
    -Dsonar.working.directory="/tmp" \
    -Dsonar.projectKey="console.redhat.com:fleet-management" \
    -Dsonar.sources="/home/jboss/." \
    -Dsonar.projectVersion="${COMMIT_SHORT}" \
    -Dsonar.go.coverage.reportPaths="/home/jboss/coverage.txt"

mkdir -p "${WORKSPACE}/artifacts"
cat << @EOF > "${WORKSPACE}/artifacts/junit-dummy.xml"
<testsuite tests="1">
    <testcase classname="dummy" name="dummytest"/>
</testsuite>
@EOF

# Archive coverage artifacts in Jenkins
cp $PWD/coverage* $WORKSPACE/artifacts/.
