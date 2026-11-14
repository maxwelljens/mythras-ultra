package session

import (
	"path/filepath"
	"testing"

	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/tools"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// TestEchoAndReplayCarryAuthorship pins the fix for "on a fresh load the
// chat no longer shows who said what": the echo is emitted with Player set,
// the GM's line with it unset, and a Resume replays both flags from the log
// so the TUI can tag and dim accordingly without any send history.
func TestEchoAndReplayCarryAuthorship(t *testing.T) {
	root := t.TempDir()
	client := llm.NewScripted(
		scriptedText("The candle gutters."),
		scriptedText("It gutters out."),
	)
	cfg := func() Config {
		return Config{
			Name:        "Authorship",
			Seed:        7,
			Store:       state.NewJSONStore(root, nil),
			Client:      client,
			Registry:    tools.NewRegistry(nil),
			SessionsDir: filepath.Join(root, "sessions"),
			Autosave:    true,
		}
	}

	first, err := New(cfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const sent = "I light the candle."
	if err := first.Send(t.Context(), tui.ChannelIC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	evs := nextEvents(t, first, 4) // echo, thinking, narration, turn end
	if evs[0].Kind != tui.EventNarrative || !evs[0].Player {
		t.Errorf("echo event must be narrative with Player set, got kind=%v player=%v", evs[0].Kind, evs[0].Player)
	}
	if evs[2].Player {
		t.Errorf("GM narration must not carry Player")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := Resume(cfg(), first.ID())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	// The replayed play arrives on the event stream: the runner owns the
	// internal queue (it clears it once delivered), so read it from there.
	var sawPlayer, sawGM, extra bool
	for _, ev := range drainReplay(t, second) {
		if ev.Kind != tui.EventNarrative {
			continue
		}
		switch ev.Markdown {
		case sent:
			sawPlayer = ev.Player
		case "The candle gutters.":
			sawGM = !ev.Player
		case "It gutters out.":
			extra = true
		}
	}
	if !sawPlayer {
		t.Error("replayed player line must carry Player so the TUI tags/dims it")
	}
	if !sawGM {
		t.Error("replayed GM line must arrive with Player unset")
	}
	if extra {
		t.Error("the second scripted turn should not have run")
	}
}
