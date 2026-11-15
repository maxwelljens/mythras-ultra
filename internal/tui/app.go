package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
)

// Options configures the app shell.
type Options struct {
	// Theme is the built-in theme name (see Names); empty means "mythras".
	Theme string
	// DiceAnimation toggles the dice theatre animation (config
	// ui.dice_animation). When false the theatre renders the result phase
	// immediately and the banner joins the transcript at once — the same
	// facts, no motion.
	DiceAnimation bool
	// MetadataDefault opens the metadata panel at startup.
	MetadataDefault bool
	// SessionName labels the session in the header and window title.
	SessionName string
	// ModelName labels the language model in the header.
	ModelName string
	// InitialChannel is the channel the input line starts on — the
	// session's last-used mode (Session.LastChannel). The zero value is
	// ChannelIC, the default for a session with no saved mode.
	InitialChannel Channel
}

// entry is one transcript item together with its cached rendering. width is
// the chat width the text was rendered for; a mismatch triggers a re-render.
// player marks a harness echo of the player's own message, tagged "You"
// rather than "GM". tagged reports whether the cached text carries the tag
// line: only the first entry of a run of messages from the same speaker on
// the same channel shows its tags (see tagVisibility).
type entry struct {
	ev     Event
	player bool
	tagged bool
	text   string
	width  int
}

// App is the chat application shell. It implements tea.Model and owns only
// UI state: layout, transcript rendering, input line and toggles. Game facts
// arrive as Events from the harness Driver; nothing is computed here.
type App struct {
	driver Driver
	events <-chan Event
	opts   Options

	themeName string
	theme     Theme
	bgKnown   bool // terminal background reported by tea.BackgroundColorMsg
	bgDark    bool

	channel Channel
	input   textinput.Model
	history []string
	histIdx int // len(history) means "fresh line, not browsing history"

	viewport viewport.Model
	entries  []entry
	follow   bool // auto-scroll to the bottom on new events

	dice    DiceTheatre // the roll currently on stage (idle when empty)
	diceEv  Event       // the event the theatre is staging
	diceIdx int         // transcript position the roll will be filed at

	throb throbber // the status-line progress indicator (idle when nothing runs)

	panelOpen bool
	helpOpen  bool
	sheetOpen bool // the view-only character sheet overlay (palette action)
	// sheetScroll is the sheet overlay's own scroll position; it never
	// touches the transcript viewport or follow mode.
	sheetScroll int
	// argMode names the command whose argument the input line is
	// collecting (only "roll"); "" means a normal chat line.
	argMode string
	// confirm is set while an EventConfirm or EventConfirmRegen waits for
	// its y/n answer: the input line shows the matching prompt (confirmRegen
	// picks the golden-rule "Y/n" line over the channel check's "N/y") and
	// only the answer keys (plus ctrl+c) reach the app. confirmText is the
	// held message the answer decides about — "n" puts it back into the
	// line (it stays empty for a regeneration, which holds no draft).
	confirm      bool
	confirmRegen bool
	confirmText  string
	pal          palette  // the ctrl+p command palette (closed by default)
	meta         Metadata // cached Driver.Metadata snapshot (see refreshMetadata)
	echoes       []string // recently sent texts awaiting their harness echo

	width  int
	height int
	chatW  int // chat column width (window width minus panel)

	md      *glamour.TermRenderer // markdown renderer cache
	mdWidth int
	mdDark  bool
}

// Bubble Tea messages the app consumes internally.
type (
	// eventMsg delivers one harness event from Driver.Events.
	eventMsg struct{ ev Event }
	// driverClosedMsg reports that the driver's event channel was closed.
	driverClosedMsg struct{}
	// sendResultMsg reports the outcome of an asynchronous Driver.Send.
	sendResultMsg struct {
		err  error
		text string // the sent text, to withdraw its echo on failure
	}
	// commandResultMsg reports the outcome of an asynchronous
	// Driver.Command issued from the palette or the input line's
	// argument mode.
	commandResultMsg struct {
		err  error
		name string // the command that was run, for the error line
	}
)

// maxEchoes bounds how many sent messages are remembered for echo
// recognition; older entries drop off.
const maxEchoes = 8

// New builds the app shell around a harness driver. The caller owns d: the
// model reads d.Events() for its lifetime but never closes it — the CLI
// closes the driver when the program exits. An invalid opts.Theme does not
// fail New; it is surfaced as a transcript error line and the default theme
// is used.
func New(d Driver, opts Options) *App {
	if opts.Theme == "" {
		opts.Theme = defaultTheme
	}
	a := &App{
		driver:    d,
		opts:      opts,
		themeName: opts.Theme,
		channel:   opts.InitialChannel,
		follow:    true,
		panelOpen: opts.MetadataDefault,
		viewport: viewport.New(
			viewport.WithWidth(1),
			viewport.WithHeight(1),
		),
	}
	a.viewport.FillHeight = true
	a.viewport.SoftWrap = true // belt: long lines the harness sends still wrap
	if d != nil {
		a.events = d.Events()
	}
	a.input = textinput.New()
	a.input.Prompt = promptFor(a.channel)
	a.input.Focus() // the input line starts focused; Init issues the blink command
	a.pal = newPalette()

	th, err := Get(a.themeName)
	if err != nil {
		a.errorLine(fmt.Errorf("theme: %w", err))
		th, _ = Get(defaultTheme) // defaultTheme is a built-in; safe fallback
		a.themeName = th.Name
	}
	a.theme = th
	a.applyInputStyles()
	a.pal.applyStyles(th)
	a.throb = newThrobber(th)
	a.dice = NewDiceTheatre(th, opts.DiceAnimation)
	a.refreshMetadata()
	return a
}

// Init starts the input cursor, the driver event stream, the terminal
// background query (whose answer adapts the theme via Adapt) and the slow
// metadata refresh tick.
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.input.Focus(), a.requestEvents(), tea.RequestBackgroundColor, a.metadataTick())
}

