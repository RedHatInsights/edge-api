# Contributing

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