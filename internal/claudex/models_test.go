package claudex

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFixedModelsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	FixedModelsHandler()(ctx)

	var response struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("model count = %d, want 1", len(response.Data))
	}
	if response.Data[0].ID != FixedModelID {
		t.Fatalf("model id = %q, want %q", response.Data[0].ID, FixedModelID)
	}
	if response.Data[0].DisplayName != "Codex GPT-5.6 Luna (xhigh)" {
		t.Fatalf("display name = %q, want fixed Codex label", response.Data[0].DisplayName)
	}
}
