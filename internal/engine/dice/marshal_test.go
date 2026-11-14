package dice

import (
	"bytes"
	"testing"
)

// TestRandMarshalRoundTrip asserts the resumption contract: a marshalled and
// restored Rand continues the exact value sequence of the original — no
// replayed and no skipped draws — and still reports its session seed.
func TestRandMarshalRoundTrip(t *testing.T) {
	r := NewRand(42)
	for i := 0; i < 7; i++ { // advance the stream before saving
		r.D100()
	}
	encoded, err := r.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	restored, err := UnmarshalRandBinary(encoded)
	if err != nil {
		t.Fatalf("UnmarshalRandBinary: %v", err)
	}
	if got, want := restored.Seed(), int64(42); got != want {
		t.Errorf("restored Seed() = %d, want %d", got, want)
	}
	var continuation []int
	for i := 0; i < 25; i++ {
		want, got := r.D100(), restored.D100()
		if want != got {
			t.Fatalf("draw %d: restored = %d, original = %d (sequence diverged)", i, got, want)
		}
		continuation = append(continuation, got)
	}

	// The encoding is self-contained: a second unmarshal of the same bytes
	// reproduces the same continuation.
	again, err := UnmarshalRandBinary(encoded)
	if err != nil {
		t.Fatalf("UnmarshalRandBinary (second): %v", err)
	}
	for i, want := range continuation {
		if got := again.D100(); got != want {
			t.Fatalf("second restore draw %d = %d, want %d", i, got, want)
		}
	}
}

// TestRandMarshalSeedOnlyReplay checks the encoding distinguishes stream
// position from seed: a fresh Rand with the same seed must differ from a
// restored mid-stream Rand, otherwise persistence would silently replay.
func TestRandMarshalSeedOnlyReplay(t *testing.T) {
	r := NewRand(7)
	for i := 0; i < 5; i++ {
		r.D(20)
	}
	encoded, err := r.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	restored, err := UnmarshalRandBinary(encoded)
	if err != nil {
		t.Fatalf("UnmarshalRandBinary: %v", err)
	}
	fresh := NewRand(7)
	same := true
	for i := 0; i < 5; i++ {
		if restored.D100() != fresh.D100() {
			same = false
			break
		}
	}
	if same {
		t.Error("restored Rand replayed the first five draws of its seed instead of continuing mid-stream")
	}
}

// TestUnmarshalRandBinaryErrors covers malformed encodings: too-short data
// and a generator section this build does not recognise must error, not panic.
func TestUnmarshalRandBinaryErrors(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"seed prefix only", make([]byte, 8)},
		{"bad generator magic", append(make([]byte, 8), "nope"...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UnmarshalRandBinary(tc.data)
			if err == nil {
				t.Fatalf("UnmarshalRandBinary(%s) = %v, want error", tc.name, got)
			}
		})
	}
	// An uninitialised Rand refuses to marshal rather than panic.
	var nilRand *Rand
	if _, err := nilRand.MarshalBinary(); err == nil {
		t.Error("(*Rand)(nil).MarshalBinary() = nil error, want error")
	}
}

// TestRandEncodingStable checks the encoding is byte-stable for equal
// stream positions (persisted fixtures stay comparable across saves).
func TestRandEncodingStable(t *testing.T) {
	marshal := func() []byte {
		r := NewRand(1234)
		r.D100()
		b, err := r.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}
		return b
	}
	if !bytes.Equal(marshal(), marshal()) {
		t.Error("equal stream positions produced different encodings")
	}
}
