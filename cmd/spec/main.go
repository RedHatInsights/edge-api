package main

import (
	"bytes"
	"encoding/json"
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

	image, _, err := openapi3gen.NewSchemaRefForValue(&models.Image{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.Image"] = image

	repo, _, err := openapi3gen.NewSchemaRefForValue(&models.Repo{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.Repo"] = repo

	updates, _, err := openapi3gen.NewSchemaRefForValue(&routes.UpdatePostJSON{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.AddUpdate"] = updates

	updateTransaction, _, err := openapi3gen.NewSchemaRefForValue(&models.UpdateTransaction{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.UpdateTransaction"] = updateTransaction

	deviceDetails, _, err := openapi3gen.NewSchemaRefForValue(&services.DeviceDetails{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.DeviceDetails"] = deviceDetails

	internalServerError, _, err := openapi3gen.NewSchemaRefForValue(&errors.InternalServerError{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.InternalServerError"] = internalServerError

	badRequest, _, err := openapi3gen.NewSchemaRefForValue(&errors.BadRequest{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.BadRequest"] = badRequest

	notFound, _, err := openapi3gen.NewSchemaRefForValue(&errors.NotFound{})
	if err != nil {
		panic(err)
	}
	components.Schemas["v1.NotFound"] = notFound

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

	b = &bytes.Buffer{}
	b.Write(schema)
	b.Write(paths)

	doc, err := openapi3.NewLoader().LoadFromData(b.Bytes())
	checkErr(err)

	jsonB, err := doc.MarshalJSON()
	checkErr(err)
	err = ioutil.WriteFile("./cmd/spec/openapi.json", jsonB, 0666)
	checkErr(err)
	err = ioutil.WriteFile("./cmd/spec/openapi.yaml", b.Bytes(), 0666)
	checkErr(err)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
