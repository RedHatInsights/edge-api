# Edge API

[![Build Status](https://app.travis-ci.com/RedHatInsights/edge-api.svg?branch=main)](https://app.travis-ci.com/RedHatInsights/edge-api)
[![codecov](https://codecov.io/gh/RedHatInsights/edge-api/branch/main/graph/badge.svg?token=1CEO1MRGUB)](https://codecov.io/gh/RedHatInsights/edge-api)

## Overview

- [Getting started](#intro)
- [Development](#development)

## Getting Started

The **edge-api** project is an API server for fleet edge management capabilities. The API server will provide [Restful web services](https://www.redhat.com/en/topics/api/what-is-a-rest-api)

### Project Architecture

Below you can see where the `edge-api` application sits in respect to the interaction with the user and the device at the edge to be managed.

```text
                                          ┌──────────┐
                                          │   USER   │
                                          └────┬─────┘
                                               │
                                               │
                                               │
┌──────────────────────────────────────────────┼─────────────────────────────────────────────────────┐
│                                              │                                                     │
│     cloud.redhat.com                ┌────────▼──────────┐                                          │
│                                     │     3-Scale       │                                          │
│                                     │   API Management  │                                          │
│                                     └────────┬──────────┘                                          │
│                                              │                                                     │
│                                              │                                                     │
│                                              │                                                     │
│                                      ┌───────▼────────┐                                            │
│                                      │   edge-api     │                                            │
│                                      │  application   │                                            │
│                                      └───────┬────────┘                                            │
│                                              │                                                     │
│                work requests                 │                    playbook run results             │
│                 (playbooks)                  │                   (ansible runner events)           │
│                                    ┌─────────▼───────────┐                                         │
│            ┌───────────────────────┤ playbook-dispatcher │◄──────────────────────────────┐         │
│            │                       │     application     │                               │         │
│            │                       └─────────────────────┘                               │         │
│            │                                                                             │         │
│   ┌────────▼────────┐                                                              ┌─────┴────┐    │
│   │ cloud-connector │                                                              │ ingress  │    │
│   │     service     │                                                              │ service  │    │
│   └────────┬────────┘                                                              └─────▲────┘    │
│            │                                                                             │         │
└────────────┼─────────────────────────────────────────────────────────────────────────────┼─────────┘
             │                                                                             │
        ┌────▼─────┐                                                                       │
        │   MQTT   │                                                                       │
        │  BROKER  │                                                                       │
        └────┬─────┘                                                              uploads  │
             │                                                                    (http)   │
             │                    ┌───────────────────────────┐                            │
             │                    │                           │                            │
             │                    │      Connected Host       │                            │
     signals │                    │   ┌───────────────────┐   │                            │
     (mqtt)  │                    │   │    RHC Client     │   │                            │
             │                    │   │ ┌───────────────┐ │   │                            │
             └────────────────────┼──►│ │playbook-worker│ ├───┼────────────────────────────┘
                                  │   │ └────┬────▲─────┘ │   │
                                  │   │      │    │       │   │
                                  │   └──────┼────┼───────┘   │
                                  │          │    │           │
                                  │        ┌─▼────┴──┐        │
                                  │        │ Ansible │        │
                                  │        │ Runner  │        │
                                  │        └─────────┘        │
                                  │                           │
                                  └───────────────────────────┘

```

## Tools

Development of this project utilizes several tools listed below:

- [Git](https://git-scm.com/)
- [Golang](https://golang.org/)
- [Python](https://www.python.org/)
- [minikube](https://minikube.sigs.k8s.io/docs/)
- [Clowder](https://github.com/RedHatInsights/clowder)
- [Bonfire](https://github.com/RedHatInsights/bonfire)
- [Podman](https://podman.io/) / [Docker](https://www.docker.com/)
- [OpenShift CLI](https://docs.openshift.com/container-platform/4.7/cli_reference/openshift_cli/getting-started-cli.html)

### Git

[Git](https://git-scm.com/) is a free and open source distributed version control system designed to handle everything from small to very large projects with speed and efficiency. You can install Git on your system if it's not already available using the following [documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

### Golang

[Golang](https://golang.org/) is the development code utilized by the `edge-api` application. You can get setup to develop with Golang by following the [install documentation](https://golang.org/doc/install).  The dependencies are handled by [Go modules](https://blog.golang.org/using-go-modules) and specified in the `go.mod` file.

### Python

[Python](https://www.python.org/) is only necessary to support the usage of [Bonfire](https://github.com/RedHatInsights/bonfire), which is used for deployment and testing of the `edge-api` application. It is recommended to use Python 3.6 with this project. While you may use the Python included with your Operating System you may also find tools like [pyenv](https://github.com/pyenv/pyenv) to be useful for maintaining multiple Python versions. Currently, the development dependencies are obtained using [pipenv](https://pipenv.pypa.io/en/latest/). Pipenv creates a Python virtual environment with the dependencies specified in the `Pipfile` and the virtual environment created is unique to this project, allowing different projects to have different dependencies.

### Minikube

[Minikube](https://minikube.sigs.k8s.io/docs/) provides a local single node [Kubernetes](https://kubernetes.io/) cluster for development purposes. You can find setup information for minikube in the following **[Get Started!](https://minikube.sigs.k8s.io/docs/start/)** docs. Before starting your cluster you will need to make several [configuration updates noted in the Clowder documentation](https://github.com/RedHatInsights/clowder#getting-clowder).

### Clowder

[Clowder](https://github.com/RedHatInsights/clowder) is a kubernetes operator designed to make it easy to deploy applications running on the cloud.redhat.com platform in production, testing and local development environments. This operator normalizes how applications are configured with common interactions, from database to message queue and topics, to object storage. Clowder also helps define consistent mechanisms for driving integration tests with noted application dependencies and Job Invocations. [Getting started with Clowder](https://github.com/RedHatInsights/clowder#getting-clowder) is quite simple using a single command to deploy the operator.

### Bonfire

[Bonfire](https://github.com/RedHatInsights/bonfire) is a CLI tool used to deploy ephemeral environments for testing cloud.redhat.com applications. `bonfire` interacts with a local configuration file to obtain applications' OpenShift templates, process them, and deploy them. There is a `Pipfile` in this repository that specifies bonfire as a dependency, and if you run the installation steps above, you will have it installed on your virtual environment.

#### Podman / Docker

[Podman](https://podman.io/) / [Docker](https://www.docker.com/) are used to build a container for `edge-api` that will run in [Kubernetes](https://kubernetes.io/) / [Red Hat OpenShift](https://www.openshift.com/). Get started with Podman following this [installation document](https://podman.io/getting-started/installation). Get started with Docker following this [installation document](https://docs.docker.com/get-docker/).

### OpenShift CLI

[OpenShift CLI](https://docs.openshift.com/container-platform/4.7/cli_reference/openshift_cli/getting-started-cli.html) is used by Bonfire because of its templating capabitilies to generate the files that will be deployed to your local Kubernetes cluster. Follow the instructions on [Installing the OpenShift CLI](https://docs.openshift.com/container-platform/4.7/cli_reference/openshift_cli/getting-started-cli.html#installing-openshift-cli) to install it in your machine.

## Setup

For these steps, only git and Docker are necessary from the information above. Keep in mind that you might need to run inside of a Kubernetes cluster if you want to test more complex use-case scenarios.

1. Clone the project.

     ```bash
     git clone git@github.com:RedHatInsights/edge-api.git
     ```

2. Change directories to the project.

     ```bash
     cd edge-api
     ```

3. Run the migrations to create the database schema (this will download dependencies).

     ```bash
     go run cmd/migrate/main.go
     ```

4. Run the project in debug mode. Debug mode allows unauthenticated calls and it's essential for local development.

     ```bash
     DEBUG=true go run main.go
     ```

5. Open another terminal and make sure you can reach the API.

     ```bash
     curl -v http://localhost:3000/
     ```

If you find an error message saying that you don't have [gcc](https://gcc.gnu.org) installed, [install it](https://gcc.gnu.org/install/).

Keep in mind that if you change the models you might need to run the migrations again. By default, Edge API will run with a sqllite database that will be created in the first run. If you want to use your own postgresql container (which is particularly good for corner cases on migrations and queries), you can do this by:

1. Run the dabatase (example with podman)

     ```bash
     podman run -d --name postgresql_database -e POSTGRESQL_USER=user -e POSTGRESQL_PASSWORD=pass -e POSTGRESQL_DATABASE=db -p 5432:5432 rhel8/postgresql-10
     ```

2. Add some environment variables

     ```bash
     DATABASE=pgsql
     PGSQL_USER=user
     PGSQL_PASSWORD=pass
     PGSQL_HOSTNAME=127.0.0.1
     PGSQL_PORT=5432
     PGSQL_DATABASE=db
     ```
### Setup with Podman/Docker
If you prefer to run the edge-api using containers, then you can use the following steps.You should have podman or docker already installed in your machine and a valid account account on quay.io

1. Clone the project.

     ```bash
     git clone git@github.com:RedHatInsights/edge-api.git
     ```
2. Create an authentication json file, replacing the word token with the generated password provided by quay.io

     ```bash
     ${HOME}/.config/containers/auth.json
     "auths": {
          "quay.io": {
 	          "auth": "token..."
               }
          }
     }
     ```
3.  Login on quay.io
Please note that you can use either podman or docker for the example below.
     ```bash
     podman login  registry.redhat.io
     ```

     to validate:
     ```bash
       podman login --get-login registry.redhat.io
     ```
4. Create a new folder outside the cloned edge-api repository where you can store all database artifacts. This will allow you to keep a database that can be reused multiple times. For example, you might want to keep your database under a special backup folder inside your $HOME directory
     ```bash
     mkdir $HOME/db_backups
     ```
5. Change into the directory where you've cloned the edge-api repository
     ```bash
     cd edge-api
     ```
6. Launch a containerized instance of a PostgreSQL database
Please note that you can use either podman or docker for the example below
     ```bash
     podman run  --detach   --env POSTGRESQL_DATABASE='edge'  --env POSTGRESQL_PASSWORD=pass  --env POSTGRESQL_USER=user  --name podmandb   --publish 5432:5432  --pull=always  --volume PATH_TO_YOUR_DATABASE_BACK_FOLDER:/var/lib/pgsql/data:Z registry.redhat.io/rhel8/postgresql-12:latest
     ```
     The example above uses PostgreSQL 12, but you can try a newer version. The important part is to remember what values you use for all the environment variables, such as POSTGRESQL_DATABASE, POSTGRESQL_PASSWORD, and POSTGRESQL_USER, as they will be required later on
    You can also use environment variables instead of passing them inline
7. Execute the project migrations
     ```bash
     podman run --rm -ti -p 3000:3000 -v $(pwd):/edge-api:Z --env DATABASE=pgsql   --env PGSQL_DATABASE=edge   --env PGSQL_HOSTNAME=YOUR_IP    --env PGSQL_PASSWORD=pass  --env PGSQL_PORT=5432 --env PGSQL_USER=user  --name edge-api --pull=always   quay.io/cloudservices/edge-api:latest /usr/bin/edge-api-migrate
     ```
     You can test that your containerized database is working by using the following command
     ```bash
     psql --host localhost --user user edge
     ```

     If you don't have the psql command available locally, you can use the containerized instance itself to check the database:
    ```bash
    docker exec -it postgres  --host localhost --user user edge
    ```
    Type \c edge to connect to the edge database and \dt to list all available tables. Type \q to exit.

    Note: You won't have any tables until you have run the migration scripts, as explained further down.
8. Start up the edge-api service
     ```bash
     podman run --rm -ti -p 3000:3000 -v $(pwd):/edge-api:Z --env DATABASE=pgsql   --env PGSQL_DATABASE=edge   --env PGSQL_HOSTNAME=YOUR_IP    --env PGSQL_PASSWORD=pass  --env PGSQL_PORT=5432 --env PGSQL_USER=user  IMAGEBUILDERURL=imagebuilder_url--env INVENTORYURL=inventory_url --env HTTP_PROXY=proxy --env HTTPS_PROXY=proxy  --name edge-api --pull=always   quay.io/cloudservices/edge-api:latest
     ```
     You can also use environment variables instead of passing them inline
     The values for the DB should be the same as you created on DB setup (step 6)
9. In another terminal or tab, test your local environment
     get
     ```bash
     curl -v http://localhost:3000/
     ```
     curl post
     ```bash
     curl --request POST --url localhost:3000/api/edge/v1/device-groups/ --header 'Content-Type: application/json' --data '{"Account": "0000000","Name":"test", "Type":"static"}'
     ```
     docs
     ```bash
     localhost:3000/docs
     ```    
### Setup with Kubernetes

Following the information above you should have Docker or Podman, a minikube cluster running with Clowder installed, and a Python environment with `bonfire` installed. Now move on to running the `edge-api` application.

1. Clone the project.

     ```bash
     git clone git@github.com:RedHatInsights/edge-api.git
     ```

2. Change directories to the project.

     ```bash
     cd edge-api
     ```

3. Setup your Python virtual environment.

     ```bash
     pipenv install --dev
     ```

4. Enter the Python virtual environment to enable access to Bonfire.

     ```bash
     pipenv shell
     ```

5. Setup access to the Docker enviroment within minikube, so you can build images directly to the cluster's registry.

     ```bash
     eval $(minikube -p minikube docker-env)
     ```

6. Build the container image.

     ```bash
     make build
     ```

7. Create Bonfire configuration. To deploy from your local repository run the following:

     ```bash
     make bonfire-config-local
     ```

     The above command will create a file named `default_config.yaml` pointing to your local repository. At times you may need to update the branch which is referred to with the `ref` parameter (defaults to `main`).

     Bonfire can also deploy from GitHub. Running the following command will setup the GitHub based configuration:

     ```bash
     make bonfire-config-github
     ```

8. Setup test namespace for deployment.

     ```bash
     make create-ns NAMESPACE=test
     ```

9. Deploy a Clowder environment (*ClowdEnviroment*) to the namespace with bonfire.

     ```bash
     make deploy-env NAMESPACE=test
     ```

10. Deploy the application to the namespace.

     ```bash
     make deploy-app NAMESPACE=test
     ```

Now the application should be running. You can test this by port-forwarding the app in one terminal and running a curl command in another as follows:

> Terminal 1

```bash
kubectl -n test port-forward service/edge-api-service 8000:8000
```

> Terminal 2

```bash
curl -v http://localhost:8000/
```

You should get a 200 response back.

## Development

Now you can build and deploy the application.

Once a code change has been performed you can rebuild the container in the minikube registry:

```bash
make build
```

Then scale down and up the deployment to pick up the image change:

```bash
make restart-app NAMESPACE=test
```

## Testing and Linting

This project makes use of Golang's built in `lint` and `vet` capabilites. You can run these with the following commands:

*lint:*

```bash
make lint
```

*vet:*

```bash
make vet
```

Golang also provides a unit test infrastructure `test`. Run unit tests with the following command:

```bash
make test
```

### API docs

[kin-openapi](https://github.com/getkin/kin-openapi) is a tool that helps us handle docs in the format of [Open API spec](https://github.com/OAI/OpenAPI-Specification). Sadly, it does not generate docs *automagically*. We have a [script](cmd/spec/main.go) that generates the docs and you have to add your model there to be picked by the code generation.

The [openapi3gen](https://github.com/getkin/kin-openapi/tree/v0.65.0/openapi3gen) package generates the docs for the models in the project and we have to update the routes by hand on the [path.yml](cmd/spec/path.yaml) file. Our generation [script](cmd/spec/main.go) adds the routes that you wrote by hand, creating an openapi.json and a openapi.yaml.

You have to commit and create a pull-request to update the docs.

To run the comand that generates the docs, you can use:

```bash
make generate-docs
```

The API will serve the docs under a `/docs` endpoint.

### Testing

The tests in this repository are meant to be unit tests. Because of how Go works, in order to mock objects properly you can't create them inside of your function. You can instead receive objects that are supposed to be mocked through function parameters, context, or variables inside of the same struct.

We are using [gomock](https://github.com/golang/mock) to generate mocks for our unit tests. The mocks are living inside of a package under the real implementation, prefixed by `mock_`. An example is the package `mock_services` under `pkg/services`.

In order to genenerate a mock for a particular file, you can run:

```
go install github.com/golang/mock/mockgen@latest
mockgen -source=pkg/filename.go -destination=pkg/mock_pkg/mock_filename.go
```

For example, to create/update mocks for ImageService, we can run:

`mockgen -source=pkg/services/images.go -destination=pkg/services/mock_services/images.go`
