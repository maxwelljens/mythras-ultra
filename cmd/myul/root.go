// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mythras-ultra/myul/internal/config"
	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/state"
)

// configPath is the value of the persistent --config flag; the empty
// string selects the default XDG config path.
var configPath string

// newRootCmd builds the myul command tree for this build. With no
// subcommand, bare `myul` plays (see runDefaultSession).
func newRootCmd() *cobra.Command {
	// noTUI is bare `myul --no-tui`: resolve the session without the TUI,
	// the headless seam for scripts and tests (flag registration below
	// resets it, so every command tree starts fresh).
	var noTUI bool
	root := &cobra.Command{
		Use:          "myul",
		Short:        "Mythras Ultra — an LLM-driven Mythras Imperative TTRPG harness",
		SilenceUsage: true, // errors already say what went wrong; no usage dump
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDefaultSession(cmd, noTUI)
		},
	}
	root.PersistentFlags().StringVar(&configPath, "config", "",
		"path to config file (default: <config dir>/config.toml)")
	root.Flags().BoolVar(&noTUI, "no-tui", false,
		"create or resume the session without starting the TUI (prints its id and exits)")

	root.AddCommand(newNewCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newSessionsCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newExportCmd())
	return root
}

// runDefaultSession implements bare `myul`: an interactive terminal gets
// the session picker (newest session preselected, plus an option to start
// a new session), while a headless run resumes infos[0] — the most
// recently updated session — when noTUI is set, and otherwise reports how
// to play (`--no-tui` is the headless seam: it flushes and prints the
// id). An empty store falls through to `myul new`: the interactive form
// when a terminal is attached, otherwise the non-interactive branch,
// which prints how to start a session and exits 1 (ARCHITECTURE.md §8).
func runDefaultSession(cmd *cobra.Command, noTUI bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	store := state.NewJSONStore(config.DataDir(), nil)
	infos, err := store.List()
	if err != nil {
		return fmt.Errorf("myul: %w", err)
	}
	if len(infos) == 0 {
		return runNew(cmd, cfg, newSessionFlags{noTUI: noTUI}, nil)
	}
	if !isTerminal(cmd) {
		// No picker without a terminal: infos[0] is the newest session
		// (Store.List orders by UpdatedAt descending).
		return runSession(cmd, cfg, sessionOptions{id: infos[0].ID, noTUI: noTUI})
	}
	// pickSession returns "" when the picker's "start a new session"
	// option is chosen.
	id, err := pickSession(cmd, store, true)
	if err != nil {
		return err
	}
	if id == "" {
		return runNew(cmd, cfg, newSessionFlags{noTUI: noTUI}, nil)
	}
	return runSession(cmd, cfg, sessionOptions{id: id, noTUI: noTUI})
}

// newVersionCmd prints the build version and date.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "myul %s (%s)\n", version, buildDate)
			return err
		},
	}
}

// newConfigCmd groups the config subcommands (ARCHITECTURE §8).
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or create the configuration file",
	}
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigInitCmd())
	return cmd
}

// newConfigPathCmd prints the config file path Load would read.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved configuration file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), config.ResolveConfigPath(configPath))
			return err
		},
	}
}

// newConfigInitCmd writes the commented default config file.
func newConfigInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a commented default config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := config.ResolveConfigPath(configPath)
			if err := writeDefaultConfig(path, force); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	return cmd
}

// writeDefaultConfig writes the commented template to path, refusing to
// touch an existing file unless force is set.
func writeDefaultConfig(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config: %s already exists (use --force to overwrite)", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("config: stat %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(defaultConfigTOML), 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// newDoctorCmd validates the config, probes the LLM API key and endpoint
// and probes the decision model: the reachability probes below, then the
// real decide client's model check. Probes report their findings but only
// config errors fail the command.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the config, probe the API key, the LLM endpoint and the decision model",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "ok: config valid (%s)\n",
				config.ResolveConfigPath(configPath)); err != nil {
				return err
			}
			if err := probeAPIKeyEnv(out, cfg); err != nil {
				return err
			}
			if err := probeLLMEndpoint(cmd, out, cfg); err != nil {
				return err
			}
			if err := noteLLMModel(out, cfg); err != nil {
				return err
			}
			if !cfg.Decision.Enabled {
				_, err := fmt.Fprintln(out, "skipped: decision model disabled")
				return err
			}
			if err := probeDecisionModel(cmd, out, cfg.Decision.URL); err != nil {
				return err
			}
			return probeDecisionModelPresence(cmd, out, cfg)
		},
	}
}

// probeAPIKeyEnv reports whether the configured provider's effective
// api-key environment variable (llm.api_key_env, else the embedded
// catalog's default for the provider) holds a value: ok / failed: $XX
// unset, or skipped when no key variable applies. Like the decision
// probes, a missing key is reported but never fails the exit code —
// doctor diagnoses, it does not gate.
func probeAPIKeyEnv(out io.Writer, cfg *config.Config) error {
	env := effectiveAPIKeyEnv(cfg.LLM)
	if env == "" {
		_, err := fmt.Fprintln(out, "skipped: no LLM api key env configured")
		return err
	}
	if os.Getenv(env) == "" {
		_, err := fmt.Fprintf(out, "failed: $%s unset\n", env)
		return err
	}
	_, err := fmt.Fprintf(out, "ok: $%s set\n", env)
	return err
}

