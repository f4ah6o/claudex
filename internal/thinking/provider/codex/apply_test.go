package codex

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/thinking"
	"github.com/tidwall/gjson"
)

func TestApplierNormalizesMaxToXHigh(t *testing.T) {
	modelInfo := &registry.ModelInfo{
		ID:   "gpt-5.6-terra",
		Type: "openai",
		Thinking: &registry.ThinkingSupport{
			Levels: []string{"low", "medium", "high", "xhigh"},
		},
	}

	result, err := NewApplier().Apply([]byte(`{"model":"gpt-5.6-terra"}`), thinking.ThinkingConfig{
		Mode:  thinking.ModeLevel,
		Level: thinking.LevelMax,
	}, modelInfo)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := gjson.GetBytes(result, "reasoning.effort").String(); got != "xhigh" {
		t.Fatalf("reasoning.effort = %q, want xhigh", got)
	}
}

func TestApplierPreservesOtherEffortLevels(t *testing.T) {
	modelInfo := &registry.ModelInfo{
		ID:   "gpt-5.6-terra",
		Type: "openai",
		Thinking: &registry.ThinkingSupport{
			Levels: []string{"low", "medium", "high", "xhigh"},
		},
	}

	for _, level := range []thinking.ThinkingLevel{
		thinking.LevelLow,
		thinking.LevelMedium,
		thinking.LevelHigh,
		thinking.LevelXHigh,
	} {
		t.Run(string(level), func(t *testing.T) {
			result, err := NewApplier().Apply([]byte(`{"model":"gpt-5.6-terra"}`), thinking.ThinkingConfig{
				Mode:  thinking.ModeLevel,
				Level: level,
			}, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got := gjson.GetBytes(result, "reasoning.effort").String(); got != string(level) {
				t.Fatalf("reasoning.effort = %q, want %q", got, level)
			}
		})
	}
}
