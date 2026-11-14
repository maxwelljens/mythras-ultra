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
	v := tui.Vitals{Name: c.Name, Culture: c.Culture, Career: c.Career}
	if v.Name == "" {
		v.Name = c.ID
	}
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
	for _, cond := range c.Conditions {
		label := cond.Name
		if cond.Location != "" {
			label += " (" + cond.Location + ")"
		}
		v.Conditions = append(v.Conditions, label)
	}
	return v
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

// keySkills renders the n highest-rated skills as "Name 45%", best first
// with ties broken by name — a stable order regardless of map iteration.
func keySkills(c *char.Character, n int) []string {
	type rated struct {
		name  string
		value int
	}
	list := make([]rated, 0, len(c.Skills))
	for name, sk := range c.Skills {
		if sk == nil {
			continue
		}
		list = append(list, rated{name: name, value: sk.Value})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].value != list[j].value {
			return list[i].value > list[j].value
		}
		return list[i].name < list[j].name
	})
	if len(list) > n {
		list = list[:n]
	}
	out := make([]string, 0, len(list))
	for _, r := range list {
		out = append(out, r.name+" "+strconv.Itoa(r.value)+"%")
	}
	return out
}

// metaDump renders the /meta system block: the panel snapshot plus the
// harness's own labels (model, decision model), which have no panel slot.
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
