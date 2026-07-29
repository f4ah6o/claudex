package claudex

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestAllowedRoutesRejectGenericProxySurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Middleware(&config.Config{}))
	router.GET(anthropicModelsPath, FixedModelsHandler())
	router.POST(anthropicMessagesPath, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST(anthropicCountPath, func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, path := range []string{anthropicModelsPath, anthropicMessagesPath, anthropicCountPath} {
		method := http.MethodPost
		body := `{"model":"gpt-5.6"}`
		if path == anthropicModelsPath {
			method = http.MethodGet
			body = ""
		}
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if path == anthropicModelsPath {
			request.Header.Set("Anthropic-Version", "2023-06-01")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code == http.StatusNotFound {
			t.Fatalf("allowed route %s %s was rejected", method, path)
		}
	}

	for _, path := range []string{"/v1/responses", "/v1/chat/completions", "/v0/management/config", "/v0/resource/plugins/foo", "/backend-api/codex"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("forbidden route %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestClaudexCompositionRootDoesNotDirectlyImportForbiddenModules(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{filepath.Join(root, "cmd", "claudex", "main.go"), filepath.Join(root, "internal", "claudex", "service.go")}
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"internal/api/modules/amp", "internal/api/modules/plugin", "internal/api/handlers/management"} {
			if strings.Contains(string(contents), forbidden) {
				t.Fatalf("%s directly imports forbidden composition module %q", path, forbidden)
			}
		}
	}
}