// Update applies one message. It never blocks: sends run as commands and
// rendering happens here only for the events that arrived.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.relayout()
		return a, nil

	case tea.BackgroundColorMsg:
		a.bgKnown, a.bgDark = true, msg.IsDark()
		a.applyTheme()
		return a, nil

	case eventMsg:
		roll := a.ingest(msg.ev)
		a.refreshMetadata()
		return a, tea.Batch(a.requestEvents(), roll)

	case driverClosedMsg:
		a.systemLine("event stream closed — the harness is no longer sending updates")
		a.throb.setState(throbIdle) // no EventTurnEnd will ever arrive
		return a, nil

	case sendResultMsg:
		if msg.err != nil {
			a.dropEcho(msg.text) // no echo will arrive for a failed send
			a.errorLine(fmt.Errorf("send failed: %w", msg.err))
		}
		return a, nil

	case commandResultMsg:
		if msg.err != nil {
			a.errorLine(fmt.Errorf("command %s failed: %w", msg.name, msg.err))
		}
		return a, nil

	case metadataTickMsg:
		a.refreshMetadata()
		return a, a.metadataTick()

	case spinner.TickMsg:
		// The throbber re-arms itself only while something runs; a late
		// tick after EventTurnEnd (or a decide stop) returns no command,
		// so no timer survives.
		return a, a.throb.update(msg)

	case diceTickMsg, diceSettleMsg, diceFinishMsg:
		cmd, done := a.dice.Update(msg)
		if done {
			a.flushDice()
		} else {
			a.syncViewport() // repaint the theatre row in place
		}
		return a, cmd

	case editorFinishedMsg:
		a.finishEditor(msg)
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case tea.MouseWheelMsg:
		// While the sheet overlay is up the wheel scrolls it; the
		// transcript keeps its own position and follow mode untouched.
		if a.sheetOpen {
			a.scrollSheet(wheelDelta(msg))
			return a, nil
		}
		vp, cmd := a.viewport.Update(msg)
		a.viewport = vp
		a.follow = a.viewport.AtBottom()
		return a, cmd
	}

	// Everything else (paste, cursor queries, …) goes to the input line.
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// View renders the whole screen: header, transcript (plus the metadata
// panel when open, or the help overlay when it is up), the status line,
// input line and footer, in the active theme.
func (a *App) View() tea.View {
	var body string
	bodyH := a.height - 4 // header, status line, input line, footer
	if bodyH < 1 {
		bodyH = 1
	}
	switch {
	case a.sheetOpen:
		// The sheet replaces transcript and panel entirely; it has its
		// own scroll (see renderSheet).
		body = a.renderSheet()
	case a.pal.open:
		// The palette sits over the transcript like the help modal.
		body = a.pal.view(a.theme, a.width, bodyH)
	case a.helpOpen:
		// The overlay takes the transcript area; the chat waits behind it.
		body = renderHelp(a.theme, a.width, bodyH)
	case a.panelOpen && a.width >= 1:
		body = lipgloss.JoinHorizontal(lipgloss.Top, a.viewport.View(), a.renderPanel())
	default:
		body = a.viewport.View()
	}
	content := strings.Join([]string{
		a.renderHeader(),
		body,
		a.renderStatus(),
		a.input.View(),
		a.renderFooter(),
	}, "\n")

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	title := "myul"
	if a.opts.SessionName != "" {
		title += " — " + a.opts.SessionName
	}
	v.WindowTitle = title
	// The terminal theme keeps the user's own background/foreground: NoColor
	// slots must not override them.
	if _, ok := a.theme.Palette[SlotBackground].(lipgloss.NoColor); !ok {
		v.BackgroundColor = a.theme.Palette[SlotBackground]
	}
	if _, ok := a.theme.Palette[SlotForeground].(lipgloss.NoColor); !ok {
		v.ForegroundColor = a.theme.Palette[SlotForeground]
	}
	return v
}

// requestEvents reads the next driver event. It is re-armed after every
// event so the stream keeps flowing without blocking Update.
func (a *App) requestEvents() tea.Cmd {
	events := a.events
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return driverClosedMsg{}
		}
		return eventMsg{ev: ev}
	}
}

// handleKey dispatches a key press: the palette, help and sheet overlays
// capture keys first (one overlay at a time), then a pending confirmation
// takes its answer (the channel check's hold or the golden-rule
// regeneration), then the argument mode collects its expression, then
// global shortcuts, then history and scrolling, with everything else
// handed to the input line.
func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.pal.open {
		return a.handlePaletteKey(msg)
	}
	if a.helpOpen {
		return a.handleHelpKey(msg)
	}
	if a.sheetOpen {
		return a.handleSheetKey(msg)
	}
	if a.confirm {
		return a.handleConfirmKey(msg)
	}
	if a.argMode != "" {
		return a.handleArgKey(msg)
	}
	switch k := msg.String(); k {
	case "ctrl+c":
		return a, tea.Quit
	case "ctrl+t":
		a.toggleChannel()
		return a, nil
	case "ctrl+m":
		a.togglePanel()
		return a, nil
	case "ctrl+e":
		return a, a.openEditor()
	case "ctrl+l":
		a.clearTranscript()
		return a, nil
	case "ctrl+p":
		return a, a.openPalette()
	case "enter":
		return a, a.submit()
	case "up":
		a.historyUp()
		return a, nil
	case "down":
		a.historyDown()
		return a, nil
	case "esc":
		a.input.Reset()
		a.histIdx = len(a.history)
		return a, nil
	case "pgup", "pgdown":
		vp, cmd := a.viewport.Update(msg)
		a.viewport = vp
		a.follow = a.viewport.AtBottom()
		return a, cmd
	case "q":
		// q quits only with an empty input line so it never steals a message.
		if a.input.Value() == "" {
			return a, tea.Quit
		}
	}
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// togglePanel flips the metadata panel (ctrl+m): a plain two-state view
// toggle that behaves the same under every overlay.
func (a *App) togglePanel() {
	a.panelOpen = !a.panelOpen
	a.refreshMetadata()
	a.relayout()
}

// handlePaletteKey runs while the command palette is up: ctrl+c still
// quits, enter executes the selection (the palette closes first), esc or
// ctrl+p close it, and every other key — q included — only filters.
func (a *App) handlePaletteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch a.pal.handleKey(msg) {
	case outQuit:
		return a, tea.Quit
	case outChosen:
		c, ok := a.pal.selected()
		a.pal.close()
		if !ok {
			return a, nil
		}
		return a, a.executePalette(c)
	default: // outFilter or outClose: the palette state changed only
		return a, nil
	}
}

