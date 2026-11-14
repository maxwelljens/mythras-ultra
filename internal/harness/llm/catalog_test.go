package llm

import (
	"cmp"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestListModelsUnfilteredAndSorted(t *testing.T) {
	models := ListModels(context.Background())
	if len(models) == 0 {
		t.Fatal("ListModels returned no entries; the embedded catalog should never be empty")
	}

	// Every embedded provider's models pass through — the old three-
	// provider filter is gone. DeepSeek must be visible.
	providers := map[string]bool{}
	for _, m := range models {
		if m.ModelID == "" {
			t.Error("entry with empty model id")
		}
		providers[m.Provider] = true
	}
	if !providers["deepseek"] {
		t.Error("ListModels omits deepseek models")
	}
	if len(providers) < 10 {
		t.Errorf("only %d providers present in ListModels, want the full catalog: %v", len(providers), providers)
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

func TestListProviders(t *testing.T) {
	providers := ListProviders(context.Background())
	if len(providers) == 0 {
		t.Fatal("ListProviders returned no entries; the embedded catalog should never be empty")
	}

	if !slices.IsSortedFunc(providers, func(a, b ProviderInfo) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	}) {
		t.Error("ListProviders output is not sorted by name then id")
	}

	byID := map[string]ProviderInfo{}
	for _, p := range providers {
		if p.Name == "" || p.ID == "" {
			t.Errorf("provider %+v has an empty name or id", p)
		}
		if strings.Contains(p.APIKeyEnv, "$") {
			t.Errorf("provider %s APIKeyEnv = %q, want the bare variable name without %q", p.ID, p.APIKeyEnv, "$")
		}
		byID[p.ID] = p
	}

	// The two providers that motivated the unfiltered listing, with the
	// env vars and endpoints their config JSONs declare.
	cases := []struct {
		id, name, keyEnv, baseURL string
	}{
		{"deepseek", "DeepSeek", "DEEPSEEK_API_KEY", "https://api.deepseek.com/v1"},
		{"opencode-go", "OpenCode Go", "OPENCODE_API_KEY", "https://opencode.ai/zen/go/v1"},
	}
	for _, tc := range cases {
		got, ok := byID[tc.id]
		if !ok {
			t.Fatalf("ListProviders omits %q", tc.id)
		}
		if got.APIKeyEnv != tc.keyEnv {
			t.Errorf("%s APIKeyEnv = %q, want %q", tc.id, got.APIKeyEnv, tc.keyEnv)
		}
		if got.BaseURL != tc.baseURL {
			t.Errorf("%s BaseURL = %q, want %q", tc.id, got.BaseURL, tc.baseURL)
		}
		if got.Name != tc.name {
			t.Errorf("%s Name = %q, want %q", tc.id, got.Name, tc.name)
		}
		if len(got.Models) == 0 {
			t.Errorf("%s lists no models", tc.id)
		}
	}

	// Providers that declare no key keep an empty APIKeyEnv.
	if vertexai, ok := byID["vertexai"]; ok && vertexai.APIKeyEnv != "" {
		t.Errorf("vertexai declares no API key, got APIKeyEnv %q", vertexai.APIKeyEnv)
	}

	again := ListProviders(context.Background())
	if !reflect.DeepEqual(providers, again) {
		t.Error("ListProviders is not deterministic across calls")
	}
}

func TestResolveProvider(t *testing.T) {
	// Case- and space-insensitive lookup by catwalk id.
	for _, id := range []string{"deepseek", "DeepSeek", " DEEPSEEK "} {
		got, err := ResolveProvider(id)
		if err != nil {
			t.Fatalf("ResolveProvider(%q): %v", id, err)
		}
		if got.ID != "deepseek" {
			t.Errorf("ResolveProvider(%q).ID = %q, want deepseek", id, got.ID)
		}
		if got.APIKeyEnv != "DEEPSEEK_API_KEY" {
			t.Errorf("ResolveProvider(%q).APIKeyEnv = %q, want DEEPSEEK_API_KEY", id, got.APIKeyEnv)
		}
	}

	_, err := ResolveProvider("no-such-provider")
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("ResolveProvider(unknown) error = %v, want ErrUnknownProvider", err)
	}
	for _, example := range []string{"openai", "deepseek"} {
		if !strings.Contains(err.Error(), example) {
			t.Errorf("unknown-provider error %q does not list the example id %q", err, example)
		}
	}
}
