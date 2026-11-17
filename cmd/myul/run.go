// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/maxwelljens/mythras-ultra/internal/config"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
	"github.com/maxwelljens/mythras-ultra/internal/harness/decide"
	"github.com/maxwelljens/mythras-ultra/internal/harness/llm"
	"github.com/maxwelljens/mythras-ultra/internal/harness/prompts"
	"github.com/maxwelljens/mythras-ultra/internal/harness/tools"
	"github.com/maxwelljens/mythras-ultra/internal/session"
	"github.com/maxwelljens/mythras-ultra/internal/state"
	"github.com/maxwelljens/mythras-ultra/internal/tui"
)

// defaultThemeName is the theme tui.Options falls back to; colour mode
// "terminal" swaps it for the built-in terminal theme (see playTUI).
const defaultThemeName = "mythras"

// sessionOptions are the resolved inputs of one `myul new` / `myul resume`
// / bare-`myul` run.
type sessionOptions struct {
	// id resumes that session; empty creates a fresh one.
	id string
	// name and seed configure a fresh session (ignored when id is set).
	name string
	seed int64
	// provider and model override the config's [llm] section for this run.
	provider, model string
	// noTUI flushes the session to disk and exits instead of starting
	// the TUI (`myul new --no-tui`, for scripting and tests).
	noTUI bool
}

// runSession builds the collaborators from config and either plays the
// session (TUI) or creates/resumes it and exits (--no-tui). It validates
// everything that can fail cheaply — the id, the model configuration, the
// terminal — before any session exists, so a failed run leaves no half-made
// files (ARCHITECTURE.md §8–§9).
func runSession(cmd *cobra.Command, cfg *config.Config, opts sessionOptions) error {
	if opts.provider != "" {
		cfg.LLM.Provider = opts.provider
	}
	if opts.model != "" {
		cfg.LLM.Model = opts.model
	}

	store := state.NewJSONStore(config.DataDir(), nil)
	if opts.id != "" {
		if err := requireSession(store, opts.id); err != nil {
			return err
		}
	}
	if !opts.noTUI {
		if cfg.LLM.Provider == "" || cfg.LLM.Model == "" {
			return fmt.Errorf(
				"no language model configured — set [llm] provider and model in %s, or pass --provider and --model",
				config.ResolveConfigPath(configPath))
		}
		if err := requireAPIKeyEnv(cfg.LLM); err != nil {
			return err
		}
		if !isTerminal(cmd) {
			return errors.New(
				"an interactive terminal is required to play — use --no-tui (create with `myul new --no-tui`, resume the latest with `myul --no-tui`)")
		}
	}

	// Headless runs never converse, so the model client is built only for
	// play: creating a session must not need provider credentials.
	//
	// The provider session id (x-opencode-session) must be stable across
	// restarts for prompt caching, so a fresh session's state — hence its
	// id — is minted before the client is built; a resumed session reuses
	// its persisted id (Store.Load guarantees state.ID == the directory
	// id, so opts.id is the same id).
	var st *resolve.State
	if opts.id == "" {
		st = resolve.NewState(opts.name, opts.seed)
	}
	sessionID := opts.id
	if st != nil {
		sessionID = st.ID
	}
	var client llm.Client
	if !opts.noTUI {
		var err error
		if client, err = newLLMClient(cfg.LLM, sessionID); err != nil {
			return err
		}
	}
	var decision *decide.Client
	if cfg.Decision.Enabled {
		decision = decide.New(decide.Config{
			Enabled: true,
			URL:     cfg.Decision.URL,
			Model:   cfg.Decision.Model,
		})
	}
	label := modelLabel(cfg.LLM)
	scfg := session.Config{
		Name:          opts.name,
		Seed:          opts.seed,
		State:         st,
		ModelName:     label,
		Store:         store,
		Client:        client,
		Registry:      tools.NewRegistry(decision),
		Decide:        decision,
		ChannelCheck:  cfg.Decision.ChannelCheck,
		GMCheck:       cfg.Decision.GMCheck,
		Overrides:     prompts.Overrides{SystemPromptPath: cfg.Prompts["system"]},
		SessionsDir:   config.SessionsDir(),
		Autosave:      cfg.Game.Autosave,
		MaxToolRounds: cfg.Game.MaxToolRounds,
		HistoryWindow: 0, // session default (32 messages)
		ReplayTools:   cfg.UI.ReplayTools,
		ContextWindow: contextWindow(cfg.LLM),
	}

	sess, err := startSession(scfg, opts)
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	// A fresh session's id is reported in both modes: scripts read it
	// from stdout, and play leaves it in the scrollback (ARCHITECTURE §8).
	if opts.id == "" {
		if _, perr := fmt.Fprintf(cmd.OutOrStdout(), "created session %s (%q)\n",
			sess.ID(), sess.Metadata().SessionName); perr != nil {
			return fmt.Errorf("report session id: %w", errors.Join(perr, sess.Close()))
		}
	}
	if opts.noTUI {
		return flushNoTUI(cmd, sess, opts)
	}
	return playTUI(cfg, sess, label)
}

