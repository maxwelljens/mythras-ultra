package tui

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestPaletteHasExactlySixteenSlots asserts the project's 16-colour budget:
// every theme exposes NumSlots == 16 slots and fills all of them.
func TestPaletteHasExactlySixteenSlots(t *testing.T) {
	if NumSlots != 16 {
		t.Fatalf("NumSlots = %d, want 16", NumSlots)
	}
	for _, name := range Names() {
		th, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		count := 0
		for i, c := range th.Palette {
			count++
			if c == nil {
				t.Errorf("theme %q: slot %d is nil — every theme must fill all 16 slots", name, i)
			}
		}
		if count != 16 {
			t.Errorf("theme %q: %d colour slots, want 16", name, count)
		}
		if th.Styles.Narrative.Render("prose") == "" {
			t.Errorf("theme %q: narrative style renders nothing", name)
		}
		if th.Styles.DiceBanner.Render("12") == "" {
			t.Errorf("theme %q: dice banner style renders nothing", name)
		}
		if th.Styles.ToolAnnotation.Render("⚙ x → y") == "" {
			t.Errorf("theme %q: tool annotation style renders nothing", name)
		}
	}
}

func TestGetUnknownThemeErrors(t *testing.T) {
	_, err := Get("nope")
	if err == nil {
		t.Fatal("Get(\"nope\") should fail")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should quote the bad name, got: %v", err)
	}
	for _, name := range Names() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should list valid theme %q, got: %v", name, err)
		}
	}
}

func TestNamesStable(t *testing.T) {
	want := []string{"mythras", "obsidian", "parchment", "terminal"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() is not stable: %v", got)
	}
	mutated := Names()
	mutated[0] = "mutated"
	if got := Names()[0]; got != "mythras" {
		t.Fatalf("Names() must return a fresh slice, first entry now %q", got)
	}
}

// TestAdapt follows the terminal background: matching the native mode is the
// identity, the opposite mode re-tunes the palette, and the terminal theme
// keeps the user's own colours either way.
func TestAdapt(t *testing.T) {
	mythras, err := Get("mythras")
	if err != nil {
		t.Fatal(err)
	}
	if got := mythras.Adapt(true); !reflect.DeepEqual(got.Palette, mythras.Palette) {
		t.Error("mythras is dark-native: adapting it to a dark background must not change the palette")
	}
	light := mythras.Adapt(false)
	if light.Dark {
		t.Error("Adapt(false) must build a light-background theme")
	}
	if reflect.DeepEqual(light.Palette, mythras.Palette) {
		t.Error("mythras on a light background must re-tune its palette")
	}
	if reflect.DeepEqual(light.Palette[SlotBackground], mythras.Palette[SlotBackground]) {
		t.Error("background slot must differ between light and dark adaptations")
	}

	parchment, err := Get("parchment")
	if err != nil {
		t.Fatal(err)
	}
	if parchment.Dark {
		t.Error("parchment is light-native")
	}
	if dark := parchment.Adapt(true); reflect.DeepEqual(dark.Palette, parchment.Palette) {
		t.Error("parchment on a dark background must re-tune its palette")
	}

	term, err := Get("terminal")
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range []Theme{term, term.Adapt(false)} {
		if _, ok := th.Palette[SlotBackground].(lipgloss.NoColor); !ok {
			t.Errorf("terminal theme (dark=%v) must leave the background to the terminal, got %T",
				th.Dark, th.Palette[SlotBackground])
		}
		if _, ok := th.Palette[SlotForeground].(color.RGBA); ok {
			t.Error("terminal theme must use the user's ANSI 16, not hex colours")
		}
	}
}

// TestHexThemesParse guards against a typo in the theme tables: lipgloss
// silently turns a malformed hex string into "no colour".
func TestHexThemesParse(t *testing.T) {
	for _, name := range []string{"mythras", "obsidian", "parchment"} {
		th, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, slot := range []Slot{SlotBackground, SlotForeground, SlotAccent} {
			if _, ok := th.Palette[slot].(color.RGBA); !ok {
				t.Errorf("theme %q slot %d did not parse as hex, got %T", name, slot, th.Palette[slot])
			}
		}
	}
}
