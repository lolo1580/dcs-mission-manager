package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/config"
	"dcsmanager/internal/updatecheck"
)

func TestUpdateEndpointDevelopmentBuild(t *testing.T) {
	server := New(config.Config{HTTPAddr: "127.0.0.1:8080"}, nil, nil, nil, nil, nil, nil, nil)
	server.SetVersion("dev")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/update", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/update: %d", response.Code)
	}
	var result updatecheck.Result
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "development" || result.CurrentVersion != "dev" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
