package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// throbState is what the model is doing right now: idle outside a turn,
// thinking while a model call is in flight, writing while its response
// streams into the transcript.
type throbState int

const (
	// throbIdle shows no indicator in the header.
	throbIdle throbState = iota
	// throbThinking is the state between EventThinking and the first
	// narrative delta (or a tool annotation asking for another call).
	throbThinking
	// throbWriting is the state while narrative arrives from the model.
	throbWriting
)

// writeFrames is the writing state's frame set: a single-cell cursor block
// pulsing between full and thin — every frame one cell wide, so the header
// never shifts, and visually distinct from the thinking dots.
var writeFrames = spinner.Spinner{
	Frames: []string{"▉", "▊", "▋", "▍", "▎", "▏", "▎", "▍", "▋", "▊"},
	FPS:    time.Second / 8,
}

// throbber is the header's thinking/writing indicator: bubbles' spinner
// ticks the frames, and the theme's lipgloss styles colour them (Accent
// while thinking, Success while writing) — lipgloss styles, bubbles ticks.
// It holds no game facts, only the harness events that move it between
// states.
type throbber struct {
	state   throbState
	model   spinner.Model
	accent  lipgloss.Style
	writing lipgloss.Style
}

// newThrobber builds an idle throbber styled from th.
func newThrobber(th Theme) throbber {
	t := throbber{model: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
	t.setTheme(th)
	return t
}

// setTheme pushes the theme's styles into the throbber; called at
// construction and on every background-driven theme change.
func (t *throbber) setTheme(th Theme) {
	t.accent = th.Styles.Accent
	t.writing = th.Styles.Success
}

// setState moves the throbber and returns the command that ticks it. A
// transition into a working state arms exactly one tick chain; idle arms
// nothing, so no timer outlives the turn, and re-entering the same state
// is a no-op — repeated thinking events never stack timers.
func (t *throbber) setState(state throbState) tea.Cmd {
	if state == t.state {
		return nil
	}
	t.state = state
	if state == throbIdle {
		return nil // the last pending tick finds idle and is dropped
	}
	sp := writeFrames
	if state == throbThinking {
		sp = spinner.MiniDot
	}
	// A fresh model restarts the frame at its first cell (the two frame
	// sets differ in length) and owns the tick chain: tick messages still
	// in flight carry the old model's ID and are ignored, so exactly one
	// chain lives per state.
	t.model = spinner.New(spinner.WithSpinner(sp))
	return t.model.Tick
}

// update advances the spinner on its tick. A tick arriving while idle is
// dropped without re-arming; a tick from a spinner superseded by a state
// change is rejected by the spinner itself (foreign ID) and likewise
// re-arms nothing — together those two rules stop the timer once the turn
// ends.
func (t *throbber) update(msg tea.Msg) tea.Cmd {
	if t.state == throbIdle {
		return nil
	}
	m, cmd := t.model.Update(msg)
	t.model = m
	return cmd
}

// view renders the current frame through the theme's style; empty when
// idle, so the header shows an indicator only while a turn runs.
func (t *throbber) view() string {
	switch t.state {
	case throbThinking:
		m := t.model
		m.Style = t.accent
		return m.View()
	case throbWriting:
		m := t.model
		m.Style = t.writing
		return m.View()
	default:
		return ""
	}
}
