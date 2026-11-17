// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/maxwelljens/mythras-ultra/internal/config"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
	"github.com/maxwelljens/mythras-ultra/internal/state"
)

// sessionStore loads the configuration through the existing --config
// plumbing (an unusable config fails here exactly as it does for doctor)
// and returns the session store over the configured data directory.
// NewJSONStore files sessions under root/sessions, so the root is
// DataDir() and sessions land in SessionsDir() (ARCHITECTURE.md §5).
func sessionStore() (*state.JSONStore, error) {
	if _, err := config.Load(configPath); err != nil {
		return nil, err
	}
	return state.NewJSONStore(config.DataDir(), nil), nil
}

// newSessionsCmd lists saved sessions, newest updated first
// (ARCHITECTURE.md §8).
func newSessionsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List saved sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := sessionStore()
			if err != nil {
				return err
			}
			// List orders by UpdatedAt descending (Store contract).
			infos, err := store.List()
			if err != nil {
				return fmt.Errorf("sessions: %w", err)
			}
			if asJSON {
				if infos == nil {
					infos = []state.SessionInfo{} // JSON array, not null
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(infos); err != nil {
					return fmt.Errorf("sessions: encode: %w", err)
				}
				return nil
			}
			return writeSessionTable(cmd.OutOrStdout(), infos)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print sessions as machine-readable JSON")
	return cmd
}

// writeSessionTable prints one aligned row per session. An empty store is a
// friendly hint, not an error: the command still exits 0.
func writeSessionTable(out io.Writer, infos []state.SessionInfo) error {
	if len(infos) == 0 {
		_, err := fmt.Fprintln(out, "no sessions yet — start one with `myul new <name>`")
		return err
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tSEED\tUPDATED"); err != nil {
		return fmt.Errorf("sessions: write header: %w", err)
	}
	for _, s := range infos {
		row := fmt.Sprintf("%s\t%s\t%s\t%s\n",
			tableCell(s.ID), tableCell(s.Name), strconv.FormatInt(s.Seed, 10),
			s.UpdatedAt.Format(time.RFC3339))
		if _, err := io.WriteString(w, row); err != nil {
			return fmt.Errorf("sessions: write row: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("sessions: flush: %w", err)
	}
	return nil
}

// tableCell keeps user-supplied text (session names are free input) from
// breaking the table layout: tabs and newlines would otherwise shift every
// column.
func tableCell(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// newDeleteCmd removes one session and everything under it — state,
// snapshots and log (ARCHITECTURE.md §8). Deletion is irreversible, so it
// asks for the typed confirmation "delete <id>" unless --force is given.
func newDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a saved session and its log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store, err := sessionStore()
			if err != nil {
				return err
			}
			infos, err := store.List()
			if err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			known := knownIDs(infos)
			if !slices.Contains(known, id) {
				return fmt.Errorf("delete: unknown session %q (known: %s)", id, idList(known))
			}
			if !force {
				confirm := "delete " + id
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Type %q to confirm deletion: ", confirm); err != nil {
					return fmt.Errorf("delete: write prompt: %w", err)
				}
				line, err := readConfirmation(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				if line != confirm {
					return fmt.Errorf("delete: aborted (expected the confirmation %q)", confirm)
				}
			}
			if err := store.Delete(id); err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", id)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete without asking for confirmation")
	return cmd
}

// knownIDs returns the session IDs sorted by name for error messages (List
// already sorts sessions by recency; messages want stability by name).
func knownIDs(infos []state.SessionInfo) []string {
	ids := make([]string, 0, len(infos))
	for _, s := range infos {
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)
	return ids
}

// idList renders known IDs for a message, or "none" for an empty store.
func idList(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

// readConfirmation reads the typed confirmation line, trimming surrounding
// whitespace. Input that ends without a newline (piped or closed stdin)
// still counts as a line; a closed stdin yields an empty string, so the
// confirmation fails to match and the delete aborts — never hangs.
func readConfirmation(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read confirmation: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// newExportCmd writes one session out: the rendered Markdown play log
// (regenerable from log.jsonl, ARCHITECTURE.md §5) by default, or the
// pretty-printed state.json with --state.
func newExportCmd() *cobra.Command {
	var outPath string
	var asState bool
	cmd := &cobra.Command{
		Use:   "export <session-id>",
		Short: "Export a session as a Markdown play log or state JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store, err := sessionStore()
			if err != nil {
				return err
			}
			st, err := store.Load(id)
			if err != nil {
				return fmt.Errorf("export: %w", err)
			}
			var entries []state.LogEntry
			if !asState {
				if entries, err = store.ReadLog(id); err != nil {
					return fmt.Errorf("export: %w", err)
				}
			}

			w := cmd.OutOrStdout()
			var file *os.File
			if outPath != "" {
				// 0o644 matches how the store writes state.json.
				if file, err = os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644); err != nil {
					return fmt.Errorf("export: create %s: %w", outPath, err)
				}
				w = file
			}
			werr := exportPayload(w, st, entries, asState)
			if file != nil {
				if cerr := file.Close(); cerr != nil {
					if werr == nil {
						werr = fmt.Errorf("close %s: %w", outPath, cerr)
					} else {
						werr = errors.Join(werr, fmt.Errorf("close %s: %w", outPath, cerr))
					}
				}
			}
			if werr != nil {
				return fmt.Errorf("export %q: %w", id, werr)
			}
			if outPath != "" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", outPath)
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "write to this file instead of stdout")
	cmd.Flags().BoolVar(&asState, "state", false, "export the pretty-printed state.json instead of the play log")
	return cmd
}

// exportPayload writes the export payload — the rendered transcript, or the
// pretty-printed state when asState is set — to w.
func exportPayload(w io.Writer, st *resolve.State, entries []state.LogEntry, asState bool) error {
	if !asState {
		return state.WriteTranscript(w, st, entries)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}
