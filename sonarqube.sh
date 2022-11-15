#!/bin/bash

set -o nounset

COMMIT_SHORT=$(git rev-parse --short=7 HEAD)

podman run \
--pull=always --rm \
-v "${PWD}":/usr/src:z   \
-e SONAR_SCANNER_OPTS="-Dsonar.scm.provider=git \
 -Dsonar.working.directory=/tmp \
 -Dsonar.projectKey=console.redhat.com:fleet-management \
 -Dsonar.projectVersion=${COMMIT_SHORT} \
 -Dsonar.sources=/usr/src/. \
 -Dsonar.tests=/usr/src/. \
 -Dsonar.test.inclusions=**/*_test.go \
 -Dsonar.go.tests.reportPaths=/usr/src/coverage.json \
 -Dsonar.go.coverage.reportPaths=/usr/src/coverage.txt \
 -Dsonar.exclusions=**/*_test.go,**/*.html,**/*.yml,**/*.yaml,**/*.json,**/*suite*,**/cmd/db*,**/cmd/kafka*,**/unleash*,**/errors*,**/mock_*" \
images.paas.redhat.com/alm/sonar-scanner-alpine:4.7.0.2747-5ec0a15 -X

mkdir -p "${WORKSPACE}/artifacts"
cat << @EOF > "${WORKSPACE}/artifacts/junit-dummy.xml"
<testsuite tests="1">
    <testcase classname="dummy" name="dummytest"/>
</testsuite>
@EOF

# Archive coverage artifacts in Jenkins
 cp $PWD/coverage* $WORKSPACE/artifacts/.