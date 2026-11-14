// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/engine/world"
	"github.com/mythras-ultra/myul/internal/tui"
)

// summarySkills is how many of the player character's highest-rated skills
// the prompt's CharacterSummary lists (mirrors the transcript's "key
// skills" count).
const summarySkills = 5

// buildMetadata assembles the TUI's panel snapshot from the engine state:
// a pure read-through of resolve/world/char shapes — no model calls, no
// computation of game facts (tui.Metadata contract). The player character
// is the first character with pc: true in resolve.ListCharacters order
// (engine IDs sort npc-* before pc-*, so a campaign's NPCs never shadow the
// panel). With no PC in play the character block stays empty.
func buildMetadata(st *resolve.State) tui.Metadata {
	m := tui.Metadata{SessionName: st.Name}
	if st.World != nil {
		m.InGameDate = inGameDate(st.World)
	}
	if pc := playerCharacter(st); pc != nil {
		m.Character = vitalsFor(pc, st)
		m.Inventory = inventoryLines(pc)
		m.Sheet = sheetFor(pc)
	}
	m.Scene = sceneLine(st.Scene)
	m.Present = presentNames(st)
	m.Clocks = clockLines(st)
	m.Flags = flagLines(st)
	return m
}

// playerCharacter returns the campaign's player character, or nil when no
// character is flagged pc yet.
func playerCharacter(st *resolve.State) *char.Character {
	for _, c := range resolve.ListCharacters(st) {
		if c.PC {
			return c
		}
	}
	return nil
}

// vitalsFor renders the character block: pooled readouts plus one row per
// hit location. Action Points appear only while an encounter is running —
// outside combat there is no per-round pool to show.
func vitalsFor(c *char.Character, st *resolve.State) tui.Vitals {
	v := tui.Vitals{Name: characterName(c), Culture: c.Culture, Career: c.Career}
	var current int
	for _, hp := range c.CurrentHP {
		current += hp
	}
	if max := c.Derived.HitPoints; max > 0 {
		v.HP = strconv.Itoa(current) + "/" + strconv.Itoa(max)
	}
	if enc := st.Encounter; enc != nil && enc.Started && !enc.Ended {
		for i, p := range enc.Participants {
			if i >= len(enc.AP) {
				continue
			}
			if p == c || (p != nil && p.ID == c.ID) {
				v.AP = strconv.Itoa(enc.AP[i]) + "/" + strconv.Itoa(c.Derived.ActionPoints)
				break
			}
		}
	}
	v.PowerPoints = strconv.Itoa(c.Derived.PowerPoints)
	v.Luck = strconv.Itoa(c.Derived.LuckPoints)
	v.Locations = locationRows(c)
	v.Conditions = conditionLabels(c)
	return v
}

// characterName renders the character's display name: its name, falling
// back to the engine ID for a character created without one.
func characterName(c *char.Character) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

