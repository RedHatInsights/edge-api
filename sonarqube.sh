#!/bin/bash

set -o nounset

COMMIT_SHORT=$(git rev-parse --short=7 HEAD)

# When doing a PR check, send sonarqube results to a separate branch.
# Otherwise, send it to the default 'master' branch.
# The variable $PR_CHECK is only used when doing a PR check (see pr_check.sh).
# Both ${GIT_BRANCH}  and ${ghprbPullId} are provided by App-Interface's Jenkins.
# SonarQube parameters can be found below:
#   https://sonarqube.corp.redhat.com/documentation/analysis/pull-request/
if [[ "${PR_CHECK}" = "true" ]]; then
    export PR_CHECK_OPTS="-Dsonar.pullrequest.branch=${GIT_BRANCH} -Dsonar.pullrequest.key=${ghprbPullId} -Dsonar.pullrequest.base=main";
fi

podman run \
--pull=always --rm \
-v "${PWD}":/usr/src:z   \
-e SONAR_SCANNER_OPTS="-Dsonar.scm.provider=git \
 ${PR_CHECK_OPTS:-} \
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
