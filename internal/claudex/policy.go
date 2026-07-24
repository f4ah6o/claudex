package claudex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	DefaultHost           = "127.0.0.1"
	DefaultPort           = 8317
	DefaultAuthDir        = "~/.claudex"
	FixedModelID          = "claude-sonnet-4-6"
	FixedUpstreamModel    = "gpt-5.6-terra"
	DefaultEffort         = "xhigh"
	maxRequestBodyBytes   = 32 << 20
	anthropicMessagesPath = "/v1/messages"
	anthropicCountPath    = "/v1/messages/count_tokens"
	anthropicModelsPath   = "/v1/models"
)

// Policy restricts the generic proxy core to the surface needed by Claude Code
// and to Codex models in the GPT-5.6 family.
type Policy struct {
	aliases map[string]string
}

// Normalize applies Claudex-specific defaults without enabling optional upstream
// providers or management features.
func Normalize(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Host) == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if strings.TrimSpace(cfg.AuthDir) == "" {
		cfg.AuthDir = DefaultAuthDir
	}
	ensureModelAliases(cfg)
}

func ensureModelAliases(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.OAuthModelAlias == nil {
		cfg.OAuthModelAlias = make(map[string][]config.OAuthModelAlias)
	}

	aliases := cfg.OAuthModelAlias["codex"]
	for _, profile := range ModelProfiles() {
		found := false
		for index := range aliases {
			if strings.EqualFold(strings.TrimSpace(aliases[index].Alias), profile.ID) {
				aliases[index].Name = profile.Upstream
				aliases[index].Fork = true
				aliases[index].ForceMapping = true
				found = true
				break
			}
		}
		if !found {
			aliases = append(aliases, config.OAuthModelAlias{
				Name:         profile.Upstream,
				Alias:        profile.ID,
				Fork:         true,
				ForceMapping: true,
			})
		}
	}
	cfg.OAuthModelAlias["codex"] = aliases
}

