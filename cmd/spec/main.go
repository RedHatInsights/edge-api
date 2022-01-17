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
)

type EdgeAPISchemaGen struct {
	Components openapi3.Components
}

func (s *EdgeAPISchemaGen) Init() {
	s.Components = openapi3.NewComponents()
	s.Components.Schemas = make(map[string]*openapi3.SchemaRef)
}

func (s *EdgeAPISchemaGen) AddSchema(name string, model interface{}) {
	schema, err := openapi3gen.NewSchemaRefForValue(model, s.Components.Schemas)
	if err != nil {
		panic(err)
	}
	s.Components.Schemas[name] = schema
}

// Used to generate openapi yaml file for components.
func main() {
	gen := EdgeAPISchemaGen{}
	gen.Init()
	gen.AddSchema("v1.OwnershipVoucherData", &[]models.OwnershipVoucherData{})
	gen.AddSchema("v1.Image", &models.Image{})
	gen.AddSchema("v1.PackageDiff", &models.Image{})
	gen.AddSchema("v1.ImageDetail", &routes.ImageDetail{})
	gen.AddSchema("v1.ImageSetImagePackages", &routes.ImageSetImagePackages{})
	gen.AddSchema("v1.ImageSetInstallerURL", &routes.ImageSetInstallerURL{})
	gen.AddSchema("v1.Repo", &models.Repo{})
	gen.AddSchema("v1.AddUpdate", &routes.UpdatePostJSON{})
	gen.AddSchema("v1.UpdateTransaction", &models.UpdateTransaction{})
	gen.AddSchema("v1.DeviceDetails", &models.DeviceDetails{})
	gen.AddSchema("v1.Device", &models.Device{})
	gen.AddSchema("v1.ImageSet", &models.ImageSet{})
	gen.AddSchema("v1.Device", &models.Device{})
	gen.AddSchema("v1.CheckImageResponse", &routes.CheckImageNameResponse{})
	var booleanResponse bool
	gen.AddSchema("v1.bool", booleanResponse)
	gen.AddSchema("v1.InternalServerError", &errors.InternalServerError{})
	gen.AddSchema("v1.BadRequest", &errors.BadRequest{})
	gen.AddSchema("v1.NotFound", &errors.NotFound{})
	gen.AddSchema("v1.ThirdPartyRepo", &models.ThirdPartyRepo{})

	type Swagger struct {
		Components openapi3.Components `json:"components,omitempty" yaml:"components,omitempty"`
	}

	swagger := Swagger{}
	swagger.Components = gen.Components

	b := &bytes.Buffer{}
	err := json.NewEncoder(b).Encode(swagger)
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
