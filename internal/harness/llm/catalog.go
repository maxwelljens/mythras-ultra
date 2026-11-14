package llm

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/catwalk/v3/pkg/embedded"
)

// ErrUnknownProvider reports that no provider in the embedded catalog has
// the requested id. [ResolveProvider] returns it wrapped.
var ErrUnknownProvider = errors.New("llm: unknown provider")

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
	// DefaultMaxTokens is the model's default max output tokens (0 when
	// the catalog does not know it).
	DefaultMaxTokens int64 `json:"default_max_tokens,omitempty"`
}

// ProviderInfo is one entry of the offline provider catalog: everything the
// guided `myul new` picker and the config resolution need to reach an
// embedded catwalk provider without a network round-trip.
type ProviderInfo struct {
	// Name is the provider's display name ("DeepSeek").
	Name string `json:"name"`
	// ID is the catwalk provider id, as accepted (case-insensitively) in
	// Config.Provider ("deepseek").
	ID string `json:"id"`
	// APIKeyEnv is the environment variable holding the API key, without
	// the "$" prefix of the catalog placeholder ("DEEPSEEK_API_KEY").
	// Empty when the provider declares no key.
	APIKeyEnv string `json:"api_key_env,omitempty"`
	// BaseURL is the provider's API endpoint. Empty when the catalog
	// declares none; a value like "$AZURE_OPENAI_API_ENDPOINT" is an
	// unresolved placeholder that the config's base_url must replace.
	BaseURL string `json:"base_url,omitempty"`
	// Type is the API the provider speaks — a catwalk endpoint type,
	// "completions" (OpenAI-compatible), "messages" (Anthropic) or
	// "responses" (OpenAI Responses).
	Type string `json:"type,omitempty"`
	// DefaultLargeModel is the provider's preferred large model id, when
	// it declares one.
	DefaultLargeModel string `json:"default_large_model,omitempty"`
	// DefaultSmallModel is the provider's preferred small model id, when
	// it declares one.
	DefaultSmallModel string `json:"default_small_model,omitempty"`
	// Models are the provider's catalogued models.
	Models []ModelInfo `json:"models,omitempty"`
}

// ListProviders returns every provider of the catwalk catalog embedded in
// the binary — sorted by name then id for stable output. It reads embedded
// data only, so no network is involved and ctx is reserved for a future
// online catalog.
func ListProviders(_ context.Context) []ProviderInfo {
	all := embedded.GetAll()
	out := make([]ProviderInfo, 0, len(all))
	for _, provider := range all {
		out = append(out, providerInfo(provider))
	}
	slices.SortFunc(out, func(a, b ProviderInfo) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

// ResolveProvider looks up one provider by its catwalk id, case- and
// space-insensitively ("DeepSeek", "deepseek" and " DEEPSEEK " all resolve).
// An id the embedded catalog does not know fails with [ErrUnknownProvider]
// wrapped, the message listing example ids. It reads embedded data only,
// like [ListProviders] and [ListModels].
func ResolveProvider(id string) (ProviderInfo, error) {
	want := strings.ToLower(strings.TrimSpace(id))
	for _, provider := range embedded.GetAll() {
		if strings.ToLower(string(provider.ID)) == want {
			return providerInfo(provider), nil
		}
	}
	return ProviderInfo{}, fmt.Errorf("%w %q (try e.g. openai, anthropic, openrouter, deepseek, opencode-go)",
		ErrUnknownProvider, id)
}

// ListModels returns the models of every provider in the catwalk catalog
// embedded in the binary — the picker's raw material, sorted by provider
// then model id for stable output. ("openaicompat", the path for arbitrary
// OpenAI-compatible servers, is deliberately absent: catwalk does not
// catalogue it.) It reads embedded data only, so no network is involved and
// ctx is reserved for a future online catalog.
func ListModels(_ context.Context) []ModelInfo {
	var out []ModelInfo
	for _, provider := range embedded.GetAll() {
		for _, model := range provider.Models {
			out = append(out, ModelInfo{
				Provider:         string(provider.ID),
				ModelID:          model.ID,
				Name:             model.Name,
				ContextWindow:    model.ContextWindow,
				DefaultMaxTokens: model.DefaultMaxTokens,
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

// providerInfo converts one catwalk provider into its [ProviderInfo] view:
// the API-key placeholder "$DEEPSEEK_API_KEY" becomes the bare variable
// name "DEEPSEEK_API_KEY" (empty when the provider declares no key).
func providerInfo(provider catwalk.Provider) ProviderInfo {
	models := make([]ModelInfo, 0, len(provider.Models))
	for _, model := range provider.Models {
		models = append(models, ModelInfo{
			Provider:         string(provider.ID),
			ModelID:          model.ID,
			Name:             model.Name,
			ContextWindow:    model.ContextWindow,
			DefaultMaxTokens: model.DefaultMaxTokens,
		})
	}
	return ProviderInfo{
		Name:              provider.Name,
		ID:                string(provider.ID),
		APIKeyEnv:         strings.TrimPrefix(provider.APIKey, "$"),
		BaseURL:           provider.APIEndpoint,
		Type:              string(provider.Type),
		DefaultLargeModel: provider.DefaultLargeModelID,
		DefaultSmallModel: provider.DefaultSmallModelID,
		Models:            models,
	}
}
