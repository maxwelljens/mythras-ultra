// SPDX-License-Identifier: EUPL-1.2

// Package config loads and validates myul's TOML configuration and
// resolves the XDG directories the harness lives in.
//
// The config file lives at <config dir>/config.toml (see [ConfigDir])
// and is read with the precedence documented on [Load]: flag path >
// MYUL_* environment > config file > built-in defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/viper"
)

// Accepted values for [UIConfig].ColourMode.
const (
	// ColourModeAdaptive follows the terminal's light/dark profile.
	ColourModeAdaptive = "adaptive"
	// ColourMode16 restricts output to the 16 ANSI colours.
	ColourMode16 = "16"
	// ColourModeTerminal uses the terminal's own ANSI palette.
	ColourModeTerminal = "terminal"
)

// Config is the complete myul configuration: one member per section of
// the TOML file ([llm], [decision], [game], [ui], [prompts]).
type Config struct {
	LLM      LLMConfig         `mapstructure:"llm"`
	Decision DecisionConfig    `mapstructure:"decision"`
	Game     GameConfig        `mapstructure:"game"`
	UI       UIConfig          `mapstructure:"ui"`
	Prompts  map[string]string `mapstructure:"prompts"`
}

// LLMConfig is the [llm] section. Every field defaults to the empty
// string (or zero) and must be set by the user before a session can reach
// a provider — except ContextWindow, which the catalog can supply.
type LLMConfig struct {
	// Provider selects the provider adapter.
	Provider string `mapstructure:"provider"`
	// Model is the provider's model name.
	Model string `mapstructure:"model"`
	// APIKeyEnv names the environment variable holding the API key;
	// the key itself never lives in the config file.
	APIKeyEnv string `mapstructure:"api_key_env"`
	// BaseURL optionally overrides the provider's endpoint — a base URL
	// ("…/v1") or the full endpoint ("…/v1/chat/completions"); the
	// endpoint suffix is normalised away by the LLM client.
	BaseURL string `mapstructure:"base_url"`
	// Headers are extra static request headers sent with every model
	// request ([llm.headers] in TOML), e.g. provider-specific routing
	// headers. myul also sends x-opencode-session automatically — see
	// the config template.
	Headers map[string]string `mapstructure:"headers"`
	// ContextWindow is the model's context window in tokens for the
	// footer's context readout; 0 (the default) means "unknown". The
	// embedded catalog usually supplies it for the configured model —
	// this is the fallback for custom or stale-catalog models.
	ContextWindow int `mapstructure:"context_window"`
}

// DecisionConfig is the [decision] section: the optional tev1/Ollama
// decision model used for cheap adjudication (ARCHITECTURE §6.3).
type DecisionConfig struct {
	// Enabled turns the decision model on. When false the harness
	// falls back to LLM judgment; nothing mechanical depends on it
	// (ARCHITECTURE §3.3).
	Enabled bool `mapstructure:"enabled"`
	// ChannelCheck asks the decision model, before each player message
	// is echoed, whether it belongs to the current IC/OOC channel and
	// holds a disagreeing one (at 0.7+ confidence, 32+ characters) for a
	// N/y confirmation. When false the runner skips the decide call
	// entirely. It is a plain bool: any value validates.
	ChannelCheck bool `mapstructure:"channel_check"`
	// URL is the decision-model base URL.
	URL string `mapstructure:"url"`
	// Model is the decision model's name.
	Model string `mapstructure:"model"`
}

// GameConfig is the [game] section.
type GameConfig struct {
	// Seed is the session dice seed; 0 picks a fresh random seed per
	// session (ARCHITECTURE §3.1).
	Seed int64 `mapstructure:"seed"`
	// Autosave persists session state after each accepted move.
	Autosave bool `mapstructure:"autosave"`
	// MaxToolRounds bounds one turn's tool-call loop: after this many
	// dispatch rounds the next model reply is reported as an EventError
	// (session-loop.md). Zero means "unset" and falls back to the
	// session default (200); any other value must be 1..10000.
	MaxToolRounds int `mapstructure:"max_tool_rounds"`
	// HouseRules are named toggles consulted by the engine rules.
	HouseRules map[string]bool `mapstructure:"house_rules"`
}

