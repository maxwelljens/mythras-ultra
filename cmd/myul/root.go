// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
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
)

// configPath is the value of the persistent --config flag; the empty
// string selects the default XDG config path.
var configPath string

// newRootCmd builds the myul command tree for this build.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "myul",
		Short:        "Mythras Ultra — an LLM-driven Mythras Imperative TTRPG harness",
		SilenceUsage: true, // errors already say what went wrong; no usage dump
	}
	root.PersistentFlags().StringVar(&configPath, "config", "",
		"path to config file (default: <config dir>/config.toml)")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newDoctorCmd())
	return root
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

// newDoctorCmd validates the config and probes the decision model.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the config and probe the decision model",
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
			if !cfg.Decision.Enabled {
				_, err := fmt.Fprintln(out, "skipped: decision model disabled")
				return err
			}
			return probeDecisionModel(cmd, out, cfg.Decision.URL)
		},
	}
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
# Empty until you set them: a session cannot reach a provider without
# provider and model.
provider = ""
model = ""
# Name of the environment variable holding the API key — never the key.
api_key_env = ""
# Optional provider endpoint override.
base_url = ""

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

# Prompt-name → file path overrides for embedded prompts, e.g.:
# [prompts]
# persona = "/home/me/prompts/persona.md"
`
