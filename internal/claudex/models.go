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

var modelProfiles = []ModelProfile{
	{ID: "claude-opus-4-8", Upstream: "gpt-5.6-sol", Label: "Codex GPT-5.6 Sol"},
	{ID: FixedModelID, Upstream: "gpt-5.6-terra", Label: "Codex GPT-5.6 Terra"},
	{ID: "claude-haiku-4-5", Upstream: "gpt-5.6-luna", Label: "Codex GPT-5.6 Luna"},
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
				"max_input_tokens": 200000,
				"max_tokens":       64000,
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
