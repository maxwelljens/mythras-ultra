package rules

import "fmt"

// Outcome is one side's finished test: the d100 roll, the (modified) skill
// value it was tested against, and the resulting grade. Both Opposed and
// Differential consume Outcomes; the harness records them verbatim.
type Outcome struct {
	Roll  int   `json:"roll"`
	Skill int   `json:"skill"`
	Grade Grade `json:"grade"`
}

// String renders "critical (12 vs 66)" for audit lines.
func (o Outcome) String() string {
	return fmt.Sprintf("%s (%d vs %d)", o.Grade, o.Roll, o.Skill)
}

// OpposedResult is the outcome of an Opposed Roll between two sides, A and B.
type OpposedResult struct {
	A          Outcome `json:"a"`
	B          Outcome `json:"b"`
	Winner     int     `json:"winner"`      // 0 = no winner, 1 = A, 2 = B
	Margin     int     `json:"margin"`      // difference in success levels (0-3)
	BothFailed bool    `json:"both_failed"` // neither side reached a success
	TieBreak   string  `json:"tie_break"`   // "grade", "roll" or ""
}

// Opposed resolves a contest where the better Level of Success wins and
// equal levels are broken by the highest dice roll still inside the success
// range; when both sides fail there is no winner and the caller re-rolls or
// narrates a fitting outcome (Mythras-Imperative.md, Skills › Opposed Rolls,
// L864; docs/rules/digest.md L499-502).
//
// Fumbles and Failures below any success leave BothFailed set: per the
// Differential matrix a contest where nobody reaches a Standard success
// grants no benefit to either side (L518-528).
func Opposed(a, b Outcome) OpposedResult {
	res := OpposedResult{A: a, B: b}
	aOK := a.Grade >= GradeSuccess
	bOK := b.Grade >= GradeSuccess

	switch {
	case aOK && !bOK:
		res.Winner, res.Margin, res.TieBreak = 1, int(a.Grade-b.Grade), "grade"
	case bOK && !aOK:
		res.Winner, res.Margin, res.TieBreak = 2, int(b.Grade-a.Grade), "grade"
	case aOK && bOK:
		switch {
		case a.Grade > b.Grade:
			res.Winner, res.Margin, res.TieBreak = 1, int(a.Grade-b.Grade), "grade"
		case b.Grade > a.Grade:
			res.Winner, res.Margin, res.TieBreak = 2, int(b.Grade-a.Grade), "grade"
		case a.Roll > b.Roll:
			res.Winner, res.Margin, res.TieBreak = 1, 0, "roll"
		case b.Roll > a.Roll:
			res.Winner, res.Margin, res.TieBreak = 2, 0, "roll"
		default:
			res.TieBreak = "roll" // identical rolls: true tie, Winner stays 0
		}
	default:
		res.BothFailed = true
	}
	return res
}

// Differential resolves a contest where both sides suffer their own results
// and the better side gains an advantage equal to the difference in success
// levels — but only when at least one side reached a Standard success; when
// nobody succeeds the exchange grants "No Benefit" to either side
// (Mythras-Imperative.md, Skills › Differential Rolls, L893-906 matrix).
// Equal levels are always "No Effect" (the roll tie-break of Opposed Rolls
// does not apply here). The returned OpposedResult reuses the shape of
// Opposed: Winner/Margin express "wins N Levels of Success".
func Differential(a, b Outcome) OpposedResult {
	res := Opposed(a, b)
	// L518-520: equal levels of success produce no effect, and the advantage
	// needs at least one Standard success on either side; otherwise the
	// matrix reads "No Benefit" regardless of the grades' ordering.
	if a.Grade == b.Grade || (a.Grade < GradeSuccess && b.Grade < GradeSuccess) {
		res.Winner, res.Margin, res.TieBreak = 0, 0, ""
	}
	return res
}

// EqualiseSkillsOver100 implements the opposed/differential handicap for
// skills over 100%: the highest-skilled participant subtracts (skill − 100)
// from every participant's skill including their own
// (Mythras-Imperative.md, Skills, L908; docs/rules/digest.md L530-532).
// Results are clamped at 0.
func EqualiseSkillsOver100(a, b int) (int, int) {
	high := a
	if b > high {
		high = b
	}
	if high <= 100 {
		return a, b
	}
	d := high - 100
	a, b = a-d, b-d
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	return a, b
}
