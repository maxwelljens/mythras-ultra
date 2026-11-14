// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/mythras-ultra/myul/internal/config"
	"github.com/mythras-ultra/myul/internal/harness/llm"
)

// newSessionFlags are `myul new`'s flags; the zero value means "not given"
// (the seed is detected through the flag's Changed state).
type newSessionFlags struct {
	name     string
	seed     int64
	model    string
	provider string
	noTUI    bool
	skipLLM  bool
}

// llmSetup is the state of `myul new`'s LLM-setup step before the form
// runs: still open to a choice, already settled by --provider/--model, or
// skipped by --skip-llm.
type llmSetup int

const (
	// setupAsk shows the "Set up the game master's LLM" select.
	setupAsk llmSetup = iota
	// setupSettled: --provider and --model gave a pair, so the guided
	// answer is already known and never asked.
	setupSettled
	// setupSkip: --skip-llm — no question; the custom path always.
	setupSkip
)

// Values of the LLM-setup select (and of promptNew's skip result).
const (
	// setupGuided picks a provider and model from the embedded catalog.
	setupGuided = "guided"
	// setupExisting keeps the configured provider/model pair.
	setupExisting = "existing"
	// setupCustom defers the LLM to the config file.
	setupCustom = "custom"
)

// newNewCmd builds `myul new [name]`: an interactive form for whatever the
// flags and config leave out, the same creation path for scripts with
// --no-tui (ARCHITECTURE.md §8). LLM setup is optional: --skip-llm (or the
// form's custom choice) writes a template config and creates the session
// without playing.
func newNewCmd() *cobra.Command {
	var f newSessionFlags
	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new session and play it",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			return runNew(cmd, cfg, f, args)
		},
	}
	cmd.Flags().StringVar(&f.name, "name", "", "session name (alternative to the [name] argument)")
	cmd.Flags().Int64Var(&f.seed, "seed", 0, "session dice seed (default: config game.seed; 0 picks one at random)")
	cmd.Flags().StringVar(&f.model, "model", "", "language model id (default: config llm.model)")
	cmd.Flags().StringVar(&f.provider, "provider", "", "provider adapter (default: config llm.provider)")
	cmd.Flags().BoolVar(&f.noTUI, "no-tui", false, "create the session, flush it to disk and exit")
	cmd.Flags().BoolVar(&f.skipLLM, "skip-llm", false,
		"skip guided LLM setup: write a template config if none exists, create the session and exit without the TUI")
	return cmd
}

// runNew resolves name/model/provider/seed and creates the session through
// runSession. Bare `myul` falls through to this with no flags when the
// store is empty, so the non-interactive branch is one code path.
//
// When the LLM setup was deferred (the form's custom choice or
// --skip-llm), the template config is prepared first and the session is
// created headless — play must not start with an unconfigured LLM.
func runNew(cmd *cobra.Command, cfg *config.Config, f newSessionFlags, args []string) error {
	name, provider, model, seed, deferSetup, err := resolveNewInputs(cmd, cfg, f, args)
	if err != nil {
		return err
	}
	if deferSetup {
		if err := deferLLMSetup(cmd); err != nil {
			return err
		}
	}
	if err := runSession(cmd, cfg, sessionOptions{
		name:     name,
		seed:     seed,
		provider: provider,
		model:    model,
		noTUI:    f.noTUI || deferSetup,
	}); err != nil {
		return err
	}
	if !deferSetup {
		return nil
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(),
		"configure [llm] as above, then run `myul resume` to play\n")
	return err
}

