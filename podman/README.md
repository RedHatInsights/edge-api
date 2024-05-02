# Local Dev with Podman Compose

The files in the podman directory are meant to help with local development and are not required to run the Edge Management API or Front End. If a Red Hat OpenShift Streams for Apache Kafka (RHOSAK) instance is used, the podman-compose files and commands are not used to manage Kafka. The SASL configuration file and topic creation steps apply to both local and RHOSAK instances.

> **NOTE**: Clowder will spin up the Edge Management application and dependencies using the same configuration files as Ephemeral, Stage, and Production with Minikube. These instructions cover only using Podman Compose to spin up pieces (or all) of the application for local development and may require additional steps beyond what Clowder will do for you.

## Install Podman and Podman Compose

    sudo dnf install -y podman podman-compose

## **Dependencies**

## Environment File

The containers described below share a common environment file for setting application specifics.

A template is provided in the GitHub repo at *podman/env/edge-api_template.env*.

[FILL IN DETAILS HERE]


## PostgreSQL

### Registries

The default postgres used in the compose files is available from registry.redhat.io. You might need to authenticate to be able to pull the image.

    podman login registry.redhat.io

### Setup Data Directories

    mkdir -p ~/devdata/postgresql
    chmod 775 ~/devdata/postgresql

### Start and Stop the Postgres Container

To start and stop only the Edge API container, use the following commands:

    podman-compose -f podman/container-compose-db.yml up -d
    podman-compose -f podman/container-compose-db.yml down

> **HELPFUL HINT**: To start a container in non-daemon mode and see the log output directly, run the up command without the -d. Otherwise, you can always use the log command to see the output when running the container in daemon mode.


### Setup DB with DB Migration

To setup the tables for Edge API before starting the application, run the DB migration:

### Start and Stop the DB Migration Container

To start and stop only the Edge API container, use the following commands:

    podman-compose -f podman/container-compose-dbmigrate.yml up -d
    podman-compose -f podman/container-compose-dbmigrate.yml down

> **HELPFUL HINT**: To start a container in non-daemon mode and see the log output directly, run the up command without the -d. Otherwise, you can always use the log command to see the output when running the container in daemon mode.


## **Optional for API Development**

## Unleash

It is not necessary to run a local Unleash server. Edge API is setup to use environment variables for feature flags.

[FILL IN DETAILS HERE]


## Frontend

Although Edge API can be queried from curl or an application such as Postman, it is possible to run the Edge Frontend locally.

[FILL IN DETAILS HERE]


## **Edge Management Applications**


## API

Edge API can be run without Podman Compose, but using the compose file sets the API container up on the same podman default network as its depenedencies above.

The Edge API container uses a micro image to reduce its size. It simplifies the dev process to use a container with additional tools (bash, wget, etc.) for development purposes. A dev container file, podman/Containerfile-dev, can be used in place of the primary containers stored in Quay.

### Building the Edge API Dev Container

To generate a local dev container, *cd* into the root of the Edge API git repo and run:

    podman build -t localhost/edge-api:fedora -f podman/Containerfile-fedora .

The dev container is now available for the Edge API compose file.

### Setup Data Directories

Set up the data directories that will be mounted as volumes with the following commands:

    mkdir -p ~/devgo
    chmod 775 ~/devgo
    mkdir -p ~/devdata/repos
    mkdir ~/devdata/vartmp
    mkdir ~/devdata/gocache
    chmod -R 775 ~/devdata/

> **NOTE** To place the devdata directories in a different location, edit the compose files to match.

### Start and Stop the Edge API Dev Container

To start and stop only the Edge API container, use the following commands:

    podman-compose -f podman/container-compose-api.yml up -d
    podman-compose -f podman/container-compose-api.yml down

> **HELPFUL HINT**: To start a container in non-daemon mode and see the log output directly, run the up command without the -d. Otherwise, you can always use the log command to see the output when running the container in daemon mode.

### Execute Commands on the Kafka Container

This is helpful for troubleshooting problems: e.g., to run /bin/bash to access the container and run commands directly.

    podman-compose -f podman/container-compose-api.yml exec edge-api-service /bin/bash


