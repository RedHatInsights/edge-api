#!/bin/bash

"${JAVA_HOME}/bin/keytool" \
  -keystore "/${PWD}/sonarqube/store/RH-IT-Root-CA.keystore" \
  -import \
  -alias "RH-IT-Root-CA" \
  -file "/${PWD}/sonarqube/certs/RH-IT-Root-CA.crt" \
  -storepass 'redhat' \
  -noprompt

export SONAR_SCANNER_OPTS="-Djavax.net.ssl.trustStore=${PWD}/sonarqube/store/RH-IT-Root-CA.keystore -Djavax.net.ssl.trustStorePassword=redhat"
export PATH="${PWD}/sonarqube/extract/${SONAR_SCANNER_NAME}/bin:${PATH}"

export SONAR_USER_HOME='/tmp'
mkdir '/tmp/edge-management'
cp -R '/home/jboss/edge-management' /tmp/edge-management

cd '/tmp' || exit 1

sonar-scanner \
  -Dsonar.projectKey='console.redhat.com:fleet-managment' \
  -Dsonar.sources='./edge-management' \
  -Dsonar.host.url="${SONARQUBE_REPORT_URL}" \
  -Dsonar.projectVersion="${COMMIT_SHORT}" \
  -Dsonar.login="${SONARQUBE_TOKEN}"
