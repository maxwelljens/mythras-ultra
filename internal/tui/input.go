package tui

import (
	"fmt"
	"strings"
	"unicode"
)

// splitTokens splits s on whitespace, honouring single and double quotes and
// backslash escapes so values like `code -w` or `'my editor'` parse safely.
// It never invokes a shell: the result feeds exec.Command directly. An
// unterminated quote or trailing escape is an error.
//
// The input line itself never parses commands (typed text is always a chat
// message — see App.submit): this only shapes $VISUAL/EDITOR argv for the
// editor action.
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
