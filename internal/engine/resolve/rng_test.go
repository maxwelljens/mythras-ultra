package resolve

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

// rollNTotals applies n 1d100 moves and returns the totals.
func rollNTotals(t *testing.T, s *State, n int) []int {
	t.Helper()
	totals := make([]int, 0, n)
	for i := 0; i < n; i++ {
		res, err := Apply(s, RollDice{Expr: "1d100", Reason: "continuity probe"})
		if err != nil {
			t.Fatalf("roll %d: %v", i, err)
		}
		if res.Audit == nil || len(res.Audit.Rolls) != 1 {
			t.Fatalf("roll %d: audit = %+v", i, res.Audit)
		}
		totals = append(totals, res.Audit.Rolls[0].Total)
	}
	return totals
}

// TestRNGContinuityAcrossSerialization is the resumption contract: roll
// through a save (JSON serialisation of the whole State), restore, keep
// rolling — the combined sequence must equal one uninterrupted run of the
// same seed. No replayed draw, no skipped draw.
func TestRNGContinuityAcrossSerialization(t *testing.T) {
	const seed = 20261114
	const before, after = 7, 11

	reference := rollNTotals(t, NewState("resume", seed), before+after)

	s := NewState("resume", seed)
	head := rollNTotals(t, s, before)

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := restored.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	tail := rollNTotals(t, &restored, after)

	combined := append(slices.Clone(head), tail...)
	if !slices.Equal(combined, reference) {
		t.Errorf("resumed sequence diverged from an uninterrupted run:\nrestored: %v\nreference: %v", combined, reference)
	}
	// The continuation is genuinely mid-stream, not a restart from the
	// seed: tail differs from reference[:after].
	if slices.Equal(tail, reference[:after]) {
		t.Error("restored session replayed the seed's first draws instead of continuing")
	}
	// And the restored state carries the stream forward for its own save.
	if string(restored.RNGState) == "" || reflect.DeepEqual(restored.RNGState, s.RNGState) {
		t.Error("continuing rolls must advance the persisted RNGState")
	}
}

// TestRNGContinuitySameSeedSameSequence documents determinism at the state
// level: two sessions with one seed produce identical roll sequences, and
// the restored copy of one produces the same continuation.
func TestRNGContinuitySameSeedSameSequence(t *testing.T) {
	const seed = 7
	a := NewState("dup", seed)
	b := NewState("dup", seed)
	if !slices.Equal(rollNTotals(t, a, 6), rollNTotals(t, b, 6)) {
		t.Fatal("same seed produced different sequences")
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c State
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !slices.Equal(rollNTotals(t, &c, 6), rollNTotals(t, b, 6)) {
		t.Error("restored session diverged from a fresh session at the same position")
	}
}