// openPalette shows the command palette. Only one overlay is ever up:
// help and the sheet close first, and the catalogue is rebuilt for the
// current channel so its dynamic entries (the channel switch) are current.
func (a *App) openPalette() tea.Cmd {
	a.helpOpen = false
	a.sheetOpen = false
	a.pal.applyStyles(a.theme)
	return a.pal.openWith(PaletteCommands(a.channel))
}

// executePalette runs one palette entry. Local actions (actNone entries
// excepted) act on the app's own view state and never reach the session —
// the character sheet is view-only by design; entries carrying a Session
// name run over Driver.Command; Roll dice switches the input line into
// argument mode instead, so its expression arrives on Enter.
func (a *App) executePalette(c PaletteCommand) tea.Cmd {
	switch c.local {
	case actSheet:
		a.openSheet()
		return nil
	case actClear:
		a.clearTranscript()
		return nil
	case actHelp:
		a.helpOpen = true
		return nil
	case actEditor:
		return a.openEditor()
	case actQuit:
		return tea.Quit
	case actChannelIC:
		a.setChannel(ChannelIC)
		return nil
	case actChannelOOC:
		a.setChannel(ChannelOOC)
		return nil
	case actTheme:
		a.applyThemeName(c.arg)
		return nil
	case actRollArg:
		a.beginArg(c.Session)
		return nil
	default:
		if c.Session == "" {
			return nil
		}
		return a.command(c.Session)
	}
}

// command returns the command that runs a named Driver.Command off the
// update loop; the outcome (if any) arrives as a commandResultMsg. It
// reports a missing driver as an error line instead of a panic.
func (a *App) command(name string, args ...string) tea.Cmd {
	d := a.driver
	if d == nil {
		a.errorLine(errors.New("cannot run command: no harness driver connected"))
		return nil
	}
	return func() tea.Msg {
		return commandResultMsg{err: d.Command(name, args...), name: name}
	}
}

// beginArg puts the input line into argument mode for a command that needs
// one (only "roll" does): the prompt becomes "expr ▸ ", the line starts
// empty, esc cancels back to the channel prompt and enter runs
// Driver.Command(name, expr).
func (a *App) beginArg(name string) {
	a.argMode = name
	a.input.Reset()
	a.input.Prompt = argPrompt
	a.sizeInput()
}

// cancelArg leaves argument mode, restoring the channel prompt.
func (a *App) cancelArg() {
	if a.argMode == "" {
		return
	}
	a.argMode = ""
	a.input.Prompt = promptFor(a.channel)
	a.sizeInput()
}

// handleArgKey runs while the input line collects a command argument: enter
// runs the command (an empty line cancels), esc cancels, ctrl+c quits, and
// every other key — q and history included — only types into the line.
func (a *App) handleArgKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit
	case "esc":
		a.cancelArg()
		return a, nil
	case "enter":
		expr := strings.TrimSpace(a.input.Value())
		name := a.argMode
		a.input.Reset()
		if expr == "" {
			a.cancelArg()
			return a, nil
		}
		a.cancelArg()
		return a, a.command(name, expr)
	case "pgup", "pgdown":
		vp, cmd := a.viewport.Update(msg)
		a.viewport = vp
		a.follow = a.viewport.AtBottom()
		return a, cmd
	}
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// beginConfirm puts the input line into confirm mode for a pending
// [EventConfirm] or [EventConfirmRegen]: the question is already on the
// transcript, the prompt becomes the question's own default — "N/y ▸ " for
// a channel hold (n counts unless the player says y), "Y/n ▸ " for a
// golden-rule regeneration (y counts unless the player says n) — and an
// open roll argument mode gives way — only the answer may be typed until
// the confirmation resolves.
func (a *App) beginConfirm(ev Event) {
	a.cancelArg()
	a.confirm = true
	a.confirmRegen = ev.Kind == EventConfirmRegen
	a.confirmText = ev.ConfirmText
	if a.confirmRegen {
		a.input.Prompt = regenPrompt
	} else {
		a.input.Prompt = confirmPrompt
	}
	a.sizeInput()
}

// leaveConfirm ends confirm mode and restores the channel prompt. restore
// puts the held message back into the input line (the channel check's "n"
// answer: the text returns unsent), keeping any text the player had
// already typed so nothing they wrote is lost; a regeneration holds no
// draft, so its answers always leave restored false.
func (a *App) leaveConfirm(restored bool) {
	if !a.confirm {
		return
	}
	a.confirm = false
	a.confirmRegen = false
	text := a.confirmText
	a.confirmText = ""
	a.input.Prompt = promptFor(a.channel)
	if restored {
		if cur := a.input.Value(); cur != "" {
			text = text + " " + cur
		}
		a.input.SetValue(text)
		a.input.CursorEnd()
	}
	a.sizeInput()
}

// handleConfirmKey runs while the input line answers a pending
// confirmation. The channel check answers y/Y with y and n/N/enter/esc
// with n (returning the held text to the line unsent); the golden-rule
// regeneration inverts the default — enter/y/Y regenerate, n/N/esc keep
// the reply on screen (nothing to restore). ctrl+c quits and every other
// key is ignored so the pending answer cannot be typed around.
func (a *App) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.confirmRegen {
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "enter", "y", "Y":
			cmd := a.command("confirm", "y")
			a.leaveConfirm(false)
			return a, cmd
		case "n", "N", "esc":
			cmd := a.command("confirm", "n")
			a.leaveConfirm(false)
			return a, cmd
		}
		return a, nil
	}
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit
	case "y", "Y":
		cmd := a.command("confirm", "y")
		a.leaveConfirm(false)
		return a, cmd
	case "n", "N", "enter", "esc":
		cmd := a.command("confirm", "n")
		a.leaveConfirm(true)
		return a, cmd
	}
	return a, nil
}

