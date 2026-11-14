package tui

import (
	"testing"
)

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
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
