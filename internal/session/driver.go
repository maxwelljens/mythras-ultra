// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/tui"
)

// ErrClosed is returned by [Session.Send] after [Session.Close]: the runner
// has exited and no further input can be processed.
var ErrClosed = errors.New("session: closed")

// ErrNoClient is reported by a turn when the session was built without a
// model client (Config.Client nil — typically `myul new --no-tui` on a host
// with no [llm] provider configured). The slash commands still work: they
// never call the model.
var ErrNoClient = errors.New("session: no language model configured (set [llm] provider and model)")

// Session implements [tui.Driver]; see the interface for the contract.
var _ tui.Driver = (*Session)(nil)

// Send queues the player's text on ch and returns without waiting for the
// model (tui.Driver). The runner echoes the text verbatim as an
// EventNarrative on the same channel — the TUI matches echoes by exact
// text, so it is never reformatted — and only then runs the turn or the
// local slash command. The error is non-nil when the session is closed, the
// text is empty, or ctx ends while the input queue is full.
func (s *Session) Send(ctx context.Context, ch tui.Channel, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("session: refusing an empty message")
	}
	if s.closed.Load() {
		return ErrClosed
	}
	select {
	case <-s.quit:
		return ErrClosed
	default:
	}
	select {
	case s.inputs <- input{ch: ch, text: text}:
		return nil
	case <-s.quit:
		return ErrClosed
	case <-ctx.Done():
		return fmt.Errorf("session: send: %w", ctx.Err())
	}
}

// Events returns the single event channel; every call returns the same one,
// which Close closes (tui.Driver).
func (s *Session) Events() <-chan tui.Event { return s.events }

// Metadata returns a fresh display snapshot of the state behind the panel:
// a cheap read-through under the read lock, never a model call
// (tui.Driver). See buildMetadata for how state shapes map to the panel.
func (s *Session) Metadata() tui.Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildMetadata(s.state)
}

// Close releases the session: it stops the runner (cancelling any in-flight
// model call), performs the final save, writes transcript.md next to
// state.json, and closes the Events channel. It is idempotent — every call
// after the first returns nil — and safe to call while a turn is running:
// the turn's partial state is whatever moves were accepted, each of which
// is atomic (session-loop.md failure policy).
func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(s.quit) // unblocks emits and pending Sends
	s.cancel()    // aborts the in-flight model call or dispatch
	<-s.done      // the runner has exited: no more writers or emitters
	var errs []error
	if err := s.save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.writeTranscript(); err != nil {
		errs = append(errs, err)
	}
	close(s.events)
	return errors.Join(errs...)
}
