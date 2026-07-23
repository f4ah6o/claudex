package claudex

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// FixedModelsHandler returns the single Claude-compatible model exposed by
// ClaudexDesktop. The client-visible ID remains Claude-shaped so Desktop can
// discover it, while request routing maps it to FixedUpstreamModel.
func FixedModelsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data": []map[string]any{{
				"id":               FixedModelID,
				"object":           "model",
				"type":             "model",
				"owned_by":         "claudex",
				"display_name":     "Codex GPT-5.6 Luna (xhigh)",
				"created_at":       "2026-01-01T00:00:00Z",
				"max_input_tokens": 200000,
				"max_tokens":       64000,
			}},
			"has_more": false,
			"first_id": FixedModelID,
			"last_id":  FixedModelID,
		})
	}
}