// sheetFor renders the full character sheet block for the panel snapshot's
// player character. It reads the same cached char.Derived snapshot the
// vitals builder reads (the engine refreshes it; the TUI never derives
// numbers), keeps the fixed characteristic order and sorts every mapped
// list by value descending, then name — never map order.
func sheetFor(c *char.Character) tui.Sheet {
	s := tui.Sheet{
		Name:       characterName(c),
		Culture:    c.Culture,
		Career:     c.Career,
		Concept:    oneLine(c.Concept),
		Skills:     ratedRows(skillMapRows(c)),
		Styles:     ratedRows(styleRows(c)),
		Passions:   ratedRows(passionRows(c)),
		Inventory:  inventoryLines(c),
		Locations:  locationRows(c),
		Conditions: conditionLabels(c),
	}
	if c.Age > 0 {
		s.Age = strconv.Itoa(c.Age)
	}
	ch := c.Chars
	s.Characteristics = []tui.NameValue{
		{Name: "STR", Value: strconv.Itoa(ch.STR)},
		{Name: "CON", Value: strconv.Itoa(ch.CON)},
		{Name: "SIZ", Value: strconv.Itoa(ch.SIZ)},
		{Name: "DEX", Value: strconv.Itoa(ch.DEX)},
		{Name: "INT", Value: strconv.Itoa(ch.INT)},
		{Name: "POW", Value: strconv.Itoa(ch.POW)},
		{Name: "CHA", Value: strconv.Itoa(ch.CHA)},
	}
	if max := c.Derived.HitPoints; max > 0 {
		var current int
		for _, hp := range c.CurrentHP {
			current += hp
		}
		s.Derived = append(s.Derived, tui.NameValue{
			Name: "HP", Value: strconv.Itoa(current) + "/" + strconv.Itoa(max),
		})
	}
	s.Derived = append(s.Derived,
		tui.NameValue{Name: "Damage Modifier", Value: c.Derived.DamageModifier},
		tui.NameValue{Name: "Power Points", Value: strconv.Itoa(c.Derived.PowerPoints)},
		tui.NameValue{Name: "Luck", Value: strconv.Itoa(c.Derived.LuckPoints)},
		tui.NameValue{Name: "Experience Bonus", Value: strconv.Itoa(c.Derived.ExperienceModifier)},
	)
	return s
}

// rated is one mapped sheet row before its final ordering.
type rated struct {
	name  string
	value int
}

// skillMapRows projects the skill map onto sortable rows (nil entries
// skipped).
func skillMapRows(c *char.Character) []rated {
	rows := make([]rated, 0, len(c.Skills))
	for name, sk := range c.Skills {
		if sk == nil {
			continue
		}
		rows = append(rows, rated{name: name, value: sk.Value})
	}
	return rows
}

// styleRows projects the combat styles onto sortable rows.
func styleRows(c *char.Character) []rated {
	rows := make([]rated, 0, len(c.CombatStyles))
	for name, st := range c.CombatStyles {
		if st == nil {
			continue
		}
		rows = append(rows, rated{name: name, value: st.Value})
	}
	return rows
}

// passionRows projects the passions onto sortable rows.
func passionRows(c *char.Character) []rated {
	rows := make([]rated, 0, len(c.Passions))
	for name, p := range c.Passions {
		if p == nil {
			continue
		}
		rows = append(rows, rated{name: name, value: p.Value})
	}
	return rows
}

// ratedSorted sorts sheet rows by value descending, then name — a stable
// order regardless of map iteration.
func ratedSorted(rows []rated) []rated {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].value != rows[j].value {
			return rows[i].value > rows[j].value
		}
		return rows[i].name < rows[j].name
	})
	return rows
}

// ratedRows sorts sheet rows (ratedSorted) and renders each as "NN%".
func ratedRows(rows []rated) []tui.NameValue {
	rows = ratedSorted(rows)
	out := make([]tui.NameValue, 0, len(rows))
	for _, r := range rows {
		out = append(out, tui.NameValue{Name: r.name, Value: strconv.Itoa(r.value) + "%"})
	}
	return out
}

// locationRows renders one row per hit location in catalog order (stable —
// never map order), with live HP against the starting value and the panel's
// severity label.
func locationRows(c *char.Character) []tui.LocationHP {
	names := make([]string, 0, len(c.Derived.Locations))
	for _, def := range c.Derived.Locations {
		names = append(names, def.Name)
	}
	if len(names) == 0 {
		names = slices.Sorted(maps.Keys(c.Derived.LocationHP)) // hand-built sheet
	}
	rows := make([]tui.LocationHP, 0, len(names))
	for _, name := range names {
		max := c.Derived.LocationHP[name]
		current, ok := c.CurrentHP[name]
		if !ok {
			current = max
		}
		rows = append(rows, tui.LocationHP{
			Name:     name,
			Current:  current,
			Max:      max,
			Severity: severityLabel(c, name),
		})
	}
	return rows
}