// UIConfig is the [ui] section.
type UIConfig struct {
	// Theme is the built-in palette name (mythras, obsidian, parchment,
	// terminal).
	Theme string `mapstructure:"theme"`
	// DiceAnimation enables the dice-theatre animation; disable it for
	// accessibility (ARCHITECTURE §7).
	DiceAnimation bool `mapstructure:"dice_animation"`
	// MetadataDefault shows the right-hand metadata panel at start-up.
	MetadataDefault bool `mapstructure:"metadata_default"`
	// ReplayTools replays the recorded tool annotations from log.jsonl
	// on resume; replayed dice always come back either way (dice always
	// replay). Off by default: the annotation strips are transient.
	ReplayTools bool `mapstructure:"replay_tools"`
	// ColourMode is one of [ColourModeAdaptive], [ColourMode16] or
	// [ColourModeTerminal].
	ColourMode string `mapstructure:"colour_mode"`
}

// DefaultConfig returns the configuration used when neither the config
// file nor the environment supplies a value. The LLM fields are empty
// and must be set by the user.
func DefaultConfig() *Config {
	return &Config{
		Decision: DecisionConfig{
			Enabled:      true,
			ChannelCheck: true,
			URL:          "http://localhost:11434",
			Model:        "tev1:4b",
		},
		Game: GameConfig{
			Seed:          0,
			Autosave:      true,
			MaxToolRounds: 200,
			HouseRules:    map[string]bool{},
		},
		UI: UIConfig{
			Theme:           "mythras",
			DiceAnimation:   true,
			MetadataDefault: false,
			ColourMode:      ColourModeAdaptive,
		},
		Prompts: map[string]string{},
	}
}

// Load reads the configuration for explicitPath — the value of the
// --config flag, or the default <config dir>/config.toml when empty —
// with the precedence: config file selected by the flag, then MYUL_*
// environment variables (dot keys map to underscores, e.g.
// MYUL_LLM_MODEL), then the file, then [DefaultConfig].
//
// A missing config file is not an error: defaults apply. The loaded
// result is validated, so an unusable value (an unknown
// ui.colour_mode, a negative game.seed) is reported as an error.
func Load(explicitPath string) (*Config, error) {
	path := ResolveConfigPath(explicitPath)

	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	v.SetEnvPrefix("MYUL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: decode %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate reports the first unusable value in the configuration.
// [Load] calls it; callers holding a *Config directly can call it too.
func (c *Config) Validate() error {
	switch c.UI.ColourMode {
	case ColourModeAdaptive, ColourMode16, ColourModeTerminal:
	default:
		return fmt.Errorf("config: ui.colour_mode %q is not one of %q, %q or %q",
			c.UI.ColourMode, ColourModeAdaptive, ColourMode16, ColourModeTerminal)
	}
	if c.Game.Seed < 0 {
		return fmt.Errorf("config: game.seed must be zero or positive, got %d", c.Game.Seed)
	}
	// 0 means "unset" and falls back to the session default (200).
	if r := c.Game.MaxToolRounds; r < 0 || r > 10000 {
		return fmt.Errorf("config: game.max_tool_rounds must be 0 (unset) or between 1 and 10000, got %d", r)
	}
	return nil
}

// setDefaults registers every scalar key with Viper: Viper only consults
// the environment for keys it already knows, and AllKeys (hence
// Unmarshal) is built from defaults, config file and flags. The values
// must match [DefaultConfig]; TestLoadDefaults enforces that.
// The map sections ([llm.headers], [game.house_rules], [prompts]) are
// omitted on purpose: Viper flattens map defaults into individual keys,
// so an empty map default would register nothing.
func setDefaults(v *viper.Viper) {
	v.SetDefault("llm.provider", "")
	v.SetDefault("llm.model", "")
	v.SetDefault("llm.api_key_env", "")
	v.SetDefault("llm.base_url", "")
	v.SetDefault("llm.context_window", 0)
	v.SetDefault("decision.enabled", true)
	v.SetDefault("decision.channel_check", true)
	v.SetDefault("decision.url", "http://localhost:11434")
	v.SetDefault("decision.model", "tev1:4b")
	v.SetDefault("game.seed", 0)
	v.SetDefault("game.autosave", true)
	v.SetDefault("game.max_tool_rounds", 200)
	v.SetDefault("ui.theme", "mythras")
	v.SetDefault("ui.dice_animation", true)
	v.SetDefault("ui.metadata_default", false)
	v.SetDefault("ui.replay_tools", false)
	v.SetDefault("ui.colour_mode", ColourModeAdaptive)
}
