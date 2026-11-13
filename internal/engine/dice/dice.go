// Package dice provides the single, seedable source of randomness for
// Mythras Ultra.
//
// Determinism contract: all mechanical chance in the game flows through a
// [Rand] created from an explicit seed. A Rand created by [NewRand] produces
// the identical sequence of values for the identical sequence of calls, on
// every run and platform. [NewRandFromEntropy] derives its seed from the
// operating system and returns it, so the session seed can be persisted:
// replaying a session with the same seed reproduces every roll exactly.
// Every roll is captured as a [Roll] value — canonical expression, each
// individual die, the net modifier, and the total — which the session log
// keeps as the audit trail for mechanical outcomes.
package dice

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// Rand is the single source of randomness for the game: every mechanical
// outcome that needs chance (dice rolls, hit locations, and so on) must flow
// through an explicit *Rand rather than any global source, so that a session
// seed is sufficient to replay it.
//
// A Rand is not safe for concurrent use; give each session its own.
type Rand struct {
	seed int64
	src  *rand.Rand
}

// NewRand returns a Rand that is fully determined by seed. The same seed
// yields the same sequence of results for the same sequence of calls,
// which is what makes a persisted session seed replayable.
func NewRand(seed int64) *Rand {
	// The two PCG seeds are derived from the caller's seed so adjacent seeds
	// produce independent streams; PCG accepts any pair of uint64 values.
	return &Rand{
		seed: seed,
		src:  rand.New(rand.NewPCG(uint64(seed), uint64(seed)+1)),
	}
}

// NewRandFromEntropy returns a Rand seeded from the operating system's
// cryptographic entropy source, along with the seed it used. Callers must
// persist the returned seed: the seed, not the entropy, is what makes the
// session replayable.
func NewRandFromEntropy() (*Rand, int64) {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		// Unreachable: crypto/rand.Read is documented never to return an error
		// (it terminates the program on entropy failure). The clock fallback
		// keeps the contract intact — the seed is returned and persisted, so
		// replays stay deterministic however the seed was obtained.
		seed := time.Now().UnixNano()
		return NewRand(seed), seed
	}
	seed := int64(binary.LittleEndian.Uint64(b[:]))
	return NewRand(seed), seed
}

// Seed returns the seed this Rand was created with, for persistence and
// replay of the session.
func (r *Rand) Seed() int64 {
	return r.seed
}

// D rolls a single die with the given number of sides and returns a value in
// 1..sides. If sides < 1 there is no meaningful roll, so D returns 1.
func (r *Rand) D(sides int) int {
	if sides < 1 {
		return 1
	}
	return r.src.IntN(sides) + 1
}

// D100 rolls a single percentile die, returning a value in 1..100.
func (r *Rand) D100() int {
	return r.D(100)
}

// Intn returns a pseudo-random int in [0, n). If n < 1, Intn returns 0.
func (r *Rand) Intn(n int) int {
	if n < 1 {
		return 0
	}
	return r.src.IntN(n)
}

// Die records a single die of a roll.
type Die struct {
	// Sides is the number of faces of the die that was rolled.
	Sides int
	// Value is the result of the roll, always in 1..Sides.
	Value int
}

// Roll is the audit record of one dice expression: the canonicalised
// expression, every individual die in roll order, the net additive modifier,
// and the total. It is what the session log captures for each roll so a
// session can be reviewed and replayed.
type Roll struct {
	// Expr is the canonicalised expression, e.g. "2d6+3".
	Expr string
	// Dice holds every individual die, in the order they were rolled.
	Dice []Die
	// Modifier is the net additive modifier after folding integer terms.
	Modifier int
	// Total is the sum of the dice (honouring each term's sign) plus
	// Modifier; it is never clamped.
	Total int
}

// Roll parses expr (see the package documentation for the grammar), rolls
// every die it names, and returns the audit record. On error the returned
// Roll is the zero value; the error wraps [ErrInvalidExpr] or
// [ErrTooManyDice] and no randomness is consumed, so a failed roll leaves the
// sequence aligned for replay.
func (r *Rand) Roll(expr string) (Roll, error) {
	terms, err := parse(expr)
	if err != nil {
		return Roll{}, err
	}
	roll := Roll{Expr: canonical(terms)}
	for _, t := range terms {
		if t.count == 0 {
			roll.Modifier += t.sign * t.mod
			continue
		}
		for i := 0; i < t.count; i++ {
			v := r.D(t.sides)
			roll.Dice = append(roll.Dice, Die{Sides: t.sides, Value: v})
			if t.sign < 0 {
				roll.Total -= v
			} else {
				roll.Total += v
			}
		}
	}
	roll.Total += roll.Modifier
	return roll, nil
}

// String renders the roll for the audit trail, for example
// "2d6+3 → [4, 2]+3 = 9". The dice are shown as the flat pool in roll order;
// Total is authoritative and honours the sign of each term in Expr.
func (r Roll) String() string {
	vals := make([]string, 0, len(r.Dice))
	for _, d := range r.Dice {
		vals = append(vals, strconv.Itoa(d.Value))
	}
	mod := ""
	switch {
	case r.Modifier > 0:
		mod = "+" + strconv.Itoa(r.Modifier)
	case r.Modifier < 0:
		mod = strconv.Itoa(r.Modifier)
	}
	return r.Expr + " → [" + strings.Join(vals, ", ") + "]" + mod + " = " + strconv.Itoa(r.Total)
}
