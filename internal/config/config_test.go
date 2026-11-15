// SPDX-License-Identifier: EUPL-1.2

package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mythras-ultra/myul/internal/config"
)

// scalarEnvKeys are every MYUL_* key Load consults. Tests blank them so
// a developer's environment cannot leak into expectations (Viper treats
// an empty variable as unset).
var scalarEnvKeys = []string{
	"MYUL_LLM_PROVIDER", "MYUL_LLM_MODEL", "MYUL_LLM_API_KEY_ENV", "MYUL_LLM_BASE_URL",
	"MYUL_LLM_CONTEXT_WINDOW", "MYUL_LLM_HEADERS",
	"MYUL_DECISION_ENABLED", "MYUL_DECISION_CHANNEL_CHECK", "MYUL_DECISION_CHECK_GM_OUTPUT",
	"MYUL_DECISION_URL", "MYUL_DECISION_MODEL",
	"MYUL_GAME_SEED", "MYUL_GAME_AUTOSAVE", "MYUL_GAME_MAX_TOOL_ROUNDS",
	"MYUL_UI_THEME", "MYUL_UI_DICE_ANIMATION", "MYUL_UI_METADATA_DEFAULT",
	"MYUL_UI_REPLAY_TOOLS", "MYUL_UI_COLOUR_MODE",
}

