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
	if len(response.Data) != len(modelProfiles) {
		t.Fatalf("model count = %d, want %d", len(response.Data), len(modelProfiles))
	}
	for index, profile := range modelProfiles {
		if response.Data[index].ID != profile.ID {
			t.Fatalf("model %d id = %q, want %q", index, response.Data[index].ID, profile.ID)
		}
		if response.Data[index].DisplayName != profile.Label {
			t.Fatalf("model %d display name = %q, want %q", index, response.Data[index].DisplayName, profile.Label)
		}
	}
}

func TestInferenceModelsValue(t *testing.T) {
	var models []struct {
		Name  string `json:"name"`
		Label string `json:"labelOverride"`
	}
	if err := json.Unmarshal([]byte(InferenceModelsValue()), &models); err != nil {
		t.Fatalf("decode inference models: %v", err)
	}
	if len(models) != len(modelProfiles) {
		t.Fatalf("inference model count = %d, want %d", len(models), len(modelProfiles))
	}
	for index, profile := range modelProfiles {
		if models[index].Name != profile.ID || models[index].Label != profile.Label {
			t.Fatalf("inference model %d = %#v, want %#v", index, models[index], profile)
		}
	}
}
