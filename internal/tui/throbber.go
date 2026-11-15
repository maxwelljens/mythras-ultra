package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// throbState is what the model is doing right now: idle outside a turn,
// thinking while a model call is in flight, writing while its response
// streams into the transcript, deciding while a decision-model call runs.
type throbState int

const (
	// throbIdle shows no indicator on the status line.
	throbIdle throbState = iota
	// throbThinking is the state between EventThinking and the first
	// narrative delta (or a tool annotation asking for another call).
	throbThinking
	// throbWriting is the state while narrative arrives from the model.
	throbWriting
	// throbDeciding is the state between EventDecideStart and
	// EventDecideStop — the decision model behind the channel check or
	// the golden-rule check is being consulted.
	throbDeciding
)

// The status-line labels, pinned exactly: both LLM-working states read
// "Thinking..." (their spinner frames still differ — dots vs cursor
// block), only the decision model reads "Deciding...".
const (
	labelThinking = "Thinking..."
	labelDeciding = "Deciding..."
)

// writeFrames is the writing state's frame set: a single-cell cursor block
// pulsing between full and thin — every frame one cell wide, so the status
// line never shifts, and visually distinct from the thinking dots.
var writeFrames = spinner.Spinner{
	Frames: []string{"▉", "▊", "▋", "▍", "▎", "▏", "▎", "▍", "▋", "▊"},
	FPS:    time.Second / 8,
}

// decideFrames is the deciding state's frame set: one quadrant of a circle
// orbiting clockwise (◜ ◝ ◞ ◟) — a single cell per frame like every other
// set here, and visibly distinct from both the thinking dots and the
// writing cursor block, so the three states read apart at a glance.
var decideFrames = spinner.Spinner{
	Frames: []string{"◜", "◝", "◞", "◟"},
	FPS:    time.Second / 8,
}

// throbber is the status line's progress indicator: bubbles' spinner ticks
// the frames, and the theme's lipgloss styles colour them (Accent while
// thinking, Success while writing, Highlight while deciding — the theme
// has no Warning slot, and Highlight is the "active element" slot, which
// is what a pending decision is) — lipgloss styles, bubbles ticks. It
// holds no game facts, only the harness events that move it between
// states.
type throbber struct {
	state    throbState
	model    spinner.Model
	accent   lipgloss.Style
	writing  lipgloss.Style
	deciding lipgloss.Style
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
	t.deciding = th.Styles.Highlight
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
	switch state {
	case throbThinking:
		sp = spinner.MiniDot
	case throbDeciding:
		sp = decideFrames
	}
	// A fresh model restarts the frame at its first cell (the frame sets
	// differ in length) and owns the tick chain: tick messages still in
	// flight carry the old model's ID and are ignored, so exactly one
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

// styleFor returns the theme style of one state (empty when idle).
func (t *throbber) styleFor(state throbState) lipgloss.Style {
	switch state {
	case throbThinking:
		return t.accent
	case throbWriting:
		return t.writing
	case throbDeciding:
		return t.deciding
	default:
		return lipgloss.Style{}
	}
}

// label returns the state's status-line text: both LLM-working states
// read "Thinking...", the decision model "Deciding...". Only called for a
// non-idle state.
func (t *throbber) label() string {
	if t.state == throbDeciding {
		return labelDeciding
	}
	return labelThinking
}

// view renders the current spinner frame through the theme's style; empty
// when idle, so an indicator shows only while something runs.
func (t *throbber) view() string {
	if t.state == throbIdle {
		return ""
	}
	m := t.model
	m.Style = t.styleFor(t.state)
	return m.View()
}

// status renders the status line's content: spinner frame, one space and
// the state's label (see label), all through the state's style, clipped
// to w display columns so a narrow terminal can never wrap the row. Empty
// when idle — the caller still consumes the row, so the layout never
// jumps.
func (t *throbber) status(w int) string {
	if t.state == throbIdle {
		return ""
	}
	st := t.styleFor(t.state)
	// The label carries its own style: the frame's SGR reset would
	// otherwise drop the outer colour halfway through the line.
	return fit(st, t.view()+" "+st.Render(t.label()), w)
}
