package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ansiRe matches SGR escape sequences so assertions can compare rendered
// output as plain text.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plain strips ANSI styling from rendered output for text assertions.
func plain(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// fakeDriver records Sends, Commands and channel switches; it stands in
// for the harness so no real terminal, network or LLM is involved.
type fakeDriver struct {
	events     chan Event
	sendErr    error
	sends      []sentRecord
	commands   []commandRecord // Driver.Command calls, in order
	commandErr error           // returned by Command
	channels   []Channel       // SetChannel calls, in order
	md         Metadata        // handed out by Metadata()
}

type sentRecord struct {
	ch   Channel
	text string
}

type commandRecord struct {
	name string
	args []string
}

var _ Driver = (*fakeDriver)(nil)

func newFakeDriver() *fakeDriver {
	return &fakeDriver{events: make(chan Event, 8)}
}

func (d *fakeDriver) Send(ctx context.Context, ch Channel, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.sends = append(d.sends, sentRecord{ch: ch, text: text})
	return d.sendErr
}

func (d *fakeDriver) Command(name string, args ...string) error {
	d.commands = append(d.commands, commandRecord{name: name, args: args})
	return d.commandErr
}

func (d *fakeDriver) SetChannel(ch Channel) error {
	d.channels = append(d.channels, ch)
	return nil
}

func (d *fakeDriver) Events() <-chan Event { return d.events }

func (d *fakeDriver) Metadata() Metadata { return d.md }

func (d *fakeDriver) Close() error {
	close(d.events)
	return nil
}

// newTestApp builds an app around a fake driver and a fixed window size.
func newTestApp(t *testing.T, opts Options, width, height int) (*App, *fakeDriver) {
	t.Helper()
	if opts.SessionName == "" {
		opts.SessionName = "Test Session"
	}
	if opts.ModelName == "" {
		opts.ModelName = "test-model"
	}
	d := newFakeDriver()
	a := New(d, opts)
	_, _ = a.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return a, d
}

// key builds a key press for a printable rune.
func key(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// ctrl builds a ctrl+letter key press.
func ctrl(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

// press feeds a key to the app and returns the resulting command.
func press(t *testing.T, a *App, k tea.KeyPressMsg) tea.Cmd {
	t.Helper()
	_, cmd := a.Update(k)
	return cmd
}

// view renders the current screen as plain text.
func view(a *App) string {
	return a.View().Content
}

func TestWindowSizeLayout(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 24)
	v := a.View()
	if !v.AltScreen {
		t.Error("chat shell should use the alternate screen")
	}
	if v.WindowTitle == "" {
		t.Error("window title should be set")
	}
	lines := strings.Split(v.Content, "\n")
	if len(lines) != 24 {
		t.Errorf("view has %d lines, want 24 (header/body/input/footer fill the window)", len(lines))
	}
	for _, want := range []string{"Test Session", "test-model", "[IC]", "save –", "decision –", "> "} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("header/input should contain %q", want)
		}
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 80 {
			t.Errorf("line wider than the window (%d cols): %q", lipgloss.Width(line), line)
		}
	}
}

func TestNarrativeEventRenders(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 20)
	_, _ = a.Update(eventMsg{ev: Event{
		Kind:     EventNarrative,
		Channel:  ChannelIC,
		Markdown: "# The Crypt Door\n\nYou notice a faint **draft** from beneath the flagstones.",
		At:       time.Now(),
	}})
	got := plain(view(a))
	for _, want := range []string{"The Crypt Door", "draft"} {
		if !strings.Contains(got, want) {
			t.Errorf("narrative should render %q, view:\n%s", want, got)
		}
	}
}

func TestToolAnnotationRenders(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 20)
	_, _ = a.Update(eventMsg{ev: Event{
		Kind:       EventToolAnnotation,
		Channel:    ChannelIC,
		Annotation: &Annotation{ToolName: "world:set-flag", Summary: "torch-lit set", OK: true},
	}})
	_, _ = a.Update(eventMsg{ev: Event{
		Kind:       EventToolAnnotation,
		Channel:    ChannelIC,
		Annotation: &Annotation{ToolName: "resolve:attack", Summary: "target out of range", OK: false},
	}})
	got := view(a)
	for _, want := range []string{"world:set-flag", "torch-lit set", "→", "resolve:attack", "✗"} {
		if !strings.Contains(got, want) {
			t.Errorf("annotation strip should contain %q, view:\n%s", want, got)
		}
	}
}

func TestDiceBannerRenders(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 20)
	_, _ = a.Update(eventMsg{ev: Event{
		Kind:    EventDice,
		Channel: ChannelIC,
		Dice: &DiceEvent{
			Expr: "3d6+2", Dice: []int{4, 6, 2}, Modifier: 2, Total: 14, Outcome: "success",
		},
	}})
	got := view(a)
	for _, want := range []string{"3d6+2", "[4 6 2]", "+2", "= 14", "SUCCESS"} {
		if !strings.Contains(got, want) {
			t.Errorf("dice banner should contain %q, view:\n%s", want, got)
		}
	}
	// The banner is centred inside the chat width.
	if a.chatW < 1 {
		t.Fatal("chat width not laid out")
	}
}

func TestCtrlTTogglesChannel(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 60, 20)
	if a.channel != ChannelIC || a.input.Prompt != "> " {
		t.Fatalf("start: channel=%v prompt=%q", a.channel, a.input.Prompt)
	}
	press(t, a, ctrl('t'))
	if a.channel != ChannelOOC {
		t.Errorf("after ctrl+t channel = %v, want ChannelOOC", a.channel)
	}
	if a.input.Prompt != "ooc> " {
		t.Errorf("OOC prompt = %q, want %q", a.input.Prompt, "ooc> ")
	}
	if got := view(a); !strings.Contains(got, "[OOC]") {
		t.Errorf("header should show the active OOC channel, view:\n%s", got)
	}
	press(t, a, ctrl('t'))
	if a.channel != ChannelIC || a.input.Prompt != "> " {
		t.Errorf("second ctrl+t: channel=%v prompt=%q", a.channel, a.input.Prompt)
	}
}

