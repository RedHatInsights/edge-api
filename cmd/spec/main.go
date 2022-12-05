// FIXME: golangci-lint
// nolint:revive
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes"
)

type edgeAPISchemaGen struct {
	Components openapi3.Components
}

func (s *edgeAPISchemaGen) init() {
	s.Components = openapi3.NewComponents()
	s.Components.Schemas = make(map[string]*openapi3.SchemaRef)
}

func (s *edgeAPISchemaGen) addSchema(name string, model interface{}) {
	schema, err := openapi3gen.NewSchemaRefForValue(model, s.Components.Schemas)
	checkErr(err)
	s.Components.Schemas[name] = schema
}

// Used to generate openapi yaml file for components.
// Typically, runs either locally or as part of GitHub Actions (.github/workflows/lint.yml)
func main() {
	gen := edgeAPISchemaGen{}
	gen.init()
	gen.addSchema("v1.OwnershipVoucherData", &[]models.OwnershipVoucherData{})
	gen.addSchema("v1.Image", &models.Image{})
	gen.addSchema("v1.PackageDiff", &models.PackageDiff{})
	gen.addSchema("v1.ImageDetail", &routes.ImageDetail{})
	gen.addSchema("v1.ImageSetImagePackages", &routes.ImageSetImagePackages{})
	gen.addSchema("v1.ImageSetInstallerURL", &routes.ImageSetInstallerURL{})
	gen.addSchema("v1.Repo", &models.Repo{})
	gen.addSchema("v1.AddUpdate", &models.DevicesUpdate{})
	gen.addSchema("v1.UpdateTransaction", &models.UpdateTransaction{})
	gen.addSchema("v1.DeviceDetails", &models.DeviceDetails{})
	gen.addSchema("v1.Device", &models.Device{})
	gen.addSchema("v1.ImageSet", &models.ImageSet{})
	gen.addSchema("v1.ImageSetView", &models.ImageSetView{})
	gen.addSchema("v1.ImageSetIDView", &services.ImageSetIDView{})
	gen.addSchema("v1.ImagesViewData", &services.ImagesViewData{})
	gen.addSchema("v1.ImageSetImageIDView", &services.ImageSetImageIDView{})
	gen.addSchema("v1.Device", &models.Device{})
	gen.addSchema("v1.CheckImageResponse", &routes.CheckImageNameResponse{})
	var booleanResponse bool
	gen.addSchema("v1.bool", booleanResponse)
	gen.addSchema("v1.InternalServerError", &errors.InternalServerError{})
	gen.addSchema("v1.BadRequest", &errors.BadRequest{})
	gen.addSchema("v1.NotFound", &errors.NotFound{})
	gen.addSchema("v1.ThirdPartyRepo", &models.ThirdPartyRepo{})
	gen.addSchema("v1.DeviceDetailsList", &models.DeviceDetailsList{})
	gen.addSchema("v1.DeviceViewList", &models.DeviceViewList{})
	gen.addSchema("v1.DeviceGroup", &models.DeviceGroup{})
	gen.addSchema("v1.DeviceGroupListDetail", &models.DeviceGroupListDetail{})
	gen.addSchema("v1.DeviceGroupDetailsView", &models.DeviceGroupDetailsView{})
	gen.addSchema("v1.DeviceGroupDetails", &models.DeviceGroupDetails{})
	gen.addSchema("v1.ValidateUpdateResponse", &routes.ValidateUpdateResponse{})
	gen.addSchema("v1.APIResponse", &common.APIResponse{})

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

	paths, err := os.ReadFile("./cmd/spec/path.yaml")
	checkErr(err)

	b = &bytes.Buffer{}
	b.Write(schema)
	b.Write(paths)

	doc, err := openapi3.NewLoader().LoadFromData(b.Bytes())
	checkErr(err)

	jsonB, err := json.MarshalIndent(doc, "", "  ")
	checkErr(err)
	err = os.WriteFile("./cmd/spec/openapi.json", jsonB, 0644) // #nosec G306
	checkErr(err)
	err = os.WriteFile("./cmd/spec/openapi.yaml", b.Bytes(), 0644) // #nosec G306
	checkErr(err)
	fmt.Println("Spec was generated successfully")
}

func checkErr(err error) {
	if err != nil {
		// This will not cause the pod to crash so don't call LogErrorAndPanic()
		panic(err)
	}
}
