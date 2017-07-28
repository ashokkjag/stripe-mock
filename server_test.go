package main

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"github.com/stripe/stripe-mock/spec"
)

func TestStubServer_SetsSpecialHeaders(t *testing.T) {
	server := getStubServer(t)

	// Does this regardless of endpoint
	req := httptest.NewRequest("GET", "https://stripe.com/", nil)
	w := httptest.NewRecorder()
	server.HandleRequest(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, version, resp.Header.Get("Stripe-Mock-Version"))
}

func TestStubServer_ParameterValidation(t *testing.T) {
	server := getStubServer(t)

	req := httptest.NewRequest("POST", "https://stripe.com/v1/charges", nil)
	req.Header.Set("Authorization", "Bearer sk_test_123")
	w := httptest.NewRecorder()
	server.HandleRequest(w, req)

	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.Contains(t, string(body), "property 'amount' is required")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestStubServer_RoutesRequest(t *testing.T) {
	server := getStubServer(t)
	var route *stubServerRoute

	route = server.routeRequest(
		&http.Request{Method: "GET", URL: &url.URL{Path: "/v1/charges"}})
	assert.NotNil(t, route)
	assert.Equal(t, chargeAllMethod, route.method)

	route = server.routeRequest(
		&http.Request{Method: "POST", URL: &url.URL{Path: "/v1/charges"}})
	assert.NotNil(t, route)
	assert.Equal(t, chargeCreateMethod, route.method)

	route = server.routeRequest(
		&http.Request{Method: "GET", URL: &url.URL{Path: "/v1/charges/ch_123"}})
	assert.NotNil(t, route)
	assert.Equal(t, chargeGetMethod, route.method)

	route = server.routeRequest(
		&http.Request{Method: "DELETE", URL: &url.URL{Path: "/v1/charges/ch_123"}})
	assert.NotNil(t, route)
	assert.Equal(t, chargeDeleteMethod, route.method)

	route = server.routeRequest(
		&http.Request{Method: "GET", URL: &url.URL{Path: "/v1/doesnt-exist"}})
	assert.Equal(t, (*stubServerRoute)(nil), route)
}

// ---

func TestCompilePath(t *testing.T) {
	assert.Equal(t, `\A/v1/charges\z`,
		compilePath(spec.Path("/v1/charges")).String())
	assert.Equal(t, `\A/v1/charges/(?P<id>[\w-_.]+)\z`,
		compilePath(spec.Path("/v1/charges/{id}")).String())
}

func TestGetValidator(t *testing.T) {
	method := &spec.Method{Parameters: []*spec.Parameter{
		{Schema: &spec.JSONSchema{
			RawFields: map[string]interface{}{
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}},
	}}
	validator, err := getValidator(method)
	assert.NoError(t, err)
	assert.NotNil(t, validator)

	goodData := map[string]interface{}{
		"name": "foo",
	}
	assert.NoError(t, validator.Validate(goodData))

	badData := map[string]interface{}{
		"name": 7,
	}
	assert.Error(t, validator.Validate(badData))
}

func TestGetValidator_NoSuitableParameter(t *testing.T) {
	method := &spec.Method{Parameters: []*spec.Parameter{
		{Schema: nil},
	}}
	validator, err := getValidator(method)
	assert.NoError(t, err)
	assert.Nil(t, validator)
}

func TestParseExpansionLevel(t *testing.T) {
	assert.Equal(t,
		&ExpansionLevel{expansions: map[string]*ExpansionLevel{
			"charge":   nil,
			"customer": nil,
		}},
		ParseExpansionLevel([]string{"charge", "customer"}))

	assert.Equal(t,
		&ExpansionLevel{expansions: map[string]*ExpansionLevel{
			"charge": {expansions: map[string]*ExpansionLevel{
				"customer": nil,
				"source":   nil,
			}},
			"customer": nil,
		}},
		ParseExpansionLevel([]string{"charge.customer", "customer", "charge.source"}))

	assert.Equal(t,
		&ExpansionLevel{expansions: map[string]*ExpansionLevel{
			"charge": {expansions: map[string]*ExpansionLevel{
				"customer": {expansions: map[string]*ExpansionLevel{
					"default_source": nil,
				}},
			}},
		}},
		ParseExpansionLevel([]string{"charge.customer.default_source", "charge"}))

	assert.Equal(t,
		&ExpansionLevel{expansions: map[string]*ExpansionLevel{}, wildcard: true},
		ParseExpansionLevel([]string{"*"}))
}

func TestValidateAuth(t *testing.T) {
	testCases := []struct {
		auth string
		want bool
	}{
		{"Basic " + encode64("sk_test_123"), true},
		{"Bearer sk_test_123", true},
		{"", false},
		{"Bearer", false},
		{"Basic", false},
		{"Bearer ", false},
		{"Basic ", false},
		{"Basic 123", false}, // "123" is not a valid key when base64 decoded
		{"Basic " + encode64("sk_test"), false},
		{"Bearer sk_test_123 extra", false},
		{"Bearer sk_test", false},
		{"Bearer sk_test_123_extra", false},
		{"Bearer sk_live_123", false},
		{"Bearer sk_test_", false},
	}
	for _, tc := range testCases {
		t.Run("Authorization: "+tc.auth, func(t *testing.T) {
			assert.Equal(t, tc.want, validateAuth(tc.auth))
		})
	}
}

//
// ---
//

func encode64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func getStubServer(t *testing.T) *StubServer {
	server := &StubServer{spec: testSpec}
	err := server.initializeRouter()
	assert.NoError(t, err)
	return server
}
