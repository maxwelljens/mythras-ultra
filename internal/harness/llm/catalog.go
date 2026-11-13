package llm

import (
	"cmp"
	"context"
	"slices"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/catwalk/v3/pkg/embedded"
)

// ModelInfo is one entry of the offline provider/model catalog, enough for a
// `myul doctor` listing or a config-assist prompt.
type ModelInfo struct {
	// Provider is the catwalk provider id ("openai", "anthropic", …).
	Provider string `json:"provider"`
	// ModelID is the provider-specific model id used in Config.Model.
	ModelID string `json:"model_id"`
	// Name is the human-readable model name.
	Name string `json:"name"`
	// ContextWindow is the model's context window in tokens (0 when the
	// catalog does not know it).
	ContextWindow int64 `json:"context_window,omitempty"`
}

// ListModels returns the models from the catwalk catalog embedded in the
// binary, filtered to the providers this package can actually reach —
// "openai", "anthropic" and "openrouter"; "openaicompat" is deliberately
// absent because it covers arbitrary servers that catwalk does not catalogue
// — sorted by provider then model id for stable output. It reads embedded
// data only, so no network is involved and ctx is reserved for a future
// online catalog.
func ListModels(_ context.Context) []ModelInfo {
	var out []ModelInfo
	for _, provider := range embedded.GetAll() {
		switch provider.ID {
		case catwalk.InferenceProviderOpenAI, catwalk.InferenceProviderAnthropic, catwalk.InferenceProviderOpenRouter:
		default:
			continue
		}
		for _, model := range provider.Models {
			out = append(out, ModelInfo{
				Provider:      string(provider.ID),
				ModelID:       model.ID,
				Name:          model.Name,
				ContextWindow: model.ContextWindow,
			})
		}
	}
	slices.SortFunc(out, func(a, b ModelInfo) int {
		if c := cmp.Compare(a.Provider, b.Provider); c != 0 {
			return c
		}
		return cmp.Compare(a.ModelID, b.ModelID)
	})
	return out
}
