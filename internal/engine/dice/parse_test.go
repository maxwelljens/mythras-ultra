package dice

import (
	"errors"
	"slices"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		expr string
		want []term
	}{
		{expr: "1d100", want: []term{{sign: 1, count: 1, sides: 100}}},
		{expr: "d20", want: []term{{sign: 1, count: 1, sides: 20}}},
		{expr: "2d6+3", want: []term{{sign: 1, count: 2, sides: 6}, {sign: 1, mod: 3}}},
		{expr: "1d8+1d6-2", want: []term{{sign: 1, count: 1, sides: 8}, {sign: 1, count: 1, sides: 6}, {sign: -1, mod: 2}}},
		{expr: "3d6", want: []term{{sign: 1, count: 3, sides: 6}}},
		{expr: "d%", want: []term{{sign: 1, count: 1, sides: 100}}},
		{expr: "1d%", want: []term{{sign: 1, count: 1, sides: 100}}},
		{expr: "2d%", want: []term{{sign: 1, count: 2, sides: 100}}},
		{expr: "1d100+10", want: []term{{sign: 1, count: 1, sides: 100}, {sign: 1, mod: 10}}},
		{expr: " 2 d 6 + 3 ", want: []term{{sign: 1, count: 2, sides: 6}, {sign: 1, mod: 3}}},
		{expr: "2D6", want: []term{{sign: 1, count: 2, sides: 6}}},
		{expr: "D%", want: []term{{sign: 1, count: 1, sides: 100}}},
		{expr: "+2d6", want: []term{{sign: 1, count: 2, sides: 6}}},
		{expr: "-2d6", want: []term{{sign: -1, count: 2, sides: 6}}},
		{expr: "d6-1d4", want: []term{{sign: 1, count: 1, sides: 6}, {sign: -1, count: 1, sides: 4}}},
		{expr: "5", want: []term{{sign: 1, mod: 5}}},
		{expr: "-5", want: []term{{sign: -1, mod: 5}}},
		{expr: "+5", want: []term{{sign: 1, mod: 5}}},
		{expr: "1d1", want: []term{{sign: 1, count: 1, sides: 1}}},
		{expr: "1000d1000", want: []term{{sign: 1, count: 1000, sides: 1000}}},
		{
			expr: "1d6+1d6+1d6+1d6+1d6+1d6+1d6+1d6",
			want: []term{
				{sign: 1, count: 1, sides: 6}, {sign: 1, count: 1, sides: 6},
				{sign: 1, count: 1, sides: 6}, {sign: 1, count: 1, sides: 6},
				{sign: 1, count: 1, sides: 6}, {sign: 1, count: 1, sides: 6},
				{sign: 1, count: 1, sides: 6}, {sign: 1, count: 1, sides: 6},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := parse(tt.expr)
			if err != nil {
				t.Fatalf("parse(%q) = error %v", tt.expr, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parse(%q) = %+v, want %+v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestCanonical(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{expr: "2 d 6 + 3", want: "2d6+3"},
		{expr: "2D6", want: "2d6"},
		{expr: "d20", want: "1d20"},
		{expr: "d%", want: "1d100"},
		{expr: "1d%", want: "1d100"},
		{expr: "2d%", want: "2d100"},
		{expr: "2d6+3-1", want: "2d6+2"},
		{expr: "2d6 - 3 + 1", want: "2d6-2"},
		{expr: "1d100+10", want: "1d100+10"},
		{expr: "1d8+1d6-2", want: "1d8+1d6-2"},
		{expr: "1d6+5-2+1d4", want: "1d6+1d4+3"},
		{expr: "+5", want: "5"},
		{expr: "-5", want: "-5"},
		{expr: "0", want: "0"},
		{expr: "-2d6", want: "-2d6"},
		{expr: "  d6 ", want: "1d6"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			terms, err := parse(tt.expr)
			if err != nil {
				t.Fatalf("parse(%q) = error %v", tt.expr, err)
			}
			got := canonical(terms)
			if got != tt.want {
				t.Errorf("canonical(parse(%q)) = %q, want %q", tt.expr, got, tt.want)
			}
			// Canonicalisation must reach a fixpoint: folding may regroup
			// terms, but re-canonicalising the canonical form is stable.
			again, err := parse(got)
			if err != nil {
				t.Fatalf("parse(%q) = error %v", got, err)
			}
			if re := canonical(again); re != got {
				t.Errorf("canonical(parse(%q)) = %q, want %q", got, re, got)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		expr string
		want error
	}{
		{expr: "", want: ErrInvalidExpr},
		{expr: "   \t ", want: ErrInvalidExpr},
		{expr: "+", want: ErrInvalidExpr},
		{expr: "-", want: ErrInvalidExpr},
		{expr: "2d6+", want: ErrInvalidExpr},
		{expr: "2d6-", want: ErrInvalidExpr},
		{expr: "2d6++3", want: ErrInvalidExpr},
		{expr: "2d6-+3", want: ErrInvalidExpr},
		{expr: "+-2d6", want: ErrInvalidExpr},
		{expr: "d", want: ErrInvalidExpr},
		{expr: "2d", want: ErrInvalidExpr},
		{expr: "0d6", want: ErrInvalidExpr},
		{expr: "d0", want: ErrInvalidExpr},
		{expr: "1d1001", want: ErrInvalidExpr},
		{expr: "1d1000000", want: ErrInvalidExpr},
		{expr: "9999d6", want: ErrTooManyDice},
		{expr: "1001d6", want: ErrTooManyDice},
		{expr: "2d6&3", want: ErrInvalidExpr},
		{expr: "2d6d6", want: ErrInvalidExpr},
		{expr: "2d6x", want: ErrInvalidExpr},
		{expr: "5%", want: ErrInvalidExpr},
		{expr: "%", want: ErrInvalidExpr},
		{expr: "1d6+1d6+1d6+1d6+1d6+1d6+1d6+1d6+1d6", want: ErrInvalidExpr},
		{expr: "1d99999999999999999999", want: ErrInvalidExpr},
		{expr: "99999999999999999999d6", want: ErrTooManyDice},
		{expr: "1d6+99999999999999999999", want: ErrInvalidExpr},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := parse(tt.expr)
			if !errors.Is(err, tt.want) {
				t.Errorf("parse(%q) error = %v, want wrapping %v", tt.expr, err, tt.want)
			}
		})
	}
}
