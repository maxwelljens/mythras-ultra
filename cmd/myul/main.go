// SPDX-License-Identifier: EUPL-1.2

// Command myul is the Mythras Ultra terminal harness: an LLM-driven
// Mythras Imperative TTRPG played in the terminal.
package main

import (
	"os"
)

// Build information, overridden at link time by the justfile and
// goreleaser:
//
//	-X main.version=v1.2.3 -X main.buildDate=2026-01-01T00:00:00Z
//
// The defaults describe an unversioned developer build.
var (
	version   = "dev"
	buildDate = "unknown"
)

// main runs the command tree and exits 1 when it reports an error.
// Errors themselves are printed by cobra on stderr.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