func TestCtrlMTogglesMetadataPanel(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	if a.panelOpen {
		t.Fatal("panel should start closed")
	}
	press(t, a, ctrl('m'))
	if !a.panelOpen {
		t.Fatal("ctrl+m should open the panel")
	}
	got := view(a)
	if !strings.Contains(got, "METADATA") {
		t.Errorf("open panel should render its placeholder, view:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 20 {
		t.Errorf("panel open: view has %d lines, want 20", len(lines))
	}
	// The panel reserves roughly 30% of the width: the chat shrinks with it.
	if a.chatW >= a.width {
		t.Errorf("chat width %d should shrink below window width %d when the panel is open", a.chatW, a.width)
	}
	press(t, a, ctrl('m'))
	if a.panelOpen {
		t.Error("second ctrl+m should close the panel")
	}
	if strings.Contains(view(a), "METADATA") {
		t.Error("closed panel should not render")
	}
}

func TestSendRoutesToCurrentChannel(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 20)

	a.input.SetValue("hello there")
	cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on non-empty input should produce a send command")
	}
	_ = cmd() // run the command: it calls driver.Send
	if len(d.sends) != 1 || d.sends[0] != (sentRecord{ch: ChannelIC, text: "hello there"}) {
		t.Fatalf("sends = %+v, want one IC send of %q", d.sends, "hello there")
	}
	if a.input.Value() != "" {
		t.Errorf("input should clear after send, got %q", a.input.Value())
	}

	press(t, a, ctrl('t'))
	a.input.SetValue("ooc remark")
	cmd = press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a send command on OOC channel")
	}
	_ = cmd()
	if len(d.sends) != 2 || d.sends[1] != (sentRecord{ch: ChannelOOC, text: "ooc remark"}) {
		t.Fatalf("sends = %+v, want an OOC send of %q", d.sends, "ooc remark")
	}
}

func TestSendErrorSurfacesAsErrorLine(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 20)
	d.sendErr = errors.New("harness offline")

	a.input.SetValue("hello")
	cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a send command")
	}
	_, _ = a.Update(cmd()) // deliver the failure back into the app

	got := view(a)
	if !strings.Contains(got, "send failed") || !strings.Contains(got, "harness offline") {
		t.Errorf("send error should appear in the transcript, view:\n%s", got)
	}
}

func TestViewportAutoScrollFollowsNewEvents(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 40, 12) // body height 9
	narrative := Event{
		Kind:     EventNarrative,
		Channel:  ChannelIC,
		Markdown: "The corridor slopes downward, the air growing cold and still.\n\nSomewhere ahead, water drips in a steady rhythm.",
	}
	for i := 0; i < 8; i++ {
		_, _ = a.Update(eventMsg{ev: narrative})
	}
	if !a.viewport.AtBottom() {
		t.Fatal("new events should keep the viewport pinned to the bottom")
	}
	if !a.follow {
		t.Fatal("follow mode should be on while at the bottom")
	}

	// Scrolling up suspends follow mode.
	press(t, a, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if a.viewport.AtBottom() {
		t.Fatal("pgup should scroll away from the bottom")
	}
	if a.follow {
		t.Error("scrolling up should suspend follow mode")
	}

	// A new event must not yank the view back down.
	_, _ = a.Update(eventMsg{ev: narrative})
	if a.viewport.AtBottom() {
		t.Error("event while scrolled up should not re-pin the viewport")
	}

	// Scrolling back to the bottom resumes follow mode.
	for i := 0; i < 5 && !a.viewport.AtBottom(); i++ {
		press(t, a, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !a.viewport.AtBottom() {
		t.Fatal("pgdown should reach the bottom")
	}
	if !a.follow {
		t.Error("reaching the bottom should resume follow mode")
	}
	_, _ = a.Update(eventMsg{ev: narrative})
	if !a.viewport.AtBottom() {
		t.Error("follow mode should pin new events to the bottom again")
	}
}

func TestWindowResizeRerendersEntries(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	_, _ = a.Update(eventMsg{ev: Event{
		Kind:       EventToolAnnotation,
		Channel:    ChannelIC,
		Annotation: &Annotation{ToolName: "state:save", Summary: "session stored", OK: true},
	}})
	if a.entries[0].width != 80 {
		t.Fatalf("entry rendered at width %d, want 80", a.entries[0].width)
	}
	_, _ = a.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	if a.entries[0].width != 40 {
		t.Errorf("entry width after resize = %d, want 40", a.entries[0].width)
	}
	if w := lipgloss.Width(strings.SplitN(view(a), "\n", 2)[1]); w > 40 {
		t.Errorf("transcript line wider than the new width: %d", w)
	}
}

func TestQQuitsOnlyWithEmptyInput(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 60, 20)
	a.input.SetValue("quiet")
	cmd := press(t, a, key('q'))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q must not quit while the input line has text")
		}
	}
	if a.input.Value() != "quietq" {
		t.Errorf("q should type into the input, got %q", a.input.Value())
	}

	a.input.Reset()
	cmd = press(t, a, key('q'))
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q with empty input should quit")
	}

	cmd = press(t, a, ctrl('c'))
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should quit")
	}
}