// handleSheetKey runs while the character sheet overlay is up: esc closes
// it, pgup/pgdn scroll it, ctrl+p swaps to the palette (closing it), the
// panel toggle keeps working, and every other key reaches the input line —
// the sheet is view-only, so the player can still type their next message.
func (a *App) handleSheetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k := msg.String(); k {
	case "ctrl+c":
		return a, tea.Quit
	case "esc":
		a.closeSheet()
		return a, nil
	case "ctrl+p":
		return a, a.openPalette()
	case "ctrl+m":
		a.togglePanel()
		return a, nil
	case "pgup", "pgdown":
		a.scrollSheet(a.sheetPage(k == "pgup"))
		return a, nil
	case "enter":
		// The sheet is view-only: a typed message still sends normally.
		return a, a.submit()
	}
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// handleHelpKey runs while the help overlay is up. esc and enter dismiss
// it, ctrl+c still quits, ctrl+p swaps to the command palette, and every
// other key reaches only the input line — shortcuts stay captured, but the
// player can still type their next message while reading.
func (a *App) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit
	case "ctrl+p":
		return a, a.openPalette()
	case "esc":
		a.helpOpen = false
		return a, nil
	case "enter":
		a.helpOpen = false
		return a, nil
	}
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// submit sends the input line to the harness as a chat message — always.
// Typed text is never parsed for commands: prose that looks like "/save"
// reaches the model as prose, and commands come from the palette
// (Driver.Command) instead. Empty input is ignored; the text is recorded in
// local history either way.
func (a *App) submit() tea.Cmd {
	text := a.input.Value()
	if text == "" {
		return nil
	}
	a.input.Reset()
	a.history = append(a.history, text)
	a.histIdx = len(a.history)

	// The text is remembered so the harness's echo of the player's own
	// message can be recognised when it comes back through Events.
	a.recordEcho(text)
	return a.send(text)
}

// send returns the command that delivers text to the harness off the update
// loop; the result (if any) comes back as a sendResultMsg.
func (a *App) send(text string) tea.Cmd {
	d := a.driver
	if d == nil {
		a.errorLine(errors.New("cannot send: no harness driver connected"))
		return nil
	}
	ch := a.channel
	return func() tea.Msg {
		return sendResultMsg{err: d.Send(context.Background(), ch, text), text: text}
	}
}

// setChannel switches the input line to another channel, updating its prompt
// and width (the prompt lengths differ), and reports the new mode to the
// driver so it survives a restart. That report is best-effort: the channel
// is only a UI hint, so a driver error (a closed session, a failed
// meta.json write) is absorbed here rather than surfaced as a scary
// transcript line — the in-memory switch above already succeeded.
func (a *App) setChannel(c Channel) {
	a.channel = c
	a.input.Prompt = promptFor(c)
	a.sizeInput()
	if a.driver != nil {
		_ = a.driver.SetChannel(c)
	}
}

// toggleChannel flips between the IC and OOC channels (ctrl+t).
func (a *App) toggleChannel() {
	if a.channel == ChannelIC {
		a.setChannel(ChannelOOC)
	} else {
		a.setChannel(ChannelIC)
	}
}

// argPrompt is the input prompt while the line collects a command
// argument (see beginArg).
const argPrompt = "expr ▸ "

// confirmPrompt is the input prompt while the line answers a pending
// channel check (see beginConfirm): the default answer is n, so the
// prompt reads "N/y" the way the question asks it.
const confirmPrompt = "N/y ▸ "

// regenPrompt is the input prompt while the line answers a golden-rule
// regeneration (see beginConfirm): the default answer is y, so the prompt
// reads "Y/n" the way the question asks it — enter regenerates, n keeps.
const regenPrompt = "Y/n ▸ "

// promptFor returns the input prompt for a channel.
func promptFor(c Channel) string {
	if c == ChannelOOC {
		return "ooc> "
	}
	return "> "
}

// historyUp walks local input history backwards (Up).
func (a *App) historyUp() {
	if len(a.history) == 0 {
		return
	}
	if a.histIdx > 0 {
		a.histIdx--
	}
	a.input.SetValue(a.history[a.histIdx])
	a.input.CursorEnd()
}

// historyDown walks local input history forwards (Down); past the newest
// entry it clears the line.
func (a *App) historyDown() {
	if a.histIdx >= len(a.history) {
		return
	}
	a.histIdx++
	if a.histIdx == len(a.history) {
		a.input.Reset()
		return
	}
	a.input.SetValue(a.history[a.histIdx])
	a.input.CursorEnd()
}

// clearTranscript drops the local transcript view (ctrl+l, the palette's
// Clear screen). The
// harness keeps its own record; this only clears what the TUI renders, and
// follow mode is re-armed.
func (a *App) clearTranscript() {
	a.entries = nil
	a.follow = true
	a.syncViewport()
}

// systemLine appends an informational line to the transcript.
func (a *App) systemLine(text string) {
	a.appendEvent(Event{
		Kind:     EventSystem,
		Channel:  a.channel,
		Markdown: text,
		At:       time.Now(),
	}, false)
}

// errorLine appends an error line to the transcript. It never returns a
// command: errors are rendered state, not side effects.
func (a *App) errorLine(err error) {
	a.appendEvent(Event{
		Kind:     EventError,
		Channel:  a.channel,
		Markdown: err.Error(),
		At:       time.Now(),
	}, false)
}

// appendEvent renders and stores one transcript event at the end, then
// refreshes the viewport (scrolling to the bottom only in follow mode).
// player marks the event as the harness's echo of the player's own message.
func (a *App) appendEvent(ev Event, player bool) {
	a.insertEvent(len(a.entries), ev, player)
}

// insertEvent files one rendered event at transcript position idx and
// refreshes the viewport. The dice theatre uses the position so a finished
// roll lands where it arrived even if other events came in during the
// animation; an out-of-range idx is clamped into the transcript.
func (a *App) insertEvent(idx int, ev Event, player bool) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(a.entries) {
		idx = len(a.entries)
	}
	a.entries = append(a.entries, entry{})
	copy(a.entries[idx+1:], a.entries[idx:])
	a.entries[idx] = entry{ev: ev, player: player, width: a.chatW}
	a.retag()
	a.syncViewport()
}

// retag recomputes run visibility across the transcript after an insertion
// (see tagVisibility) and re-renders only the entries whose rendering went
// stale: the new one, or one whose tags changed because something was
// spliced ahead of it. An append never changes anyone else's tags, so the
// cost stays one render per event.
func (a *App) retag() {
	starts := tagVisibility(a.entries)
	for i := range a.entries {
		e := &a.entries[i]
		if e.width == a.chatW && e.text != "" && e.tagged == starts[i] {
			continue
		}
		e.tagged = starts[i]
		if a.chatW >= 1 {
			e.text = a.renderEvent(e.ev, a.chatW, e.player, e.tagged)
		}
		e.width = a.chatW
	}
}