// probeLLMEndpoint sends one 2s HTTP GET to <base>/models with the
// configured API key and [llm] headers, reporting ok / failed on out.
// Like the other probes it never fails the command: doctor diagnoses, it
// does not gate. On a non-2xx reply the provider's own error body is
// surfaced verbatim when it parses (e.g. "failed: 401 api_key_expired —
// This API key has expired…"), so an expired key reads as itself instead
// of an opaque HTTP status. It is skipped — with a skipped: line — when
// no endpoint or no API key is configured.
func probeLLMEndpoint(cmd *cobra.Command, out io.Writer, cfg *config.Config) error {
	base, info, ok := llmEndpoint(cfg.LLM)
	if !ok {
		_, err := fmt.Fprintln(out, "skipped: no LLM endpoint configured")
		return err
	}
	key := ""
	if env := effectiveAPIKeyEnv(cfg.LLM); env != "" {
		if key = os.Getenv(env); key == "" {
			_, err := fmt.Fprintf(out, "skipped: LLM endpoint probe ($%s unset)\n", env)
			return err
		}
	}
	endpoint := base + "/models"
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		_, ferr := fmt.Fprintf(out, "failed: LLM endpoint %q: %v\n", endpoint, err)
		return ferr
	}
	for k, v := range cfg.LLM.Headers {
		req.Header.Set(k, v)
	}
	if key != "" {
		if info.Type == "messages" { // Anthropic-style endpoints authenticate with x-api-key.
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	req.Header.Set("User-Agent", "myul/"+version)
	// Stable probe session id: gateways that route on x-opencode-session
	// must not reject the probe before auth has even run.
	req.Header.Set("x-opencode-session", "myul-doctor")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_, ferr := fmt.Fprintf(out, "failed: LLM endpoint unreachable at %s: %v\n", endpoint, err)
		return ferr
	}
	defer resp.Body.Close() //nolint:errcheck // discarded body; a close error cannot change reachability
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, err = fmt.Fprintf(out, "ok: LLM endpoint reachable at %s (HTTP %d)\n", endpoint, resp.StatusCode)
		return err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxLLMProbeBody))
	if msg := llmErrorSummary(resp.StatusCode, body); msg != "" {
		_, err = fmt.Fprintf(out, "failed: %s\n", msg)
	} else {
		_, err = fmt.Fprintf(out, "failed: LLM endpoint %s: HTTP %d\n", endpoint, resp.StatusCode)
	}
	return err
}

// maxLLMProbeBody bounds how much of a non-2xx reply the probe reads; an
// error message lives in the first kilobytes.
const maxLLMProbeBody = 64 << 10

// llmEndpoint resolves the base URL the LLM probe should hit: the
// configured base_url, else the embedded catalog endpoint for the
// provider, normalised so a full endpoint URL ("…/v1/chat/completions")
// probes the same base the client will use. ok is false when nothing
// usable is configured (no provider/base_url, an unknown provider, or an
// unresolved "$ENV" placeholder).
func llmEndpoint(llmCfg config.LLMConfig) (string, llm.ProviderInfo, bool) {
	var info llm.ProviderInfo
	if llmCfg.Provider != "" {
		if i, err := llm.ResolveProvider(llmCfg.Provider); err == nil {
			info = i
		}
	}
	base := strings.TrimSpace(llmCfg.BaseURL)
	if base == "" {
		base = info.BaseURL
	}
	base = llm.NormalizeBaseURL(base)
	if base == "" || (!strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://")) {
		return "", info, false
	}
	return base, info, true
}

// llmErrorSummary renders a provider error body as
// "401 api_key_expired — message", understanding both the OpenAI shape
// ({"error": {…}}) and the Anthropic/OpenCode shape
// ({"type":"error","error":{…}}); the payload may also be a bare string.
// It returns "" when the body is neither, so the caller falls back to a
// plain status line.
func llmErrorSummary(status int, body []byte) string {
	var env struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil || len(env.Error) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(env.Error, &s); err == nil && s != "" {
		return fmt.Sprintf("%d %s", status, s)
	}
	var obj struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(env.Error, &obj); err != nil {
		return ""
	}
	code := obj.Code
	if code == "" {
		code = obj.Type
	}
	switch {
	case code != "" && obj.Message != "":
		return fmt.Sprintf("%d %s — %s", status, code, obj.Message)
	case obj.Message != "":
		return fmt.Sprintf("%d %s", status, obj.Message)
	case code != "":
		return fmt.Sprintf("%d %s", status, code)
	}
	return ""
}

