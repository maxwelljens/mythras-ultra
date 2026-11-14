package dice

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
)

// randBinarySeedLen is the length of the session-seed prefix every Rand
// encoding starts with; the generator state follows it verbatim.
const randBinarySeedLen = 8

// MarshalBinary implements encoding.BinaryMarshaler: it encodes the Rand's
// session seed followed by the live state of the underlying PCG generator,
// so a persisted and restored Rand continues the exact same value sequence —
// no replayed draws and no skipped draws.
//
// math/rand/v2's *Rand has no binary form of its own, so the encoding is
// built from the *PCG source the Rand draws from (which does implement
// encoding.BinaryMarshaler) plus the seed [Rand.Seed] reports, keeping
// Seed() intact after a restore.
func (r *Rand) MarshalBinary() ([]byte, error) {
	if r == nil || r.pcg == nil {
		return nil, errors.New("dice: cannot marshal an uninitialised Rand")
	}
	src, err := r.pcg.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("dice: marshal Rand generator: %w", err)
	}
	out := make([]byte, 0, randBinarySeedLen+len(src))
	out = binary.LittleEndian.AppendUint64(out, uint64(r.seed))
	out = append(out, src...)
	return out, nil
}

// UnmarshalRandBinary restores a Rand previously encoded by
// [Rand.MarshalBinary]. The restored generator continues the sequence
// exactly where the encoded one stopped, and [Rand.Seed] reports the
// original session seed, so Save → Load of a session never replays or skips
// a single draw.
//
// Errors report data too short to hold a seed, or a generator encoding this
// build does not recognise; no randomness is consumed on error.
func UnmarshalRandBinary(data []byte) (*Rand, error) {
	if len(data) < randBinarySeedLen {
		return nil, fmt.Errorf("dice: Rand encoding too short (%d bytes, want at least %d)", len(data), randBinarySeedLen)
	}
	pcg := new(rand.PCG)
	if err := pcg.UnmarshalBinary(data[randBinarySeedLen:]); err != nil {
		return nil, fmt.Errorf("dice: Rand generator encoding: %w", err)
	}
	return &Rand{
		seed: int64(binary.LittleEndian.Uint64(data[:randBinarySeedLen])),
		src:  rand.New(pcg),
		pcg:  pcg,
	}, nil
}