// ingest files one harness event. The status kinds never reach the
// transcript: EventThinking, EventTurnEnd, EventDecideStart and
// EventDecideStop drive the status-line throbber only (EventError clears
// it as well as being filed). Dice events go to
// the theatre instead of the transcript: the roll animates in place while
// the transcript keeps accepting events after it, and the banner is filed
// when the animation finishes (at once when animation is off) — a roll
// replayed from the log is history, so it files as its finished banner
// without being performed again. Everything else — including the harness's
// echo of the player's own message, recognised here — appends immediately,
// except a continuation batch, which extends the entry already open.
// It returns the command that drives the theatre or the throbber, if any.
func (a *App) ingest(ev Event) tea.Cmd {
	switch ev.Kind {
	case EventThinking:
		// The model wins over a pending decide bracket: deciding →
		// thinking on the call that follows it.
		return a.throb.setState(throbThinking)
	case EventDecideStart:
		// The decision model runs — from idle (the pre-echo channel
		// gate) or from any busy state (the golden-rule check): the
		// line reads "Deciding..." until its stop.
		return a.throb.setState(throbDeciding)
	case EventDecideStop:
		// The call returned: idle again. The stop always precedes the
		// events its outcome leads to — a confirm wait shows an empty
		// line (the prompt is the signal), a turn event re-establishes
		// idle or moves the line on.
		return a.throb.setState(throbIdle)
	case EventTurnEnd:
		return a.throb.setState(throbIdle)
	case EventConfirm, EventConfirmRegen:
		// The question still appends below; entering confirm mode only
		// switches the input line over to the pending answer (the two
		// kinds differ in prompt and answer keys — see beginConfirm).
		a.beginConfirm(ev)
	}
	var cmd tea.Cmd
	if ev.Kind == EventError {
		cmd = a.throb.setState(throbIdle) // a failed turn leaves no throbber behind
	}
	if ev.Kind == EventDice && ev.Dice != nil {
		if ev.Replay {
			a.appendEvent(ev, false) // finished banner: no theatre, no re-performance
			return cmd
		}
		return tea.Batch(cmd, a.startRoll(ev))
	}
	// Authorship: an explicit Player flag (echo or replayed line) wins;
	// otherwise fall back to matching recently sent text. A continuation
	// belongs to the entry it extends, a replayed line is history and a
	// golden-rule replacement is the GM's rewrite — none may claim an echo.
	matched := ev.Kind == EventNarrative && !ev.Continuation && !ev.Replay && !ev.Replace &&
		a.consumeEcho(ev.Markdown)
	switch {
	case ev.Kind == EventNarrative && ev.Replace:
		// The rewritten reply swaps in for the message it replaces.
		a.replaceEntry(ev)
	case !ev.Continuation || !a.extendEntry(ev):
		a.appendEvent(ev, ev.Player || matched)
	}
	switch {
	case ev.Kind == EventNarrative && a.throb.state == throbThinking:
		// The model's first words after EventThinking: it is writing now.
		cmd = tea.Batch(cmd, a.throb.setState(throbWriting))
	case ev.Kind == EventToolAnnotation && a.throb.state != throbIdle:
		// The round is dispatched and the model re-invoked; a local
		// command's annotation (no turn running) must not start one.
		cmd = tea.Batch(cmd, a.throb.setState(throbThinking))
	}
	return cmd
}

// extendEntry appends a continuation batch to the last transcript entry
// when that entry can take it — a narrative entry on the same channel and
// the same side (player line or GM narration) — and re-renders just that
// entry. Tags are untouched: runs group by channel and speaker, so an
// extended entry never re-tags and the rest of the transcript is not
// re-rendered. It reports false when the batch must start a fresh entry
// instead (nothing open, or the last entry is another kind/channel/side).
func (a *App) extendEntry(ev Event) bool {
	if len(a.entries) == 0 {
		return false
	}
	last := &a.entries[len(a.entries)-1]
	if last.ev.Kind != EventNarrative || last.ev.Channel != ev.Channel || last.player != ev.Player {
		return false
	}
	last.ev.Markdown += ev.Markdown
	last.text = a.renderEvent(last.ev, a.chatW, last.player, last.tagged)
	last.width = a.chatW
	a.syncViewport()
	return true
}

// replaceEntry swaps a golden-rule rewrite into the transcript: every
// narrative entry carrying the replaced MsgSeq is removed — the
// replacement files at the first one's position, so the rewritten message
// keeps its place in the play — the run tags are recomputed and the
// viewport refreshes (follow mode still keeps the bottom in view). A
// ReplaceSeq nothing on screen carries (0, or an already-swapped message)
// files the replacement at the end instead.
func (a *App) replaceEntry(ev Event) {
	idx := -1   // transcript slot the replacement files at
	before := 0 // removed entries ahead of the theatre's filing spot
	kept := make([]entry, 0, len(a.entries)+1)
	for i, e := range a.entries {
		if ev.ReplaceSeq != 0 && e.ev.Kind == EventNarrative && e.ev.MsgSeq == ev.ReplaceSeq {
			if idx < 0 {
				idx = len(kept) // the first removed entry's slot
			}
			if i < a.diceIdx {
				before++
			}
			continue
		}
		kept = append(kept, e)
	}
	if idx < 0 {
		idx = len(kept)
	}
	// The theatre's filing position shifts with the entries around it:
	// removals before it pull it forward, the insertion pushes it past idx.
	a.diceIdx -= before
	if a.diceIdx >= idx {
		a.diceIdx++
	}
	kept = append(kept, entry{})
	copy(kept[idx+1:], kept[idx:])
	kept[idx] = entry{ev: ev, player: false, width: a.chatW}
	a.entries = kept
	a.retag()
	a.syncViewport()
}

// startRoll puts ev on stage. One roll animates at a time: a roll arriving
// mid-animation is filed into the transcript at once so the new one can take
// the stage. When there is nothing to animate (animation off) the result is
// filed immediately.
func (a *App) startRoll(ev Event) tea.Cmd {
	if a.dice.Active() {
		a.flushDice()
	}
	a.diceEv = ev
	a.diceIdx = len(a.entries)
	cmd := a.dice.Start(*ev.Dice)
	if cmd == nil {
		a.flushDice()
		return nil
	}
	a.syncViewport() // put the theatre row on screen straight away
	return cmd
}

