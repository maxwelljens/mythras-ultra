package world

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestCanAcquireExhaustive walks the entire economy × war × rarity matrix
// of the rule table: strain = economy(0..2) + war(0..2), allowed iff
// strain ≤ rarity limit (common 3, uncommon 2, rare 1, legendary 0).
func TestCanAcquireExhaustive(t *testing.T) {
	economies := []struct {
		state  string
		strain int
	}{
		{EconomyStable, 0}, {EconomyStrained, 1}, {EconomyCollapsed, 2},
	}
	wars := []struct {
		state  string
		strain int
	}{
		{WarPeace, 0}, {WarCold, 1}, {WarActive, 2},
	}
	rarities := []struct {
		grade string
		limit int
	}{
		{RarityCommon, 3}, {RarityUncommon, 2}, {RarityRare, 1}, {RarityLegendary, 0},
	}
	for _, e := range economies {
		for _, wr := range wars {
			for _, rar := range rarities {
				name := e.state + "/" + wr.state + "/" + rar.grade
				t.Run(name, func(t *testing.T) {
					w := New("t", "t")
					if err := w.SetEconomyState(e.state, "setup"); err != nil {
						t.Fatalf("SetEconomyState: %v", err)
					}
					if err := w.SetWarState(wr.state, "setup"); err != nil {
						t.Fatalf("SetWarState: %v", err)
					}
					d, err := w.CanAcquire("iron sword", rar.grade, "")
					if err != nil {
						t.Fatalf("CanAcquire: %v", err)
					}
					strain := e.strain + wr.strain
					wantAllowed := strain <= rar.limit
					if d.Allowed != wantAllowed {
						t.Errorf("Allowed = %v (strain %d, limit %d), want %v",
							d.Allowed, strain, rar.limit, wantAllowed)
					}
					if len(d.Reasons) == 0 {
						t.Fatal("Reasons empty")
					}
					verdict := d.Reasons[len(d.Reasons)-1]
					if wantAllowed && !strings.Contains(verdict, "are available") {
						t.Errorf("verdict = %q, want 'are available'", verdict)
					}
					if !wantAllowed && !strings.Contains(verdict, "are unavailable") {
						t.Errorf("verdict = %q, want 'are unavailable'", verdict)
					}
					// Governing flags in fixed evaluation order.
					var wantGoverning []string
					if e.strain > 0 {
						wantGoverning = append(wantGoverning, FlagEconomy)
					}
					if wr.strain > 0 {
						wantGoverning = append(wantGoverning, FlagWar)
					}
					if !slices.Equal(d.GoverningFlags, wantGoverning) {
						t.Errorf("GoverningFlags = %v, want %v", d.GoverningFlags, wantGoverning)
					}
				})
			}
		}
	}
}

func TestCanAcquireRegionBlockade(t *testing.T) {
	tests := []struct {
		name       string
		region     string // region the flag is set on
		flagValue  string
		query      string // region passed to CanAcquire
		wantStrain int
		wantFlag   string
	}{
		{name: "blockaded region", region: "Altea", flagValue: "true", query: "Altea", wantStrain: 2, wantFlag: "region:altea:blockade"},
		{name: "case-insensitive region", region: "Altea", flagValue: "true", query: "  alteA ", wantStrain: 2, wantFlag: "region:altea:blockade"},
		{name: "other region not affected", region: "Altea", flagValue: "true", query: "Borl", wantStrain: 0},
		{name: "inactive blockade flag", region: "Altea", flagValue: "false", query: "Altea", wantStrain: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := New("t", "t")
			if err := w.SetRegionFlag(tc.region, AspectBlockade, tc.flagValue, "setup"); err != nil {
				t.Fatalf("SetRegionFlag: %v", err)
			}
			d, err := w.CanAcquire("salt", RarityRare, tc.query)
			if err != nil {
				t.Fatalf("CanAcquire: %v", err)
			}
			wantAllowed := tc.wantStrain <= 1 // rare limit is 1
			if d.Allowed != wantAllowed {
				t.Errorf("Allowed = %v (strain %d), want %v", d.Allowed, tc.wantStrain, wantAllowed)
			}
			var wantGoverning []string
			if tc.wantFlag != "" {
				wantGoverning = append(wantGoverning, tc.wantFlag)
			}
			if !slices.Equal(d.GoverningFlags, wantGoverning) {
				t.Errorf("GoverningFlags = %v, want %v", d.GoverningFlags, wantGoverning)
			}
			// An active blockade on some region must not leak into a
			// global (empty-region) query.
			if tc.wantStrain > 0 {
				dGlobal, err := w.CanAcquire("salt", RarityRare, "")
				if err != nil {
					t.Fatalf("CanAcquire global: %v", err)
				}
				if !dGlobal.Allowed {
					t.Errorf("global query applied blockade strain; reasons: %v", dGlobal.Reasons)
				}
			}
		})
	}
}

