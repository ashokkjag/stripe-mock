package main

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
)

var listSchema *JSONSchema

func init() {
	listSchema = &JSONSchema{
		Properties: map[string]*JSONSchema{
			"data": &JSONSchema{
				Items: &JSONSchema{
					Ref: "#/definitions/charge",
				},
			},
			"has_more": nil,
			"object": &JSONSchema{
				Enum: []string{"list"},
			},
			"total_count": nil,
			"url":         nil,
		},
	}
}

func TestGenerateResponseData(t *testing.T) {
	var data interface{}
	var err error
	var generator DataGenerator

	// basic reference
	generator = DataGenerator{testSpec.Definitions, testFixtures}
	data, err = generator.Generate(
		&JSONSchema{Ref: "#/definitions/charge"}, "")

	assert.Nil(t, err)
	assert.Equal(t,
		testFixtures.Resources["charge"].(map[string]interface{})["id"],
		data.(map[string]interface{})["id"])

	// list
	generator = DataGenerator{testSpec.Definitions, testFixtures}
	data, err = generator.Generate(listSchema, "/v1/charges")
	assert.Nil(t, err)
	assert.Equal(t, "list", data.(map[string]interface{})["object"])
	assert.Equal(t, "/v1/charges", data.(map[string]interface{})["url"])
	assert.Equal(t,
		testFixtures.Resources["charge"].(map[string]interface{})["id"],
		data.(map[string]interface{})["data"].([]interface{})[0].(map[string]interface{})["id"])

	// nested list
	generator = DataGenerator{
		testSpec.Definitions,
		&Fixtures{
			Resources: map[ResourceID]interface{}{
				ResourceID("charge"): map[string]interface{}{"id": "ch_123"},
				ResourceID("with_charges_list"): map[string]interface{}{
					"charges_list": map[string]interface{}{
						"url": "/v1/from_charges_list",
					},
				},
			},
		},
	}
	data, err = generator.Generate(
		&JSONSchema{
			Properties: map[string]*JSONSchema{
				"charges_list": listSchema,
			},
			XResourceID: "with_charges_list",
		}, "")
	assert.Nil(t, err)
	chargesList := data.(map[string]interface{})["charges_list"]
	assert.Equal(t, "list", chargesList.(map[string]interface{})["object"])
	assert.Equal(t, "/v1/from_charges_list", chargesList.(map[string]interface{})["url"])
	assert.Equal(t,
		testFixtures.Resources["charge"].(map[string]interface{})["id"],
		chargesList.(map[string]interface{})["data"].([]interface{})[0].(map[string]interface{})["id"])

	// no fixture (returns an empty object)
	generator = DataGenerator{
		testSpec.Definitions,
		// this is an empty set of fixtures
		&Fixtures{
			Resources: map[ResourceID]interface{}{},
		},
	}
	data, err = generator.Generate(
		&JSONSchema{Ref: "#/definitions/charge"}, "")
	assert.Nil(t, err)
	assert.Equal(t, map[string]interface{}{}, data)

	// error: unhandled JSON schema type
	generator = DataGenerator{testSpec.Definitions, testFixtures}
	data, err = generator.Generate(
		&JSONSchema{Type: []string{"string"}}, "")
	assert.Equal(t,
		fmt.Errorf("Expected response to be a list or include $ref"),
		err)

	// error: no definition in OpenAPI
	generator = DataGenerator{testSpec.Definitions, testFixtures}
	data, err = generator.Generate(
		&JSONSchema{Ref: "#/definitions/doesnt-exist"}, "")
	assert.Equal(t,
		fmt.Errorf("Couldn't dereference: #/definitions/doesnt-exist"),
		err)
}

// ---

func TestDefinitionFromJSONPointer(t *testing.T) {
	definition, err := definitionFromJSONPointer("#/definitions/charge")
	assert.Nil(t, err)
	assert.Equal(t, "charge", definition)
}