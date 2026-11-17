// SPDX-License-Identifier: EUPL-1.2

package session

// Offline tests for IC/OOC mode persistence (meta.json): the mode a session
// was last used on survives Close/Resume whether it changed through Send or
// through SetChannel, fresh and corrupt metadata both default to the
// in-character channel, and SetChannel stays a no-op after Close.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/harness/tools"
	"github.com/maxwelljens/mythras-ultra/internal/state"
	"github.com/maxwelljens/mythras-ultra/internal/tui"
)

// channelConfig builds a config over root for the channel tests; the same
// root is reused so Resume finds what New persisted.
func channelConfig(root string) Config {
	return Config{
		Name:        "Channel Memory",
		Seed:        11,
		Store:       state.NewJSONStore(root, nil),
		Registry:    tools.NewRegistry(nil),
		SessionsDir: filepath.Join(root, "sessions"),
		Autosave:    true,
	}
}

// TestChannelPersistsAcrossResume pins the Send path: a message sent on OOC
// moves the session's mode immediately, Close keeps it, and Resume reopens
// on it. A fresh session with no meta.json starts in character, and a
// SetChannel after Close is a documented no-op that cannot clobber the
// persisted mode.
func TestChannelPersistsAcrossResume(t *testing.T) {
	root := t.TempDir()
	first, err := New(channelConfig(root))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := first.LastChannel(); got != tui.ChannelIC {
		t.Fatalf("fresh session LastChannel = %v, want ChannelIC (no meta.json)", got)
	}
	if err := first.Send(t.Context(), tui.ChannelOOC, "ooc remark"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := first.LastChannel(); got != tui.ChannelOOC {
		t.Errorf("after Send on OOC LastChannel = %v, want ChannelOOC", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := first.SetChannel(tui.ChannelIC); err != nil {
		t.Errorf("SetChannel after Close should be a no-op, got %v", err)
	}
	if got := first.LastChannel(); got != tui.ChannelOOC {
		t.Errorf("SetChannel after Close changed the mode to %v, want it unchanged", got)
	}

	resumed, err := Resume(channelConfig(root), first.ID())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { _ = resumed.Close() })
	if got := resumed.LastChannel(); got != tui.ChannelOOC {
		t.Errorf("resumed LastChannel = %v, want ChannelOOC", got)
	}
}

// TestSetChannelPersistsDirectly pins the toggle path: SetChannel alone
// (ctrl+t, no message sent) writes meta.json, and Resume reopens on that
// mode.
func TestSetChannelPersistsDirectly(t *testing.T) {
	root := t.TempDir()
	first, err := New(channelConfig(root))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := first.SetChannel(tui.ChannelOOC); err != nil {
		t.Fatalf("SetChannel: %v", err)
	}
	if got := first.LastChannel(); got != tui.ChannelOOC {
		t.Errorf("LastChannel after SetChannel = %v, want ChannelOOC", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	resumed, err := Resume(channelConfig(root), first.ID())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { _ = resumed.Close() })
	if got := resumed.LastChannel(); got != tui.ChannelOOC {
		t.Errorf("resumed LastChannel = %v, want ChannelOOC", got)
	}
}

// TestCorruptMetaFallsBackToIC: a mangled meta.json degrades to the default
// channel instead of failing the resume.
func TestCorruptMetaFallsBackToIC(t *testing.T) {
	root := t.TempDir()
	first, err := New(channelConfig(root))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := first.SetChannel(tui.ChannelOOC); err != nil {
		t.Fatalf("SetChannel: %v", err)
	}
	id := first.ID()
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	meta := filepath.Join(root, "sessions", id, "meta.json")
	if err := os.WriteFile(meta, []byte(`{"channel": "OC`), 0o644); err != nil {
		t.Fatalf("corrupt meta.json: %v", err)
	}

	resumed, err := Resume(channelConfig(root), id)
	if err != nil {
		t.Fatalf("Resume with corrupt meta.json must not fail: %v", err)
	}
	t.Cleanup(func() { _ = resumed.Close() })
	if got := resumed.LastChannel(); got != tui.ChannelIC {
		t.Errorf("corrupt meta LastChannel = %v, want ChannelIC", got)
	}
}
