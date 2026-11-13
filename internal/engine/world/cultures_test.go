package world

import (
	"errors"
	"testing"
)

func TestDefaultCultures(t *testing.T) {
	c := DefaultCultures()
	wantKeys := []string{"barbarian", "civilised", "nomadic", "primitive"}
	if len(c) != len(wantKeys) {
		t.Fatalf("DefaultCultures has %d entries, want %d", len(c), len(wantKeys))
	}
	for _, k := range wantKeys {
		info, ok := c[k]
		if !ok {
			t.Errorf("missing culture %q", k)
			continue
		}
		if info.Name == "" || info.Description == "" {
			t.Errorf("%q: empty Name/Description: %+v", k, info)
		}
		if len(info.Traits) == 0 || len(info.Values) == 0 {
			t.Errorf("%q: needs at least one trait and one value: %+v", k, info)
		}
	}
}

func TestDefaultCulturesFreshCopies(t *testing.T) {
	// No shared mutable state: mutating one result must not affect the
	// next call or another caller's map (golden rule: no globals).
	first := DefaultCultures()
	first["barbarian"].Traits[0] = "tampered"
	first["barbarian"] = CultureInfo{Name: "Tampered"}
	second := DefaultCultures()
	if second["barbarian"].Name != "Barbarian" {
		t.Errorf("second call sees mutation: %+v", second["barbarian"])
	}
	if second["barbarian"].Traits[0] == "tampered" {
		t.Errorf("traits aliased across calls: %v", second["barbarian"].Traits)
	}
}

func TestCultureLookup(t *testing.T) {
	w := New("t", "t")
	tests := []struct {
		name  string
		query string
	}{
		{"exact key", "barbarian"},
		{"display name", "Civilised"},
		{"mixed case and space", "  Nomadic "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info, err := w.Culture(tc.query)
			if err != nil {
				t.Fatalf("Culture(%q): %v", tc.query, err)
			}
			if info.Name == "" {
				t.Errorf("Culture(%q) returned empty name", tc.query)
			}
		})
	}
	// The returned struct is a clone: mutations cannot reach the world.
	info, err := w.Culture("primitive")
	if err != nil {
		t.Fatalf("Culture: %v", err)
	}
	info.Traits[0] = "tampered"
	info.Values[0] = "tampered"
	again, err := w.Culture("primitive")
	if err != nil {
		t.Fatalf("Culture: %v", err)
	}
	if again.Traits[0] == "tampered" {
		t.Errorf("Traits aliased through Culture: %v", again.Traits)
	}
	if again.Values[0] == "tampered" {
		t.Errorf("Values aliased through Culture: %v", again.Values)
	}
}

func TestCultureLookupErrors(t *testing.T) {
	w := New("t", "t")
	for _, query := range []string{"knight", "", "   ", "barbarians"} {
		if _, err := w.Culture(query); !errors.Is(err, ErrUnknownCulture) {
			t.Errorf("Culture(%q) error = %v, want ErrUnknownCulture", query, err)
		}
	}
	// A world without seeded cultures (hand-built) errors rather than
	// inventing data.
	bare := &World{}
	if _, err := bare.Culture("barbarian"); !errors.Is(err, ErrUnknownCulture) {
		t.Errorf("bare Culture error = %v, want ErrUnknownCulture", err)
	}
}
