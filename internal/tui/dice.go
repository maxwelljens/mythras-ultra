package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Dice theatre timing (ARCHITECTURE §7). The animation only ever replays
// facts carried by a DiceEvent: faces cycle for show and snap to the
// engine's numbers, which are never recomputed here.
const (
	// diceTickBase is the interval of the first rolling tick.
	diceTickBase = 40 * time.Millisecond
	// diceTickRamp lengthens every successive rolling tick so the spin
	// decelerates: 18 ticks from 40ms to 74ms ≈ 1.03s of rolling, inside
	// the designed 0.8–1.2s window.
	diceTickRamp = 2 * time.Millisecond
	// diceRollingTicks is how many ticks the spin lasts.
	diceRollingTicks = 18
	// diceSlowEvery slows each die's face cycling every N rolling ticks,
	// independently of the tick interval.
	diceSlowEvery = 6
	// diceSettleTick paces the die-by-die snap in the settle phase.
	diceSettleTick = 80 * time.Millisecond
	// diceResultHold is how long the finished banner is held in place
	// before it joins the transcript.
	diceResultHold = 700 * time.Millisecond
)

// Bubble Tea messages that drive the theatre. They are produced only by the
// commands the theatre itself returns (tea.Tick); tests inject them
// directly, so no test ever waits on a real timer.
type (
	// diceTickMsg advances the rolling phase by one step.
	diceTickMsg struct{}
	// diceSettleMsg snaps the next die to its final face.
	diceSettleMsg struct{}
	// diceFinishMsg ends the result hold; the host then flushes the roll
	// into the transcript.
	diceFinishMsg struct{}
)

// dicePhase is where a roll is in its lifecycle.
type dicePhase int

const (
	// diceIdle means no roll is on stage.
	diceIdle dicePhase = iota
	// diceRolling is the fast, decelerating number cycling.
	diceRolling
	// diceSettle snaps the dice to their final values one by one.
	diceSettle
	// diceResult holds the finished banner until it is flushed.
	diceResult
)

// DiceTheatre stages one finished roll: the dice spin, snap to their final
// faces one at a time, and the result banner holds before the host files it
// into the transcript. It is a pure UI component with its own Update and
// View: game numbers arrive only through Start's DiceEvent, and the phase
// machine advances solely on the tick messages this type schedules (or that
// tests inject).
type DiceTheatre struct {
	theme   Theme
	enabled bool // config ui.dice_animation; false renders the result at once
	width   int  // display width for centring; < 1 means "not laid out yet"

	phase   dicePhase
	ev      DiceEvent
	sides   int   // largest die in the expression, for plausible spinning
	faces   []int // faces currently displayed
	tick    int   // rolling ticks elapsed
	settled int   // dice snapped so far
}

// NewDiceTheatre builds a theatre for a theme and animation preference.
// With enabled false (config ui.dice_animation off) Start jumps straight to
// the result phase, which is the accessibility path: the same banner, no
// motion. The zero value is an idle theatre and is safe to use.
func NewDiceTheatre(th Theme, enabled bool) DiceTheatre {
	return DiceTheatre{theme: th, enabled: enabled}
}

// SetTheme points the theatre at a theme, after a theme switch or
// background adaptation.
func (d *DiceTheatre) SetTheme(th Theme) { d.theme = th }

// SetWidth sets the display width the view centres itself in; the app passes
// the chat column width on every relayout.
func (d *DiceTheatre) SetWidth(w int) { d.width = w }

// Active reports whether a roll is on stage (spinning, settling or holding
// its result banner).
func (d *DiceTheatre) Active() bool { return d.phase != diceIdle }

// Reset returns the theatre to idle and drops the current roll; the host
// calls it after flushing the banner into the transcript. View renders to
// the empty string afterwards.
func (d *DiceTheatre) Reset() {
	d.phase = diceIdle
	d.ev = DiceEvent{}
	d.faces = nil
	d.tick, d.settled = 0, 0
}

