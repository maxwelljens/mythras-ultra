// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/config"
	"github.com/maxwelljens/mythras-ultra/internal/harness/llm"
)

// TestContextWindow pins the footer's window resolution: the config's
// [llm] context_window wins, otherwise the embedded catalog's entry for
// the configured provider and model, otherwise 0 — unknown, which is a
// supported state for a custom model or a stale catalog, never an error.
func TestContextWindow(t *testing.T) {
	catalogued := catalogModel(t)
	tests := []struct {
		name string
		cfg  config.LLMConfig
		want int
	}{
		{"config wins over the catalog", config.LLMConfig{
			Provider: catalogued.Provider, Model: catalogued.ModelID, ContextWindow: 65536,
		}, 65536},
		{"catalog supplies it", config.LLMConfig{
			Provider: catalogued.Provider, Model: catalogued.ModelID,
		}, int(catalogued.ContextWindow)},
		{"unknown provider", config.LLMConfig{Provider: "no-such-provider", Model: "x"}, 0},
		{"unknown model", config.LLMConfig{
			Provider: catalogued.Provider, Model: "no-such-model",
		}, 0},
		{"nothing configured", config.LLMConfig{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contextWindow(tt.cfg); got != tt.want {
				t.Errorf("contextWindow(%+v) = %d, want %d", tt.cfg, got, tt.want)
			}
		})
	}
}

// catalogModel returns the first embedded catalog model that declares a
// context window, so the test never hardcodes an id a catalog update could
// rename.
func catalogModel(t *testing.T) llm.ModelInfo {
	t.Helper()
	for _, m := range llm.ListModels(context.Background()) {
		if m.Provider != "" && m.ContextWindow > 0 {
			return m
		}
	}
	t.Fatal("the embedded catalog declares no model context window")
	return llm.ModelInfo{}
}