// severityLabel maps the engine's wound levels onto the panel vocabulary
// (tui.LocationHP.Severity): "" unhurt, "wound", "severe", "critical".
func severityLabel(c *char.Character, location string) string {
	sev, err := c.SeverityAt(location)
	if err != nil {
		return "" // unknown location: render with the neutral panel colour
	}
	switch sev {
	case char.SeverityMinor:
		return "wound"
	case char.SeveritySerious:
		return "severe"
	case char.SeverityMajor:
		return "critical"
	default:
		return ""
	}
}

// inventoryLines renders the carried items tersely: name, quantity when it
// is not one, and the equip slot when wielded or worn.
func inventoryLines(c *char.Character) []string {
	lines := make([]string, 0, len(c.Inventory))
	for _, item := range c.Inventory {
		if item == nil {
			continue
		}
		line := item.Name
		if item.Qty != 1 {
			line += " ×" + strconv.Itoa(item.Qty)
		}
		if item.Equipped {
			line += " (equipped"
			if item.Slot != "" {
				line += ": " + item.Slot
			}
			line += ")"
		}
		lines = append(lines, line)
	}
	return lines
}

// sceneLine renders the scene as one sentence: the location, with the GM's
// description when there is one.
func sceneLine(sc resolve.Scene) string {
	switch {
	case sc.Location != "" && sc.Description != "":
		return sc.Location + ": " + sc.Description
	case sc.Location != "":
		return sc.Location
	default:
		return sc.Description
	}
}

// presentNames resolves scene labels to character names where the label is
// a registered character ID; free-text labels pass through unchanged
// (Scene.Present is labels, not validated references).
func presentNames(st *resolve.State) []string {
	out := make([]string, 0, len(st.Scene.Present))
	for _, label := range st.Scene.Present {
		if c, ok := st.Characters[label]; ok && c != nil {
			if c.Name != "" {
				label = c.Name
			} else {
				label = c.ID
			}
		}
		out = append(out, label)
	}
	return out
}

// clockLines renders the running clocks as "name: filled/segments", sorted
// by name so the panel never flickers with map order.
func clockLines(st *resolve.State) []string {
	if st.World == nil {
		return nil
	}
	out := make([]string, 0, len(st.World.Clocks))
	for _, name := range slices.Sorted(maps.Keys(st.World.Clocks)) {
		clock := st.World.Clocks[name]
		if clock == nil {
			continue
		}
		out = append(out, clock.Name+": "+strconv.Itoa(clock.Filled)+"/"+strconv.Itoa(clock.Segments))
	}
	return out
}

// flagLines renders the world flags as "key = value", sorted by key.
func flagLines(st *resolve.State) []string {
	if st.World == nil {
		return nil
	}
	out := make([]string, 0, len(st.World.Flags))
	for _, key := range slices.Sorted(maps.Keys(st.World.Flags)) {
		flag := st.World.Flags[key]
		out = append(out, key+" = "+flag.Value)
	}
	return out
}

// inGameDate renders the world calendar as one label, the same shape
// world.Summary prints for its Date line.
func inGameDate(w *world.World) string {
	if w == nil {
		return ""
	}
	cal := w.Calendar
	return fmt.Sprintf("Year %d, Month %d, Day %d (%s, %s)",
		cal.Year, cal.Month, cal.Day, cal.Season, cal.TimeOfDay)
}

// worldSummary renders the world for prompts.Vars, or a placeholder when a
// hand-built state carries none.
func worldSummary(st *resolve.State) string {
	if st.World == nil {
		return "(no world state)"
	}
	return st.World.Summary()
}

// recapText joins the stored end-of-session recaps for prompts.Vars; an
// empty session gets the template's "(none)"-style placeholder.
func recapText(st *resolve.State) string {
	recaps := resolve.SessionRecap(st)
	if len(recaps) == 0 {
		return "(nothing yet — this is a fresh session)"
	}
	return "- " + strings.Join(recaps, "\n- ")
}

