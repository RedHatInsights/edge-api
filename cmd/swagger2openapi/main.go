package main

import (
	"encoding/json"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
)

// Convert a swagger v2 json document to an OpenApi v3 json document
func main() {
	if len(os.Args) != 3 {
		panic("Usage: ./command  input_filename output_filename")
	}
	inputFile := os.Args[1]
	outputFile := os.Args[2]

	input, err := os.ReadFile(inputFile)
	if err != nil {
		panic(err)
	}

	var doc openapi2.T
	if err = json.Unmarshal(input, &doc); err != nil {
		panic(err)
	}

	v3, err := openapi2conv.ToV3(&doc)
	if err != nil {
		panic(err)
	}

	openapiJSON, err := json.MarshalIndent(v3, "", "    ")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(outputFile, openapiJSON, 0644) //nolint:gosec
	if err != nil {
		panic(err)
	}
}
