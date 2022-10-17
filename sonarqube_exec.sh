#!/bin/bash

set -o nounset

if [ -w "${HOME}/passwd" ];
then
  sed "/^jboss/s/[^:]*/$(id --user)/3" /etc/passwd > "${HOME}/passwd"
fi

if [ -w "${HOME}/group" ];
then
  sed "/^jboss/s/[^:]*/$(id --group)/3" /etc/group > "${HOME}/group"
fi

KEYSTORE="${PWD}/sonarqube/store/RH-IT-Root-CA.keystore"

"${JAVA_HOME}/bin/keytool" \
  -keystore "${KEYSTORE}" \
  -import \
  -alias "RH-IT-Root-CA" \
  -file "/${PWD}/sonarqube/certs/RH-IT-Root-CA.crt" \
  -storepass "${KEYSTORE_PASSWORD}" \
  -noprompt

export SONAR_SCANNER_OPTS="-Djavax.net.ssl.trustStore=${KEYSTORE} -Djavax.net.ssl.trustStorePassword=${KEYSTORE_PASSWORD}"
export PATH="${PWD}/sonarqube/extract/${SONAR_SCANNER_NAME}/bin:${PATH}"

export SONAR_USER_HOME='/tmp'

APP='edge-api'

SONAR_APP_DIR="${SONAR_USER_HOME}/${APP}"

mkdir "${SONAR_APP_DIR}"
cp -R '/home/jboss' "${SONAR_APP_DIR}"

cd "${SONAR_USER_HOME}" || exit 1

sonar-scanner \
  -Dsonar.projectKey='console.redhat.com:fleet-management' \
  -Dsonar.sources="./${APP}" \
  -Dsonar.host.url="${SONARQUBE_REPORT_URL}" \
  -Dsonar.projectVersion="${COMMIT_SHORT}" \
  -Dsonar.go.coverage.reportPaths="./${APP}/coverage.txt" \
  -Dsonar.login="${SONARQUBE_TOKEN}"
