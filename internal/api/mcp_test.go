package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPHandlerIsMountedAtRoot(t *testing.T) {
	server := new(APIServer)
	server.SetMCP(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/mcp" {
			t.Fatalf("MCP path = %q", request.URL.Path)
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	response := httptest.NewRecorder()
	server.Router().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("MCP status = %d, want %d", response.Code, http.StatusAccepted)
	}
}

func TestMCPRouteReturnsNotFoundWhenDisabled(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	response := httptest.NewRecorder()
	new(APIServer).Router().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled MCP status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
