package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// editorFinishedMsg carries the result of an external editor run. path is the
// temporary file the editor was pointed at; err reports the editor's exit.
type editorFinishedMsg struct {
	err  error
	path string
}

// editorArgv resolves the editor command from the environment: $VISUAL, then
// $EDITOR, then "vi". The value may carry arguments (VISUAL="code -w"); it is
// split quote-aware and never passed through a shell. getenv is injected so
// tests can supply a fake environment.
func editorArgv(getenv func(string) string) ([]string, error) {
	src := "VISUAL"
	spec := strings.TrimSpace(getenv("VISUAL"))
	if spec == "" {
		src = "EDITOR"
		spec = strings.TrimSpace(getenv("EDITOR"))
	}
	if spec == "" {
		return []string{"vi"}, nil
	}
	argv, err := splitTokens(spec)
	if err != nil {
		return nil, fmt.Errorf("$%s=%q: %w", src, spec, err)
	}
	if len(argv) == 0 {
		return []string{"vi"}, nil
	}
	return argv, nil
}

// openEditor opens the current input line in the external editor and returns
// the command that runs it. The edited text is loaded back into the input
// line on return — it is not sent automatically; the player reviews it and
// presses enter. Environment and temp-file failures become transcript error
// lines, never panics.
func (a *App) openEditor() tea.Cmd {
	argv, err := editorArgv(os.Getenv)
	if err != nil {
		a.errorLine(err)
		return nil
	}
	f, err := os.CreateTemp("", "myul-input-*.md")
	if err != nil {
		a.errorLine(fmt.Errorf("open editor: create temp file: %w", err))
		return nil
	}
	path := f.Name()
	text := a.input.Value()
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n" // editors warn about a missing final newline
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		a.errorLine(fmt.Errorf("open editor: write temp file: %w", err))
		return nil
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		a.errorLine(fmt.Errorf("open editor: close temp file: %w", err))
		return nil
	}
	cmd := exec.Command(argv[0], append(argv[1:], path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, path: path}
	})
}

// finishEditor loads an editor result back into the input line and removes
// the temporary file. A non-zero editor exit or an unreadable file becomes a
// transcript error line and the input line is left untouched.
func (a *App) finishEditor(m editorFinishedMsg) {
	data, readErr := os.ReadFile(m.path)
	rmErr := os.Remove(m.path)

	switch {
	case m.err != nil:
		a.errorLine(fmt.Errorf("editor failed: %w", m.err))
	case readErr != nil:
		a.errorLine(fmt.Errorf("read edited text: %w", readErr))
	default:
		// Undo exactly the one trailing newline openEditor added.
		text := strings.TrimSuffix(string(data), "\n")
		text = strings.TrimSuffix(text, "\r")
		a.input.SetValue(text)
		a.histIdx = len(a.history)
	}
	if rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		a.errorLine(fmt.Errorf("clean up %s: %w", m.path, rmErr))
	}
}
