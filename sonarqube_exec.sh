#!/bin/bash

if [ "${COMMIT_SHORT}" == '' ];
then
  echo "COMMIT_SHORT environment variable not set"
  exit 1
fi

if [ "${KEYSTORE_PASSWORD}" == '' ];
then
  echo "KEYSTORE_PASSWORD environment variable not set"
  exit 1
fi

if [ "${SONAR_SCANNER_NAME}" == '' ];
then
  echo "SONAR_SCANNER_NAME environment variable not set"
  exit 1
fi

if [ "${SONARQUBE_REPORT_URL}" == '' ];
then
  echo "SONARQUBE_REPORT_URL environment variable not set"
  exit 1
fi

if [ "${SONARQUBE_TOKEN}" == '' ];
then
  echo "SONARQUBE_TOKEN environment variable not set"
  exit 1
fi

if [ ! -r "/home/jboss/passwd" ];
then
  ln -s /etc/passwd /home/jboss/passwd
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
  -Dsonar.login="${SONARQUBE_TOKEN}"

rm /home/jboss/passwd
