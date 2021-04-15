  
#!/bin/bash

echo "os: $OSTYPE"
echo "shell: $SHELL"
export PATH=$PATH:$PWD

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="edge"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="edge-api"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/edge-api"

# IQE_PLUGINS="edge-api"
# IQE_MARKER_EXPRESSION="smoke"
# IQE_FILTER_EXPRESSION=""

echo "LABEL quay.expires-after=3d" >> ./Dockerfile # tag expire in 3 days

# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh -o bootstrap.sh
source bootstrap.sh  # checks out bonfire and changes to "cicd" dir...

echo "cicd: $CICD_ROOT"
export PATH=$PATH:$PWD

source build.sh
source deploy_ephemeral_env.sh
# source smoke_test.sh

mkdir -p $WORKSPACE/artifacts
cat << EOF > ${WORKSPACE}/artifacts/junit-dummy.xml
<testsuite tests="1">
    <testcase classname="dummy" name="dummytest"/>
</testsuite>
EOF