// resolveNewInputs merges flags, config and — when a terminal is attached
// and something is missing — the interactive form. Without a terminal it
// never prompts: a missing name is an error naming the usage. The returned
// deferSetup flag selects the custom path (template config, headless
// session, no TUI).
func resolveNewInputs(cmd *cobra.Command, cfg *config.Config, f newSessionFlags, args []string) (name, provider, model string, seed int64, deferSetup bool, err error) {
	name = strings.TrimSpace(f.name)
	if len(args) > 0 {
		arg := strings.TrimSpace(args[0])
		if name != "" && name != arg {
			return "", "", "", 0, false, fmt.Errorf("new: name given twice and differently (%q and %q)", name, arg)
		}
		name = arg
	}
	provider, model = f.provider, f.model
	if provider == "" {
		provider = cfg.LLM.Provider
	}
	if model == "" {
		model = cfg.LLM.Model
	}
	seed = cfg.Game.Seed
	if cmd.Flags().Changed("seed") {
		seed = f.seed
		if seed < 0 {
			return "", "", "", 0, false, fmt.Errorf("new: --seed must be zero or positive, got %d", seed)
		}
	}
	// The LLM-setup step: --skip-llm always takes the custom path; a
	// provider and model both passed as flags settle it (guided, no
	// questions).
	setup := setupAsk
	switch {
	case f.skipLLM:
		setup = setupSkip
	case cmd.Flags().Changed("provider") && cmd.Flags().Changed("model") &&
		f.provider != "" && f.model != "":
		setup = setupSettled
	}
	// The form asks only when it can (a terminal, no --no-tui) and only
	// when something is still missing.
	if !f.noTUI && isTerminal(cmd) && (name == "" || provider == "" || model == "") {
		name, provider, model, seed, deferSetup, err = promptNew(cmd, setup, name, provider, model, seed)
		if err != nil {
			return "", "", "", 0, false, err
		}
	}
	if setup == setupSkip {
		deferSetup = true
	}
	if name == "" {
		return "", "", "", 0, false, errors.New("session name required — start one with `myul new <name>` (or --name)")
	}
	return name, provider, model, seed, deferSetup, nil
}

// promptNew runs the huh form for the LLM-setup choice (when the step is
// still open), the session name and the dice seed. The guided branch is a
// provider select over [llm.ListProviders] — every embedded catalog
// provider, labelled with its API-key env variable — feeding a model
// select over that provider's models. Choosing custom (or --skip-llm)
// returns skip=true: the caller then writes a template config and creates
// the session headless instead of playing.
func promptNew(cmd *cobra.Command, setup llmSetup, name, provider, model string, seed int64) (string, string, string, int64, bool, error) {
	configured := ""
	if provider != "" && model != "" {
		configured = provider + "/" + model
	}

	// Prefill the guided picker with the configured provider when the
	// catalog knows it; otherwise (empty or custom, e.g. "openaicompat")
	// start at the first catalog entry — the form can never silently
	// discard the configured pair because that keeps its own choice.
	providers := llm.ListProviders(cmd.Context())
	providerID := strings.TrimSpace(provider)
	if _, err := llm.ResolveProvider(providerID); err != nil && len(providers) > 0 {
		providerID = providers[0].ID
	}
	modelKey := ""
	if providerID != "" && model != "" {
		modelKey = providerID + "/" + model
	}

	// setupChoice starts at the preselected option of the setup select:
	// existing when a pair is configured, guided otherwise — and for the
	// settled/skipped modes it is what hides the guided group.
	setupChoice := setupCustom
	switch setup {
	case setupAsk:
		setupChoice = setupGuided
		if configured != "" {
			setupChoice = setupExisting
		}
	case setupSettled:
		setupChoice = setupExisting
	}

	seedText := strconv.FormatInt(seed, 10)
	var fields []huh.Field
	if setup == setupAsk {
		setupOptions := make([]huh.Option[string], 0, 3)
		if configured != "" {
			setupOptions = append(setupOptions,
				huh.NewOption("Use existing configuration ("+configured+")", setupExisting))
		}
		setupOptions = append(setupOptions,
			huh.NewOption("Guided — pick a provider and model", setupGuided),
			huh.NewOption("Custom / later — configure "+config.ResolveConfigPath(configPath), setupCustom),
		)
		fields = append(fields, huh.NewSelect[string]().
			Title("Set up the game master's LLM").
			Description("Guided walks the embedded catalog; custom leaves provider/model to the config file").
			Options(setupOptions...).
			Value(&setupChoice))
	}
	fields = append(fields,
		huh.NewInput().
			Title("Session name").
			Description("What this campaign session is called").
			Value(&name).
			Validate(huh.ValidateNotEmpty()),
		huh.NewInput().
			Title("Dice seed").
			Description("0 picks a fresh random seed per session").
			Value(&seedText).
			Validate(validateSeed),
	)
	guided := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider").
			Description("API-key env variable shown in parentheses").
			Options(providerOptions(providers)...).
			Value(&providerID),
		huh.NewSelect[string]().
			Title("Language model").
			Description("Model id sent to the provider").
			OptionsFunc(func() []huh.Option[string] {
				return modelOptions(providerID)
			}, &providerID).
			Value(&modelKey),
		// The guided questions appear only while guided is the choice.
	).WithHideFunc(func() bool { return setupChoice != setupGuided })

	form := huh.NewForm(huh.NewGroup(fields...), guided).
		WithInput(cmd.InOrStdin()).WithOutput(cmd.OutOrStdout())
	if err := form.Run(); err != nil {
		return "", "", "", 0, false, fmt.Errorf("new: form: %w", err)
	}
	parsed, perr := strconv.ParseInt(strings.TrimSpace(seedText), 10, 64)
	if perr != nil || parsed < 0 {
		return "", "", "", 0, false, fmt.Errorf("new: seed %q must be a non-negative integer", seedText)
	}
	name = strings.TrimSpace(name)
	switch setupChoice {
	case setupCustom:
		// Custom/later: the LLM answers stay empty on purpose; the
		// caller writes the template config and creates the session
		// without the TUI.
		return name, "", "", parsed, true, nil
	case setupExisting:
		prov, mdl, ok := strings.Cut(configured, "/")
		if !ok || prov == "" || mdl == "" {
			return "", "", "", 0, false, fmt.Errorf("new: configured pair %q must have the form provider/model", configured)
		}
		return name, prov, mdl, parsed, false, nil
	default:
		prov, mdl, ok := strings.Cut(modelKey, "/")
		if !ok || prov == "" || mdl == "" {
			return "", "", "", 0, false, fmt.Errorf("new: model choice %q must have the form provider/model", modelKey)
		}
		return name, prov, mdl, parsed, false, nil
	}
}

