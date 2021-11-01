# Contributing

This is a project that welcome contributors from all backgrounds and technical levels and discrimination will not be tolerated.

The following is a deep dive into what is important to know to contribute to the code, and it is important to read and follow the steps on the [README](README.md) file.

## Folder structure

Edge API's code is organized in a couple of top-level important folders:

```
- edge-api/
-- cmd/
-- config/
-- deploy/
-- logger/
-- pkg/
-- scripts/
```

The `cmd` folder contains scripts like the openapi generation script and the database migration script. These scripts can be invoked locally and/or by the containers during build or execution, depending on what they are and when it's important for them to run.

The `config` folder has the code to retrieve configuration values from environment variables and Clowder. Every time you need to add or change anything related to the main configuration of Edge API, you should go there.

The `deploy` folder contains configuration values for the stage and production environment. You'll likely add something there if you create a new variable that needs to have specific default value for an environment or a specific value that is retrieved by deployment parameters.

The `logger` folder contains code related to logging on Edge API. It has the main logger used by Edge API and initializes it with some configuration.

The `pkg` folder contains the actual API code. We'll dive more into this in the above section.

The `scripts` folder has scripts used by Edge API or end users, like the ansible registration playbook used by users to register a new Edge device.


## Code structure

```
- edge-api/
(...)
-- pkg/
--- clients/
--- db/
--- dependencies/
--- errors/
--- models/
--- routes/
--- services/
```

![Code Architecture Diagram](code-arch-diagram.png)

The four main components of this structure are `routes`, `services`, `models` and `clients`.

Ideally, you write your code around a **domain**. The two main domains on Edge API are Images and Devices. Everything happens around them. Important domains are UpdateTransactions, Commits, Installers, ImageSets and (soon) DeviceSets.

The routes of a particular domain are in a file under the `routes` folder. For image API routes, for example, we have `images.go` living under that folder. Routes can also be called handlers as they are the HTTP Handler for a given URL pattern. Each route file exposes one or more routes and created a sub router with those routes. The responsability of a route is to receive an API call, get the data from the API call in a way that is easy to pass forward - either creating a struct, a variable, whatever fits the use case better -, call the correspondent `service` and handle the service response to deal with HTTP status. This is the only layer that is responsible for the HTTP server bits of our code and knows the status codes and request/responses from our API.

The `service` layer is responsible for all the code related to the business logic. If necessary, it interacts with the database and API clients to achieve what is needed. The service layer does not know HTTP status or anything related to the API server, as the main idea is that its code could be easily used if we decide to serve our API through a different interface that is not HTTP (gRPC, for example).

The `models` layer defines the database schema and the API models. They are essentially the domain that guides what goes on the other layers, and useful domain around it.

The `clients` packages are implementation of API calls to internal services. Those APIs do not have go clients, therefore we create a client for it. This is particularly important for us to mock API calls to other services while unit tests our service layer.

Other folders were created in a peer-need basis, always following the idea that we want to avoid a `helpers` or `utils` package, otherwise this will be a package where we'll put everything in and there is a high chance that we aren't designing our software correctly.