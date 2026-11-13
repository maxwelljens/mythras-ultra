package decide

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"slices"
	"sync"
)

// cacheMaxEntries bounds the session cache; once full, the oldest verdict is
// evicted first so memory stays fixed at roughly 256 entries.
const cacheMaxEntries = 256

// cache is a bounded, thread-safe FIFO memo of verdicts keyed by a hash of
// the question. Entries never expire: a verdict only makes sense while the
// session asks the same questions, and a process-scoped bound is enough —
// simplest thing that works. No clock to inject, no TTL to test; revisit only
// if callers start sharing verdicts across processes or long-lived stores.
type cache struct {
	mu      sync.Mutex
	entries map[string]Verdict
	order   []string
	max     int
}

// newCache returns an empty cache holding at most max entries.
func newCache(max int) *cache {
	return &cache{entries: make(map[string]Verdict), max: max}
}

// get returns the verdict cached for key with [Verdict.Cached] set.
func (c *cache) get(key string) (Verdict, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	if !ok {
		return Verdict{}, false
	}
	v.Cached = true
	return v, true
}

// put stores v (a fresh verdict) under key and evicts the oldest entries
// beyond the cache's bound. Re-putting a known key keeps its original
// position in the eviction order.
func (c *cache) put(key string, v Verdict) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; ok {
		return
	}
	c.entries[key] = v
	c.order = append(c.order, key)
	for len(c.order) > c.max {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

// cacheKey hashes (State, Question, Options) with length-prefixed fields so
// distinct questions cannot collide through field-boundary ambiguity. The
// options are sorted on a copy so their order does not change the key, and
// hashing keeps oversized states from doubling as map keys.
func cacheKey(q Question) string {
	opts := slices.Clone(q.Options)
	slices.Sort(opts)
	var buf []byte
	write := func(s string) {
		var l [8]byte
		binary.LittleEndian.PutUint64(l[:], uint64(len(s)))
		buf = append(buf, l[:]...)
		buf = append(buf, s...)
	}
	write(q.State)
	write(q.Question)
	for _, o := range opts {
		write(o)
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
