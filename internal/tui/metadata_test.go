package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// populatedMetadata is a fully populated panel snapshot.
func populatedMetadata() Metadata {
	return Metadata{
		Character: Vitals{
			Name:        "Ashen Vael",
			Culture:     "Sassanid",
			Career:      "Brigand",
			HP:          "34/40",
			AP:          "3/4",
			PowerPoints: "5/7",
			Luck:        "2",
			Locations: []LocationHP{
				{Name: "Chest", Current: 14, Max: 14, Severity: ""},
				{Name: "Right Arm", Current: 5, Max: 8, Severity: "wound"},
				{Name: "Head", Current: 2, Max: 5, Severity: "critical"},
			},
			Conditions: []string{"poisoned"},
		},
		Inventory:   []string{"Rope", "Torch", "Flask"},
		Scene:       "The flooded crypt",
		Present:     []string{"Ione", "Boatswain"},
		Clocks:      []string{"Tide rising: 3/6"},
		Flags:       []string{"Gatehouse opened"},
		SessionName: "Session 12",
		InGameDate:  "13th of Seedfall",
	}
}

func TestRenderMetadataPopulated(t *testing.T) {
	th, err := Get("mythras")
	if err != nil {
		t.Fatal(err)
	}
	md := populatedMetadata()
	raw := renderMetadata(th, md, 80, 40)
	got := plain(raw)
	for _, want := range []string{
		"METADATA", "CHARACTER", "Ashen Vael", "Sassanid · Brigand",
		"HP 34/40", "AP 3/4", "PP 5/7", "Luck 2", "Conditions: poisoned",
		"LOCATIONS", "Chest", "Right Arm", "Head",
		"INVENTORY", "Rope", "Torch", "Flask",
		"SCENE", "The flooded crypt", "Present:", "Ione", "Boatswain",
		"CLOCKS", "Tide rising: 3/6",
		"FLAGS", "Gatehouse opened",
		"SESSION", "Session 12", "13th of Seedfall",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("panel should contain %q, panel:\n%s", want, got)
		}
	}
	// The inventory stays on one terse line when the column has room.
	if !strings.Contains(got, "Rope · Torch · Flask") {
		t.Errorf("inventory should render as one terse line, panel:\n%s", got)
	}
	// HP bars carry their severity colour through palette slots.
	for _, want := range []string{
		th.Styles.Success.Render("Chest ▓▓▓▓ 14/14"),
		th.Styles.Failure.Render("Right Arm ▓▓▓░ 5/8"),
		th.Styles.Fumble.Render("Head ▓▓░░ 2/5"),
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("panel should render the bar %q in its severity style", want)
		}
	}
	// The box is exactly the transcript height: no layout break.
	if lines := strings.Split(raw, "\n"); len(lines) != 37 {
		t.Errorf("panel has %d lines, want 37 (height 40 minus header/input/footer)", len(lines))
	}
}

func TestRenderMetadataAllEmpty(t *testing.T) {
	th, err := Get("mythras")
	if err != nil {
		t.Fatal(err)
	}
	// The zero-value Metadata must render, without panics, as dim dashes.
	got := plain(renderMetadata(th, Metadata{}, 80, 24))
	for _, want := range []string{
		"METADATA", "CHARACTER", "LOCATIONS", "INVENTORY", "SCENE",
		"CLOCKS", "FLAGS", "SESSION",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("empty panel should still show the %q section, panel:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "—"); n < 7 {
		t.Errorf("empty panel should show one placeholder per section, got %d:\n%s", n, got)
	}
	// Content lines never exceed the panel slot.
	for _, line := range strings.Split(renderMetadata(th, Metadata{}, 80, 24), "\n") {
		if w := lipgloss.Width(line); w > 24 {
			t.Errorf("panel line wider than the panel slot (%d cols): %q", w, plain(line))
		}
	}
}

func TestHPBarMaths(t *testing.T) {
	tests := []struct {
		name     string
		cur, max int
		want     string
	}{
		{"empty", 0, 10, "░░░░"},
		{"half", 5, 10, "▓▓░░"},
		{"full", 10, 10, "▓▓▓▓"},
		{"over max clamps to full", 15, 10, "▓▓▓▓"},
		{"negative clamps to empty", -3, 10, "░░░░"},
		{"max zero renders empty", 4, 0, "░░░░"},
		{"rounds up from a sliver", 1, 4, "▓░░░"},
		{"rounds down below half a cell", 1, 10, "░░░░"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hpBar(tt.cur, tt.max); got != tt.want {
				t.Errorf("hpBar(%d, %d) = %q, want %q", tt.cur, tt.max, got, tt.want)
			}
		})
	}
}

func TestSeverityStyleMapping(t *testing.T) {
	th, err := Get("mythras")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		severity string
		want     lipgloss.Style
	}{
		{"", th.Styles.Success},
		{"wound", th.Styles.Failure},
		{"severe", th.Styles.Critical},
		{"critical", th.Styles.Fumble},
		{"unknown-future-severity", th.Styles.Panel},
	}
	for _, tt := range tests {
		got := severityStyle(th, tt.severity).Render("bar")
		if want := tt.want.Render("bar"); got != want {
			t.Errorf("severityStyle(%q) rendered %q, want %q", tt.severity, got, want)
		}
	}
}
