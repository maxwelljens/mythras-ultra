package tui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// CommandKind identifies what a parsed input line wants the app to do. Plain
// text becomes a chat message; slash commands either act locally in the TUI
// (ic, ooc, theme, editor, clear, help, quit) or are forwarded to the harness
// (roll, save, recap, meta), which wires them to engine moves later.
type CommandKind int

const (
	// CmdMessage is plain chat text, forwarded to the driver as typed.
	CmdMessage CommandKind = iota
	// CmdIC switches the input line to the in-character channel.
	CmdIC
	// CmdOOC switches the input line to the out-of-character channel.
	CmdOOC
	// CmdTheme applies a named theme ("/theme <name>").
	CmdTheme
	// CmdEditor opens the input line in the $VISUAL editor.
	CmdEditor
	// CmdClear clears the local transcript view.
	CmdClear
	// CmdHelp lists commands and keybindings in the transcript.
	CmdHelp
	// CmdQuit exits the app.
	CmdQuit
	// CmdRoll forwards "/roll <expr>" to the harness (wired to the engine later).
	CmdRoll
	// CmdSave forwards "/save" to the harness (wired to persistence later).
	CmdSave
	// CmdRecap forwards "/recap" to the harness (wired later).
	CmdRecap
	// CmdMeta forwards "/meta" to the harness (wired later).
	CmdMeta
)

// Command is the result of ParseInput.
type Command struct {
	// Kind selects how the app dispatches the input.
	Kind CommandKind
	// Text is the input exactly as the player typed it. It is the payload
	// sent to the driver for messages and for harness commands (/roll, …).
	Text string
	// Args holds the arguments after the command name, split on whitespace
	// with shell-style quoting honoured: /theme "obsidian" → ["obsidian"].
	Args []string
}

// commandNames lists every slash command, in help-output order.
var commandNames = []string{
	"ic", "ooc", "theme", "editor", "clear", "help", "quit",
	"roll", "save", "recap", "meta",
}

// commandKinds maps command names (lower case) to their kind.
var commandKinds = map[string]CommandKind{
	"ic":     CmdIC,
	"ooc":    CmdOOC,
	"theme":  CmdTheme,
	"editor": CmdEditor,
	"clear":  CmdClear,
	"help":   CmdHelp,
	"quit":   CmdQuit,
	"roll":   CmdRoll,
	"save":   CmdSave,
	"recap":  CmdRecap,
	"meta":   CmdMeta,
}

// validCommands renders the command list used in error messages.
func validCommands() string {
	quoted := make([]string, 0, len(commandNames))
	for _, name := range commandNames {
		quoted = append(quoted, "/"+name)
	}
	return strings.Join(quoted, " ")
}

// ParseInput parses one input line. Text without a leading "/" is a plain
// chat message (Kind CmdMessage, Text unchanged). A leading "/" selects a
// slash command, matched case-insensitively; unknown commands and commands
// missing a required argument return an error listing the valid forms. The
// returned Text always preserves the input as typed, so harness commands can
// be forwarded verbatim.
func ParseInput(s string) (Command, error) {
	if !strings.HasPrefix(s, "/") {
		return Command{Kind: CmdMessage, Text: s}, nil
	}

	body := strings.TrimPrefix(s, "/")
	name := body
	rest := ""
	if i := strings.IndexFunc(body, unicode.IsSpace); i >= 0 {
		name, rest = body[:i], strings.TrimLeftFunc(body[i:], unicode.IsSpace)
	}
	name = strings.ToLower(name)
	if name == "" {
		return Command{}, fmt.Errorf("empty command — commands: %s", validCommands())
	}

	args, err := splitTokens(rest)
	if err != nil {
		return Command{}, fmt.Errorf("/%s: %w", name, err)
	}

	kind, ok := commandKinds[name]
	if !ok {
		return Command{}, fmt.Errorf("unknown command %q — commands: %s", name, validCommands())
	}

	switch kind {
	case CmdTheme:
		if len(args) == 0 {
			return Command{}, fmt.Errorf("/theme needs a name — themes: %s",
				strings.Join(Names(), ", "))
		}
	case CmdRoll:
		if len(args) == 0 {
			return Command{}, errors.New("/roll needs an expression, e.g. /roll 3d6+2")
		}
	}

	return Command{Kind: kind, Text: s, Args: args}, nil
}

// splitTokens splits s on whitespace, honouring single and double quotes and
// backslash escapes so values like `code -w` or `'my editor'` parse safely.
// It never invokes a shell: the result feeds exec.Command directly. An
// unterminated quote or trailing escape is an error.
func splitTokens(s string) ([]string, error) {
	var (
		out     []string
		cur     strings.Builder
		quote   rune // 0 outside quotes, otherwise ' or "
		escaped bool
	)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		return nil, fmt.Errorf("trailing backslash in %q", s)
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote in %q", quote, s)
	}
	flush()
	return out, nil
}