// startSession picks the constructor: New for a fresh session, Resume for
// an id.
func startSession(cfg session.Config, opts sessionOptions) (*session.Session, error) {
	if opts.id != "" {
		return session.Resume(cfg, opts.id)
	}
	return session.New(cfg)
}

// flushNoTUI implements --no-tui: Close performs the final save and
// writes transcript.md, then a resumed session's identity is printed for
// scripts — a created one was already reported by runSession.
func flushNoTUI(cmd *cobra.Command, sess *session.Session, opts sessionOptions) error {
	id, name := sess.ID(), sess.Metadata().SessionName
	if err := sess.Close(); err != nil {
		return fmt.Errorf("flush session: %w", err)
	}
	if opts.id == "" {
		return nil
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "resumed session %s (%q)\n", id, name)
	return err
}

// playTUI runs the Bubble Tea program over the session driver and closes
// the session when the program returns — whether the player quit or the
// program failed (tea.Quit → sess.Close is the clean-shutdown path).
func playTUI(cfg *config.Config, sess *session.Session, label string) error {
	theme := cfg.UI.Theme
	// The alt screen is the model's own: tui.App sets View.AltScreen, so
	// the program needs no screen option here (bubbletea v2 dropped
	// WithAltScreen).
	var progOpts []tea.ProgramOption
	switch cfg.UI.ColourMode {
	case config.ColourMode16:
		// Restrict output to the 16 ANSI colours.
		progOpts = append(progOpts, tea.WithColorProfile(colorprofile.ANSI))
	case config.ColourModeTerminal:
		// The terminal theme hands the palette to the user's own ANSI 16
		// (ARCHITECTURE §7); it only replaces the shipped default, so an
		// explicitly chosen theme still wins.
		if theme == "" || theme == defaultThemeName {
			theme = "terminal"
		}
	}
	app := tui.New(sess, tui.Options{
		Theme:           theme,
		DiceAnimation:   cfg.UI.DiceAnimation,
		MetadataDefault: cfg.UI.MetadataDefault,
		SessionName:     sess.Metadata().SessionName,
		ModelName:       label,
		InitialChannel:  sess.LastChannel(),
	})
	_, runErr := tea.NewProgram(app, progOpts...).Run()
	closeErr := sess.Close()
	if err := errors.Join(runErr, closeErr); err != nil {
		return fmt.Errorf("session ended: %w", err)
	}
	return nil
}

// requireSession fails fast, before anything launches, when the id is not
// in the store — the error lists the sessions that do exist, matching
// `myul delete`'s phrasing.
func requireSession(store *state.JSONStore, id string) error {
	infos, err := store.List()
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	known := knownIDs(infos)
	if !slices.Contains(known, id) {
		return fmt.Errorf("resume: unknown session %q (known: %s)", id, idList(known))
	}
	return nil
}

