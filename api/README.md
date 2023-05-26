# Using Swaggo to Update API Docs

Swaggo is a Golang library for generating API docs (openapi.json) from code. It allows for creating documentation in the code (e.g., the structs and handlers) used for accepting REST API requests.

## The Swaggo Package Overview

### GitHub Repo and Docs

The Swaggo docs and code are in the following GitHub repo. Please read them before submitting changes to the Edge API docs.

`https://github.com/swaggo/swag`

## How to Implement in Edge API

### **Make Endpoints Visible to Swaggo**

To make the endpoints visible to Swaggo, it is necessary to add annotations in the form of structured comments above each endpoint.

#### **Adding Annotation Comments to Handlers**

The handlers for incoming API Requests are found in the **pkg/routes** directory under the application root.

e.g., pkg/routes/images.go

    // CreateImage creates an image on hosted image builder.
    // @Summary      Create an image
    // @Description  Create an ostree commit and/or installer ISO
    // @Tags         Images
    // @Accept       json
    // @Produce      json
    // @Param        body	body	models.CreateImageAPI	true	"request body"
    // @Success      200 {object} models.ImageResponseAPI
    // @Failure      400 {object} errors.BadRequest
    // @Failure      500 {object} errors.InternalServerError
    // @Router       /images [post]
    func CreateImage(w http.ResponseWriter, r *http.Request) {
	    ctxServices := dependencies.ServicesFromContext(r.Context())

The **@Param** entry tells Swaggo which struct in the application to map to this endpoint. **models.CreateImageAPI** points Swaggo to the specific struct (described below) that is used for generating the docs.

The **@Success and @Failure** entries point Swaggo to the structs that handle each specific return value.

The **@Router** entry is the URL path and HTTP Method for the endpoint. In Edge API, there is only one Router for each path and method.

### **Define Available and Required Fields and Map to @Param**

#### **Create New API-specific Structs**

The structs used throughout the Edge API can contain fields beyond what is necessary for using an endpoint and make the API Docs confusing for development purposes.

To fix this, create an API-specific struct in a new file under the pkg/models directory with **API** appended to the anem of the struct.

e.g., pkg/models/images_api.go

    // CreateImageAPI is the /images POST endpoint struct for openapi.json auto-gen
    type CreateImageAPI struct {
	    Commit         CommitAPI           `json:"commit"`
	    CustomPackages []CustomPackagesAPI `json:"customPackages"`                                 // An optional list of custom repositories
	    Description    string              `json:"description" example:"This is an example image"` // A short description of the image
    	Distribution   string              `json:"distribution" example:"rhel-92"`                 // The RHEL for Edge OS version
    	// Available image types:
	    // * rhel-edge-installer - Installer ISO
	    // * rhel-edge-commit - Commit only
	    ImageType              string               `json:"imageType" example:"rhel-edge-installer"` // The image builder assigned image type
	    Installer              InstallerAPI         `json:"installer"`
	    Name                   string               `json:"name"  example:"my-edge-image"`
	    Packages               []PackagesAPI        `json:"packages"`
	    OutputTypes            []string             `json:"outputTypes" example:"rhel-edge-installer,rhel-edge-commit"`
	    ThirdPartyRepositories []ThirdPartyReposAPI `json:"thirdPartyRepositories"`
	    Version                int                  `json:"version" example:"0"`
    } // @name CreateImage

The struct is a typical Golang struct with a couple of additions to make documentation generation more simple.

The **CreateImageAPI** struct corresponds the **Image** struct used by the application, but only includes available and required fields used to handle the API request and is currently only usd for the generation of API docs.

#### **Add Descriptions for Fields**

To add a field description that will show up in the API Docs, simply add a comment to the end of the field definition.

#### **Add an Example for Fields**

To add an example for a field, add a tag named example with the example value.

e.g.,

``Distribution   string              `json:"distribution" example:"rhel-92"`                 // The RHEL for Edge OS``

In the above example, **Distribution** is the name as used internally by the application and **distribution** is the name as it should appear in the incoming REST API call.

**example:"rhel-92"** is a tag that will be read by Swaggo and result in **rhel-92** being displayed in a Swagger UI form.

**// the RHEL for Edge OS** is the description comment that will be read by Swaggo and will appear a description of the field in the Swagger UI form.


### **Define Response Options**

The last step in providing definitions to Swaggo is to update the HTTP Response entries in the annotations with all of the response options (e.g., 200, 400, 500) and the corresponding struct that contains the data.

#### **Create New API-specific Structs**

For the purpose of only providing the available and required fields, it might be necessary to create new Response structs.

## Manually Running Swaggo

The Makefile has been updated to simplify local generation of API docs during development.

To generate a new openapi.json file under the /api directory, run the following command.

`make swaggo`

Opening the api/openapi.json file in an OpenAPI viewer, like Swagger UI or a VSCode plugin (try **OpenAPI (Swagger) Editor**), will display the Swagger UI form.

### **Dockerfile**

Edge API docs are automatically generated with each container build. It is not necessary to run swaggo manually and push an openapi.json file via PR.

    RUN go install github.com/swaggo/swag/cmd/swag@latest
    RUN mkdir -p api
    RUN ~/go/bin/swag init --generalInfo api.go --o ./api --dir pkg/models,pkg/routes --parseDependency
    RUN go run cmd/swagger2openapi/main.go  api/swagger.json api/openapi.json