func TestCanAcquireMaterialScarcity(t *testing.T) {
	w := New("t", "t")
	if err := w.SetMaterialScarcity("metal", true, "the forges went to the war"); err != nil {
		t.Fatalf("SetMaterialScarcity: %v", err)
	}

	// Item mentioning the scarce material: +1 strain refuses legendary.
	d, err := w.CanAcquire("metal sword", RarityLegendary, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	if d.Allowed {
		t.Error("legendary allowed while metal is scarce")
	}
	if !slices.Equal(d.GoverningFlags, []string{"metal_scarce"}) {
		t.Errorf("GoverningFlags = %v, want [metal_scarce]", d.GoverningFlags)
	}

	// Item not mentioning it: no strain even at legendary.
	d, err = w.CanAcquire("wooden club", RarityLegendary, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	if !d.Allowed {
		t.Errorf("wooden club refused; reasons: %v", d.Reasons)
	}
	if len(d.GoverningFlags) != 0 {
		t.Errorf("GoverningFlags = %v, want none", d.GoverningFlags)
	}

	// Scarcity turned off: metal is fine again.
	if err := w.SetMaterialScarcity("metal", false, "the mines reopened"); err != nil {
		t.Fatalf("clear scarcity: %v", err)
	}
	d, err = w.CanAcquire("metal sword", RarityLegendary, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	if !d.Allowed {
		t.Errorf("legendary refused after scarcity cleared; reasons: %v", d.Reasons)
	}

	// Several scarce materials: sorted evaluation order, both counted.
	if err := w.SetMaterialScarcity("iron", true, "scrap for the war"); err != nil {
		t.Fatalf("SetMaterialScarcity iron: %v", err)
	}
	if err := w.SetMaterialScarcity("metal", true, "and metal too"); err != nil {
		t.Fatalf("SetMaterialScarcity metal: %v", err)
	}
	d, err = w.CanAcquire("metal iron sword", RarityUncommon, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	want := []string{"iron_scarce", "metal_scarce"}
	if !slices.Equal(d.GoverningFlags, want) {
		t.Errorf("GoverningFlags = %v, want sorted %v", d.GoverningFlags, want)
	}
	if !d.Allowed {
		t.Errorf("uncommon at strain 2 should meet its limit of 2; reasons: %v", d.Reasons)
	}
}

func TestCanAcquireUnrecognisedFlagValues(t *testing.T) {
	w := New("t", "t")
	if err := w.SetFlag(FlagEconomy, "famine", "the crops failed"); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	if err := w.SetFlag(FlagWar, "border skirmishes", "small raids"); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	d, err := w.CanAcquire("anything", RarityLegendary, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	if !d.Allowed {
		t.Errorf("unrecognised values should be neutral; reasons: %v", d.Reasons)
	}
	if len(d.GoverningFlags) != 0 {
		t.Errorf("GoverningFlags = %v, want none (neutral values do not govern)", d.GoverningFlags)
	}
	joined := strings.Join(d.Reasons, " | ")
	if !strings.Contains(joined, `economy flag value "famine" is not a recognised state`) {
		t.Errorf("Reasons missing economy note: %v", d.Reasons)
	}
	if !strings.Contains(joined, `war flag value "border skirmishes" is not a recognised state`) {
		t.Errorf("Reasons missing war note: %v", d.Reasons)
	}
}

func TestCanAcquireCombinedStrain(t *testing.T) {
	w := New("t", "t")
	if err := w.SetEconomyState(EconomyCollapsed, "setup"); err != nil {
		t.Fatalf("SetEconomyState: %v", err)
	}
	if err := w.SetWarState(WarActive, "setup"); err != nil {
		t.Fatalf("SetWarState: %v", err)
	}
	if err := w.SetRegionFlag("Altea", AspectBlockade, "true", "setup"); err != nil {
		t.Fatalf("SetRegionFlag: %v", err)
	}
	if err := w.SetMaterialScarcity("metal", true, "setup"); err != nil {
		t.Fatalf("SetMaterialScarcity: %v", err)
	}
	// Strain 2+2+2+1 = 7 refuses everything, even common.
	for _, rarity := range []string{RarityCommon, RarityUncommon, RarityRare, RarityLegendary} {
		d, err := w.CanAcquire("metal sword", rarity, "altea")
		if err != nil {
			t.Fatalf("CanAcquire(%s): %v", rarity, err)
		}
		if d.Allowed {
			t.Errorf("%s allowed at strain 7; reasons: %v", rarity, d.Reasons)
		}
	}
	d, err := w.CanAcquire("metal sword", RarityCommon, "altea")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	want := []string{FlagEconomy, FlagWar, "region:altea:blockade", "metal_scarce"}
	if !slices.Equal(d.GoverningFlags, want) {
		t.Errorf("GoverningFlags = %v, want %v", d.GoverningFlags, want)
	}
}

func TestCanAcquireErrors(t *testing.T) {
	w := New("t", "t")
	tests := []struct {
		name   string
		item   string
		rarity string
		region string
		want   error
	}{
		{"invalid rarity", "sword", "mythic", "", ErrInvalidRarity},
		{"empty rarity", "sword", "", "", ErrInvalidRarity},
		{"blank rarity", "sword", "  rare-ish ", "", ErrInvalidRarity},
		{"empty item", "", RarityCommon, "", nil},
		{"blank item", "   ", RarityCommon, "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := w.CanAcquire(tc.item, tc.rarity, tc.region)
			if err == nil {
				t.Fatal("CanAcquire succeeded, want error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
	// Invalid region propagates the RegionFlagKey error.
	if _, err := w.CanAcquire("sword", RarityCommon, "al:tea"); err == nil {
		t.Error("region with colon succeeded, want error")
	}
	// Rarity is trimmed and case-insensitive.
	if _, err := w.CanAcquire("sword", "  Common ", ""); err != nil {
		t.Errorf("normalised rarity rejected: %v", err)
	}
	// CanAcquire never mutates.
	if len(w.Flags) != 0 {
		t.Errorf("CanAcquire mutated flags: %+v", w.Flags)
	}
}
