package llm

import (
	"cmp"
	"context"
	"slices"
	"testing"
)

func TestListModelsFilteredAndSorted(t *testing.T) {
	models := ListModels(context.Background())
	if len(models) == 0 {
		t.Fatal("ListModels returned no entries; the embedded catalog should never be empty")
	}

	allowed := map[string]bool{"openai": true, "anthropic": true, "openrouter": true}
	providers := map[string]bool{}
	for _, m := range models {
		if !allowed[m.Provider] {
			t.Errorf("model %q lists unsupported provider %q", m.ModelID, m.Provider)
		}
		if m.ModelID == "" {
			t.Error("entry with empty model id")
		}
		providers[m.Provider] = true
	}
	if len(providers) < 2 {
		t.Errorf("only %d supported providers present in the catalog, want at least 2: %v", len(providers), providers)
	}

	if !slices.IsSortedFunc(models, func(a, b ModelInfo) int {
		if c := cmp.Compare(a.Provider, b.Provider); c != 0 {
			return c
		}
		return cmp.Compare(a.ModelID, b.ModelID)
	}) {
		t.Error("ListModels output is not sorted by provider then model id")
	}

	again := ListModels(context.Background())
	if !slices.Equal(models, again) {
		t.Error("ListModels is not deterministic across calls")
	}
}