// flushDice files the staged roll into the transcript at the position where
// it arrived and clears the theatre row.
func (a *App) flushDice() {
	if !a.dice.Active() {
		return
	}
	ev, idx := a.diceEv, a.diceIdx
	a.dice.Reset()
	a.insertEvent(idx, ev, false)
}

// refreshMetadata re-reads the panel snapshot from the driver into the app's
// cache. It runs on every processed event, on every panel toggle and on the
// slow metadata tick — never during View, which only reads the cache.
func (a *App) refreshMetadata() {
	if a.driver == nil {
		return
	}
	a.meta = a.driver.Metadata()
}

// metadataTick returns the command that fires the slow refresh; Update
// re-arms it so the panel keeps up with changes no event reports (autosave,
// clocks).
func (a *App) metadataTick() tea.Cmd {
	return tea.Tick(metadataTick, func(time.Time) tea.Msg { return metadataTickMsg{} })
}

// recordEcho remembers sent text so the harness's echo of the player's own
// message can be recognised when it arrives back through Events. A bounded
// tail is kept: at most maxEchoes outstanding messages, so an echo the
// harness never sends cannot accumulate.
func (a *App) recordEcho(text string) {
	a.echoes = append(a.echoes, strings.TrimSpace(text))
	if len(a.echoes) > maxEchoes {
		a.echoes = a.echoes[len(a.echoes)-maxEchoes:]
	}
}

// consumeEcho reports whether md is the echo of a recently sent message and
// withdraws it, so each echo is recognised at most once. Matching is exact
// on trimmed text: an echo the harness re-formats simply renders as ordinary
// narrative.
func (a *App) consumeEcho(md string) bool {
	text := strings.TrimSpace(md)
	for i, sent := range a.echoes {
		if sent == text {
			a.echoes = append(a.echoes[:i:i], a.echoes[i+1:]...)
			return true
		}
	}
	return false
}

// dropEcho withdraws a recorded echo after its send failed, so the text can
// never be mistaken for an echo that will not come.
func (a *App) dropEcho(text string) {
	text = strings.TrimSpace(text)
	for i := len(a.echoes) - 1; i >= 0; i-- {
		if a.echoes[i] == text {
			a.echoes = append(a.echoes[:i:i], a.echoes[i+1:]...)
			return
		}
	}
}

// rerender re-renders every entry at the current chat width — used when the
// window size or the theme changes.
func (a *App) rerender() {
	starts := tagVisibility(a.entries)
	for i := range a.entries {
		e := &a.entries[i]
		e.tagged = starts[i]
		e.text = a.renderEvent(e.ev, a.chatW, e.player, e.tagged)
		e.width = a.chatW
	}
	a.syncViewport()
}

// syncViewport pushes the transcript into the viewport, keeping the bottom
// in view only while follow mode is on (the player scrolling up suspends it).
// While a roll is on stage, its theatre row is spliced in at the position
// the roll arrived, so newer events render after it and the banner takes
// that exact spot when it is flushed.
func (a *App) syncViewport() {
	dice := a.dice.View()
	spliced := false
	parts := make([]string, 0, len(a.entries)+1)
	for i, e := range a.entries {
		if dice != "" && i == a.diceIdx {
			parts = append(parts, dice)
			spliced = true
		}
		// Glamour renders every block with a leading and trailing newline;
		// the "\n\n" join supplies the separation, so trim BOTH edges of
		// each part or every entry pair shows two blank lines (and a
		// whitespace-only batch, e.g. a flushed paragraph break, adds a
		// third). Skip parts that end up empty.
		if t := strings.Trim(e.text, "\n"); strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
	}
	if dice != "" && !spliced {
		parts = append(parts, dice)
	}
	a.viewport.SetContent(strings.Join(parts, "\n\n"))
	if a.follow {
		a.viewport.GotoBottom()
	}
}

// relayout recomputes component sizes for the current window and re-renders
// the transcript at the new widths.
func (a *App) relayout() {
	if a.width < 1 || a.height < 1 {
		return
	}
	bodyH := a.height - 4 // header, status line, input line, footer
	if bodyH < 1 {
		bodyH = 1
	}
	chatW := a.width
	if a.panelOpen {
		panelW := a.width * 3 / 10 // reserve ~30% for the metadata panel
		chatW = a.width - panelW
		if chatW < 1 {
			chatW = 1
		}
	}
	a.chatW = chatW
	a.viewport.SetWidth(chatW)
	a.viewport.SetHeight(bodyH)
	a.dice.SetWidth(chatW)
	a.sizeInput()
	a.rerender()
}

// sizeInput fits the text input to the window: the component's width covers
// the value area only, so the current prompt (channel, argument or
// confirm) and the cursor cell are subtracted.
func (a *App) sizeInput() {
	w := a.width - lipgloss.Width(a.input.Prompt) - 1
	if w < 1 {
		w = 1
	}
	a.input.SetWidth(w)
}

// applyTheme rebuilds the theme for the known terminal background and
// re-renders the transcript in the new colours.
func (a *App) applyTheme() {
	th, err := Get(a.themeName)
	if err != nil {
		// Unreachable: the name is validated wherever it is set. Report
		// rather than silently keeping a stale theme.
		a.errorLine(fmt.Errorf("reload theme: %w", err))
		return
	}
	if a.bgKnown {
		th = th.Adapt(a.bgDark)
	}
	a.theme = th
	a.applyInputStyles()
	a.pal.applyStyles(th)
	a.throb.setTheme(th)
	a.dice.SetTheme(th)
	a.rerender()
}

// applyThemeName switches to a named theme and reports it on the
// transcript. An unknown name surfaces as an error line — defensive only:
// the palette offers the built-in names exclusively.
func (a *App) applyThemeName(name string) {
	th, err := Get(name)
	if err != nil {
		a.errorLine(err)
		return
	}
	a.themeName = th.Name
	a.applyTheme()
	a.systemLine("theme → " + th.Name)
}

// applyInputStyles pushes the theme's input styles into the text input.
func (a *App) applyInputStyles() {
	st := a.input.Styles()
	st.Focused.Prompt = a.theme.Styles.InputPrompt
	st.Focused.Text = a.theme.Styles.InputText
	st.Blurred.Prompt = a.theme.Styles.Muted
	st.Blurred.Text = a.theme.Styles.InputText
	st.Cursor.Color = a.theme.Palette[SlotAccent]
	a.input.SetStyles(st)
}

