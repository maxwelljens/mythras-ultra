package tui

import (
	"strings"
	"testing"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		kind    CommandKind
		text    string
		args    []string
		wantErr string // substring; empty means no error
	}{
		{
			name: "plain text",
			in:   "You open the crypt door.",
			kind: CmdMessage,
			text: "You open the crypt door.",
		},
		{
			name: "plain text containing a slash",
			in:   "the ratio is 2/3",
			kind: CmdMessage,
			text: "the ratio is 2/3",
		},
		{
			name: "leading space is plain text",
			in:   " /ic",
			kind: CmdMessage,
			text: " /ic",
		},
		{
			name: "empty input",
			in:   "",
			kind: CmdMessage,
			text: "",
		},
		{
			name: "ic",
			in:   "/ic",
			kind: CmdIC,
			text: "/ic",
		},
		{
			name: "ooc is case-insensitive but preserves the raw text",
			in:   "/OOC",
			kind: CmdOOC,
			text: "/OOC",
		},
		{
			name: "theme with name",
			in:   "/theme mythras",
			kind: CmdTheme,
			text: "/theme mythras",
			args: []string{"mythras"},
		},
		{
			name: "quoted arg",
			in:   `/theme "obsidian"`,
			kind: CmdTheme,
			text: `/theme "obsidian"`,
			args: []string{"obsidian"},
		},
		{
			name: "quoted arg keeps spaces",
			in:   `/roll "3d6 + 2"`,
			kind: CmdRoll,
			text: `/roll "3d6 + 2"`,
			args: []string{"3d6 + 2"},
		},
		{
			name: "roll with expression",
			in:   "/roll 3d6+2",
			kind: CmdRoll,
			text: "/roll 3d6+2",
			args: []string{"3d6+2"},
		},
		{
			name: "save",
			in:   "/save",
			kind: CmdSave,
			text: "/save",
		},
		{
			name: "recap",
			in:   "/recap",
			kind: CmdRecap,
			text: "/recap",
		},
		{
			name: "meta",
			in:   "/meta",
			kind: CmdMeta,
			text: "/meta",
		},
		{
			name: "editor",
			in:   "/editor",
			kind: CmdEditor,
			text: "/editor",
		},
		{
			name: "clear",
			in:   "/clear",
			kind: CmdClear,
			text: "/clear",
		},
		{
			name: "help",
			in:   "/help",
			kind: CmdHelp,
			text: "/help",
		},
		{
			name: "quit",
			in:   "/quit",
			kind: CmdQuit,
			text: "/quit",
		},
		{
			name:    "unknown command",
			in:      "/frobnicate",
			wantErr: `unknown command "frobnicate"`,
		},
		{
			name:    "bare slash",
			in:      "/",
			wantErr: "empty command",
		},
		{
			name:    "theme without a name",
			in:      "/theme",
			wantErr: "themes: mythras, obsidian, parchment, terminal",
		},
		{
			name:    "roll without an expression",
			in:      "/roll",
			wantErr: "needs an expression",
		},
		{
			name:    "unterminated quote",
			in:      `/theme "mythras`,
			wantErr: "unterminated",
		},
		{
			name:    "trailing backslash",
			in:      `/editor foo\`,
			wantErr: "trailing backslash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInput(tt.in)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseInput(%q) = %+v, want error containing %q", tt.in, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseInput(%q): %v", tt.in, err)
			}
			if got.Kind != tt.kind {
				t.Errorf("Kind = %d, want %d", got.Kind, tt.kind)
			}
			if got.Text != tt.text {
				t.Errorf("Text = %q, want %q", got.Text, tt.text)
			}
			if len(got.Args) != len(tt.args) {
				t.Fatalf("Args = %q, want %q", got.Args, tt.args)
			}
			for i := range tt.args {
				if got.Args[i] != tt.args[i] {
					t.Errorf("Args[%d] = %q, want %q", i, got.Args[i], tt.args[i])
				}
			}
		})
	}
}

func TestUnknownCommandListsValidOnes(t *testing.T) {
	_, err := ParseInput("/wibble")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, name := range commandNames {
		if !strings.Contains(err.Error(), "/"+name) {
			t.Errorf("error should list /%s, got: %v", name, err)
		}
	}
}

func TestSplitTokens(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{name: "empty", in: "", want: nil},
		{name: "single word", in: "code", want: []string{"code"}},
		{name: "arguments", in: "code -w", want: []string{"code", "-w"}},
		{name: "double quoted", in: `"my editor" -f`, want: []string{"my editor", "-f"}},
		{name: "single quoted", in: `'my editor' -f`, want: []string{"my editor", "-f"}},
		{name: "quotes inside double", in: `"say \"hi\""`, want: []string{`say "hi"`}},
		{name: "backslash outside quotes", in: `a\ b`, want: []string{"a b"}},
		{name: "extra whitespace", in: "  a   b  ", want: []string{"a", "b"}},
		{name: "unterminated", in: `a "b`, wantErr: true},
		{name: "trailing escape", in: `a\`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitTokens(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitTokens(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitTokens(%q): %v", tt.in, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got %q, want %q", got, tt.want)
				}
			}
		})
	}
}
