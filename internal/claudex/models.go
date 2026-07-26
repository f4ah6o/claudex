package claudex

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ModelProfile describes one Codex model exposed through the Anthropic gateway.
type ModelProfile struct {
	ID       string
	Upstream string
	Label    string
}

const (
	FableModelID        = "claude-fable-5"
	FableUpstreamModel  = "gpt-5.6-sol"
	OpusModelID         = "claude-opus-5"
	OpusUpstreamModel   = "gpt-5.6-terra"
	SonnetModelID       = "claude-sonnet-5"
	SonnetUpstreamModel = "gpt-5.6-luna"
	DefaultModelID      = SonnetModelID
	modelMaxInputTokens = 372000
	modelMaxTokens      = 128000
)

var modelProfiles = []ModelProfile{
	{ID: FableModelID, Upstream: FableUpstreamModel, Label: "Codex GPT-5.6 Sol"},
	{ID: OpusModelID, Upstream: OpusUpstreamModel, Label: "Codex GPT-5.6 Terra"},
	{ID: SonnetModelID, Upstream: SonnetUpstreamModel, Label: "Codex GPT-5.6 Luna"},
}

var compatibilityModelAliases = []ModelProfile{
	{ID: "claude-opus-4-8", Upstream: OpusUpstreamModel},
	{ID: "claude-opus-4-7", Upstream: OpusUpstreamModel},
	{ID: "claude-opus-4-6", Upstream: OpusUpstreamModel},
	{ID: "claude-sonnet-4-6", Upstream: SonnetUpstreamModel},
}

func supportedModelAliases() []ModelProfile {
	aliases := ModelProfiles()
	return append(aliases, compatibilityModelAliases...)
}

// ModelProfiles returns the models exposed by ClaudexDesktop.
func ModelProfiles() []ModelProfile {
	return append([]ModelProfile(nil), modelProfiles...)
}

// InferenceModelsValue returns the JSON string expected by Claude Desktop's
// inferenceModels setting.
func InferenceModelsValue() string {
	models := make([]map[string]string, 0, len(modelProfiles))
	for _, profile := range modelProfiles {
		models = append(models, map[string]string{
			"name":          profile.ID,
			"labelOverride": profile.Label,
		})
	}
	value, _ := json.Marshal(models)
	return string(value)
}

// FixedModelsHandler returns the Codex models exposed by ClaudexDesktop.
func FixedModelsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		models := ModelProfiles()
		data := make([]map[string]any, 0, len(models))
		for _, profile := range models {
			data = append(data, map[string]any{
				"id":               profile.ID,
				"object":           "model",
				"type":             "model",
				"owned_by":         "claudex",
				"display_name":     profile.Label,
				"created_at":       "2026-01-01T00:00:00Z",
				"max_input_tokens": modelMaxInputTokens,
				"max_tokens":       modelMaxTokens,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"data":     data,
			"has_more": false,
			"first_id": models[0].ID,
			"last_id":  models[len(models)-1].ID,
		})
	}
}