// runKey identifies who is speaking on the transcript: the channel and
// whether the entry is the harness's echo of the player's own message.
// Two entries share a key exactly while they belong to the same run.
type runKey struct {
	ch     Channel
	player bool
}

// isSpeakerTurn reports whether an event kind speaks in its own voice:
// narrative, system and error lines carry words from the player or the GM,
// while tool-annotation and dice strips are metadata inside the current
// speaker's turn — they neither show tags nor break a run. The pending
// confirmation questions are the same kind of metadata: one untagged line.
func isSpeakerTurn(k EventKind) bool {
	switch k {
	case EventToolAnnotation, EventDice, EventConfirm, EventConfirmRegen:
		return false
	default:
		return true
	}
}

// tagVisibility reports, for every transcript entry, whether it must show
// its speaker tags ("[ooc]", "You", "GM"). A tag appears only where a run
// begins: the first entry, and any entry whose channel or speaker differs
// from the last speaking entry before it. Callers render tags on whenever
// the width is unknown (renderEvent forces them at w < 1), so no sender or
// channel information can ever be lost.
func tagVisibility(entries []entry) []bool {
	starts := make([]bool, len(entries))
	var prev runKey
	speaking := false // a speaking entry has established the current run
	for i, e := range entries {
		if i == 0 {
			starts[0] = true // the transcript's first entry always shows its tags
		}
		if !isSpeakerTurn(e.ev.Kind) {
			continue // metadata: keeps the run around it intact
		}
		k := runKey{ch: e.ev.Channel, player: e.player}
		if !speaking || k != prev {
			starts[i] = true
			prev, speaking = k, true
		}
	}
	return starts
}

// renderEvent renders one transcript event for a chat width of w columns
// (w < 1 means "not laid out yet": markdown falls back to a default wrap,
// single-line renderings are not clipped, and tags are always shown).
// player marks the event as the harness's echo of the player's own message,
// which is tagged "You" through the speaker style; GM narration is tagged
// "GM" the same way; out-of-character lines keep their "[ooc]" tag either
// way, and system/error lines never carry a speaker tag. tagged reports
// whether this entry starts a run (see tagVisibility): only that entry
// carries the tag line, so a streamed run of paragraphs is annotated once,
// not per paragraph.
func (a *App) renderEvent(ev Event, w int, player bool, tagged bool) string {
	wrap := w
	if wrap < 1 {
		wrap = 80
		tagged = true // no layout yet: prefer showing the tag to losing it
	}
	var body string
	switch ev.Kind {
	case EventNarrative:
		body = a.renderMarkdown(ev.Markdown, wrap)
		if player {
			// Player-submitted text reads visibly dimmer than GM
			// narration, so authorship is obvious at a glance.
			body = faintBlock(body)
		}
	case EventToolAnnotation:
		body = a.annotationStrip(ev.Annotation, w)
	case EventDice:
		body = a.diceBanner(ev.Dice, w)
	case EventError:
		body = a.theme.Styles.Critical.Render("✗ " + a.wrapText(ev.Markdown, w))
	case EventConfirm, EventConfirmRegen:
		// The pending confirmation question: one styled line, clipped
		// to the chat width (it never wraps into a paragraph).
		body = fit(a.theme.Styles.Accent, ev.Markdown, w)
	default:
		// EventSystem, and any future kind we do not know yet: a muted line.
		body = a.theme.Styles.Muted.Render(a.wrapText(ev.Markdown, w))
	}
	var tags []string
	if ev.Channel == ChannelOOC {
		tags = append(tags, "[ooc]")
	}
	switch {
	case player:
		tags = append(tags, "You")
	case ev.Kind == EventNarrative:
		// GM narration opens its run with a speaker tag of its own, so
		// both sides of the table are named; system and error lines
		// carry no speaker.
		tags = append(tags, "GM")
	}
	if tagged && len(tags) > 0 {
		// Trim the body's edge newlines: glamour renders blocks with a
		// leading newline, which would otherwise sit between the tag line
		// and its text (the player's dimmed block is trimmed already).
		body = a.theme.Styles.Speaker.Render(strings.Join(tags, " ")) +
			"\n" + strings.Trim(body, "\n")
	}
	return body
}

// wrapText wraps plain text to w columns; widths below 1 mean "not laid out
// yet" and are returned unchanged.
func (a *App) wrapText(s string, w int) string {
	if w < 1 {
		return s
	}
	return lipgloss.Wrap(s, w, "")
}

// renderMarkdown renders Markdown through glamour using the style matching
// the theme's background, wrapped to width. On any renderer failure the raw
// text is shown in the Narrative style instead — the words are never lost.
func (a *App) renderMarkdown(md string, width int) string {
	r, err := a.ensureMD(width)
	if err != nil {
		return a.theme.Styles.Narrative.Render(md)
	}
	out, err := r.Render(md)
	if err != nil {
		return a.theme.Styles.Narrative.Render(md)
	}
	return out
}

// faintReassert re-asserts the SGR faint attribute after every escape reset:
// glamour and lipgloss reset the whole SGR state ("\x1b[m" / "\x1b[0m") when
// they change style, which would otherwise switch faint back off mid-block
// and let the rest of the text glow at full strength.
var faintReassert = strings.NewReplacer(
	"\x1b[0m", "\x1b[0m\x1b[2m",
	"\x1b[m", "\x1b[m\x1b[2m",
)

// faintBlock dims one rendered narrative block (player-submitted text) by
// re-asserting faint after every reset and closing with a clean reset, so
// the next transcript entry is unaffected. Edges are trimmed to the same
// normalisation syncViewport applies to every part — otherwise the wrapper's
// trailing reset would hide the newlines from the trim and reintroduce the
// double-blank-line bug.
func faintBlock(s string) string {
	s = strings.Trim(s, "\n")
	if s == "" {
		return s
	}
	return "\x1b[2m" + faintReassert.Replace(s) + "\x1b[0m"
}

// ensureMD returns the cached glamour renderer for the current width and
// theme background, rebuilding it when either changes.
func (a *App) ensureMD(width int) (*glamour.TermRenderer, error) {
	style := "light"
	if a.theme.Dark {
		style = "dark"
	}
	if a.md != nil && a.mdWidth == width && a.mdDark == a.theme.Dark {
		return a.md, nil
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, fmt.Errorf("markdown renderer (%s): %w", style, err)
	}
	a.md, a.mdWidth, a.mdDark = r, width, a.theme.Dark
	return r, nil
}