// noteLLMModel prints an advisory when the configured model is missing
// from the embedded catalog for its provider: catalogs go stale and model
// ids are free-form, so this never blocks anything — it is only a note.
func noteLLMModel(out io.Writer, cfg *config.Config) error {
	if cfg.LLM.Model == "" || cfg.LLM.Provider == "" {
		return nil
	}
	info, err := llm.ResolveProvider(cfg.LLM.Provider)
	if err != nil {
		return nil // not a catalog provider (e.g. openaicompat): nothing to check
	}
	for _, m := range info.Models {
		if m.ModelID == cfg.LLM.Model {
			return nil
		}
	}
	_, err = fmt.Fprintf(out, "note: model %q is not in the embedded catalog (may still work — catalogs can be stale)\n", cfg.LLM.Model)
	return err
}

// probeDecisionModelPresence asks the real decide client whether the
// configured model is actually present (the reachability probe only proves
// the server answers). Like the probe, a failure is reported but never
// returned: an unreachable or unpulled decision model is a supported
// degraded state (ARCHITECTURE §3.3).
func probeDecisionModelPresence(cmd *cobra.Command, out io.Writer, cfg *config.Config) error {
	client := decide.New(decide.Config{
		Enabled: true,
		URL:     cfg.Decision.URL,
		Model:   cfg.Decision.Model,
		Timeout: 2 * time.Second,
	})
	if err := client.Healthy(cmd.Context()); err != nil {
		_, ferr := fmt.Fprintf(out, "failed: decision model check: %v\n", err)
		return ferr
	}
	_, err := fmt.Fprintf(out, "ok: decision model available (%s)\n", cfg.Decision.Model)
	return err
}

// probeDecisionModel sends one 2s HTTP GET to decision.url + /api/tags
// (the Ollama health endpoint) and reports ok/failed on out.
//
// Reachability is diagnostic only: an unreachable decision model is a
// supported degraded state (ARCHITECTURE §3.3), so failures are
// reported but never returned as command errors. A URL that cannot
// even form a request is a config error and does fail.
func probeDecisionModel(cmd *cobra.Command, out io.Writer, baseURL string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/tags"
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("doctor: invalid decision url %q: %w", baseURL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_, ferr := fmt.Fprintf(out, "failed: decision model unreachable at %s: %v\n", endpoint, err)
		return ferr
	}
	defer resp.Body.Close() //nolint:errcheck // discarded body; a close error cannot change reachability
	_, err = fmt.Fprintf(out, "ok: decision model reachable at %s (HTTP %d)\n", endpoint, resp.StatusCode)
	return err
}

// defaultConfigTOML is the commented template written by
// `myul config init`. Its values mirror [config.DefaultConfig];
// TestConfigInit keeps the two in sync by loading the written file.
const defaultConfigTOML = `# myul configuration — written by ` + "`myul config init`" + `.
# Precedence: MYUL_* environment variables > this file > built-in defaults.

[llm]
# The game master's provider and model. Empty until you set them: guided
# myul new asks, or fill these in yourself. Any provider in the embedded
# catalog works with just provider + model — api_key_env and base_url then
# fall back to that provider's catalog defaults (explicit values below
# always win). Examples (provider -> api_key_env):
#   openai      -> $OPENAI_API_KEY       anthropic  -> $ANTHROPIC_API_KEY
#   openrouter  -> $OPENROUTER_API_KEY   deepseek   -> $DEEPSEEK_API_KEY
#   opencode-go -> $OPENCODE_API_KEY
provider = ""
model = ""
# Name of the environment variable holding the API key — never the key
# itself. Empty -> the catalog default for the provider above.
api_key_env = ""
# Optional endpoint override. base_url accepts the provider base
# (".../v1") or the full endpoint (".../v1/chat/completions") — the
# endpoint suffix is stripped automatically. Catalog providers carry
# their own endpoint; for an arbitrary OpenAI-compatible server, pick
# the nearest catalog provider (or "openaicompat", the arbitrary-server
# adapter) and point base_url at it, e.g.:
#   provider = "openaicompat"
#   base_url = "http://localhost:11434/v1"
#   api_key_env = ""   # local servers usually need no key
#
# OpenCode Go (free tier), as documented by OpenCode:
#   provider = "opencode-go"
#   model = "beam-free"
#   api_key_env = "OPENCODE_API_KEY"
#   base_url = "https://opencode.ai/inference/go/openai/v1/chat/completions"
base_url = ""
# Extra static request headers sent with every model request:
# [llm.headers]
# x-custom-header = "value"
# myul also sends x-opencode-session: <session id> on every request so
# the provider can route and cache per conversation; the id is stable
# across restarts of the same session.

[decision]
# Optional tev1 decision model (Ollama); when unreachable the harness
# falls back to LLM judgment.
enabled = true
url = "http://localhost:11434"
model = "tev1:4b"

[game]
# Session dice seed; 0 picks a fresh random seed per session.
seed = 0
autosave = true
# House-rule toggles consulted by the engine, e.g.:
# [game.house_rules]
# extended_fumble = true

[ui]
theme = "mythras"
dice_animation = true
metadata_default = false
# One of "adaptive", "16", "terminal".
colour_mode = "adaptive"

# Prompt overrides: system = <path> replaces the embedded GM system prompt
# wholesale (text/template syntax, same {{.SessionName}}-style placeholders).
# [prompts]
# system = "/home/me/prompts/system.md"
`
