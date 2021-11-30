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

IQE_PLUGINS="edge"
IQE_MARKER_EXPRESSION="edge_smoke"
IQE_FILTER_EXPRESSION=""

echo "LABEL quay.expires-after=3d" >> ./Dockerfile # tag expire in 3 days

# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh

source $CICD_ROOT/build.sh
source $CICD_ROOT/deploy_ephemeral_env.sh
source $CICD_ROOT/cji_smoke_test.sh