func TestInputHistory(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 20)

	a.input.SetValue("first message")
	if cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	a.input.SetValue("second message")
	if cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	if len(d.sends) != 2 {
		t.Fatalf("sends = %+v, want 2", d.sends)
	}

	press(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	if a.input.Value() != "second message" {
		t.Errorf("first up = %q, want %q", a.input.Value(), "second message")
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	if a.input.Value() != "first message" {
		t.Errorf("second up = %q, want %q", a.input.Value(), "first message")
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	if a.input.Value() != "second message" {
		t.Errorf("down = %q, want %q", a.input.Value(), "second message")
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	if a.input.Value() != "" {
		t.Errorf("down past newest should clear, got %q", a.input.Value())
	}

	// Esc clears whatever is on the line.
	a.input.SetValue("scratch")
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
	if a.input.Value() != "" {
		t.Errorf("esc should clear the input, got %q", a.input.Value())
	}
}

func TestDriverEventStream(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 20)

	d.events <- Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: "The bell tolls twice."}
	cmd := a.requestEvents()
	if cmd == nil {
		t.Fatal("expected a read command while the stream is open")
	}
	_, _ = a.Update(cmd())
	if !strings.Contains(view(a), "bell tolls") {
		t.Errorf("event from the driver should reach the transcript, view:\n%s", view(a))
	}

	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	cmd = a.requestEvents()
	if cmd == nil {
		t.Fatal("expected a read command before the close is observed")
	}
	_, next := a.Update(cmd())
	if !strings.Contains(view(a), "event stream closed") {
		t.Errorf("closing the driver should be reported, view:\n%s", view(a))
	}
	if next != nil {
		t.Error("a closed event stream must not be re-armed")
	}
}

// TestPaletteLocalActions drives the palette's view actions end to end:
// every entry is picked by injecting key presses (open → type to filter →
// enter), and the local effects land without the driver seeing a thing.
func TestPaletteLocalActions(t *testing.T) {
	t.Run("theme", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 20)
		paletteChoose(t, a, "obsidian")
		if a.theme.Name != "obsidian" {
			t.Errorf("theme = %q, want obsidian", a.theme.Name)
		}
		if !strings.Contains(view(a), "theme → obsidian") {
			t.Errorf("theme change should be reported, view:\n%s", view(a))
		}

		// Defensive: the palette only offers built-ins, but an unknown
		// name must still surface instead of silently keeping the theme.
		a.applyThemeName("nope")
		if !strings.Contains(view(a), "valid themes") {
			t.Errorf("bad theme should list the valid ones, view:\n%s", view(a))
		}
	})

	t.Run("clear", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 20)
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventSystem, Markdown: "old news"}})
		if len(a.entries) == 0 {
			t.Fatal("expected an entry before clearing")
		}
		paletteChoose(t, a, "clear screen")
		if len(a.entries) != 0 {
			t.Errorf("Clear screen should empty the transcript, got %d entries", len(a.entries))
		}
		if !a.follow {
			t.Error("Clear screen should re-arm follow mode")
		}
	})

	t.Run("help", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 20)
		paletteChoose(t, a, "help")
		if !a.helpOpen {
			t.Fatal("Help should open the overlay")
		}
		got := view(a)
		for _, want := range []string{"Roll dice", "Theme: mythras", "ctrl+t", "ctrl+p"} {
			if !strings.Contains(got, want) {
				t.Errorf("help should mention %q, view:\n%s", want, got)
			}
		}
	})

	t.Run("quit", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 20)
		cmd := paletteChoose(t, a, "quit")
		if cmd == nil {
			t.Fatal("Quit should return a command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Error("Quit should quit")
		}
	})

	t.Run("session commands go over Driver.Command", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 60, 20)
		if cmd := paletteChoose(t, a, "save session"); cmd != nil {
			_ = cmd() // the command runs off the update loop, like a send
		}
		if len(d.commands) != 1 || d.commands[0].name != "save" {
			t.Fatalf("commands = %+v, want one Driver.Command(\"save\")", d.commands)
		}
		if len(d.sends) != 0 {
			t.Errorf("sends = %+v, want none — commands are not chat", d.sends)
		}
	})

	t.Run("typed slash text is plain chat", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 60, 20)
		a.input.SetValue("/frobnicate")
		cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("typed text should be sent")
		}
		_ = cmd()
		if len(d.sends) != 1 || d.sends[0].text != "/frobnicate" {
			t.Errorf("sends = %+v, want the typed text verbatim", d.sends)
		}
		if len(d.commands) != 0 {
			t.Errorf("commands = %+v, want none — typed text never parses", d.commands)
		}
		if strings.Contains(view(a), "unknown command") {
			t.Errorf("no parse error may appear, view:\n%s", view(a))
		}
	})
}

// TestRollArgMode pins the palette's only argument command: choosing Roll
// dice closes the palette and puts the input line into expr mode, Enter
// runs Driver.Command("roll", expr), and esc cancels back to normal.
func TestRollArgMode(t *testing.T) {
	t.Run("enter runs the roll command", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 60, 20)
		paletteChoose(t, a, "roll dice")
		if a.argMode != "roll" {
			t.Fatalf("argMode = %q, want roll", a.argMode)
		}
		if a.pal.open {
			t.Fatal("choosing an entry must close the palette")
		}
		if a.input.Prompt != "expr ▸ " {
			t.Errorf("prompt = %q, want %q", a.input.Prompt, "expr ▸ ")
		}
		if len(d.commands) != 0 {
			t.Fatalf("commands = %+v, want none until enter", d.commands)
		}

		typeKeys(a, "2d6+3")
		cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("enter in arg mode should produce a command")
		}
		_ = cmd()
		if len(d.commands) != 1 || d.commands[0].name != "roll" ||
			len(d.commands[0].args) != 1 || d.commands[0].args[0] != "2d6+3" {
			t.Fatalf("commands = %+v, want roll 2d6+3", d.commands)
		}
		if a.argMode != "" || a.input.Prompt != "> " {
			t.Errorf("after enter: argMode=%q prompt=%q, want back to normal", a.argMode, a.input.Prompt)
		}
		if a.input.Value() != "" {
			t.Errorf("input = %q, want it cleared", a.input.Value())
		}
	})

	t.Run("esc cancels to the normal prompt", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 60, 20)
		paletteChoose(t, a, "roll dice")
		typeKeys(a, "3d6")
		press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
		if a.argMode != "" || a.input.Prompt != "> " {
			t.Fatalf("after esc: argMode=%q prompt=%q, want normal line", a.argMode, a.input.Prompt)
		}
		if len(d.commands) != 0 {
			t.Errorf("commands = %+v, want none after cancel", d.commands)
		}
		// An empty enter also leaves arg mode without running anything.
		paletteChoose(t, a, "roll dice")
		press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
		if a.argMode != "" {
			t.Errorf("empty enter should cancel arg mode, got %q", a.argMode)
		}
		if len(d.commands) != 0 {
			t.Errorf("commands = %+v, want none for an empty expression", d.commands)
		}
	})

	t.Run("command failure surfaces as an error line", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 60, 20)
		d.commandErr = errors.New("session closed")
		cmd := paletteChoose(t, a, "save session")
		if cmd != nil {
			_, _ = a.Update(cmd())
		}
		if !strings.Contains(view(a), "command save failed") ||
			!strings.Contains(view(a), "session closed") {
			t.Errorf("command errors should surface, view:\n%s", view(a))
		}
	})
}

func TestBackgroundColorAdaptsTheme(t *testing.T) {
	a, _ := newTestApp(t, Options{Theme: "parchment"}, 60, 20)
	if a.theme.Dark {
		t.Fatal("parchment should start light-native")
	}
	_, _ = a.Update(tea.BackgroundColorMsg{Color: lipgloss.Color("#000000")})
	if !a.theme.Dark {
		t.Error("a dark terminal background report should flip parchment to its dark variant")
	}
}