// providerOptions renders the guided provider choices, each labelled with
// its API-key env variable, e.g. "DeepSeek ($DEEPSEEK_API_KEY)".
func providerOptions(providers []llm.ProviderInfo) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(providers))
	for _, p := range providers {
		label := p.Name
		if p.APIKeyEnv != "" {
			label += " ($" + p.APIKeyEnv + ")"
		}
		options = append(options, huh.NewOption(label, p.ID))
	}
	return options
}

// modelOptions lists one provider's models as "provider/model" keys — the
// form promptNew's selection parser expects. An id the catalog does not
// resolve yields no options (the guided flow only offers catalog models).
func modelOptions(providerID string) []huh.Option[string] {
	info, err := llm.ResolveProvider(providerID)
	if err != nil {
		return nil
	}
	options := make([]huh.Option[string], 0, len(info.Models))
	for _, m := range info.Models {
		label := m.ModelID
		if m.Name != "" && m.Name != m.ModelID {
			label = m.Name + " (" + m.ModelID + ")"
		}
		options = append(options, huh.NewOption(label, info.ID+"/"+m.ModelID))
	}
	return options
}

// deferLLMSetup implements the custom/later path of `myul new`: the
// `myul config init` template is written when no config file exists yet —
// an existing file is never touched — and the resolved path plus the [llm]
// keys to set are printed. The caller then creates the session headless;
// play must not start with an unconfigured LLM.
func deferLLMSetup(cmd *cobra.Command) error {
	path := config.ResolveConfigPath(configPath)
	out := cmd.OutOrStdout()
	_, statErr := os.Stat(path)
	switch {
	case statErr == nil:
		if _, err := fmt.Fprintf(out, "using existing config %s\n", path); err != nil {
			return fmt.Errorf("new: report config: %w", err)
		}
	case errors.Is(statErr, fs.ErrNotExist):
		if err := writeDefaultConfig(path, false); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "wrote %s\n", path); err != nil {
			return fmt.Errorf("new: report config: %w", err)
		}
	default:
		return fmt.Errorf("new: stat %s: %w", path, statErr)
	}
	if _, err := fmt.Fprintf(out, "set [llm] provider, model, api_key_env, base_url in %s\n", path); err != nil {
		return fmt.Errorf("new: report config: %w", err)
	}
	return nil
}

// validateSeed accepts a non-negative integer seed; 0 means "pick one at
// session creation".
func validateSeed(s string) error {
	value, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || value < 0 {
		return fmt.Errorf("seed must be a non-negative integer (0 picks one at random), got %q", s)
	}
	return nil
}