## Helpful Tools

### API Calls with Postman

[FILL IN DETAILS HERE]

### API Calls with Curl

[FILL IN DETAILS HERE]

### Database Queries with pgAdmin4

[FILL IN DETAILS HERE]


## Kafka

> **NOTE**: This Kafka section was used for the Event-Driven Architecture dev work. It's only current use would be to fake Inventory create/update/delete events.

### Recommended Reading Before You Start

#### Kafka Documentation

[Apache Kafka Documentation](https://kafka.apache.org/documentation/)

#### RHOSAK Documentation

[Getting started with Red Hat OpenShift Streams for Apache Kafka](https://access.redhat.com/documentation/en-us/red_hat_openshift_streams_for_apache_kafka/1/guide/f351c4bd-9840-42ef-bcf2-b0c9be4ee30a)

[Configuring and connecting Kafka scripts with Red Hat OpenShift Streams for Apache Kafka](https://access.redhat.com/documentation/en-us/red_hat_openshift_streams_for_apache_kafka/1/guide/c0ab8d79-8b74-4876-955d-6d5b6912a966)

[Red Hat RHOSAK: Learn By Doing](https://developers.redhat.com/products/red-hat-openshift-streams-for-apache-kafka/hello-kafka?extIdCarryOver=true&sc_cid=701f2000001OH7JAAW)


### Setup Data Directories

To persist Kafka configuration, topics, and events, the compose file configures Podman to mount local data directories to specific mountpoints in the container(s). The Kafka data directories are by default configured to be at */home/YOUR_HOMEDIR/dev/kafkadata*. They can be placed anywhere, but you will need to update the compose files to use your custom location.

    mkdir -p ~/dev/kafkadata/kraft-combined-logs
    mkdir -p ~/dev/kafkadata/logs

### Start Kafka and KafkaUI

#### Start the Services

    podman-compose -f podman/container-compose-kafka.yml up -d

#### Check the Services

To list the running containers, use the following command:

    podman ps

To follow the log for the Kafka pod, run the following command:
> **NOTE**: Press Ctrl-C to exit

    podman logs -f kafka

### Create Topics via kafka-topics.sh

Before spinning up the Edge API to use Kafka, you will need to create some topics.

> **NOTE**: This list of topics will change as we implement our Event Driven Architecture (EDA). The current list can be referenced in the deploy/clowdapp.yml file.

Topics can be created with the *kafka-topics.sh* script provided in the bin directory of the Kafka container. The following command examples will create local topics. If using a RHOSAK Kafka instance, replacing localhost:9092 with the appropriate broker hostname and port will create the topics on that instance.

    podman-compose -f podman/container-compose-kafka.yml exec kafka bin/kafka-topics.sh --create --topic platform.edge.fleetmgmt.image-build --bootstrap-server localhost:9092

    podman-compose -f podman/container-compose-kafka.yml exec kafka bin/kafka-topics.sh --create --topic platform.edge.fleetmgmt.image-iso-build --bootstrap-server localhost:9092

    podman-compose -f podman/container-compose-kafka.yml exec kafka bin/kafka-topics.sh --create --topic platform.edge.fleetmgmt.device-update --bootstrap-server localhost:9092

    podman-compose -f podman/container-compose-kafka.yml exec kafka bin/kafka-topics.sh --create --topic platform.inventory.events --bootstrap-server localhost:9092

    podman-compose -f podman/container-compose-kafka.yml exec kafka bin/kafka-topics.sh --create --topic platform.playbook-dispatcher.runs --bootstrap-server localhost:9092

### Environment File

To make the config file described in the next section available to Edge API for configuration of Kafka, add the following to the *edge-api.env* file.

If using a local Kafka without authentication:

    EDGEMGMT_CONFIG=/tmp/edgemgmt_config.json

If using a RHOSAK Kafka instance:

    EDGEMGMT_CONFIG=/tmp/edgemgmt_config_sasl.json

> **NOTE**: See the sections below for more information on local and RHOSAK Kafka instances.

### Config Files

Kafka configuration in the consoledot space requires authentication. Typically, running Kafka on your local dev device does not. The configuration in the example *edgemgmt_config.json* file will connect to a local Kafka instance without authentication.

The topics section of the following config files should always reflect the topics listed in the clowdapp.yml file. When running Edge API where Clowder is unavailable, these config files mock the same config format provided by Clowder.

podman/env/edgemgmt_config.json

    {
        "kafka": {
            "brokers": [
                {
                    "hostname": "kafka",
                    "port": 9092,
                }
            ],
            "topics": [
                {
                    "requestedName": "platform.edge.fleetmgmt.image-build",
                    "name": "platform.edge.fleetmgmt.image-build"
                },
                {
                    "requestedName": "platform.edge.fleetmgmt.image-iso-build",
                    "name": "platform.edge.fleetmgmt.image-iso-build"
                },
                {
                    "requestedName": "platform.playbook-dispatcher.runs",
                    "name": "platform.playbook-dispatcher.runs"
                },
                {
                    "requestedName": "platform.inventory.events",
                    "name": "platform.inventory.events"
                }
            ]
        }
    }

The *edgemgmt_config_sasl.json* file provides the additional authentication information. Replace Username and Password with the information provided when you created your RHOSAK Service Account.

podman/env/edgemgmt_config_sasl.json

    {
        "kafka": {
            "brokers": [
                {
                    "hostname": "<INSERT RHOSAK BROKER HOST HERE>",
                    "port": "<INSERT RHOSAK PORT HERE>,
                    "sasl": {
                        "SaslMechanism": "PLAIN",
                        "SecurityProtocol": "SASL_SSL",
                        "Username": "<INSERT RHOSAK SERVICE ACCT USER>",
                        "Password": "<INSERT RHOSAK SERVICE ACCT PASSWORD>"
                    }
                }
            ],
            "topics": [
                {
                    "requestedName": "platform.edge.fleetmgmt.image-build",
                    "name": "platform.edge.fleetmgmt.image-build"
                },
                {
                    "requestedName": "platform.edge.fleetmgmt.image-iso-build",
                    "name": "platform.edge.fleetmgmt.image-iso-build"
                },
                {
                    "requestedName": "platform.playbook-dispatcher.runs",
                    "name": "platform.playbook-dispatcher.runs"
                },
                {
                    "requestedName": "platform.inventory.events",
                    "name": "platform.inventory.events"
                }
            ]
        }
    }


### Stop Kafka and KafkaUI

To stop both the Kafka and KafkaUI containers, use the following command:

    podman-compose -f podman/container-compose-kafka.yml down

### Execute Commands on the Kafka Container

This is helpful for troubleshooting problems or running Kafka-related scripts. e.g., kafka-topics.sh to create topics or /bin/bash to access the container and run commands directly.

    podman-compose -f podman/container-compose-kafka.yml exec kafka /bin/bash

### Start and Stop Kafka Only

To start and stop only the Kafka container, use the following commands:

    podman-compose -f podman/container-compose-kafka.yml up -d kafka
    podman-compose -f podman/container-compose-kafka.yml down kafka

> **HELPFUL HINT**: To start a container in non-daemon mode and see the log output directly, run the up command without the -d. Otherwise, you can always use the log command to see the output when running the container in daemon mode.

### Start and Stop KafkaUI Only

To start and stop only the KafkaUI container, use the following commands:

    podman-compose -f podman/container-compose-kafka.yml up -d kafka-ui
    podman-compose -f podman/container-compose-kafka.yml down kafka-ui

## Using KafkaUI

KafkaUI can be used to manage some features of a Kafka instance. It provides a web UI for tasks such as creating topics, creating messages on a topic, and watching a live stream of events coming across a topic.

Once the KafkaUI container is running, you can navigate to the Web UI via the URL:

    http://localhost:8090/

For more information on KafkaUI, see https://github.com/provectus/kafka-ui


## **Edge Management Applications**

## Utility -- *aka (and soon to be formerly) IBvents*

## Microservices
