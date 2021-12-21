package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/services"
)

// Used to generate openapi yaml file for components.
func main() {
	components := openapi3.NewComponents()
	components.Schemas = make(map[string]*openapi3.SchemaRef)

	ovData, err := openapi3gen.NewSchemaRefForValue(&[]models.OwnershipVoucherData{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.OwnershipVoucherData"] = ovData

	image, err := openapi3gen.NewSchemaRefForValue(&models.Image{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.Image"] = image

	imageDetail, err := openapi3gen.NewSchemaRefForValue(&routes.ImageDetail{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.ImageDetail"] = imageDetail

	ImageSetImagePackages, err := openapi3gen.NewSchemaRefForValue(&routes.ImageSetImagePackages{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.ImageSetImagePackages"] = ImageSetImagePackages

	ImageSetInstallerURL, err := openapi3gen.NewSchemaRefForValue(&routes.ImageSetInstallerURL{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.ImageSetInstallerURL"] = ImageSetInstallerURL

	repo, err := openapi3gen.NewSchemaRefForValue(&models.Repo{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.Repo"] = repo

	updates, err := openapi3gen.NewSchemaRefForValue(&routes.UpdatePostJSON{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.AddUpdate"] = updates

	updateTransaction, err := openapi3gen.NewSchemaRefForValue(&models.UpdateTransaction{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.UpdateTransaction"] = updateTransaction

	deviceDetails, err := openapi3gen.NewSchemaRefForValue(&services.DeviceDetails{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.DeviceDetails"] = deviceDetails
	device, err := openapi3gen.NewSchemaRefForValue(&models.Device{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.Device"] = device

	imageSetDetails, err := openapi3gen.NewSchemaRefForValue(&models.ImageSet{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.ImageSetDetails"] = imageSetDetails

	checkImageResponse, err := openapi3gen.NewSchemaRefForValue(&routes.CheckImageNameResponse{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.CheckImageResponse"] = checkImageResponse

	var checkNameBool bool
	boolSchema, err := openapi3gen.NewSchemaRefForValue(checkNameBool)
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.bool"] = boolSchema

	internalServerError, err := openapi3gen.NewSchemaRefForValue(&errors.InternalServerError{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.InternalServerError"] = internalServerError

	badRequest, err := openapi3gen.NewSchemaRefForValue(&errors.BadRequest{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.BadRequest"] = badRequest

	notFound, err := openapi3gen.NewSchemaRefForValue(&errors.NotFound{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.NotFound"] = notFound

	thirdpartyrepo, err := openapi3gen.NewSchemaRefForValue(&models.ThirdPartyRepo{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.CreateThirdPartyRepo"] = thirdpartyrepo

	type Swagger struct {
		Components openapi3.Components `json:"components,omitempty" yaml:"components,omitempty"`
	}

	swagger := Swagger{}
	swagger.Components = components

	b := &bytes.Buffer{}
	err = json.NewEncoder(b).Encode(swagger)
	checkErr(err)

	schema, err := yaml.JSONToYAML(b.Bytes())
	checkErr(err)

	paths, err := ioutil.ReadFile("./cmd/spec/path.yaml")
	checkErr(err)

	b = &bytes.Buffer{}
	b.Write(schema)
	b.Write(paths)

	doc, err := openapi3.NewLoader().LoadFromData(b.Bytes())
	checkErr(err)

	jsonB, err := json.MarshalIndent(doc, "", "  ")
	checkErr(err)
	err = ioutil.WriteFile("./cmd/spec/openapi.json", jsonB, 0666)
	checkErr(err)
	err = ioutil.WriteFile("./cmd/spec/openapi.yaml", b.Bytes(), 0666)
	checkErr(err)
	fmt.Println("Spec was generated successfully")
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
