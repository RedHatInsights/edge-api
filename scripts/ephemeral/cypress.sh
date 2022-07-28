#!/bin/bash
set -e
set -x
oc project ${NAMESPACE}
echo ${NAMESPACE}
script_directory="$( dirname -- "$0"; )";

export UI_URL=`oc get route front-end-aggregator -o jsonpath='https://{.spec.host}{"\n"}' -n $NAMESPACE`
export IQE_IMAGE="quay.io/cloudservices/automation-analytics-cypress-image:latest"
export CYPRESS_RECORD_KEY=cfxxxxfd-402d-4da1-a3ad-f5f8xxxxfff2
export IQE_SERVICE_ACCOUNT=$(oc get serviceaccount | grep iqe | awk '{print $1}')

sed "s/IQE_SERVICE_ACCOUNT/$IQE_SERVICE_ACCOUNT/g" -i $script_directory/cypress.yml
oc apply -f $script_directory/cypress.yml -n $NAMESPACE

RUNNING=$(oc get pod cypress | tail -n 1 | awk '{print $3}')
while [ "$RUNNING" != "Running" ]; do
    echo "Waiting for cypress pod.."
    sleep 10
    RUNNING=$(oc get pod cypress | tail -n 1 | awk '{print $3}')
done


rm -rf /tmp/edge-frontend
git clone --depth 1 --branch master https://github.com/RedHatInsights/edge-frontend.git /tmp/edge-frontend
cd /tmp/edge-frontend
# git fetch origin pull/$ghprbPullId/head:pr-$ghprbPullId
# git checkout pr-$ghprbPullId

cat >/tmp/edge-frontend/cypress_run.sh <<EOL
export CYPRESS_RECORD_KEY=${CYPRESS_RECORD_KEY}
export CYPRESS_ProjectID=who-knows
export CYPRESS_RECORD=true
export CYPRESS_USERNAME=jdoe
export CYPRESS_PASSWORD=redhat
export CYPRESS_defaultCommandTimeout=10000
export CYPRESS_baseUrl=$UI_URL/beta/ansible/edge
cd /tmp/edge-frontend
npm ci
/src/node_modules/cypress/bin/cypress run integration --record --key ${CYPRESS_RECORD_KEY} --browser chrome --headless
EOL

chmod +x /tmp/edge-frontend/cypress_run.sh
oc rsync /tmp/edge-frontend cypress:/tmp/

oc exec -n ${NAMESPACE} cypress -- bash -c "/tmp/edge-frontend/cypress_run.sh"