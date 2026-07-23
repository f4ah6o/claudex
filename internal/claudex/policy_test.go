package claudex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"github.com/tidwall/gjson"
)

func TestIsGPT56Model(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"gpt-5.6":          true,
		"gpt-5.6-sol":      true,
		"gpt-5.6-codex":    true,
		" GPT-5.6-SOL ":    true,
		"gpt-5.60":         false,
		"gpt-5.5":          false,
		"team/gpt-5.6-sol": false,
		"claude-opus-4-6":  false,
	}
	for model, want := range tests {
		model, want := model, want
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			if got := IsGPT56Model(model); got != want {
				t.Fatalf("IsGPT56Model(%q) = %v, want %v", model, got, want)
			}
		})
	}
}

func TestValidateRejectsForeignProviders(t *testing.T) {
	t.Parallel()

	cfg := focusedConfig()
	cfg.ClaudeKey = []config.ClaudeKey{{APIKey: "sk-ant-test"}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "claude-api-key") {
		t.Fatalf("Validate() error = %v, want claude-api-key rejection", err)
	}
}

func TestValidateRejectsAliasOutsideGPT56(t *testing.T) {
	t.Parallel()

	cfg := focusedConfig()
	cfg.OAuthModelAlias["codex"] = append(cfg.OAuthModelAlias["codex"], config.OAuthModelAlias{
		Name:  "gpt-5.5",
		Alias: "claude-haiku-4-5",
	})
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "gpt-5.5") {
		t.Fatalf("Validate() error = %v, want unsupported model rejection", err)
	}
}

func TestPolicyAllowsConfiguredClaudeAliases(t *testing.T) {
	t.Parallel()

	policy := NewPolicy(focusedConfig())
	for _, model := range []string{"gpt-5.6", "gpt-5.6-sol", "claude-opus-4-6", "claude-sonnet-4-6"} {
		if !policy.AllowsModel(model) {
			t.Fatalf("AllowsModel(%q) = false, want true", model)
		}
	}
	if policy.AllowsModel("claude-opus-4-8") {
		t.Fatal("AllowsModel(claude-opus-4-8) = true, want false")
	}
}

func TestNormalizeAddsFixedModelAlias(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	Normalize(cfg)

	if len(cfg.OAuthModelAlias["codex"]) != 1 {
		t.Fatalf("fixed aliases = %#v, want one alias", cfg.OAuthModelAlias["codex"])
	}
	alias := cfg.OAuthModelAlias["codex"][0]
	if alias.Name != FixedUpstreamModel || alias.Alias != FixedModelID || !alias.Fork || !alias.ForceMapping {
		t.Fatalf("fixed alias = %#v, want %s -> %s with forced fork", alias, FixedModelID, FixedUpstreamModel)
	}
}

func TestForceFixedEffort(t *testing.T) {
	t.Parallel()

	body, err := forceFixedEffort([]byte(`{"model":"claude-sonnet-4-6","thinking":{"type":"disabled"}}`))
	if err != nil {
		t.Fatalf("forceFixedEffort() error = %v", err)
	}
	if got := gjson.GetBytes(body, "thinking.type").String(); got != "adaptive" {
		t.Fatalf("thinking.type = %q, want adaptive", got)
	}
	if got := gjson.GetBytes(body, "output_config.effort").String(); got != FixedEffort {
		t.Fatalf("output_config.effort = %q, want %q", got, FixedEffort)
	}
}

func TestMiddlewareRestrictsRoutesAndModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Middleware(focusedConfig()))
	handled := 0
	router.POST(anthropicMessagesPath, func(c *gin.Context) {
		handled++
		c.Status(http.StatusNoContent)
	})
	router.POST("/v1/responses", func(c *gin.Context) {
		handled++
		c.Status(http.StatusNoContent)
	})

	for _, model := range []string{"gpt-5.6-sol", "claude-opus-4-6"} {
		request := httptest.NewRequest(http.MethodPost, anthropicMessagesPath, strings.NewReader(`{"model":"`+model+`"}`))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("model %q status = %d, want %d; body=%s", model, response.Code, http.StatusNoContent, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, anthropicModelsPath, nil)
	request.Header.Set("Anthropic-Version", "2023-06-01")
	response := httptest.NewRecorder()
	router.GET(anthropicModelsPath, func(c *gin.Context) {
		handled++
		c.Status(http.StatusNoContent)
	})
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("Anthropic models status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, anthropicMessagesPath, strings.NewReader(`{"model":"gpt-5.5"}`))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unsupported model status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol"}`))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("generic route status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if handled != 3 {
		t.Fatalf("handler called %d times, want 3", handled)
	}
}

func focusedConfig() *config.Config {
	return &config.Config{
		Host:    DefaultHost,
		Port:    DefaultPort,
		AuthDir: DefaultAuthDir,
		SDKConfig: config.SDKConfig{
			APIKeys: []string{"local-test-key"},
		},
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "gpt-5.6-sol", Alias: "claude-opus-4-6", Fork: true, ForceMapping: true},
				{Name: "gpt-5.6-sol", Alias: "claude-sonnet-4-6", Fork: true, ForceMapping: true},
			},
		},
	}
}