func TestEditorArgvFromEnv(t *testing.T) {
	env := func(kv map[string]string) func(string) string {
		return func(k string) string { return kv[k] }
	}
	tests := []struct {
		name    string
		env     map[string]string
		want    []string
		wantErr string
	}{
		{
			name: "VISUAL with arguments",
			env:  map[string]string{"VISUAL": "code -w"},
			want: []string{"code", "-w"},
		},
		{
			name: "EDITOR fallback",
			env:  map[string]string{"EDITOR": "vim"},
			want: []string{"vim"},
		},
		{
			name: "vi default",
			env:  map[string]string{},
			want: []string{"vi"},
		},
		{
			name: "VISUAL wins over EDITOR",
			env:  map[string]string{"VISUAL": "emacs", "EDITOR": "vim"},
			want: []string{"emacs"},
		},
		{
			name: "quoted path with spaces",
			env:  map[string]string{"VISUAL": `'my editor' -f`},
			want: []string{"my editor", "-f"},
		},
		{
			name:    "unterminated quote",
			env:     map[string]string{"VISUAL": `code "oops`},
			wantErr: "unterminated",
		},
		{
			name:    "trailing backslash",
			env:     map[string]string{"VISUAL": `vi\`},
			wantErr: "trailing backslash",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editorArgv(env(tt.env))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("got %q, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("editorArgv: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("argv[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEditorResultLoadsInput(t *testing.T) {
	t.Run("edited text loads and temp file is removed", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 20)
		f, err := os.CreateTemp(t.TempDir(), "edit-*.md")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("a longer message\n"); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		_, _ = a.Update(editorFinishedMsg{path: f.Name()})
		if a.input.Value() != "a longer message" {
			t.Errorf("input = %q, want the edited text", a.input.Value())
		}
		if _, err := os.Stat(f.Name()); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("temp file should be removed, stat err = %v", err)
		}
	})

	t.Run("editor failure becomes a system error line", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 20)
		a.input.SetValue("keep me")
		_, _ = a.Update(editorFinishedMsg{
			err:  errors.New("exit status 1"),
			path: filepath.Join(t.TempDir(), "missing.md"),
		})
		if a.input.Value() != "keep me" {
			t.Errorf("input = %q, want it untouched after a failed edit", a.input.Value())
		}
		if !strings.Contains(view(a), "editor failed") {
			t.Errorf("editor errors should be reported, view:\n%s", view(a))
		}
	})
}

func TestMetadataPanelRefreshPolicies(t *testing.T) {
	a, d := newTestApp(t, Options{}, 80, 30)

	// Toggling the panel on fetches the snapshot.
	d.md = Metadata{Scene: "Rope bridge at dusk"}
	press(t, a, ctrl('m'))
	if !strings.Contains(plain(view(a)), "Rope bridge at dusk") {
		t.Errorf("panel should render the snapshot fetched on toggle, view:\n%s", plain(view(a)))
	}

	// Every processed event refreshes it.
	d.md = Metadata{Scene: "Collapsed mine shaft"}
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventSystem, Channel: ChannelIC, Markdown: "the clock advances"}})
	if !strings.Contains(plain(view(a)), "Collapsed mine shaft") {
		t.Errorf("panel should refresh on every event, view:\n%s", plain(view(a)))
	}

	// The slow two-second tick refreshes it too.
	d.md = Metadata{Scene: "Flooded reliquary"}
	_, _ = a.Update(metadataTickMsg{})
	if !strings.Contains(plain(view(a)), "Flooded reliquary") {
		t.Errorf("panel should refresh on the slow tick, view:\n%s", plain(view(a)))
	}

	// Layout: the chat column shrinks, and nothing spills the window.
	if a.chatW >= a.width {
		t.Errorf("chat width %d should shrink below window width %d", a.chatW, a.width)
	}
	lines := strings.Split(plain(view(a)), "\n")
	if len(lines) != 30 {
		t.Errorf("panel open: view has %d lines, want 30", len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 80 {
			t.Errorf("line wider than the window (%d cols): %q", lipgloss.Width(line), line)
		}
	}
}

func TestDiceEventRoutesThroughTheatre(t *testing.T) {
	a, _ := newTestApp(t, Options{DiceAnimation: true}, 80, 24)
	roll := rollEvent()
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDice, Channel: ChannelIC, Dice: &roll}})

	if !a.dice.Active() {
		t.Fatal("the roll should go on stage")
	}
	if len(a.entries) != 0 {
		t.Errorf("the banner joins the transcript only once the animation ends, got %d entries", len(a.entries))
	}
	got := plain(view(a))
	for _, want := range []string{"3d6+2", "rolling"} {
		if !strings.Contains(got, want) {
			t.Errorf("theatre row should contain %q, view:\n%s", want, got)
		}
	}
	if strings.Contains(got, "= 14") {
		t.Errorf("the rolling view must not spoil the total, view:\n%s", got)
	}

	// The transcript keeps accepting events during the animation; they
	// render after the theatre row.
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC, Markdown: "The gong sounds.",
	}})
	if len(a.entries) != 1 {
		t.Fatalf("narrative should file immediately, got %d entries", len(a.entries))
	}
	got = plain(view(a))
	rollAt, narrAt := strings.Index(got, "rolling"), strings.Index(got, "gong")
	if rollAt < 0 || narrAt < 0 || rollAt > narrAt {
		t.Errorf("events should render after the animating roll (roll=%d, narrative=%d):\n%s",
			rollAt, narrAt, got)
	}

	// Drive the animation to completion with synthetic ticks.
	for i := 0; i < diceRollingTicks; i++ {
		_, _ = a.Update(diceTickMsg{})
	}
	for i := 0; i < len(roll.Dice); i++ {
		_, _ = a.Update(diceSettleMsg{})
	}
	if a.dice.phase != diceResult {
		t.Fatalf("phase = %v, want result", a.dice.phase)
	}
	got = plain(view(a))
	for _, want := range []string{"SUCCESS", "= 14"} {
		if !strings.Contains(got, want) {
			t.Errorf("held banner should contain %q, view:\n%s", want, got)
		}
	}
	if len(a.entries) != 1 {
		t.Errorf("the roll is not filed while its banner is held, got %d entries", len(a.entries))
	}

	// The finish flushes the banner at the position the roll arrived.
	_, _ = a.Update(diceFinishMsg{})
	if a.dice.Active() {
		t.Error("the theatre should be idle after the flush")
	}
	if len(a.entries) != 2 {
		t.Fatalf("after flush: %d entries, want 2", len(a.entries))
	}
	if a.entries[0].ev.Kind != EventDice {
		t.Error("the roll should be filed before the narrative that arrived during the animation")
	}
	if a.entries[1].ev.Kind != EventNarrative {
		t.Error("the narrative should stay after the roll")
	}
	got = plain(view(a))
	if strings.Contains(got, "rolling") {
		t.Errorf("no roll-in-progress should remain after the flush, view:\n%s", got)
	}
	for _, want := range []string{"3d6+2", "[4 6 2]", "+2", "= 14", "SUCCESS"} {
		if !strings.Contains(got, want) {
			t.Errorf("filed banner should contain %q, view:\n%s", want, got)
		}
	}
}

func TestDiceAnimationOffRendersImmediately(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 24) // DiceAnimation defaults to false
	roll := rollEvent()
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDice, Channel: ChannelIC, Dice: &roll}})

	if a.dice.Active() {
		t.Error("no roll should stay on stage when animation is off")
	}
	if len(a.entries) != 1 {
		t.Fatalf("the banner should join the transcript at once, got %d entries", len(a.entries))
	}
	got := plain(view(a))
	if strings.Contains(got, "rolling") {
		t.Errorf("animation off must show no roll-in-progress, view:\n%s", got)
	}
	for _, want := range []string{"3d6+2", "[4 6 2]", "+2", "= 14", "SUCCESS"} {
		if !strings.Contains(got, want) {
			t.Errorf("instant banner should contain %q, view:\n%s", want, got)
		}
	}
}

func TestSecondRollPreemptsRunningTheatre(t *testing.T) {
	a, _ := newTestApp(t, Options{DiceAnimation: true}, 80, 24)
	first := rollEvent()
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDice, Channel: ChannelIC, Dice: &first}})
	second := DiceEvent{
		Expr: "1d20", Dice: []int{17}, Modifier: 0, Total: 17, Outcome: "critical",
	}
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDice, Channel: ChannelIC, Dice: &second}})

	// The running roll is filed at once; the new one takes the stage.
	if len(a.entries) != 1 || a.entries[0].ev.Dice == nil || a.entries[0].ev.Dice.Expr != "3d6+2" {
		t.Fatalf("preempted roll should be filed first, entries = %+v", a.entries)
	}
	if !a.dice.Active() || a.dice.ev.Expr != "1d20" {
		t.Fatalf("the new roll should be on stage, active=%v expr=%q", a.dice.Active(), a.dice.ev.Expr)
	}

	for i := 0; i < diceRollingTicks; i++ {
		_, _ = a.Update(diceTickMsg{})
	}
	for i := 0; i < len(second.Dice); i++ {
		_, _ = a.Update(diceSettleMsg{})
	}
	_, _ = a.Update(diceFinishMsg{})
	if len(a.entries) != 2 {
		t.Fatalf("after the second flush: %d entries, want 2", len(a.entries))
	}
	if a.entries[1].ev.Dice == nil || a.entries[1].ev.Dice.Expr != "1d20" {
		t.Error("the second roll should be filed after the first")
	}
}

func TestPlayerEchoRendersYou(t *testing.T) {
	a, d := newTestApp(t, Options{}, 80, 20)
	you := a.theme.Styles.Speaker.Render("You")

	// GM narrative carries no player tag.
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC, Markdown: "The bell tolls twice.",
	}})
	if strings.Contains(view(a), you) {
		t.Error("GM narrative must not be tagged as the player")
	}

	// The player sends; the TUI never echoes its own input.
	a.input.SetValue("the door is barred.")
	if cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	if len(d.sends) != 1 {
		t.Fatalf("sends = %+v, want one send", d.sends)
	}
	if strings.Contains(view(a), you) {
		t.Error("the TUI must not tag its own input before the harness echoes it")
	}

	// The harness echoes the message back through Events: tagged "You".
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC, Markdown: "the door is barred.",
	}})
	if !strings.Contains(view(a), you) {
		t.Errorf("the harness echo should render with the You tag, view:\n%s", plain(view(a)))
	}

	// Later narrative is not tagged, and the echo is consumed exactly once.
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC, Markdown: "Indeed it is.",
	}})
	if n := strings.Count(view(a), you); n != 1 {
		t.Errorf("exactly one You tag expected, got %d", n)
	}
}

// TestTagVisibilityGroupsRuns pins the run-based annotation rule: a tag
// shows only at the start of a run of entries from the same speaker on the
// same channel — narrative/system/error entries count as chat, while tool
// and dice strips are metadata inside the current run.
func TestTagVisibilityGroupsRuns(t *testing.T) {
	narr := func(ch Channel, player bool) entry {
		return entry{ev: Event{Kind: EventNarrative, Channel: ch}, player: player}
	}
	sys := func(ch Channel, player bool) entry {
		return entry{ev: Event{Kind: EventSystem, Channel: ch}, player: player}
	}
	ann := func(ch Channel) entry {
		return entry{ev: Event{Kind: EventToolAnnotation, Channel: ch,
			Annotation: &Annotation{ToolName: "t", Summary: "s", OK: true}}}
	}
	dice := func(ch Channel) entry {
		return entry{ev: Event{Kind: EventDice, Channel: ch,
			Dice: &DiceEvent{Expr: "1d6", Dice: []int{1}, Total: 1}}}
	}
	tests := []struct {
		name    string
		entries []entry
		want    []bool
	}{
		{
			"transcript start is always tagged",
			[]entry{narr(ChannelIC, false)},
			[]bool{true},
		},
		{
			"a run tags only its first entry",
			[]entry{narr(ChannelIC, false), narr(ChannelIC, false), narr(ChannelIC, false)},
			[]bool{true, false, false},
		},
		{
			"a channel switch re-tags",
			[]entry{narr(ChannelIC, false), narr(ChannelOOC, false), narr(ChannelOOC, false)},
			[]bool{true, true, false},
		},
		{
			"a player to GM switch re-tags",
			[]entry{narr(ChannelIC, true), narr(ChannelIC, false)},
			[]bool{true, true},
		},
		{
			"tool strips do not break the run",
			[]entry{narr(ChannelIC, false), ann(ChannelIC), narr(ChannelIC, false)},
			[]bool{true, false, false},
		},
		{
			"dice strips do not break the run",
			[]entry{narr(ChannelIC, false), dice(ChannelIC), narr(ChannelIC, false)},
			[]bool{true, false, false},
		},
		{
			"system lines count as chat",
			[]entry{narr(ChannelIC, true), sys(ChannelIC, false), narr(ChannelIC, false)},
			[]bool{true, true, false},
		},
		{
			"a transcript opening on metadata still tags every speaker start",
			[]entry{ann(ChannelIC), narr(ChannelIC, false)},
			[]bool{true, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagVisibility(tt.entries)
			if len(got) != len(tt.want) {
				t.Fatalf("tagVisibility returned %d flags, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("entry %d tagged = %t, want %t (all: %v)", i, got[i], want, got)
				}
			}
		})
	}
}

// TestRenderEventTagSuppression checks the rendered side: a continuation
// entry drops the tag, a run start keeps it, and an unlaid-out rendering
// (w < 1) shows it rather than lose the sender.
func TestRenderEventTagSuppression(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	ev := Event{Kind: EventNarrative, Channel: ChannelOOC, Markdown: "Sure."}

	if got := plain(a.renderEvent(ev, 80, false, true)); !strings.Contains(got, "[ooc]") {
		t.Errorf("a run-start entry should carry its tag: %q", got)
	}
	if got := plain(a.renderEvent(ev, 80, false, false)); strings.Contains(got, "[ooc]") {
		t.Errorf("a continuation entry must not repeat the tag: %q", got)
	}
	if got := plain(a.renderEvent(ev, 0, false, false)); !strings.Contains(got, "[ooc]") {
		t.Errorf("before layout the tag must show rather than be lost: %q", got)
	}
}

// TestOOCRunTagsOnce drives a full OOC exchange through ingest: the player
// echo, the GM's first paragraph and its continuation — tagged per speaker,
// not per paragraph.
func TestOOCRunTagsOnce(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	a.recordEcho("how far is the abbey?")
	for _, md := range []string{
		"how far is the abbey?",      // the harness echo of the player
		"The abbey lies a day east.", // the GM starts a run
		"Past the salt road.",        // continuation of the same run
	} {
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelOOC, Markdown: md}})
	}
	if len(a.entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(a.entries))
	}
	for i, want := range []bool{true, true, false} {
		if a.entries[i].tagged != want {
			t.Errorf("entry %d tagged = %t, want %t", i, a.entries[i].tagged, want)
		}
	}
	if n := strings.Count(plain(view(a)), "[ooc]"); n != 2 {
		t.Errorf("OOC run renders %d [ooc] tags, want 2 (once per speaker):\n%s", n, plain(view(a)))
	}
}

// TestGMSpeakerTag pins the rendered side of the GM annotation: a run-start
// GM line carries the speaker tag exactly like a player's "You", a
// continuation drops it, an un-laid-out rendering (w < 1) forces it rather
// than lose the speaker, an OOC GM run joins the channel tag as
// "[ooc] GM", and system/error lines never carry a speaker tag.
func TestGMSpeakerTag(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	narr := Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: "The lamp gutters."}

	if got := plain(a.renderEvent(narr, 80, false, true)); !strings.Contains(got, "GM") {
		t.Errorf("a GM run start should carry the GM tag: %q", got)
	}
	if got := plain(a.renderEvent(narr, 80, false, false)); strings.Contains(got, "GM") {
		t.Errorf("a second GM line in the run must not repeat the tag: %q", got)
	}
	if got := plain(a.renderEvent(narr, 0, false, false)); !strings.Contains(got, "GM") {
		t.Errorf("before layout the GM tag must show rather than be lost: %q", got)
	}
	if got := plain(a.renderEvent(narr, 80, true, true)); !strings.Contains(got, "You") || strings.Contains(got, "GM") {
		t.Errorf("a player run keeps the You tag and no GM tag: %q", got)
	}
	ooc := Event{Kind: EventNarrative, Channel: ChannelOOC, Markdown: "The inn closes at dusk."}
	if got := plain(a.renderEvent(ooc, 80, false, true)); !strings.Contains(got, "[ooc] GM") {
		t.Errorf("an OOC GM run should read %q: %q", "[ooc] GM", got)
	}
	for _, kind := range []EventKind{EventSystem, EventError} {
		line := Event{Kind: kind, Channel: ChannelIC, Markdown: "the clock advances"}
		if got := plain(a.renderEvent(line, 80, false, true)); strings.Contains(got, "GM") {
			t.Errorf("kind %v lines carry no speaker tag: %q", kind, got)
		}
	}
}

// TestGMRunTagsOnce drives a full exchange through ingest — two GM lines, a
// player line, another GM line — and pins the run rule on the rendered
// transcript: GM appears once per GM run, You once for the player run, and
// each speaker switch re-tags.
func TestGMRunTagsOnce(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	for _, ev := range []Event{
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "The bell tolls twice."},
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "It tolls for the dead."},
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "I count the peals.", Player: true},
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "Night falls over the moor."},
	} {
		_, _ = a.Update(eventMsg{ev: ev})
	}
	if len(a.entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(a.entries))
	}
	for i, want := range []bool{true, false, true, true} {
		if a.entries[i].tagged != want {
			t.Errorf("entry %d tagged = %t, want %t", i, a.entries[i].tagged, want)
		}
	}
	got := plain(view(a))
	if n := strings.Count(got, "GM"); n != 2 {
		t.Errorf("GM lines render %d GM tags, want 2 (once per run):\n%s", n, got)
	}
	if n := strings.Count(got, "You"); n != 1 {
		t.Errorf("player run renders %d You tags, want 1:\n%s", n, got)
	}
}

// TestOptionsInitialChannelStartsOnOOC pins the persisted-mode handoff: the
// app opens on the channel Options names — the session's last-used mode —
// in both the input prompt and the header switch.
func TestOptionsInitialChannelStartsOnOOC(t *testing.T) {
	a, _ := newTestApp(t, Options{InitialChannel: ChannelOOC}, 60, 20)
	if a.channel != ChannelOOC || a.input.Prompt != "ooc> " {
		t.Fatalf("start: channel=%v prompt=%q", a.channel, a.input.Prompt)
	}
	got := view(a)
	for _, want := range []string{"[OOC]", "ooc> "} {
		if !strings.Contains(got, want) {
			t.Errorf("OOC start should show %q, view:\n%s", want, got)
		}
	}
}

// TestChannelSwitchNotifiesDriver pins the mode-persistence hook: every
// local switch path — ctrl+t and the palette's channel entry — reports the
// new channel to the driver, which persists it for the next resume.
func TestChannelSwitchNotifiesDriver(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 20)
	if len(d.channels) != 0 {
		t.Fatalf("construction must not report a channel, got %v", d.channels)
	}
	press(t, a, ctrl('t'))
	if len(d.channels) != 1 || d.channels[0] != ChannelOOC {
		t.Fatalf("after ctrl+t channels = %v, want [OOC]", d.channels)
	}
	paletteChoose(t, a, "switch to ic")
	if len(d.channels) != 2 || d.channels[1] != ChannelIC {
		t.Fatalf("after the palette switch channels = %v, want [OOC IC]", d.channels)
	}
	paletteChoose(t, a, "switch to ooc")
	if len(d.channels) != 3 || d.channels[2] != ChannelOOC {
		t.Errorf("after the second palette switch channels = %v, want [OOC IC OOC]", d.channels)
	}
}

// TestContinuationAppendsToOpenEntry pins the streaming display rule: a
// narrative batch flagged Continuation extends the entry already open on
// the same channel — one entry, one blank line before it, the combined
// markdown — instead of starting a new one.
func TestContinuationAppendsToOpenEntry(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 30)
	for _, ev := range []Event{
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "Earlier."},
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: "The lamp gutters."},
		{Kind: EventNarrative, Channel: ChannelIC, Markdown: " It flickers once.", Continuation: true},
	} {
		_, _ = a.Update(eventMsg{ev: ev})
	}
	if len(a.entries) != 2 {
		t.Fatalf("entries = %d, want 2 (a continuation never opens one)", len(a.entries))
	}
	const combined = "The lamp gutters. It flickers once."
	if got := a.entries[1].ev.Markdown; got != combined {
		t.Errorf("entry markdown = %q, want the combined %q", got, combined)
	}
	lines := strings.Split(plain(view(a)), "\n")
	first, second := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "Earlier.") {
			first = i
		}
		if strings.Contains(line, "The lamp gutters.") {
			second = i
		}
	}
	if first < 0 || second < 0 {
		t.Fatalf("both entries should render, view:\n%s", plain(view(a)))
	}
	if second-first != 2 {
		t.Errorf("entries must stay separated by exactly one blank line, got %d lines between them:\n%s",
			second-first-1, plain(view(a)))
	}
}

// TestContinuationAfterMismatchStartsNewEntry pins the defensive side: a
// continuation whose last entry cannot take it — another channel, another
// kind, or the player's own line — starts a fresh entry instead of
// splicing across a boundary.
func TestContinuationAfterMismatchStartsNewEntry(t *testing.T) {
	t.Run("channel", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 30)
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: "On the IC channel."}})
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelOOC,
			Markdown: "Across the divide.", Continuation: true}})
		if len(a.entries) != 2 {
			t.Fatalf("entries = %d, want 2 (the channel changed)", len(a.entries))
		}
		if got := a.entries[1].ev.Markdown; got != "Across the divide." {
			t.Errorf("second entry = %q, want the continuation on its own", got)
		}
	})

	t.Run("kind", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 30)
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventSystem, Channel: ChannelIC, Markdown: "the clock advances"}})
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC,
			Markdown: "Narrative after a system line.", Continuation: true}})
		if len(a.entries) != 2 {
			t.Fatalf("entries = %d, want 2 (the system line is not narrative)", len(a.entries))
		}
	})

	t.Run("speaker", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 30)
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC,
			Markdown: "the door is barred.", Player: true}})
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC,
			Markdown: "GM words must not join it.", Continuation: true}})
		if len(a.entries) != 2 {
			t.Fatalf("entries = %d, want 2 (the player's line is a different speaker)", len(a.entries))
		}
	})
}

// TestReplayedDiceSkipsTheTheatre pins the resume display: a dice event
// flagged Replay files its finished banner at once — no theatre, no
// animation command, nothing re-performed — while a live roll still goes
// on stage exactly as before.
func TestReplayedDiceSkipsTheTheatre(t *testing.T) {
	a, _ := newTestApp(t, Options{DiceAnimation: true}, 80, 24)
	roll := rollEvent()

	cmd := a.ingest(Event{Kind: EventDice, Channel: ChannelIC, Dice: &roll, Replay: true})
	if cmd != nil {
		t.Error("a replayed roll must not return an animation command")
	}
	if a.dice.Active() {
		t.Error("a replayed roll must never take the stage")
	}
	if len(a.entries) != 1 || a.entries[0].ev.Dice == nil {
		t.Fatalf("entries = %+v, want the finished banner filed at once", a.entries)
	}
	if !a.entries[0].ev.Replay {
		t.Error("the filed banner should keep its Replay flag")
	}
	if !strings.Contains(plain(view(a)), "= 14") {
		t.Errorf("the finished banner should show the total, view:\n%s", plain(view(a)))
	}

	cmd = a.ingest(Event{Kind: EventDice, Channel: ChannelIC, Dice: &roll})
	if cmd == nil {
		t.Error("a live roll should still start the theatre")
	}
	if !a.dice.Active() {
		t.Error("a live roll should go on stage")
	}
}

// TestContextUsageText pins the footer's readout format: tokens below a
// thousand as-is and above as "42.3K", the percentage of the window
// rounded and clamped at 100%, nothing before the first model call, and
// tokens alone when the window is unknown.
func TestContextUsageText(t *testing.T) {
	tests := []struct {
		name string
		c    ContextUsage
		want string
	}{
		{"no model call yet", ContextUsage{}, ""},
		{"negative tokens read as no call", ContextUsage{Tokens: -5, Window: 1000}, ""},
		{"plain int below a thousand", ContextUsage{Tokens: 999}, "999"},
		{"thousands with an unknown window", ContextUsage{Tokens: 42340}, "42.3K"},
		{"thousands against the window", ContextUsage{Tokens: 42340, Window: 128000}, "42.3K (33%)"},
		{"rounds the percentage", ContextUsage{Tokens: 500, Window: 1000}, "500 (50%)"},
		{"exactly the window", ContextUsage{Tokens: 1000, Window: 1000}, "1.0K (100%)"},
		{"over the window clamps to 100%", ContextUsage{Tokens: 1500, Window: 1000}, "1.5K (100%)"},
		{"over the window as a plain int", ContextUsage{Tokens: 999, Window: 500}, "999 (100%)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contextUsageText(tt.c); got != tt.want {
				t.Errorf("contextUsageText(%+v) = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

// TestFooterShowsContextUsageRightAligned pins the footer layout: the
// hints stay left, the readout sits flush to the right edge of the
// terminal, the line never exceeds the window (the hints clip when the
// window is too narrow for both), and no readout shows before the first
// model call.
func TestFooterShowsContextUsageRightAligned(t *testing.T) {
	footerLine := func(a *App) string {
		lines := strings.Split(view(a), "\n")
		return lines[len(lines)-1]
	}
	refresh := func(a *App) {
		_, _ = a.Update(eventMsg{ev: Event{Kind: EventSystem, Channel: ChannelIC, Markdown: "tick"}})
	}

	a, d := newTestApp(t, Options{}, 80, 24)
	refresh(a)
	if got := plain(footerLine(a)); strings.Contains(got, "%") || strings.Contains(got, "42.3K") {
		t.Errorf("no context readout before the first model call, footer: %q", got)
	}

	d.md = Metadata{ContextUsage: ContextUsage{Tokens: 42340, Window: 128000}}
	refresh(a)
	got := plain(footerLine(a))
	if !strings.HasPrefix(got, "enter send") {
		t.Errorf("footer should keep its hints on the left, got %q", got)
	}
	if !strings.HasSuffix(got, "42.3K (33%)") {
		t.Errorf("footer should end with the readout, got %q", got)
	}
	if w := lipgloss.Width(footerLine(a)); w != 80 {
		t.Errorf("right-aligned footer width = %d, want exactly the window width 80", w)
	}

	// A narrow window clips the hints rather than pushing the readout
	// off the edge.
	_, _ = a.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	got = plain(footerLine(a))
	if !strings.HasSuffix(got, "42.3K (33%)") {
		t.Errorf("the readout should keep the right edge on a narrow window, got %q", got)
	}
	if w := lipgloss.Width(footerLine(a)); w > 40 {
		t.Errorf("footer wider than the window: %d > 40 (%q)", w, got)
	}
}

// TestReplayedNarrativeLeavesTurnStateAlone pins the Replay guards: a
// line replayed from the log files as history — no throbber transition,
// no echo claimed — so resuming can never disturb a turn that has not
// started or steal the recognition of a message the player just sent.
func TestReplayedNarrativeLeavesTurnStateAlone(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 24)
	a.recordEcho("I open the door.")

	cmd := a.ingest(Event{Kind: EventNarrative, Channel: ChannelIC,
		Markdown: "I open the door.", Player: true, Replay: true})
	if cmd != nil {
		t.Error("a replayed line must not return a throbber command")
	}
	if a.throb.state != throbIdle {
		t.Errorf("throbber state = %v, want idle for a replayed line", a.throb.state)
	}
	if len(a.entries) != 1 || !a.entries[0].player {
		t.Fatalf("entries = %+v, want the replayed player line filed as the player", a.entries)
	}
	if len(a.echoes) != 1 {
		t.Errorf("echo list = %v, want the outstanding echo untouched by replay", a.echoes)
	}
}

// TestConfirmMode pins the input line's confirm mode (the roll arg-mode
// pattern): an EventConfirm files its question as one styled, width-clipped
// transcript line and switches the prompt to "N/y ▸ "; y/Y answer y and
// leave the line (the echo arrives next), n/N/enter/esc answer n, leave the
// line and return the held message to it unsent, every other key is
// ignored, and ctrl+c still quits.
func TestConfirmMode(t *testing.T) {
	const (
		question = "You are in IC mode, but your response looks like a message for OOC. Is this correct? N/y"
		held     = "I slide the forged writ across the table."
	)
	// ask feeds one EventConfirm to the app, as the harness emits it.
	ask := func(t *testing.T, a *App, md string) {
		t.Helper()
		_, _ = a.Update(eventMsg{ev: Event{
			Kind: EventConfirm, Channel: ChannelIC, Markdown: md, ConfirmText: held,
		}})
	}
	// answer runs the command a key produced, off the update loop.
	answer := func(t *testing.T, cmd tea.Cmd) {
		t.Helper()
		if cmd == nil {
			t.Fatal("the answer key should produce a command")
		}
		_ = cmd()
	}
	wantCommand := func(d *fakeDriver, want string) bool {
		return len(d.commands) == 1 && d.commands[0].name == "confirm" &&
			len(d.commands[0].args) == 1 && d.commands[0].args[0] == want
	}

	t.Run("question renders and the prompt switches", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 100, 24)
		ask(t, a, question)
		if !a.confirm {
			t.Fatal("confirm mode should be on")
		}
		if a.input.Prompt != "N/y ▸ " {
			t.Errorf("prompt = %q, want %q", a.input.Prompt, "N/y ▸ ")
		}
		if got := plain(view(a)); !strings.Contains(got, question) {
			t.Errorf("the question should render, view:\n%s", got)
		}
		last := a.entries[len(a.entries)-1]
		if last.ev.Kind != EventConfirm || last.ev.ConfirmText != held {
			t.Fatalf("last entry = %+v, want the confirm question with its held text", last.ev)
		}
		if strings.Contains(last.text, "\n") {
			t.Errorf("the question must render as one line, got %q", last.text)
		}
		if last.text == plain(last.text) {
			t.Error("the question should be styled, not plain text")
		}
	})

	t.Run("a long question clips to the chat width", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 60, 24)
		ask(t, a, strings.Repeat("channel check ", 12))
		last := a.entries[len(a.entries)-1]
		if w := lipgloss.Width(last.text); w > a.chatW {
			t.Errorf("question rendered %d columns, want at most %d", w, a.chatW)
		}
		if strings.Contains(last.text, "\n") {
			t.Errorf("a clipped question must stay one line, got %q", last.text)
		}
	})

	t.Run("y and Y answer y and leave the line", func(t *testing.T) {
		for _, k := range []tea.KeyPressMsg{key('y'), key('Y')} {
			a, d := newTestApp(t, Options{}, 80, 24)
			ask(t, a, question)
			answer(t, press(t, a, k))
			if !wantCommand(d, "y") {
				t.Fatalf("%s: commands = %+v, want confirm y", k.String(), d.commands)
			}
			if a.confirm || a.input.Prompt != "> " {
				t.Errorf("%s: confirm=%v prompt=%q, want the normal line", k.String(), a.confirm, a.input.Prompt)
			}
			if a.input.Value() != "" {
				t.Errorf("%s: input = %q, want it untouched — y does not restore the text", k.String(), a.input.Value())
			}
		}
	})

	t.Run("n, N, enter and esc answer n and restore the held text", func(t *testing.T) {
		keys := []struct {
			name string
			msg  tea.KeyPressMsg
		}{
			{"n", key('n')},
			{"N", key('N')},
			{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
			{"esc", tea.KeyPressMsg{Code: tea.KeyEsc}},
		}
		for _, k := range keys {
			t.Run(k.name, func(t *testing.T) {
				a, d := newTestApp(t, Options{}, 80, 24)
				ask(t, a, question)
				answer(t, press(t, a, k.msg))
				if !wantCommand(d, "n") {
					t.Fatalf("commands = %+v, want confirm n", d.commands)
				}
				if a.confirm || a.input.Prompt != "> " {
					t.Errorf("confirm=%v prompt=%q, want the normal line", a.confirm, a.input.Prompt)
				}
				if a.input.Value() != held {
					t.Errorf("input = %q, want the held message returned unsent", a.input.Value())
				}
			})
		}
	})

	t.Run("other keys are ignored while the answer is pending", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 80, 24)
		ask(t, a, question)
		for _, k := range []tea.KeyPressMsg{
			key('x'), key('q'),
			tea.KeyPressMsg{Code: tea.KeyUp}, tea.KeyPressMsg{Code: tea.KeyDown},
			tea.KeyPressMsg{Code: tea.KeyPgUp}, ctrl('t'), ctrl('p'), ctrl('m'),
		} {
			press(t, a, k)
		}
		if len(d.commands) != 0 || len(d.sends) != 0 {
			t.Errorf("ignored keys reached the harness: commands=%+v sends=%+v", d.commands, d.sends)
		}
		if a.channel != ChannelIC {
			t.Errorf("channel = %v, want ctrl+t ignored during a confirm", a.channel)
		}
		if a.pal.open || a.helpOpen {
			t.Error("overlays must stay closed during a confirm")
		}
		if !a.confirm {
			t.Fatal("the confirm should still be pending")
		}
		if a.input.Value() != "" {
			t.Errorf("input = %q, want ignored keys kept out of the line", a.input.Value())
		}
	})

	t.Run("ctrl+c still quits", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 24)
		ask(t, a, question)
		cmd := press(t, a, ctrl('c'))
		if cmd == nil {
			t.Fatal("ctrl+c should produce a quit command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Error("ctrl+c from confirm mode should quit")
		}
	})

	t.Run("esc at rest still clears the line", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 24)
		ask(t, a, question)
		answer(t, press(t, a, key('n'))) // resolve the hold first
		a.input.SetValue("draft")
		press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
		if a.input.Value() != "" {
			t.Errorf("input = %q, want esc at rest to clear it", a.input.Value())
		}
		if a.input.Prompt != "> " {
			t.Errorf("prompt = %q, want the channel prompt back", a.input.Prompt)
		}
	})
}
