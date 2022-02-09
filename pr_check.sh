#!/bin/bash

export APP_NAME="edge"  # name of app-sre "application" folder this component lives in
export COMPONENT_NAME="edge-api"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
export IMAGE="quay.io/cloudservices/edge-api"  # image location on quay

export IQE_PLUGINS="edge"  # name of the IQE plugin for this app.
export IQE_MARKER_EXPRESSION="edge_smoke"  # This is the value passed to pytest -m
export IQE_FILTER_EXPRESSION=""  # This is the value passed to pytest -k
export IQE_CJI_TIMEOUT="30m"  # This is the time to wait for smoke test to complete or fail

# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > ${WORKSPACE}/cicd_bootstrap.sh && source ${WORKSPACE}/cicd_bootstrap.sh

# Build the image and push to quay
source $CICD_ROOT/build.sh

# Run the unit tests with an ephemeral db
# source $APP_ROOT/unit_test.sh

# Deploy edge to an ephemeral namespace for testing
source $CICD_ROOT/deploy_ephemeral_env.sh

# This code is to create a 'dummy' result file so Jenkins will not fail when smoke tests are disabled
#mkdir -p $ARTIFACTS_DIR
#cat << EOF > $ARTIFACTS_DIR/junit-dummy.xml
#<testsuite tests="1">
#    <testcase classname="dummy" name="dummytest"/>
#</testsuite>
#EOF

# Run smoke tests with ClowdJobInvocation
source $CICD_ROOT/cji_smoke_test.sh
