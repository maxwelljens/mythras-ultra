package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// characterTools returns the character and NPC tools.
func characterTools() []Tool {
	return []Tool{
		{
			Name: "create_character",
			Description: "Create a PC or NPC from the creation steps — characteristics " +
				"(rolled when not supplied), culture, career, skill overrides. Use when " +
				"a new character enters play; never invent statistics in narration." + factsNote,
			Schema: objSchema(map[string]any{
				"name":    strProp("Character name."),
				"pc":      boolProp("true for a player character, false for an NPC (IDs: pc-N / npc-N)."),
				"culture": strProp("Culture package name, e.g. \"Civilised\", \"Barbarian\", \"Nomadic\", \"Primitive\"."),
				"career":  strProp("Career package name, e.g. \"Hunter\", \"Scholar\", \"Bounty Hunter\"."),
				"age":     intProp("Age in years (0 leaves it unset); sizes the Experience Table bonus pool.", 0),
				"characteristics": objProp("The seven characteristics; all seven required when given — "+
					"omit to roll them on the session RNG.", map[string]any{
					"str": intProp("Strength.", 0),
					"con": intProp("Constitution (0 = death).", 0),
					"siz": intProp("Size.", 0),
					"dex": intProp("Dexterity.", 0),
					"int": intProp("Intelligence.", 0),
					"pow": intProp("Power.", 0),
					"cha": intProp("Charisma.", 0),
				}, "str", "con", "siz", "dex", "int", "pow", "cha"),
				"skills": map[string]any{
					"type":                 "object",
					"description":          "Starting skill rating overrides applied after culture and career grants.",
					"additionalProperties": map[string]any{"type": "integer"},
				},
			}, "name", "pc", "culture", "career"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Name            string                `json:"name"`
					PC              *bool                 `json:"pc"`
					Culture         string                `json:"culture"`
					Career          string                `json:"career"`
					Age             int                   `json:"age"`
					Characteristics *char.Characteristics `json:"characteristics"`
					Skills          map[string]int        `json:"skills"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				for _, f := range []struct{ name, value string }{
					{"name", a.Name}, {"culture", a.Culture}, {"career", a.Career},
				} {
					if err := requireField(f.name, f.value); err != nil {
						return Result{}, err
					}
				}
				if a.PC == nil {
					return Result{}, fmt.Errorf("%w: missing required field %q", ErrInvalidArgs, "pc")
				}
				res, err := resolve.Apply(s, resolve.CreateCharacter{
					Name:            a.Name,
					PC:              *a.PC,
					Culture:         a.Culture,
					Career:          a.Career,
					Age:             a.Age,
					Characteristics: a.Characteristics,
					Skills:          a.Skills,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "finalize_character",
			Description: "Grant starting equipment and Passions and report the finished " +
				"sheet's totals. Use right after create_character to finish the sheet; " +
				"may be repeated for additions." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character ID being finalized."),
				"equipment": arrProp("Items to add; sample-table names take their printed statistics, "+
					"other names become plain items.", map[string]any{
					"type":                 "object",
					"properties":           map[string]any{"name": strProp("Item name."), "qty": intProp("Quantity (0 means 1).", 0)},
					"required":             []string{"name"},
					"additionalProperties": false,
				}),
				"passions": arrProp("Starting Passions to add or replace.", map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  strProp("Passion object, e.g. \"Hate (Orcs)\"."),
						"kind":  enumProp("Passion Table row.", "person", "organization", "race", "place", "object", "trait"),
						"order": intProp("Importance 1–3 (0 means 1).", 0),
					},
					"required":             []string{"name", "kind"},
					"additionalProperties": false,
				}),
			}, "actor"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor     string                  `json:"actor"`
					Equipment []resolve.EquipmentSpec `json:"equipment"`
					Passions  []resolve.PassionSpec   `json:"passions"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.FinalizeCharacter{
					Actor: a.Actor, Equipment: a.Equipment, Passions: a.Passions,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "get_character",
			Description: "Read one character's full sheet — identity, characteristics, " +
				"derived values, hit points per location, wounds, conditions, inventory " +
				"and skills. Use before narrating anything about that character's state." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character ID (PC or NPC)."),
			}, "actor"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string `json:"actor"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				c, err := resolve.GetCharacter(s, a.Actor)
				if err != nil {
					return Result{}, err
				}
				return Result{OK: true, Summary: sheetSummary(c), Facts: characterFacts(c)}, nil
			},
		},
		{
			Name: "update_character",
			Description: "Update a character's free-text fields — concept, appearance, " +
				"notes. Use for narrative edits only; characteristics, skills, inventory " +
				"and conditions move through their own tools." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character ID to update."),
				"changes": objProp("Fields to replace; set at least one. Mechanical "+
					"values never belong here.", map[string]any{
					"concept":    strProp("New concept text (empty string clears it)."),
					"appearance": strProp("New appearance text."),
					"notes":      strProp("New notes text."),
				}),
			}, "actor", "changes"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor   string `json:"actor"`
					Changes *struct {
						Concept    *string `json:"concept"`
						Appearance *string `json:"appearance"`
						Notes      *string `json:"notes"`
					} `json:"changes"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if a.Changes == nil {
					return Result{}, fmt.Errorf("%w: missing required field %q", ErrInvalidArgs, "changes")
				}
				if a.Changes.Concept == nil && a.Changes.Appearance == nil && a.Changes.Notes == nil {
					return Result{}, fmt.Errorf("%w: changes must set at least one of concept, appearance, notes", ErrInvalidArgs)
				}
				res, err := resolve.Apply(s, resolve.UpdateCharacter{
					Actor: a.Actor, Concept: a.Changes.Concept,
					Appearance: a.Changes.Appearance, Notes: a.Changes.Notes,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "list_characters",
			Description: "List every character with an id, name, tags and status line. " +
				"Use to recall who is in play before narrating or calling other tools." + factsNote,
			Schema: objSchema(map[string]any{
				"filter": enumProp("Restrict the listing; omit for everyone.", "pc", "npc"),
			}),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Filter string `json:"filter"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if a.Filter != "" && a.Filter != "pc" && a.Filter != "npc" {
					return Result{}, fmt.Errorf("%w: filter must be \"pc\" or \"npc\" (got %q)", ErrInvalidArgs, a.Filter)
				}
				people := resolve.ListCharacters(s)
				facts := make([]string, 0, len(people)+1)
				shown := 0
				for _, c := range people {
					if a.Filter == "pc" && !c.PC || a.Filter == "npc" && c.PC {
						continue
					}
					shown++
					facts = append(facts, fmt.Sprintf("%s: %s (%s, %s) — %s", c.ID, c.Name, kindLabel(c), c.Career, lifeLabel(c)))
				}
				if shown == 0 {
					facts = append(facts, "no characters match the filter")
				}
				return Result{
					OK:      true,
					Summary: fmt.Sprintf("%d character(s) listed", shown),
					Facts:   facts,
				}, nil
			},
		},
		{
			Name: "skill_advance",
			Description: "Spend an Experience Roll to improve a skill or Passion, or " +
				"apply the free +1% a fumble grants (free: true). Use at the improvement " +
				"step; each target takes one Experience Roll per session." + factsNote,
			Schema: objSchema(map[string]any{
				"actor": strProp("Character ID improving."),
				"skill": strProp("Skill or Passion name to improve."),
				"free":  boolProp("Apply the free fumble +1% instead of spending an Experience Roll (default false)."),
			}, "actor", "skill"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor string `json:"actor"`
					Skill string `json:"skill"`
					Free  bool   `json:"free"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("skill", a.Skill); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.SkillAdvance{Actor: a.Actor, Skill: a.Skill, Free: a.Free})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
	}
}

// kindLabel renders the PC/NPC tag for status facts.
func kindLabel(c *char.Character) string {
	if c.PC {
		return "PC"
	}
	return "NPC"
}

// lifeLabel renders the life flag and wound count for a status line.
func lifeLabel(c *char.Character) string {
	if c.Dead {
		return "dead"
	}
	return fmt.Sprintf("alive, %d wound(s)", len(c.Wounds))
}

// sheetSummary is the one-line get_character summary.
func sheetSummary(c *char.Character) string {
	status := "alive"
	if c.Dead {
		status = "dead"
	}
	return fmt.Sprintf("%s sheet: %s (%s) — %s, %d total hit points",
		c.ID, c.Name, kindLabel(c), status, c.Derived.HitPoints)
}

// characterFacts renders a full sheet as narration-ready facts: identity,
// characteristics, derived values, live hit points per location, wounds,
// conditions, inventory, skills, passions and combat styles.
func characterFacts(c *char.Character) []string {
	facts := []string{
		fmt.Sprintf("%s: %s (%s)", c.ID, c.Name, kindLabel(c)),
		fmt.Sprintf("culture: %s; career: %s; age: %d", orDash(c.Culture), orDash(c.Career), c.Age),
		fmt.Sprintf("characteristics: STR %d, CON %d, SIZ %d, DEX %d, INT %d, POW %d, CHA %d",
			c.Chars.STR, c.Chars.CON, c.Chars.SIZ, c.Chars.DEX, c.Chars.INT, c.Chars.POW, c.Chars.CHA),
		fmt.Sprintf("derived: hit points %d, damage modifier %s, magic points %d, luck %d, action points %d, initiative %d, movement %d",
			c.Derived.HitPoints, c.Derived.DamageModifier, c.Derived.MagicPoints,
			c.Derived.LuckPoints, c.Derived.ActionPoints, c.Derived.Initiative, c.Derived.MovementRate),
	}
	if c.Concept != "" {
		facts = append(facts, "concept: "+c.Concept)
	}
	if c.Appearance != "" {
		facts = append(facts, "appearance: "+c.Appearance)
	}
	if c.Notes != "" {
		facts = append(facts, "notes: "+c.Notes)
	}
	if c.Dead {
		facts = append(facts, "dead: true")
	}
	facts = append(facts, "hit points by location: "+locationLine(c))
	facts = append(facts, "wounds: "+woundLine(c))
	facts = append(facts, "conditions: "+conditionLine(c))
	facts = append(facts, "inventory: "+inventoryLine(c))
	facts = append(facts, "skills: "+skillLine(c))
	facts = append(facts, "passions: "+passionLine(c))
	facts = append(facts, "combat styles: "+styleLine(c))
	return facts
}

// locationLine renders live and starting hit points per location in catalog
// order.
func locationLine(c *char.Character) string {
	parts := make([]string, 0, len(c.Derived.Locations))
	for _, loc := range c.Derived.Locations {
		parts = append(parts, fmt.Sprintf("%s %d/%d", loc.Name, c.CurrentHP[loc.Name], c.Derived.LocationHP[loc.Name]))
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ", ")
}

// woundLine renders the injury log.
func woundLine(c *char.Character) string {
	if len(c.Wounds) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(c.Wounds))
	for _, w := range c.Wounds {
		parts = append(parts, fmt.Sprintf("%s %d damage (%s)", w.Location, w.Damage, w.Severity))
	}
	return strings.Join(parts, ", ")
}

// conditionLine renders active conditions.
func conditionLine(c *char.Character) string {
	if len(c.Conditions) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(c.Conditions))
	for _, cond := range c.Conditions {
		line := cond.Name
		if cond.Location != "" {
			line += " (" + cond.Location
			if cond.Rounds > 0 {
				line += fmt.Sprintf(", %d rounds", cond.Rounds)
			}
			line += ")"
		} else if cond.Rounds > 0 {
			line += fmt.Sprintf(" (%d rounds)", cond.Rounds)
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, ", ")
}

// inventoryLine renders every inventory item with quantity and equip state.
func inventoryLine(c *char.Character) string {
	if len(c.Inventory) == 0 {
		return "(empty)"
	}
	parts := make([]string, 0, len(c.Inventory))
	for _, it := range c.Inventory {
		if it == nil {
			continue
		}
		line := fmt.Sprintf("%s ×%d (%s)", it.Name, it.Qty, it.ID)
		if it.Equipped {
			line += " equipped"
			if it.Slot != "" {
				line += " " + it.Slot
			}
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, ", ")
}

// skillLine renders every skill rating, sorted by name.
func skillLine(c *char.Character) string {
	if len(c.Skills) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(c.Skills))
	for _, name := range slices.Sorted(maps.Keys(c.Skills)) {
		if sk := c.Skills[name]; sk != nil {
			parts = append(parts, fmt.Sprintf("%s %d%%", name, sk.Value))
		}
	}
	return strings.Join(parts, ", ")
}

// passionLine renders every Passion, sorted by name.
func passionLine(c *char.Character) string {
	if len(c.Passions) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(c.Passions))
	for _, name := range slices.Sorted(maps.Keys(c.Passions)) {
		if p := c.Passions[name]; p != nil {
			parts = append(parts, fmt.Sprintf("%s %d%%", name, p.Value))
		}
	}
	return strings.Join(parts, ", ")
}

// styleLine renders every combat style, sorted by name.
func styleLine(c *char.Character) string {
	if len(c.CombatStyles) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(c.CombatStyles))
	for _, name := range slices.Sorted(maps.Keys(c.CombatStyles)) {
		if st := c.CombatStyles[name]; st != nil {
			parts = append(parts, fmt.Sprintf("%s %d%%", name, st.Value))
		}
	}
	return strings.Join(parts, ", ")
}

// orDash renders an empty string readably in facts.
func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
