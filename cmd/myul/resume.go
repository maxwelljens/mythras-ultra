// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"errors"
	"fmt"
	"time"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/mythras-ultra/myul/internal/config"
	"github.com/mythras-ultra/myul/internal/state"
)

// newResumeCmd builds `myul resume [session-id]`: load a saved session and
// play it. Without an id the interactive picker opens when a terminal is
// attached; otherwise the most recently updated session is picked (the same
// rule bare `myul` applies).
func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume a saved session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			} else {
				store := state.NewJSONStore(config.DataDir(), nil)
				if id, err = pickSession(cmd, store, false); err != nil {
					return err
				}
			}
			return runSession(cmd, cfg, sessionOptions{id: id})
		},
	}
}

// pickSession resolves an omitted session id: the huh picker when a
// terminal is attached (newest session preselected, and — when allowNew
// makes it the bare-`myul` launcher — a final "start a new session"
// option that returns the empty id), otherwise the most recently updated
// session — Store.List orders by UpdatedAt descending. An empty store is
// an error; there is nothing to resume.
func pickSession(cmd *cobra.Command, store *state.JSONStore, allowNew bool) (string, error) {
	infos, err := store.List()
	if err != nil {
		return "", fmt.Errorf("resume: %w", err)
	}
	if len(infos) == 0 {
		return "", errors.New("resume: no sessions yet — start one with `myul new <name>`")
	}
	if !isTerminal(cmd) {
		return infos[0].ID, nil
	}
	// The "new session" entry sits last so that the preselected newest
	// session stays the one plain Enter confirms.
	options := make([]huh.Option[string], 0, len(infos)+1)
	for _, info := range infos {
		label := fmt.Sprintf("%s — %s (%s)",
			info.ID, tableCell(info.Name), info.UpdatedAt.Format(time.RFC3339))
		options = append(options, huh.NewOption(label, info.ID))
	}
	if allowNew {
		options = append(options, huh.NewOption("+ start a new session", ""))
	}
	id := infos[0].ID
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Resume session").
			Description("Newest first").
			Options(options...).
			Value(&id),
	)).WithInput(cmd.InOrStdin()).WithOutput(cmd.OutOrStdout())
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("resume: session picker: %w", err)
	}
	return id, nil
}
