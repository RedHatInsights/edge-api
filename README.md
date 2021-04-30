
# Overview
- [Getting started](#intro)
- [Development](#development)

# <a name="intro">Getting Started</a>

The **edge-api** project is an API server for fleet edge management capabilities. The API server will provide [Restful web services](https://www.redhat.com/en/topics/api/what-is-a-rest-api).
This is a [Golang](https://golang.org/) project developed using Golang 1.14. *Make sure you have at least this version installed.*


Below you can see where the `edge-api application sits in respect to the interaction with the user and the device at the edge to be managed.

```
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





# <a name="development">Development</a>

## Tools

Development of this project utilizes several tools listed below:
- [Git](https://git-scm.com/)
- [Golang](https://golang.org/)
- [Python](https://www.python.org/)
- [minikube](https://minikube.sigs.k8s.io/docs/)
- [Clowder](https://github.com/RedHatInsights/clowder)
- [Bonfire](https://github.com/RedHatInsights/bonfire)
- [Podman](https://podman.io/) / [Docker](https://www.docker.com/)

### Git
[Git](https://git-scm.com/) is a free and open source distributed version control system designed to handle everything from small to very large projects with speed and efficiency. You can install Git on your system if its not already avaible using the following [documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

### Golang
[Golang](https://golang.org/) is the development code utilized by the `edge-api application. You can get setup to develop with Golang by following the [install documentation](https://golang.org/doc/install).


### Python
[Python](https://www.python.org/) is only necessary to support the usage of [Bonfire](https://github.com/RedHatInsights/bonfire), which is used for deployment and testing of the `edge-api` application. It is recommended to use Python 3.6 with this project. While you may use the Python included with your Operating System you may also find tools like `[pyenv](https://github.com/pyenv/pyenv)` to be useful for maintaining multiple Python versions. Currently, the development dependencies are obtained using `[pipenv](https://pipenv.pypa.io/en/latest/)`.

### Minikube
[Minikube](https://minikube.sigs.k8s.io/docs/)) provides a local single node [Kubernetes](https://kubernetes.io/) cluster for development purposes. You can find setup information for minikube in the following **[Get Started!](https://minikube.sigs.k8s.io/docs/start/)** docs. Before starting your cluster you will need to make several [configuration updates noted in the Clowder documentation](https://github.com/RedHatInsights/clowder#getting-clowder).

### Clowder
[Clowder](https://github.com/RedHatInsights/clowder) is a kubernetes operator designed to make it easy to deploy applications running on the cloud.redhat.com platform in production, testing and local development environments. This operator normalizes how application are configured with common interactions from database to, message queue and topics, to object storage. Clowder also helps define consistent mechanisms for driving integration tests with noted application dependencies and Job Invocations. [Getting started with Clowder](https://github.com/RedHatInsights/clowder#getting-clowder) is quite simple using a single command to deploy the operator.

### Bonfire
[Bonfire](https://github.com/RedHatInsights/bonfire) is CLI tool used to deploy ephemeral environments for testing cloud.redhat.com applications. `bonfire` interacts with a local configuration file to obtain applications' OpenShift templates, process them, and deploy them. Once you have obtained the Python related tools above you should simple be able to run the following command to obtain Bonfire:
```
pipenv install --dev
```
Run the following command to start using Bonfire in the new virtual environment:
```
pipenv shell
```

### Podman / Docker
[Podman](https://podman.io/) / [Docker](https://www.docker.com/) are used to build a container for the `edge-api` that will run in [Kubernetes](https://kubernetes.io/) / [Red Hat OpenShift](https://www.openshift.com/). Get started with Podman following this [installation document](https://podman.io/getting-started/installation). Get started with Docker folling this [installation document](https://docs.docker.com/get-docker/).


## Setup

Follwing the information above you should have Docker or Podman, a minikube cluster running with Clowder installed, a Python environment with `bonfire` installed. Now move on to running the `edge-api` application.

1. Clone the project
```
git clone git@github.com:RedHatInsights/edge-api.git
```
2. Change directories to the project
```
cd edge-api
```
3. Setup your Python virtual environement
```
pipenv install --dev
```
4. Enter the Python virtual environment to enable access to Bonfire
```
pipenv shell
```
5. Setup access to the Docker enviroment within minikube, so you can build images directly to the cluster's registry
```
eval $(minikube -p minikube docker-env)
```
6. Build the container image
```
make build
```
7. Create Bonfire configuration
To deploy from your local repository run the following:
```
make bonfire-config-local
```
The above command will create a file named `default_config.yaml` pointing to your local repositry. At times you may need to update the branch which is referred to with the `ref` parameter (defaults to `main`).

Bonfire can also deploy from github running the following command will setup the github based configuration:
```
make bonfire-config-github
```
8. Setup test namespace for deployment
```
make create-ns NAMESPACE=test
```
9. Deploy a Clowder environment (*ClowdEnviroment*) to the namespace with bonfire
```
make deploy-env NAMESPACE=test
```
10. Deploy the application to the namespace
```
make deploy NAMESPACE=test
```

Now the application should be running. You can test this by port-forwarding the app in one terminal and running a curl command in another as follows:
**Terminal 1**
```
kubectl -n test port-forward service/edge-api-service 8000:8000
```
**Terminal 2**
```
curl -v http://localhost:8000/
```

You should get a 200 response back.

## Development
Now you can build and deploy the application you can deploy code updates by applying what you have already done.

Once a code change has been performed you can rebuild the container to the minikube registry:
```
make build
```
Then scale down and up the deployment to pick up the image change:
```
make restart-app NAMESPACE=test
```

## Testing and Linting
This project makes use of Golang's built in `lint` and `vet` capabilites. You can run these with the following commands:

*lint:*
```
make lint
```

*vet:*
```
make vet
```

Golang also provides a unit test infrastructure `test`. Run unit tests with the following command:
```
make test
```