// blankEnv clears the MYUL_* scalar keys for the duration of t.
func blankEnv(t *testing.T) {
	t.Helper()
	for _, key := range scalarEnvKeys {
		t.Setenv(key, "")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if !reflect.DeepEqual(cfg.LLM, config.LLMConfig{}) {
		t.Errorf("llm defaults = %+v, want zero values (must be set by the user)", cfg.LLM)
	}
	if !cfg.Decision.Enabled {
		t.Error("decision.enabled default = false, want true")
	}
	if !cfg.Decision.ChannelCheck {
		t.Error("decision.channel_check default = false, want true")
	}
	if !cfg.Decision.GMCheck {
		t.Error("decision.check_gm_output default = false, want true")
	}
	if want := "http://localhost:11434"; cfg.Decision.URL != want {
		t.Errorf("decision.url default = %q, want %q", cfg.Decision.URL, want)
	}
	if want := "tev1:4b"; cfg.Decision.Model != want {
		t.Errorf("decision.model default = %q, want %q", cfg.Decision.Model, want)
	}
	if cfg.Game.Seed != 0 {
		t.Errorf("game.seed default = %d, want 0 (random per session)", cfg.Game.Seed)
	}
	if !cfg.Game.Autosave {
		t.Error("game.autosave default = false, want true")
	}
	if want := 200; cfg.Game.MaxToolRounds != want {
		t.Errorf("game.max_tool_rounds default = %d, want %d", cfg.Game.MaxToolRounds, want)
	}
	if cfg.Game.HouseRules == nil {
		t.Error("game.house_rules default = nil, want an empty map")
	}
	if want := "mythras"; cfg.UI.Theme != want {
		t.Errorf("ui.theme default = %q, want %q", cfg.UI.Theme, want)
	}
	if !cfg.UI.DiceAnimation {
		t.Error("ui.dice_animation default = false, want true")
	}
	if cfg.UI.MetadataDefault {
		t.Error("ui.metadata_default default = true, want false")
	}
	if cfg.UI.ReplayTools {
		t.Error("ui.replay_tools default = true, want false (dice still replay)")
	}
	if cfg.LLM.ContextWindow != 0 {
		t.Errorf("llm.context_window default = %d, want 0 (unknown — the catalog supplies it)", cfg.LLM.ContextWindow)
	}
	if want := config.ColourModeAdaptive; cfg.UI.ColourMode != want {
		t.Errorf("ui.colour_mode default = %q, want %q", cfg.UI.ColourMode, want)
	}
	if cfg.Prompts == nil {
		t.Error("prompts default = nil, want an empty map")
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		toml    string            // config.toml contents; "" means no file
		env     map[string]string // MYUL_* overrides applied after the file
		want    *config.Config    // expected result; nil (without wantErr) means DefaultConfig
		wantErr bool
	}{
		{
			name: "no file applies defaults",
		},
		{
			name: "file values replace defaults",
			toml: `
[llm]
provider = "fantasy"
model = "gpt-x"
api_key_env = "PROVIDER_KEY"
base_url = "https://api.example/v1"
context_window = 128000

[decision]
enabled = false
channel_check = false
check_gm_output = false

[game]
seed = 42
max_tool_rounds = 7
[game.house_rules]
crit_fumble = true

[ui]
theme = "obsidian"
dice_animation = false
metadata_default = true
replay_tools = true
colour_mode = "terminal"

[prompts]
persona = "/tmp/persona.md"
`,
			want: func() *config.Config {
				want := config.DefaultConfig()
				want.LLM = config.LLMConfig{
					Provider:      "fantasy",
					Model:         "gpt-x",
					APIKeyEnv:     "PROVIDER_KEY",
					BaseURL:       "https://api.example/v1",
					ContextWindow: 128000,
				}
				want.Decision.Enabled = false
				want.Decision.ChannelCheck = false
				want.Decision.GMCheck = false
				want.Game.Seed = 42
				want.Game.MaxToolRounds = 7
				want.Game.HouseRules = map[string]bool{"crit_fumble": true}
				want.UI = config.UIConfig{
					Theme:           "obsidian",
					DiceAnimation:   false,
					MetadataDefault: true,
					ReplayTools:     true,
					ColourMode:      config.ColourModeTerminal,
				}
				want.Prompts = map[string]string{"persona": "/tmp/persona.md"}
				return want
			}(),
		},
		{
			name: "partial file keeps defaults for other keys",
			toml: `[ui]
theme = "parchment"
`,
			want: func() *config.Config {
				want := config.DefaultConfig()
				want.UI.Theme = "parchment"
				return want
			}(),
		},
		{
			name: "env overrides file",
			toml: `
[llm]
model = "file-model"
[ui]
theme = "obsidian"
colour_mode = "16"
`,
			env: map[string]string{
				"MYUL_LLM_MODEL": "env-model",
				"MYUL_UI_THEME":  "env-theme",
			},
			want: func() *config.Config {
				want := config.DefaultConfig()
				want.LLM.Model = "env-model"
				want.UI.Theme = "env-theme"
				want.UI.ColourMode = config.ColourMode16
				return want
			}(),
		},
		{
			name: "invalid colour mode",
			toml: `[ui]
colour_mode = "neon"
`,
			wantErr: true,
		},
		{
			name: "negative seed",
			toml: `[game]
seed = -1
`,
			wantErr: true,
		},
		{
			name: "malformed toml",
			toml: `[ui
theme =`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blankEnv(t)
			dir := t.TempDir()
			if tt.toml != "" {
				path := filepath.Join(dir, "config.toml")
				if err := os.WriteFile(path, []byte(tt.toml), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			t.Setenv("MYUL_CONFIG_DIR", dir)

			got, err := config.Load("")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			want := tt.want
			if want == nil {
				want = config.DefaultConfig()
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Load() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestLoadExplicitPath(t *testing.T) {
	blankEnv(t)
	// Point the default location at an empty directory: only the
	// explicit path may be read.
	dir := t.TempDir()
	t.Setenv("MYUL_CONFIG_DIR", dir)

	path := filepath.Join(t.TempDir(), "custom.toml")
	if err := os.WriteFile(path, []byte(`[ui]
theme = "parchment"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", path, err)
	}
	if want := "parchment"; got.UI.Theme != want {
		t.Errorf("ui.theme = %q, want %q", got.UI.Theme, want)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr bool
	}{
		{name: "defaults are valid"},
		{name: "adaptive", mutate: func(c *config.Config) { c.UI.ColourMode = config.ColourModeAdaptive }},
		{name: "16", mutate: func(c *config.Config) { c.UI.ColourMode = config.ColourMode16 }},
		{name: "terminal", mutate: func(c *config.Config) { c.UI.ColourMode = config.ColourModeTerminal }},
		{name: "unknown colour mode", mutate: func(c *config.Config) { c.UI.ColourMode = "rainbow" }, wantErr: true},
		{name: "empty colour mode", mutate: func(c *config.Config) { c.UI.ColourMode = "" }, wantErr: true},
		{name: "negative seed", mutate: func(c *config.Config) { c.Game.Seed = -1 }, wantErr: true},
		{name: "zero seed", mutate: func(c *config.Config) { c.Game.Seed = 0 }},
		{name: "large seed", mutate: func(c *config.Config) { c.Game.Seed = 1 << 40 }},
		{name: "zero max_tool_rounds means unset", mutate: func(c *config.Config) { c.Game.MaxToolRounds = 0 }},
		{name: "max_tool_rounds in range", mutate: func(c *config.Config) { c.Game.MaxToolRounds = 10000 }},
		{name: "negative max_tool_rounds", mutate: func(c *config.Config) { c.Game.MaxToolRounds = -1 }, wantErr: true},
		{name: "max_tool_rounds over the cap", mutate: func(c *config.Config) { c.Game.MaxToolRounds = 10001 }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}