// Validate verifies that a generic CLIProxyAPI configuration stays within the
// intentionally narrow Claudex product boundary.
func Validate(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is required")
	}

	var problems []string
	if !isLoopbackHost(cfg.Host) {
		problems = append(problems, "host must be loopback-only (127.0.0.1, ::1, or localhost)")
	}
	if cfg.RemoteManagement.AllowRemote || strings.TrimSpace(cfg.RemoteManagement.SecretKey) != "" {
		problems = append(problems, "remote management must remain disabled")
	}
	if cfg.Plugins.Enabled {
		problems = append(problems, "plugins must remain disabled")
	}

	foreignProviders := make([]string, 0, 6)
	if len(cfg.GeminiKey) > 0 {
		foreignProviders = append(foreignProviders, "gemini-api-key")
	}
	if len(cfg.InteractionsKey) > 0 {
		foreignProviders = append(foreignProviders, "interactions-api-key")
	}
	if len(cfg.XAIKey) > 0 {
		foreignProviders = append(foreignProviders, "xai-api-key")
	}
	if len(cfg.ClaudeKey) > 0 {
		foreignProviders = append(foreignProviders, "claude-api-key")
	}
	if len(cfg.OpenAICompatibility) > 0 {
		foreignProviders = append(foreignProviders, "openai-compatibility")
	}
	if len(cfg.VertexCompatAPIKey) > 0 {
		foreignProviders = append(foreignProviders, "vertex-api-key")
	}
	if len(foreignProviders) > 0 {
		problems = append(problems, "unsupported provider configuration: "+strings.Join(foreignProviders, ", "))
	}

	for provider, aliases := range cfg.OAuthModelAlias {
		if len(aliases) == 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(provider), "codex") {
			problems = append(problems, fmt.Sprintf("oauth-model-alias.%s is outside the Codex-only scope", provider))
			continue
		}
		for _, alias := range aliases {
			if !IsGPT56Model(alias.Name) {
				problems = append(problems, fmt.Sprintf("Codex alias %q targets unsupported model %q", alias.Alias, alias.Name))
			}
		}
	}

	for provider, patterns := range cfg.OAuthExcludedModels {
		if len(patterns) > 0 && !strings.EqualFold(strings.TrimSpace(provider), "codex") {
			problems = append(problems, fmt.Sprintf("oauth-excluded-models.%s is outside the Codex-only scope", provider))
		}
	}

	for keyIndex, key := range cfg.CodexKey {
		for _, model := range key.Models {
			if !IsGPT56Model(model.Name) {
				problems = append(problems, fmt.Sprintf("codex-api-key[%d] alias %q targets unsupported model %q", keyIndex, model.Alias, model.Name))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid Claudex configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

// ValidateServe adds checks required only while serving requests. Login can run
// without a client API key, but the proxy cannot.
func ValidateServe(cfg *config.Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	for _, key := range cfg.APIKeys {
		key = strings.TrimSpace(key)
		lower := strings.ToLower(key)
		if key != "" && !strings.HasPrefix(lower, "replace-") && !strings.HasPrefix(lower, "your-api-key") {
			return nil
		}
	}
	return fmt.Errorf("api-keys must contain a non-placeholder key for Claude Code")
}

// NewPolicy builds the client-visible alias map accepted by the model gate.
func NewPolicy(cfg *config.Config) Policy {
	p := Policy{aliases: make(map[string]string)}
	for _, profile := range ModelProfiles() {
		p.aliases[profile.ID] = profile.Upstream
	}
	if cfg == nil {
		return p
	}
	for provider, aliases := range cfg.OAuthModelAlias {
		if !strings.EqualFold(strings.TrimSpace(provider), "codex") {
			continue
		}
		for _, alias := range aliases {
			if IsGPT56Model(alias.Name) && strings.TrimSpace(alias.Alias) != "" {
				p.aliases[strings.TrimSpace(alias.Alias)] = strings.TrimSpace(alias.Name)
			}
		}
	}
	for _, key := range cfg.CodexKey {
		for _, model := range key.Models {
			if IsGPT56Model(model.Name) && strings.TrimSpace(model.Alias) != "" {
				p.aliases[strings.TrimSpace(model.Alias)] = strings.TrimSpace(model.Name)
			}
		}
	}
	return p
}

// IsGPT56Model accepts the base model and all hyphenated GPT-5.6 variants,
// including current and future entries such as gpt-5.6-sol.
func IsGPT56Model(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return model == "gpt-5.6" || strings.HasPrefix(model, "gpt-5.6-")
}

// AllowsModel reports whether a direct GPT-5.6 model or a configured Codex
// alias is allowed.
func (p Policy) AllowsModel(model string) bool {
	model = strings.TrimSpace(model)
	if IsGPT56Model(model) {
		return true
	}
	target, ok := p.aliases[model]
	return ok && IsGPT56Model(target)
}

// Middleware hides every generic upstream API surface except the Anthropic
// Messages endpoints used by Claude Code and rejects non-GPT-5.6 models before
// they reach provider routing.
func Middleware(cfg *config.Config) gin.HandlerFunc {
	policy := NewPolicy(cfg)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" {
			c.Next()
			return
		}
		if path == anthropicModelsPath {
			if c.Request.Method != http.MethodGet || !isAnthropicModelsRequest(c) {
				abortAnthropic(c, http.StatusNotFound, "not_found_error", "Claudex exposes only the Anthropic Messages API")
				return
			}
			c.Next()
			return
		}
		if path != anthropicMessagesPath && path != anthropicCountPath {
			abortAnthropic(c, http.StatusNotFound, "not_found_error", "Claudex exposes only the Anthropic Messages API")
			return
		}
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if c.Request.Method != http.MethodPost {
			abortAnthropic(c, http.StatusMethodNotAllowed, "invalid_request_error", "only POST is supported")
			return
		}
		if c.Request.Body == nil {
			abortAnthropic(c, http.StatusBadRequest, "invalid_request_error", "request body is required")
			return
		}

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxRequestBodyBytes+1))
		if err != nil {
			abortAnthropic(c, http.StatusBadRequest, "invalid_request_error", "could not read request body")
			return
		}
		_ = c.Request.Body.Close()
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		if len(body) > maxRequestBodyBytes {
			abortAnthropic(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body is too large")
			return
		}

		var request struct {
			Model string `json:"model"`
		}
		if err = json.Unmarshal(body, &request); err != nil {
			abortAnthropic(c, http.StatusBadRequest, "invalid_request_error", "request body must be valid JSON")
			return
		}
		if !policy.AllowsModel(request.Model) {
			abortAnthropic(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("model %q is not allowed; use one of the configured Claudex models", request.Model))
			return
		}
		if path == anthropicMessagesPath {
			body, err = applyDefaultEffort(body)
			if err != nil {
				abortAnthropic(c, http.StatusBadRequest, "invalid_request_error", "could not apply the default effort setting")
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			c.Request.ContentLength = int64(len(body))
		}
		c.Next()
	}
}

func applyDefaultEffort(body []byte) ([]byte, error) {
	thinkingType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String()))
	if thinkingType == "disabled" || gjson.GetBytes(body, "output_config.effort").Exists() {
		return body, nil
	}
	updated, err := sjson.SetBytes(body, "thinking.type", "adaptive")
	if err != nil {
		return nil, err
	}
	return sjson.SetBytes(updated, "output_config.effort", DefaultEffort)
}

func isAnthropicModelsRequest(c *gin.Context) bool {
	if c.GetHeader("Anthropic-Version") != "" {
		return true
	}
	return strings.HasPrefix(c.GetHeader("User-Agent"), "claude-cli")
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func abortAnthropic(c *gin.Context, status int, errorType, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errorType,
			"message": message,
		},
	})
}
