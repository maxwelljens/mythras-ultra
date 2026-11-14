// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"errors"
	"fmt"
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
}

// newNewCmd builds `myul new [name]`: an interactive form for whatever the
// flags and config leave out, the same creation path for scripts with
// --no-tui (ARCHITECTURE.md §8).
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
	return cmd
}

// runNew resolves name/model/provider/seed and creates the session through
// runSession. Bare `myul` falls through to this with no flags when the
// store is empty, so the non-interactive branch is one code path.
func runNew(cmd *cobra.Command, cfg *config.Config, f newSessionFlags, args []string) error {
	name, provider, model, seed, err := resolveNewInputs(cmd, cfg, f, args)
	if err != nil {
		return err
	}
	return runSession(cmd, cfg, sessionOptions{
		name:     name,
		seed:     seed,
		provider: provider,
		model:    model,
		noTUI:    f.noTUI,
	})
}

// resolveNewInputs merges flags, config and — when a terminal is attached
// and something is missing — the interactive form. Without a terminal it
// never prompts: a missing name is an error naming the usage.
func resolveNewInputs(cmd *cobra.Command, cfg *config.Config, f newSessionFlags, args []string) (name, provider, model string, seed int64, err error) {
	name = strings.TrimSpace(f.name)
	if len(args) > 0 {
		arg := strings.TrimSpace(args[0])
		if name != "" && name != arg {
			return "", "", "", 0, fmt.Errorf("new: name given twice and differently (%q and %q)", name, arg)
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
			return "", "", "", 0, fmt.Errorf("new: --seed must be zero or positive, got %d", seed)
		}
	}
	// The form asks only when it can (a terminal, no --no-tui) and only
	// when something is still missing.
	if !f.noTUI && isTerminal(cmd) && (name == "" || provider == "" || model == "") {
		name, provider, model, seed, err = promptNew(cmd, name, provider, model, seed)
		if err != nil {
			return "", "", "", 0, err
		}
	}
	if name == "" {
		return "", "", "", 0, errors.New("session name required — start one with `myul new <name>` (or --name)")
	}
	return name, provider, model, seed, nil
}

// promptNew runs the huh form for name, model/provider and seed, prefilled
// with whatever is already known. Models come from the offline catalog
// (llm.ListModels); a configured pair the catalog does not know — an
// openaicompat endpoint pointed at a local server — is offered first so the
// form can never silently discard it.
func promptNew(cmd *cobra.Command, name, provider, model string, seed int64) (string, string, string, int64, error) {
	configured := ""
	if provider != "" && model != "" {
		configured = provider + "/" + model
	}
	keys := modelKeys(configured, llm.ListModels(cmd.Context()))
	modelKey := configured
	if modelKey == "" && len(keys) > 0 {
		modelKey = keys[0]
	}
	options := make([]huh.Option[string], 0, len(keys))
	for _, key := range keys {
		options = append(options, huh.NewOption(key, key))
	}
	seedText := strconv.FormatInt(seed, 10)
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Session name").
			Description("What this campaign session is called").
			Value(&name).
			Validate(huh.ValidateNotEmpty()),
		huh.NewSelect[string]().
			Title("Language model").
			Description("Provider and model the game master talks to").
			Options(options...).
			Value(&modelKey),
		huh.NewInput().
			Title("Dice seed").
			Description("0 picks a fresh random seed per session").
			Value(&seedText).
			Validate(validateSeed),
	)).WithInput(cmd.InOrStdin()).WithOutput(cmd.OutOrStdout())
	if err := form.Run(); err != nil {
		return "", "", "", 0, fmt.Errorf("new: form: %w", err)
	}
	prov, mdl, ok := strings.Cut(modelKey, "/")
	if !ok || prov == "" || mdl == "" {
		return "", "", "", 0, fmt.Errorf("new: model choice %q must have the form provider/model", modelKey)
	}
	parsed, perr := strconv.ParseInt(strings.TrimSpace(seedText), 10, 64)
	if perr != nil || parsed < 0 {
		return "", "", "", 0, fmt.Errorf("new: seed %q must be a non-negative integer", seedText)
	}
	return strings.TrimSpace(name), prov, mdl, parsed, nil
}

// modelKeys lists the selectable "provider/model" keys: the configured
// pair first (added when the catalog does not know it), then the catalog.
func modelKeys(configured string, models []llm.ModelInfo) []string {
	keys := make([]string, 0, len(models)+1)
	known := false
	for _, m := range models {
		key := m.Provider + "/" + m.ModelID
		if key == configured {
			known = true
		}
		keys = append(keys, key)
	}
	if configured != "" && !known {
		keys = append([]string{configured}, keys...)
	}
	return keys
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