// Start puts ev on stage. It returns the command driving the first
// animation step, or nil when there is nothing to animate: with the theatre
// disabled, Start jumps straight to the result phase and the host is
// expected to flush the roll into the transcript immediately.
func (d *DiceTheatre) Start(ev DiceEvent) tea.Cmd {
	d.ev = ev
	d.tick, d.settled = 0, 0
	d.sides = dieSides(ev.Expr)
	d.faces = make([]int, len(ev.Dice))
	for i, face := range ev.Dice {
		// Start each die on a face other than its result so nothing looks
		// pre-settled while the roll is still running.
		f := (i*3+1)%d.sides + 1
		if f == face {
			f = f%d.sides + 1
		}
		d.faces[i] = f
	}
	if !d.enabled {
		d.phase = diceResult
		return nil
	}
	d.phase = diceRolling
	return d.tickCmd()
}

// Update advances the theatre on one of its tick messages. It returns the
// command that schedules the next step (nil when there is none) and reports
// done when the result banner has been held: the host should then flush the
// roll into the transcript and call Reset. Messages that do not belong to
// the current phase are ignored, so a stray tick after a flush is harmless.
func (d *DiceTheatre) Update(msg tea.Msg) (tea.Cmd, bool) {
	if d.phase == diceIdle {
		return nil, false
	}
	switch msg.(type) {
	case diceTickMsg:
		if d.phase != diceRolling {
			return nil, false
		}
		d.cycleFaces()
		d.tick++
		if d.tick >= diceRollingTicks {
			d.phase = diceSettle
			return d.settleCmd(), false
		}
		return d.tickCmd(), false
	case diceSettleMsg:
		if d.phase != diceSettle {
			return nil, false
		}
		if d.settled < len(d.faces) {
			d.faces[d.settled] = d.ev.Dice[d.settled]
			d.settled++
			if d.settled < len(d.faces) {
				return d.settleCmd(), false
			}
		}
		// Last die snapped (or a roll with no dice at all): banner up.
		d.phase = diceResult
		return d.holdCmd(), false
	case diceFinishMsg:
		if d.phase != diceResult {
			return nil, false
		}
		return nil, true
	}
	return nil, false
}

// View renders the current phase, centred in the theatre's width: a
// recognisable "roll in progress" line while spinning, the dice row (settled
// faces highlighted) while snapping, the bold result banner when finished,
// and the empty string when idle. It is a pure function of the theatre's
// state — no timers, no queries.
func (d *DiceTheatre) View() string {
	switch d.phase {
	case diceRolling:
		return d.viewRun("rolling")
	case diceSettle:
		return d.viewRun("settling")
	case diceResult:
		return d.viewResult()
	default:
		return ""
	}
}

// viewRun renders a running roll: the head line names the roll in progress,
// the second line shows the dice (settled ones highlighted in the settle
// phase) and the modifier.
func (d *DiceTheatre) viewRun(kind string) string {
	head := "⟳ " + d.ev.Expr + " " + kind + "…"
	parts := make([]string, len(d.faces))
	for i, face := range d.faces {
		st := d.theme.Styles.DiceBanner
		if d.phase == diceSettle && i < d.settled {
			st = d.theme.Styles.Highlight
		}
		parts[i] = st.Render("[" + strconv.Itoa(face) + "]")
	}
	row := strings.Join(parts, " ")
	if mod := modSuffix(d.ev.Modifier); mod != "" {
		row += " " + d.theme.Styles.Muted.Render(mod)
	}
	return d.centre(strings.Join([]string{
		d.theme.Styles.Muted.Render(head),
		row,
	}, "\n"))
}

// viewResult renders the bold result banner: the outcome label in its
// outcome style (when the roll has one) over the expression, dice,
// modifier and total.
func (d *DiceTheatre) viewResult() string {
	var lines []string
	if label, style := d.outcome(); label != "" {
		lines = append(lines, style.Render(label))
	}
	lines = append(lines, d.theme.Styles.DiceBanner.Render(rollSummary(d.ev)))
	return d.centre(strings.Join(lines, "\n"))
}

