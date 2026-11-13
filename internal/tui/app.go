package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
}

// entry is one transcript item together with its cached rendering. width is
// the chat width the text was rendered for; a mismatch triggers a re-render.
// player marks a harness echo of the player's own message, rendered with the
// speaker tag instead of the GM's.
type entry struct {
	ev     Event
	player bool
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

	panelOpen bool
	helpOpen  bool
	meta      Metadata // cached Driver.Metadata snapshot (see refreshMetadata)
	echoes    []string // recently sent texts awaiting their harness echo

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
		channel:   ChannelIC,
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

	th, err := Get(a.themeName)
	if err != nil {
		a.errorLine(fmt.Errorf("theme: %w", err))
		th, _ = Get(defaultTheme) // defaultTheme is a built-in; safe fallback
		a.themeName = th.Name
	}
	a.theme = th
	a.applyInputStyles()
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
		return a, nil

	case sendResultMsg:
		if msg.err != nil {
			a.dropEcho(msg.text) // no echo will arrive for a failed send
			a.errorLine(fmt.Errorf("send failed: %w", msg.err))
		}
		return a, nil

	case metadataTickMsg:
		a.refreshMetadata()
		return a, a.metadataTick()

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

// View renders the whole screen: header, transcript (plus the metadata panel
// when open, or the help overlay when it is up), input line and footer, in
// the active theme.
func (a *App) View() tea.View {
	var body string
	bodyH := a.height - 3 // header, input line, footer
	if bodyH < 1 {
		bodyH = 1
	}
	switch {
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

// handleKey dispatches a key press: the help overlay captures everything
// first, then global shortcuts, then history and scrolling, with everything
// else handed to the input line.
func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.helpOpen {
		return a.handleHelpKey(msg)
	}
	switch k := msg.String(); k {
	case "ctrl+c":
		return a, tea.Quit
	case "ctrl+t":
		a.toggleChannel()
		return a, nil
	case "ctrl+m":
		a.panelOpen = !a.panelOpen
		a.refreshMetadata()
		a.relayout()
		return a, nil
	case "ctrl+e":
		return a, a.openEditor()
	case "ctrl+l":
		a.clearTranscript()
		return a, nil
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

// handleHelpKey runs while the help overlay is up. esc and enter dismiss it
// (an enter on a freshly typed "/help" clears that line too, so the dismiss
// keystroke never lingers as sendable text), ctrl+c still quits, and every
// other key reaches only the input line — shortcuts stay captured, but the
// player can still type their next message or "/help" while reading.
func (a *App) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit
	case "esc":
		a.helpOpen = false
		return a, nil
	case "enter":
		if strings.EqualFold(strings.TrimSpace(a.input.Value()), "/help") {
			a.input.Reset()
		}
		a.helpOpen = false
		return a, nil
	}
	ti, cmd := a.input.Update(msg)
	a.input = ti
	return a, cmd
}

// submit validates the input line: empty input is ignored, slash commands are
// parsed and dispatched, and everything else is sent to the harness on the
// current channel. The input is recorded in local history either way.
func (a *App) submit() tea.Cmd {
	text := a.input.Value()
	if text == "" {
		return nil
	}
	a.input.Reset()
	a.history = append(a.history, text)
	a.histIdx = len(a.history)

	cmd, err := ParseInput(text)
	if err != nil {
		a.errorLine(err)
		return nil
	}
	switch cmd.Kind {
	case CmdIC:
		a.setChannel(ChannelIC)
		return nil
	case CmdOOC:
		a.setChannel(ChannelOOC)
		return nil
	case CmdTheme:
		th, err := Get(cmd.Args[0])
		if err != nil {
			a.errorLine(err)
			return nil
		}
		a.themeName = th.Name
		a.applyTheme()
		a.systemLine("theme → " + th.Name)
		return nil
	case CmdEditor:
		return a.openEditor()
	case CmdClear:
		a.clearTranscript()
		return nil
	case CmdHelp:
		// The overlay lists keys and commands; while it is up the enter key
		// closes it (handleHelpKey), so the toggle mostly serves the open.
		a.helpOpen = !a.helpOpen
		return nil
	case CmdQuit:
		return tea.Quit
	default:
		// CmdMessage and the harness commands (/roll, /save, /recap, /meta)
		// go to the driver verbatim; the harness interprets the latter when
		// they are wired to engine moves. The text is remembered so the
		// harness's echo of the player's own message can be recognised when
		// it comes back through Events.
		a.recordEcho(text)
		return a.send(text)
	}
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
// and width (the prompt lengths differ).
func (a *App) setChannel(c Channel) {
	a.channel = c
	a.input.Prompt = promptFor(c)
	a.sizeInput()
}

// toggleChannel flips between the IC and OOC channels (ctrl+t).
func (a *App) toggleChannel() {
	if a.channel == ChannelIC {
		a.setChannel(ChannelOOC)
	} else {
		a.setChannel(ChannelIC)
	}
}

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

// clearTranscript drops the local transcript view (ctrl+l, /clear). The
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
	e := entry{ev: ev, player: player, width: a.chatW}
	if a.chatW >= 1 {
		e.text = a.renderEvent(ev, a.chatW, player)
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(a.entries) {
		idx = len(a.entries)
	}
	a.entries = append(a.entries, entry{})
	copy(a.entries[idx+1:], a.entries[idx:])
	a.entries[idx] = e
	a.syncViewport()
}

// ingest files one harness event. Dice events go to the theatre instead of
// the transcript: the roll animates in place while the transcript keeps
// accepting events after it, and the banner is filed when the animation
// finishes (at once when animation is off). Everything else — including the
// harness's echo of the player's own message, recognised here — appends
// immediately. It returns the command that drives the theatre, if any.
func (a *App) ingest(ev Event) tea.Cmd {
	if ev.Kind == EventDice && ev.Dice != nil {
		return a.startRoll(ev)
	}
	player := ev.Kind == EventNarrative && a.consumeEcho(ev.Markdown)
	a.appendEvent(ev, player)
	return nil
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
	for i := range a.entries {
		a.entries[i].text = a.renderEvent(a.entries[i].ev, a.chatW, a.entries[i].player)
		a.entries[i].width = a.chatW
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
		parts = append(parts, strings.TrimRight(e.text, "\n"))
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
	bodyH := a.height - 3 // header, input line, footer
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
// the value area only, so the prompt and the cursor cell are subtracted.
func (a *App) sizeInput() {
	w := a.width - lipgloss.Width(promptFor(a.channel)) - 1
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
	a.dice.SetTheme(th)
	a.rerender()
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

// renderEvent renders one transcript event for a chat width of w columns
// (w < 1 means "not laid out yet": markdown falls back to a default wrap and
// single-line renderings are not clipped). player marks the event as the
// harness's echo of the player's own message, which is tagged "You" through
// the speaker style; out-of-character lines keep their "[ooc]" tag either
// way.
func (a *App) renderEvent(ev Event, w int, player bool) string {
	wrap := w
	if wrap < 1 {
		wrap = 80
	}
	var body string
	switch ev.Kind {
	case EventNarrative:
		body = a.renderMarkdown(ev.Markdown, wrap)
	case EventToolAnnotation:
		body = a.annotationStrip(ev.Annotation, w)
	case EventDice:
		body = a.diceBanner(ev.Dice, w)
	case EventError:
		body = a.theme.Styles.Critical.Render("✗ " + a.wrapText(ev.Markdown, w))
	default:
		// EventSystem, and any future kind we do not know yet: a muted line.
		body = a.theme.Styles.Muted.Render(a.wrapText(ev.Markdown, w))
	}
	var tags []string
	if ev.Channel == ChannelOOC {
		tags = append(tags, "[ooc]")
	}
	if player {
		tags = append(tags, "You")
	}
	if len(tags) > 0 {
		body = a.theme.Styles.Speaker.Render(strings.Join(tags, " ")) + "\n" + body
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

// renderHeader builds the status line: session, segmented IC/OOC switch,
// model, and static save/decision placeholders (filled in by later waves).
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

// renderFooter builds the terse keybinding hints.
func (a *App) renderFooter() string {
	hints := "enter send · ctrl+t ic/ooc · ctrl+m meta · ctrl+e editor · " +
		"ctrl+l clear · pgup/pgdn scroll · q/ctrl+c quit · /help"
	return fit(a.theme.Styles.Footer, hints, a.width)
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
