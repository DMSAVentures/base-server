//go:build integration
// +build integration

package tests

import (
	"net/http"
	"testing"
)

func TestAPI_Health(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "health check returns ok",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				message, ok := response["message"].(string)
				if !ok {
					t.Fatal("Expected 'message' field in response")
				}

				if message != "ok" {
					t.Errorf("Expected message 'ok', got '%s'", message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(t, http.MethodGet, "/health", nil, nil)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}