// outcome maps the roll's outcome onto its banner label and theme style.
// Labels are fixed per outcome; colour comes only from the theme's outcome
// slots — critical → Critical (the gold critical slot), success → Success,
// failure → Failure, fumble → Fumble — so the 16-colour budget stays in the
// themes. An outcome from a future engine is still shown, upper-cased, in
// the dice banner style rather than dropped.
func (d *DiceTheatre) outcome() (string, lipgloss.Style) {
	switch o := strings.ToLower(strings.TrimSpace(d.ev.Outcome)); o {
	case "":
		return "", d.theme.Styles.DiceBanner
	case "critical":
		return "✦ CRITICAL ✦", d.theme.Styles.Critical
	case "success":
		return "SUCCESS", d.theme.Styles.Success
	case "failure":
		return "FAILURE", d.theme.Styles.Failure
	case "fumble":
		return "☠ FUMBLE ☠", d.theme.Styles.Fumble
	default:
		return strings.ToUpper(o), d.theme.Styles.DiceBanner
	}
}

// centre places the rendered block in the middle of the display width; a
// width below 1 means "not laid out yet" and is left alone.
func (d *DiceTheatre) centre(s string) string {
	if d.width < 1 {
		return s
	}
	return lipgloss.PlaceHorizontal(d.width, lipgloss.Center, s)
}

// tickCmd schedules the next rolling step. Each tick is a touch slower than
// the last, which runs the spin down without a separate easing curve.
func (d *DiceTheatre) tickCmd() tea.Cmd {
	delay := diceTickBase + time.Duration(d.tick)*diceTickRamp
	return tea.Tick(delay, func(time.Time) tea.Msg { return diceTickMsg{} })
}

// settleCmd schedules the next die snap.
func (d *DiceTheatre) settleCmd() tea.Cmd {
	return tea.Tick(diceSettleTick, func(time.Time) tea.Msg { return diceSettleMsg{} })
}

// holdCmd schedules the end of the result hold.
func (d *DiceTheatre) holdCmd() tea.Cmd {
	return tea.Tick(diceResultHold, func(time.Time) tea.Msg { return diceFinishMsg{} })
}

// cycleFaces advances the spinning faces one step. Each die flips on its own
// cadence — offset by index and slowed as the roll runs down — so the dice
// visibly chatter out of step instead of changing in unison.
func (d *DiceTheatre) cycleFaces() {
	for i := range d.faces {
		interval := 1 + i + d.tick/diceSlowEvery
		if (d.tick+i)%interval == 0 {
			d.faces[i] = d.faces[i]%d.sides + 1
		}
	}
}

// dieSides reports the largest die named in expr (6 for "3d6+2", 20 for
// "d20"), defaulting to 6 when nothing parses. It only shapes the spin: the
// final faces always come from the DiceEvent.
func dieSides(expr string) int {
	sides := 6
	lower := strings.ToLower(expr)
	for i := 0; i+1 < len(lower); i++ {
		if lower[i] != 'd' {
			continue
		}
		j := i + 1
		for j < len(lower) && lower[j] >= '0' && lower[j] <= '9' {
			j++
		}
		if j == i+1 {
			continue
		}
		if n, err := strconv.Atoi(lower[i+1 : j]); err == nil && n > sides {
			sides = n // out-of-range or junk numbers keep the previous value
		}
	}
	return sides
}

// rollSummary renders a roll as "expr [dice] modifier = total", the single
// summary format shared by the transcript banner and the theatre's result.
func rollSummary(ev DiceEvent) string {
	parts := make([]string, 0, len(ev.Dice)+3)
	parts = append(parts, ev.Expr)
	if len(ev.Dice) > 0 {
		dice := make([]string, 0, len(ev.Dice))
		for _, face := range ev.Dice {
			dice = append(dice, strconv.Itoa(face))
		}
		parts = append(parts, "["+strings.Join(dice, " ")+"]")
	}
	if mod := modSuffix(ev.Modifier); mod != "" {
		parts = append(parts, mod)
	}
	return strings.Join(parts, " ") + " = " + strconv.Itoa(ev.Total)
}

// modSuffix renders a roll's flat modifier for a banner: empty for zero,
// "+2"/"-3" otherwise (a negative value already carries its sign).
func modSuffix(m int) string {
	if m == 0 {
		return ""
	}
	if m > 0 {
		return "+" + strconv.Itoa(m)
	}
	return strconv.Itoa(m)
}