// newLLMClient builds the production model client from the [llm] section.
// sessionID becomes the provider's x-opencode-session header (empty when no
// session is playing, e.g. headless runs never build a client at all).
// runSession only calls it for play: headless runs (`--no-tui`) never
// converse, so creating a session needs no credentials, and play with no
// model is refused earlier by runSession.
func newLLMClient(cfg config.LLMConfig, sessionID string) (llm.Client, error) {
	if cfg.Provider == "" {
		return nil, nil
	}
	var key string
	if env := effectiveAPIKeyEnv(cfg); env != "" {
		key = os.Getenv(env)
	}
	client, err := llm.NewFantasyClient(llm.Config{
		Provider:  cfg.Provider,
		Model:     cfg.Model,
		APIKey:    key,
		BaseURL:   cfg.BaseURL,
		Headers:   cfg.Headers,
		SessionID: sessionID,
		UserAgent: "myul/" + version,
	})
	if err != nil {
		return nil, fmt.Errorf("llm client: %w", err)
	}
	return client, nil
}

// effectiveAPIKeyEnv returns the environment variable holding the
// provider's API key: the config's llm.api_key_env when set, otherwise the
// embedded catalog's default for the provider — so `provider = "deepseek"`
// alone works with $DEEPSEEK_API_KEY. Precedence: explicit config >
// catalog defaults. Only the variable name resolves here; its value comes
// from the environment (os.Getenv), never from the config file.
func effectiveAPIKeyEnv(cfg config.LLMConfig) string {
	if cfg.APIKeyEnv != "" {
		return cfg.APIKeyEnv
	}
	info, err := llm.ResolveProvider(cfg.Provider)
	if err != nil {
		return ""
	}
	return info.APIKeyEnv
}

// requireAPIKeyEnv fails play — before any session exists — when the
// provider's effective api-key variable holds no value, naming the
// variable and the config file. Providers that declare no key (and native
// "openaicompat" setups) need none.
func requireAPIKeyEnv(cfg config.LLMConfig) error {
	env := effectiveAPIKeyEnv(cfg)
	if env == "" || os.Getenv(env) != "" {
		return nil
	}
	return fmt.Errorf(
		"no API key — export $%s or set [llm] api_key_env in %s",
		env, config.ResolveConfigPath(configPath))
}

// modelLabel renders the model for the TUI header and /meta: the provider
// and model joined when both are known, otherwise whichever is set.
func modelLabel(cfg config.LLMConfig) string {
	switch {
	case cfg.Provider != "" && cfg.Model != "":
		return cfg.Provider + "/" + cfg.Model
	case cfg.Model != "":
		return cfg.Model
	default:
		return cfg.Provider
	}
}

// contextWindow resolves the model's context window in tokens for the
// footer's context readout: the config's [llm] context_window wins, else
// the embedded catalog's entry for the configured provider and model,
// else 0 — unknown, which the footer renders as tokens without a
// percentage. Catalogs go stale and model ids are free-form, so 0 is a
// supported state, never an error.
func contextWindow(cfg config.LLMConfig) int {
	if cfg.ContextWindow > 0 {
		return cfg.ContextWindow
	}
	info, err := llm.ResolveProvider(cfg.Provider)
	if err != nil {
		return 0 // not a catalog provider (e.g. openaicompat)
	}
	for _, model := range info.Models {
		if model.ModelID == cfg.Model {
			return int(model.ContextWindow)
		}
	}
	return 0
}

// isTerminal reports whether the command's input and output are both real
// terminals — huh forms and the TUI need one. Tests and pipes drive the
// commands through buffers, so they take the non-interactive branches.
func isTerminal(cmd *cobra.Command) bool {
	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(in.Fd()) {
		return false
	}
	out, ok := cmd.OutOrStdout().(*os.File)
	return ok && term.IsTerminal(out.Fd())
}
