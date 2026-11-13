package dice

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Parsing limits: at most [maxTerms] +/- separated terms, each with at most
// [maxDice] dice of at most [maxSides] sides.
const (
	maxTerms = 8
	maxDice  = 1000
	maxSides = 1000
)

// Sentinels returned (wrapped, with the offending expression as context) when
// a dice expression cannot be parsed. Callers can branch on them with
// errors.Is:
//
//	if errors.Is(err, dice.ErrTooManyDice) {
//		// ask the GM for a saner expression
//	}
var (
	// ErrInvalidExpr reports a malformed dice expression: empty input, an
	// unknown character, a missing or trailing operator, zero dice or zero
	// sides, a bare "%", more than 8 terms, or sides/modifiers out of range.
	ErrInvalidExpr = errors.New("dice: invalid expression")

	// ErrTooManyDice reports a term asking for more than 1000 dice, or a
	// dice count too large to represent.
	ErrTooManyDice = errors.New("dice: too many dice")
)

// term is one parsed expression term: either a dice term (count > 0) or a
// plain integer modifier (count == 0, value in mod). sign is +1 or -1.
type term struct {
	sign  int
	count int
	sides int
	mod   int
}

// parse turns a dice expression into its terms. It validates the grammar
// (whitespace-tolerant, terms separated by '+' or '-', optional leading sign)
// and the limits above; errors wrap [ErrInvalidExpr] or [ErrTooManyDice] and
// quote the original expression.
func parse(expr string) ([]term, error) {
	s := normalise(expr)
	if s == "" {
		return nil, fmt.Errorf("%w: expression is empty", ErrInvalidExpr)
	}

	var (
		terms []term
		t     term
		i     int
		err   error
	)
	// The first term may carry a leading sign.
	sign := 1
	if c := s[i]; c == '+' || c == '-' {
		if c == '-' {
			sign = -1
		}
		i++
		if i == len(s) {
			return nil, fmt.Errorf("%w: %q: trailing operator", ErrInvalidExpr, expr)
		}
	}
	t, i, err = parseTerm(s, i, sign, expr)
	if err != nil {
		return nil, err
	}
	terms = append(terms, t)

	// Every following term must be introduced by an explicit separator.
	for i < len(s) {
		if c := s[i]; c != '+' && c != '-' {
			return nil, fmt.Errorf("%w: %q: expected %q or %q between terms", ErrInvalidExpr, expr, "+", "-")
		}
		sign = 1
		if s[i] == '-' {
			sign = -1
		}
		i++
		if i == len(s) {
			return nil, fmt.Errorf("%w: %q: trailing operator", ErrInvalidExpr, expr)
		}
		t, i, err = parseTerm(s, i, sign, expr)
		if err != nil {
			return nil, err
		}
		terms = append(terms, t)
		if len(terms) > maxTerms {
			return nil, fmt.Errorf("%w: %q: at most %d terms allowed, got %d", ErrInvalidExpr, expr, maxTerms, len(terms))
		}
	}
	return terms, nil
}

// parseTerm parses the term starting at s[i] (after its sign has been
// consumed) and returns it with the index just past the term.
func parseTerm(s string, i, sign int, expr string) (term, int, error) {
	count, j := scanDigits(s, i)
	if j < len(s) && s[j] == 'd' {
		// Dice term: [N]dM, or [N]d% as shorthand for [N]d100.
		j++
		sides := 100 // d% / 2d% shorthand for the percentile die
		if j < len(s) && s[j] == '%' {
			j++
		} else {
			text, k := scanDigits(s, j)
			if text == "" {
				return term{}, i, fmt.Errorf("%w: %q: missing number of sides after %q", ErrInvalidExpr, expr, "d")
			}
			j = k
			var err error
			sides, err = strconv.Atoi(text)
			if err != nil {
				return term{}, i, fmt.Errorf("%w: %q: number of sides out of range", ErrInvalidExpr, expr)
			}
		}
		n := 1 // "dM" alone means "1dM"
		if count != "" {
			var err error
			n, err = strconv.Atoi(count)
			if err != nil {
				return term{}, i, fmt.Errorf("%w: %q: number of dice out of range", ErrTooManyDice, expr)
			}
		}
		switch {
		case n < 1:
			return term{}, i, fmt.Errorf("%w: %q: number of dice must be at least 1", ErrInvalidExpr, expr)
		case n > maxDice:
			return term{}, i, fmt.Errorf("%w: %q: %d dice exceeds the limit of %d", ErrTooManyDice, expr, n, maxDice)
		case sides < 1:
			return term{}, i, fmt.Errorf("%w: %q: number of sides must be at least 1", ErrInvalidExpr, expr)
		case sides > maxSides:
			return term{}, i, fmt.Errorf("%w: %q: %d sides exceeds the limit of %d", ErrInvalidExpr, expr, sides, maxSides)
		}
		return term{sign: sign, count: n, sides: sides}, j, nil
	}
	if count == "" {
		r, _ := utf8.DecodeRuneInString(s[i:])
		return term{}, i, fmt.Errorf("%w: %q: unexpected character %q", ErrInvalidExpr, expr, string(r))
	}
	if j < len(s) && s[j] == '%' {
		return term{}, i, fmt.Errorf("%w: %q: %q must follow a die (write %q or %q)",
			ErrInvalidExpr, expr, "%", "d%", "2d%")
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return term{}, i, fmt.Errorf("%w: %q: integer modifier out of range", ErrInvalidExpr, expr)
	}
	return term{sign: sign, mod: n}, j, nil
}

// canonical renders terms back into the canonical expression string: no
// whitespace, lowercase "d"/"%", dice terms in written order, and all integer
// modifiers folded into one trailing term (omitted when the net is zero).
func canonical(terms []term) string {
	var b strings.Builder
	net := 0
	for _, t := range terms {
		if t.count == 0 {
			net += t.sign * t.mod
			continue
		}
		switch {
		case t.sign < 0:
			b.WriteByte('-')
		case b.Len() > 0:
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(t.count))
		b.WriteString("d")
		b.WriteString(strconv.Itoa(t.sides))
	}
	switch {
	case net > 0:
		if b.Len() > 0 {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(net))
	case net < 0:
		b.WriteByte('-')
		b.WriteString(strconv.Itoa(-net))
	}
	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}

// normalise prepares an expression for parsing: lowercases "d"/"D" and drops
// all whitespace.
func normalise(expr string) string {
	return strings.ToLower(strings.Join(strings.Fields(expr), ""))
}

// scanDigits returns the run of ASCII digits starting at s[i] and the index
// just past it (equal to i when there is no digit).
func scanDigits(s string, i int) (string, int) {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	return s[i:j], j
}