// characterSummary renders the player character for prompts.Vars: identity,
// condition, key skills and concept — the prompt's continuity record. The
// tools remain the only source of mechanical numbers during play.
func characterSummary(st *resolve.State) string {
	pc := playerCharacter(st)
	if pc == nil {
		return "(no player character yet — guide creation with the character tools)"
	}
	name := pc.Name
	if name == "" {
		name = pc.ID
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)", name, pc.ID)
	if pc.Culture != "" || pc.Career != "" {
		fmt.Fprintf(&b, " — %s %s", oneLine(pc.Culture), oneLine(pc.Career))
	}
	var current int
	for _, hp := range pc.CurrentHP {
		current += hp
	}
	if max := pc.Derived.HitPoints; max > 0 {
		fmt.Fprintf(&b, "; HP %d/%d", current, max)
	}
	if conds := conditionNames(pc); len(conds) > 0 {
		fmt.Fprintf(&b, "; conditions: %s", strings.Join(conds, ", "))
	}
	if skills := keySkills(pc, summarySkills); len(skills) > 0 {
		fmt.Fprintf(&b, "; key skills: %s", strings.Join(skills, ", "))
	}
	if concept := oneLine(pc.Concept); concept != "" {
		fmt.Fprintf(&b, "; concept: %s", concept)
	}
	return b.String()
}

// oneLine collapses a value's whitespace into single spaces so free text
// (concepts, culture names) always renders as one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// conditionNames lists the character's active conditions, ordered as the
// character sheet holds them.
func conditionNames(c *char.Character) []string {
	out := make([]string, 0, len(c.Conditions))
	for _, cond := range c.Conditions {
		out = append(out, cond.Name)
	}
	return out
}

// conditionLabels renders active conditions the way the panel shows them:
// the name, with the body location appended when the condition has one.
func conditionLabels(c *char.Character) []string {
	out := make([]string, 0, len(c.Conditions))
	for _, cond := range c.Conditions {
		label := cond.Name
		if cond.Location != "" {
			label += " (" + cond.Location + ")"
		}
		out = append(out, label)
	}
	return out
}

// keySkills renders the n highest-rated skills as "Name 45%", best first
// with ties broken by name — a stable order regardless of map iteration.
func keySkills(c *char.Character, n int) []string {
	rows := ratedSorted(skillMapRows(c))
	if len(rows) > n {
		rows = rows[:n]
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.name+" "+strconv.Itoa(r.value)+"%")
	}
	return out
}

// metaDump renders the meta command's system block: the panel snapshot plus
// the harness's own labels (model, decision model), which have no panel slot.
func (s *Session) metaDump() string {
	m := buildMetadata(s.state)
	var b strings.Builder
	fmt.Fprintf(&b, "session: %s\n", m.SessionName)
	model := s.model
	if model == "" {
		model = "not configured"
	}
	fmt.Fprintf(&b, "model: %s\n", model)
	decision := "disabled"
	if s.decision != nil {
		decision = "configured"
	}
	fmt.Fprintf(&b, "decision model: %s\n", decision)
	if m.InGameDate != "" {
		fmt.Fprintf(&b, "date: %s\n", m.InGameDate)
	}
	if m.Scene != "" {
		fmt.Fprintf(&b, "scene: %s\n", m.Scene)
	}
	if name := m.Character.Name; name != "" {
		line := "character: " + name
		if m.Character.HP != "" {
			line += " — HP " + m.Character.HP
		}
		if m.Character.AP != "" {
			line += ", AP " + m.Character.AP
		}
		fmt.Fprintln(&b, line)
	}
	if len(m.Character.Conditions) > 0 {
		fmt.Fprintln(&b, "conditions: "+strings.Join(m.Character.Conditions, ", "))
	}
	if len(m.Inventory) > 0 {
		fmt.Fprintln(&b, "inventory: "+strings.Join(m.Inventory, ", "))
	}
	if len(m.Present) > 0 {
		fmt.Fprintln(&b, "present: "+strings.Join(m.Present, ", "))
	}
	if len(m.Clocks) > 0 {
		fmt.Fprintln(&b, "clocks: "+strings.Join(m.Clocks, ", "))
	}
	if len(m.Flags) > 0 {
		fmt.Fprintln(&b, "flags: "+strings.Join(m.Flags, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}
