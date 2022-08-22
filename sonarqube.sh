#!/bin/bash

set -x

mkdir "${PWD}/sonarqube/"
mkdir "${PWD}/sonarqube/download/"
mkdir "${PWD}/sonarqube/extract/"
mkdir "${PWD}/sonarqube/certs/"
mkdir "${PWD}/sonarqube/store/"

RH_IT_ROOT_CA_CRT="${PWD}/sonarqube/certs/RH-IT-Root-CA.crt"
EXPECTED_SHA1_FINGERPRINT='SHA1 Fingerprint=E0:A7:13:80:9D:96:3E:EE:5F:8B:74:24:74:8D:EF:3D:0C:0F:C4:0E'

if [ "${ROOT_CA_CERT_URL:-}" == '' ];
then
  echo "ROOT_CA_CERT_URL is not defined"
  exit 1
fi

curl --output "${RH_IT_ROOT_CA_CRT}" "${ROOT_CA_CERT_URL}"
FOUND_SHA1_FINGERPRINT="$(openssl x509 -fingerprint -in "${RH_IT_ROOT_CA_CRT}" -noout | grep "^${EXPECTED_SHA1_FINGERPRINT}$")"
if [ "${EXPECTED_SHA1_FINGERPRINT}" != "${FOUND_SHA1_FINGERPRINT}" ];
then
  echo "Fingerprints do not match:"
  echo -e "\tExpecting '$EXPECTED_SHA1_FINGERPRINT}"
  echo -e "\tFound: '${FOUND_SHA1_FINGERPRINT}'"
  exit 2
fi

if [ "${BUILD_NUMBER:-}" == '' ];
then
  sudo mv -i -v "${RH_IT_ROOT_CA_CRT}" "/etc/pki/ca-trust/source/anchors/"
  sudo update-ca-trust extract
fi

KEYSTORE_PASSWORD="$(openssl rand -base64 32)"

KEYSTORE_PATH="${PWD}/sonarqube/store/RH-IT-Root-CA.keystore"
"${JAVA_HOME}/bin/keytool" \
  -keystore "${KEYSTORE_PATH}" \
  -import \
  -alias "RH-IT-Root-CA" \
  -file "${RH_IT_ROOT_CA_CRT}" \
  -storepass "${KEYSTORE_PASSWORD}" \
  -noprompt

export SONAR_SCANNER_OPTS="-Djavax.net.ssl.trustStore=${KEYSTORE_PATH} -Djavax.net.ssl.trustStorePassword=${KEYSTORE_PASSWORD}"
export SONAR_SCANNER_OS="linux"
export SONAR_SCANNER_CLI_VERSION="4.7.0.2747"
export SONAR_SCANNER_DOWNLOAD_NAME="sonar-scanner-cli-${SONAR_SCANNER_CLI_VERSION}-${SONAR_SCANNER_OS}"
export SONAR_SCANNER_NAME="sonar-scanner-${SONAR_SCANNER_CLI_VERSION}-${SONAR_SCANNER_OS}"

curl --output "${PWD}/sonarqube/download/${SONAR_SCANNER_DOWNLOAD_NAME}.zip" "${SONARQUBE_CLI_URL}"

unzip -d "${PWD}/sonarqube/extract/" "${PWD}/sonarqube/download/${SONAR_SCANNER_DOWNLOAD_NAME}.zip"

export PATH="${PWD}/sonarqube/extract/${SONAR_SCANNER_NAME}/bin:${PATH}"

COMMIT_SHORT=$(git rev-parse --short=7 HEAD)

OPENJDK_CONTAINER_IMAGE='registry.redhat.io/ubi8/openjdk-11-runtime:latest'

podman pull "${OPENJDK_CONTAINER_IMAGE}"

{ \
  echo "COMMIT_SHORT=${COMMIT_SHORT}";
  echo "KEYSTORE_PASSWORD=${KEYSTORE_PASSWORD}";
  echo "SONAR_SCANNER_NAME=${SONAR_SCANNER_NAME}";
  echo "SONARQUBE_REPORT_URL=${SONARQUBE_REPORT_URL}";
  echo "SONARQUBE_TOKEN=${SONARQUBE_TOKEN}";
} >> "${PWD}/sonarqube/my-env.txt"

#chcon --recursive --type container_file_t --verbose "${PWD}"
podman run \
    --volume "${PWD}":/home/jboss:z \
    --env-file "${PWD}/sonarqube/my-env.txt" \
    "${OPENJDK_CONTAINER_IMAGE}" \
    /bin/bash "sonarqube_exec.sh"

mkdir -p "${WORKSPACE}/artifacts"
cat << EOF > "${WORKSPACE}/artifacts/junit-dummy.xml"
<testsuite tests="1">
    <testcase classname="dummy" name="dummytest"/>
</testsuite>
EOF