// annotationStrip renders a tool call as one compact line: ⚙ name → summary
// (✗ when the call failed), clipped to w columns.
func (a *App) annotationStrip(an *Annotation, w int) string {
	if an == nil {
		return a.theme.Styles.Muted.Render("⚙ tool call (no details)")
	}
	glyph := "⚙"
	style := a.theme.Styles.ToolAnnotation
	if !an.OK {
		glyph = "✗"
		style = a.theme.Styles.Critical
	}
	summary := strings.ReplaceAll(an.Summary, "\n", " ")
	line := glyph + " " + an.ToolName + " → " + summary
	return fit(style, line, w)
}

// diceBanner renders a finished roll as a bold centred banner, coloured by
// outcome when one is present. The numbers are only ever displayed, never
// recomputed.
func (a *App) diceBanner(d *DiceEvent, w int) string {
	if d == nil {
		return a.theme.Styles.Muted.Render("roll (no details)")
	}
	body := rollSummary(*d)

	style := a.theme.Styles.DiceBanner
	if outcome := strings.TrimSpace(d.Outcome); outcome != "" {
		body += " · " + strings.ToUpper(outcome)
		switch strings.ToLower(outcome) {
		case "critical":
			style = a.theme.Styles.Critical
		case "success":
			style = a.theme.Styles.Success
		case "failure":
			style = a.theme.Styles.Failure
		case "fumble":
			style = a.theme.Styles.Fumble
		}
	}
	rendered := style.Render("── " + body + " ──")
	if w < 1 {
		return rendered
	}
	if lipgloss.Width(rendered) > w {
		rendered = fit(style, "── "+body+" ──", w)
	}
	return lipgloss.PlaceHorizontal(w, lipgloss.Center, rendered)
}

// renderHeader builds the status header: session, segmented IC/OOC switch,
// model, and static save/decision placeholders (filled in by later waves).
// Progress lives on its own line above the input (renderStatus), never
// here — the header is stable while a turn runs.
func (a *App) renderHeader() string {
	var parts []string
	if a.opts.SessionName != "" {
		parts = append(parts, a.theme.Styles.Header.Render(a.opts.SessionName))
	}
	parts = append(parts, a.channelSwitch())
	if a.opts.ModelName != "" {
		parts = append(parts, a.theme.Styles.Muted.Render(a.opts.ModelName))
	}
	parts = append(parts,
		a.theme.Styles.Muted.Render("save –"),
		a.theme.Styles.Muted.Render("decision –"),
	)
	sep := a.theme.Styles.Muted.Render("·")
	return fit(a.theme.Styles.Header, strings.Join(parts, " "+sep+" "), a.width)
}

// renderStatus builds the status line above the input: the throbber's
// spinner frame and label while a model or decision call runs, an empty
// row when idle. The row is always rendered (one line, left-aligned,
// clipped to the window) so the layout never jumps between turns, and it
// sits outside the View switch, so every view mode shows it.
func (a *App) renderStatus() string {
	return a.throb.status(a.width)
}

// channelSwitch renders the segmented IC/OOC indicator with the active
// channel bracketed and highlighted.
func (a *App) channelSwitch() string {
	ic, ooc := a.theme.Styles.Muted, a.theme.Styles.Muted
	if a.channel == ChannelIC {
		ic = a.theme.Styles.Highlight
		return ic.Render("[IC]") + " " + ooc.Render("OOC")
	}
	ooc = a.theme.Styles.Highlight
	return ic.Render("IC") + " " + ooc.Render("[OOC]")
}

// renderFooter builds the terse keybinding hints on the left and the
// context readout on the right, muted and flush to the terminal edge. The
// full key list lives in the help overlay (and the sheet swaps in its own
// hint while up), so the footer stays one short line that always fits: the
// readout wins the right edge and the hints share whatever is left.
func (a *App) renderFooter() string {
	hints := "enter send · ctrl+c quit · ctrl+p commands"
	if a.sheetOpen {
		hints = "esc close · pgup/pgdn scroll"
	}
	usage := contextUsageText(a.meta.ContextUsage)
	if usage == "" || a.width < 1 {
		return fit(a.theme.Styles.Footer, hints, a.width)
	}
	right := a.theme.Styles.Muted.Render(usage)
	leftW := a.width - lipgloss.Width(usage)
	if leftW < 1 {
		return fit(a.theme.Styles.Muted, usage, a.width) // window narrower than the readout
	}
	left := fit(a.theme.Styles.Footer, hints, leftW)
	// fit clips to leftW, so the padding is never negative (max keeps a
	// pathological clip from panicking the view).
	return left + strings.Repeat(" ", max(0, leftW-lipgloss.Width(left))) + right
}

// contextUsageText formats the footer's context readout: tokens as "999"
// below a thousand and "42.3K" at or above, followed by " (18%)" — the
// share of the window in use — when the window is known, and clamped to
// 100% when the last call already reports more tokens than the window.
// No call yet (Tokens 0) reads as "" so the footer shows hints alone.
func contextUsageText(c ContextUsage) string {
	if c.Tokens <= 0 {
		return ""
	}
	tokens := strconv.Itoa(c.Tokens)
	if c.Tokens >= 1000 {
		tokens = fmt.Sprintf("%.1fK", float64(c.Tokens)/1000)
	}
	if c.Window <= 0 {
		return tokens
	}
	pct := int(math.Round(100 * float64(c.Tokens) / float64(c.Window)))
	if pct > 100 {
		pct = 100
	}
	return tokens + " (" + strconv.Itoa(pct) + "%)"
}

// renderPanel builds the right-hand metadata panel from the cached snapshot;
// the cache is refreshed on events, toggles and the slow tick (see
// refreshMetadata), so rendering stays a pure read.
func (a *App) renderPanel() string {
	return renderMetadata(a.theme, a.meta, a.width, a.height)
}

// fit clips an already-rendered line to w display columns, preserving its
// ANSI styling. Widths below 1 mean "no layout yet" and are left alone.
func fit(style lipgloss.Style, s string, w int) string {
	if w < 1 || lipgloss.Width(s) <= w {
		return style.Render(s)
	}
	return style.MaxWidth(w).Render(s)
}
