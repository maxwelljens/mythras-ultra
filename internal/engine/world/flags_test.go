package world

import (
	"errors"
	"testing"
)

func TestSetFlagRecordsOldToNewHistory(t *testing.T) {
	w := New("t", "t")

	// Creation: Old is empty.
	if err := w.SetFlag("economy", EconomyStable, "start of play"); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Transition: Old → New is recorded.
	if err := w.SetFlag("economy", EconomyStrained, "bad harvest"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	// No-op: same value leaves history untouched.
	if err := w.SetFlag("economy", EconomyStrained, "bad harvest again"); err != nil {
		t.Fatalf("no-op: %v", err)
	}
	// Clearing is a transition to the empty value, not a deletion.
	if err := w.SetFlag("economy", "", "markets reopen"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	f, ok := w.Lookup("economy")
	if !ok {
		t.Fatal("flag lost")
	}
	want := []FlagChange{
		{Key: "economy", Old: "", New: EconomyStable, Reason: "start of play"},
		{Key: "economy", Old: EconomyStable, New: EconomyStrained, Reason: "bad harvest"},
		{Key: "economy", Old: EconomyStrained, New: "", Reason: "markets reopen"},
	}
	if len(f.History) != len(want) {
		t.Fatalf("history = %d entries, want %d: %+v", len(f.History), len(want), f.History)
	}
	for i, entry := range f.History {
		if entry != want[i] {
			t.Errorf("history[%d] = %+v, want %+v", i, entry, want[i])
		}
	}
	if f.Value != "" || f.Reason != "markets reopen" {
		t.Errorf("flag = %+v, want cleared with latest reason", f)
	}
}

func TestSetFlagValidation(t *testing.T) {
	w := New("t", "t")
	if err := w.SetFlag("", "x", "why"); !errors.Is(err, ErrUnknownFlag) {
		t.Errorf("empty key error = %v, want ErrUnknownFlag", err)
	}
	if err := w.SetFlag("   ", "x", "why"); !errors.Is(err, ErrUnknownFlag) {
		t.Errorf("blank key error = %v, want ErrUnknownFlag", err)
	}
	if err := w.SetFlag("key", "x", ""); err == nil {
		t.Error("empty reason succeeded, want error")
	}
	if err := w.SetFlag("key", "x", "   "); err == nil {
		t.Error("blank reason succeeded, want error")
	}
	if len(w.Flags) != 0 {
		t.Errorf("failed SetFlag mutated state: %+v", w.Flags)
	}
	// Keys are trimmed on write.
	if err := w.SetFlag("  wealth  ", "poor", "the taxman came"); err != nil {
		t.Fatalf("trimmed key: %v", err)
	}
	if _, err := w.Get("wealth"); err != nil {
		t.Errorf("Get after trimmed write: %v", err)
	}
}

func TestGetAndLookup(t *testing.T) {
	w := New("t", "t")
	if _, err := w.Get("nope"); !errors.Is(err, ErrUnknownFlag) {
		t.Errorf("Get missing error = %v, want ErrUnknownFlag", err)
	}
	if _, ok := w.Lookup("nope"); ok {
		t.Error("Lookup of missing flag reported existence")
	}
	if err := w.SetFlag("war", WarCold, "standoff"); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	v, err := w.Get("war")
	if err != nil || v != WarCold {
		t.Errorf("Get = %q, %v; want cold, nil", v, err)
	}
	f, ok := w.Lookup("war")
	if !ok || f.Key != "war" || f.Value != WarCold || f.Reason != "standoff" {
		t.Errorf("Lookup = %+v, %v", f, ok)
	}
}

func TestSetEconomyState(t *testing.T) {
	w := New("t", "t")
	// Normalisation: trimmed and lower-cased.
	if err := w.SetEconomyState("  Collapsed ", "war economy"); err != nil {
		t.Fatalf("SetEconomyState: %v", err)
	}
	v, err := w.Get(FlagEconomy)
	if err != nil || v != EconomyCollapsed {
		t.Errorf("economy flag = %q, %v; want collapsed", v, err)
	}
	if err := w.SetEconomyState("famine", "bad"); !errors.Is(err, ErrInvalidFlagValue) {
		t.Errorf("invalid state error = %v, want ErrInvalidFlagValue", err)
	}
	// Rejected writes leave state untouched.
	if v, _ := w.Get(FlagEconomy); v != EconomyCollapsed {
		t.Errorf("invalid state mutated flag to %q", v)
	}
}

func TestSetWarState(t *testing.T) {
	w := New("t", "t")
	if err := w.SetWarState("ACTIVE", "invasion"); err != nil {
		t.Fatalf("SetWarState: %v", err)
	}
	v, err := w.Get(FlagWar)
	if err != nil || v != WarActive {
		t.Errorf("war flag = %q, %v; want active", v, err)
	}
	for _, bad := range []string{"", "ceasefire", "hot"} {
		if err := w.SetWarState(bad, "x"); !errors.Is(err, ErrInvalidFlagValue) {
			t.Errorf("SetWarState(%q) error = %v, want ErrInvalidFlagValue", bad, err)
		}
	}
}

func TestRegionFlagKey(t *testing.T) {
	tests := []struct {
		name   string
		region string
		aspect string
		want   string
		wantOK bool
	}{
		{"plain", "Altea", "blockade", "region:altea:blockade", true},
		{"trims and lower-cases", "  Coast  ", "  Blockade ", "region:coast:blockade", true},
		{"empty region", "", "blockade", "", false},
		{"empty aspect", "altea", "", "", false},
		{"colon in region", "al:tea", "blockade", "", false},
		{"colon in aspect", "altea", "war:yes", "", false},
		{"both empty", "", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RegionFlagKey(tc.region, tc.aspect)
			if (err == nil) != tc.wantOK {
				t.Fatalf("RegionFlagKey(%q, %q) error = %v, wantOK %v", tc.region, tc.aspect, err, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("RegionFlagKey = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSetRegionFlag(t *testing.T) {
	w := New("t", "t")
	if err := w.SetRegionFlag("Altea", AspectBlockade, "true", "the fleet blockades the harbour"); err != nil {
		t.Fatalf("SetRegionFlag: %v", err)
	}
	v, err := w.Get("region:altea:blockade")
	if err != nil || v != "true" {
		t.Errorf("region flag = %q, %v; want true", v, err)
	}
	if err := w.SetRegionFlag("", AspectBlockade, "true", "x"); err == nil {
		t.Error("empty region succeeded, want error")
	}
	if err := w.SetRegionFlag("al:tea", AspectBlockade, "true", "x"); err == nil {
		t.Error("colon region succeeded, want error")
	}
}

func TestSetMaterialScarcity(t *testing.T) {
	w := New("t", "t")
	if err := w.SetMaterialScarcity("Iron", true, "the forges went to the war"); err != nil {
		t.Fatalf("SetMaterialScarcity: %v", err)
	}
	v, err := w.Get("iron_scarce")
	if err != nil || v != "true" {
		t.Errorf("scarcity flag = %q, %v; want true", v, err)
	}
	if err := w.SetMaterialScarcity("", true, "x"); err == nil {
		t.Error("empty material succeeded, want error")
	}
	if err := w.SetMaterialScarcity("Iron", false, "mines reopened"); err != nil {
		t.Fatalf("clear scarcity: %v", err)
	}
	if v, _ := w.Get("iron_scarce"); v != "false" {
		t.Errorf("scarcity cleared to %q, want false", v)
	}
}

func TestFlagActive(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"", false}, {"false", false}, {"0", false}, {"no", false},
		{"off", false}, {"clear", false}, {"none", false}, {" FALSE ", false},
		{"true", true}, {"yes", true}, {"blockaded", true}, {"garbage", true},
	}
	for _, tc := range tests {
		if got := flagActive(tc.value); got != tc.want {
			t.Errorf("flagActive(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
