#!/bin/bash
#
# $Id$
#
echo "Initializing..."

echo "Create container network"
EDGE_NETWORK_NAME='edge'
podman network create ${EDGE_NETWORK_NAME}

echo "Determine host ip address"
POSTGRES_HOSTNAME=$(ip addr show dev virbr2 | grep ' inet ' | cut -d' ' -f6 | cut -d'/' -f1)
echo "${POSTGRES_HOSTNAME}"

echo "Create host directory"
DATA_DIR="$(pwd)/data"
if [ ! -d "${DATA_DIR}" ];
then
	mkdir --verbose "${DATA_DIR}"
fi

if [ "${PGPASSWORD:-}" == '' ];
then
   echo "Please set PGPASSWORD"
   # export PGPASSWORD=$(tr --complement --delete [:graph:] < /dev/urandom | head --bytes 32)
   exit 2
fi
PGDATABASE=edge
PGPORT=5432
PGUSER=edge
POSTGRES_CONTAINER_NAME='edge_postgresql'
echo "Run database container '${POSTGRES_CONTAINER_NAME}'"
podman run \
   --detach \
   --env POSTGRESQL_DATABASE=${PGDATABASE} \
   --env POSTGRESQL_PASSWORD="${PGPASSWORD}" \
   --env POSTGRESQL_USER=${PGUSER} \
   --name ${POSTGRES_CONTAINER_NAME} \
   --network ${EDGE_NETWORK_NAME} \
   --publish ${PGPORT}:${PGPORT} \
   --volume "$(pwd)/data":/var/lib/pgsql/data:Z \
   docker://registry.redhat.io/rhel8/postgresql-12:latest

echo "Wait for database container to startup"
sleep 2

EDGE_API_CONTAINER_NAME='edge_api'
echo "Run application container '${EDGE_API_CONTAINER_NAME}'"
podman run \
   --detach \
   --env DATABASE=pgsql \
   --env PGSQL_DATABASE=${PGDATABASE} \
   --env PGSQL_HOSTNAME="${POSTGRES_HOSTNAME}" \
   --env PGSQL_PASSWORD="${PGPASSWORD}" \
   --env PGSQL_PORT=${PGPORT} \
   --env PGSQL_USER=${PGUSER} \
   --name ${EDGE_API_CONTAINER_NAME} \
   --network ${EDGE_NETWORK_NAME} \
  quay.io/cloudservices/edge-api:latest

echo "Verify container(s)"
podman ps

echo "Check postgres container logs"
podman logs ${POSTGRES_CONTAINER_NAME}

echo "Check edge_api container name"
podman logs ${EDGE_API_CONTAINER_NAME}

exit 0
#
#
#
