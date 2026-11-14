package resolve

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
	"github.com/mythras-ultra/myul/internal/engine/rules"
)

// CreateCharacter builds a PC or NPC from the digest's creation steps:
// characteristics (rolled when not supplied), culture, career, age band,
// skill overrides, then Derive/Refresh for the cached attributes
// (tool: create_character; docs/rules/digest.md Steps 2–7).
type CreateCharacter struct {
	// Name is the character's name (required).
	Name string `json:"name"`
	// PC marks a player character; false creates an NPC. The ID prefix
	// follows: engine-generated "pc-N" / "npc-N".
	PC bool `json:"pc"`
	// Culture and Career are required package names (digest Steps 5–6);
	// unknown names refuse the move before anything is stored.
	Culture string `json:"culture"`
	Career  string `json:"career"`
	// Age optionally sizes the Bonus pool from the Experience Table
	// (digest Step 7); 0 means unset.
	Age int `json:"age,omitempty"`
	// Characteristics optionally supplies the seven characteristics; nil
	// rolls them on the session RNG with the standard method (rules are
	// never invented — char.Generate owns the table).
	Characteristics *char.Characteristics `json:"characteristics,omitempty"`
	// Skills optionally sets starting skill ratings after culture and
	// career grant their bases (GM override, applied in sorted name order).
	Skills map[string]int `json:"skills,omitempty"`
}

// Kind implements [Move].
func (m CreateCharacter) Kind() string { return "create_character" }

// Apply builds the character completely off-state and only inserts it into
// State.Characters at the end, so any validation or package error leaves
// the state untouched. Characteristics are rolled before the packages are
// checked — a failed creation consumes those draws but [Apply] rolls the
// RNG back with the state.
func (m CreateCharacter) Apply(s *State) (Result, error) {
	if strings.TrimSpace(m.Name) == "" {
		return Result{}, fmt.Errorf("%w: empty character name", ErrInvalidArgument)
	}
	if strings.TrimSpace(m.Culture) == "" {
		return Result{}, fmt.Errorf("%w: culture is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(m.Career) == "" {
		return Result{}, fmt.Errorf("%w: career is required", ErrInvalidArgument)
	}
	chars := m.Characteristics
	source := "assigned"
	if chars == nil {
		generated, err := char.Generate(s.Rand(), char.GenRoll)
		if err != nil {
			return Result{}, err
		}
		chars = &generated
		source = "rolled"
	}
	c := &char.Character{
		Name:  m.Name,
		PC:    m.PC,
		Chars: *chars,
	}
	// ApplyCulture/ApplyCareer grant the Standard skills at their
	// characteristic bases and build the point pools (digest Steps 4–6);
	// they validate the package names.
	if err := char.ApplyCulture(c, m.Culture); err != nil {
		return Result{}, err
	}
	if err := char.ApplyCareer(c, m.Career); err != nil {
		return Result{}, err
	}
	if m.Age != 0 {
		if err := c.ApplyAge(m.Age); err != nil {
			return Result{}, err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(m.Skills)) {
		if err := c.SetSkill(name, m.Skills[name]); err != nil {
			return Result{}, err
		}
	}
	if err := c.Refresh(); err != nil {
		return Result{}, err
	}
	prefix := "npc"
	kind := "NPC"
	if m.PC {
		prefix = "pc"
		kind = "player character"
	}
	c.ID = char.NextID(prefix, s.characterIDs())
	if s.Characters == nil { // hand-built states: never panic on insert
		s.Characters = make(map[string]*char.Character)
	}
	s.Characters[c.ID] = c

	d := c.Derived
	facts := []string{
		"id: " + c.ID,
		fmt.Sprintf("name: %s (%s)", c.Name, kind),
		"culture: " + m.Culture,
		"career: " + m.Career,
		fmt.Sprintf("characteristics (%s): STR %d, CON %d, SIZ %d, DEX %d, INT %d, POW %d, CHA %d",
			source, c.Chars.STR, c.Chars.CON, c.Chars.SIZ, c.Chars.DEX, c.Chars.INT, c.Chars.POW, c.Chars.CHA),
		fmt.Sprintf("derived: hit points %d, damage modifier %s, luck %d, magic points %d, initiative %d",
			d.HitPoints, d.DamageModifier, d.LuckPoints, d.MagicPoints, d.Initiative),
	}
	if m.Age != 0 && c.Pools != nil {
		facts = append(facts, fmt.Sprintf("age: %d (bonus pool %d, max increase %d)", m.Age, c.Pools.Bonus, c.Pools.MaxIncrease))
	}
	if c.Pools != nil {
		facts = append(facts, fmt.Sprintf("creation pools left: culture %d, career %d, bonus %d",
			c.Pools.Culture, c.Pools.Career, c.Pools.Bonus))
	}
	for _, name := range slices.Sorted(maps.Keys(m.Skills)) {
		sk, _ := c.Skill(name)
		facts = append(facts, fmt.Sprintf("skill %s: %d", name, sk.Value))
	}
	a := Audit{}
	a.addCitation("Mythras Imperative, Characters: Step 2: Characteristics")
	a.addCitation("Mythras Imperative, Characters: Step 5: Culture")
	a.addCitation("Mythras Imperative, Characters: Step 6: Career")
	summary := fmt.Sprintf("created %s %s: %s (%s %s)", kind, c.ID, c.Name, m.Culture, m.Career)
	return newResult(summary, facts, a), nil
}

// EquipmentSpec is one item granted by [FinalizeCharacter]: a sample-table
// name (weapon, armour or shield) is taken with its game statistics, any
// other name becomes a plain statless item.
type EquipmentSpec struct {
	// Name is the item's name.
	Name string `json:"name"`
	// Qty is the quantity; <= 0 means 1.
	Qty int `json:"qty,omitempty"`
}

// PassionSpec is one starting Passion granted by [FinalizeCharacter],
// valued by char.AddPassion from its kind and importance (digest Step 8).
type PassionSpec struct {
	// Name is the Passion's object ("Hate (Orcs)").
	Name string `json:"name"`
	// Kind is the Passion Table row ("person", "race", "place", ...).
	Kind string `json:"kind"`
	// Order is the Passion's importance, 1–3; 0 means 1.
	Order int `json:"order,omitempty"`
}

// FinalizeCharacter adds starting equipment and Passions and reports the
// finished sheet's totals (tool: finalize_character — HP/PP/DM totals and
// carried bulk).
type FinalizeCharacter struct {
	// Actor is the character being finished (ID).
	Actor string `json:"actor"`
	// Equipment adds inventory items, each with an engine-generated
	// "item-N" ID.
	Equipment []EquipmentSpec `json:"equipment,omitempty"`
	// Passions adds or replaces Passions on the sheet.
	Passions []PassionSpec `json:"passions,omitempty"`
}

// Kind implements [Move].
func (m FinalizeCharacter) Kind() string { return "finalize_character" }

// Apply validates every spec (item names, passion kinds and importance)
// before mutating, adds them through the char APIs, refreshes the derived
// snapshot and reports totals. It may run repeatedly — additions are
// idempotent in the sense that Passion records replace and items append.
func (m FinalizeCharacter) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	for _, spec := range m.Equipment {
		if strings.TrimSpace(spec.Name) == "" {
			return Result{}, fmt.Errorf("%w: empty equipment name", ErrInvalidArgument)
		}
	}
	for _, spec := range m.Passions {
		if strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Kind) == "" {
			return Result{}, fmt.Errorf("%w: passion needs a name and a kind", ErrInvalidArgument)
		}
		order := spec.Order
		if order == 0 {
			order = 1
		}
		if _, ok := char.PassionBase(spec.Kind, c.Chars); !ok {
			return Result{}, fmt.Errorf("%w: unknown passion kind %q", char.ErrInvalidPassion, spec.Kind)
		}
		if _, ok := char.PassionBonus(order); !ok {
			return Result{}, fmt.Errorf("%w: importance %d outside 1–3", char.ErrInvalidPassion, order)
		}
	}

	carriedBefore := c.Carrying()
	var facts, changes []string
	a := Audit{}
	for _, spec := range m.Equipment {
		item, detailed := buildItem(spec.Name, spec.Qty)
		if err := c.AddItem(item); err != nil {
			return Result{}, err
		}
		line := fmt.Sprintf("added: %s ×%d (%s)", item.Name, item.Qty, item.ID)
		if !detailed {
			line += " — no game statistics"
		}
		facts = append(facts, line)
		changes = append(changes, fmt.Sprintf("inventory: +%s ×%d (%s)", item.Name, item.Qty, item.ID))
	}
	for _, spec := range m.Passions {
		order := spec.Order
		if order == 0 {
			order = 1
		}
		if err := c.AddPassion(spec.Name, spec.Kind, order); err != nil {
			return Result{}, err
		}
		value := c.Passions[spec.Name].Value
		facts = append(facts, fmt.Sprintf("passion: %s = %d%% (%s, importance %d)", spec.Name, value, spec.Kind, order))
		changes = append(changes, fmt.Sprintf("passion: %s = %d%%", spec.Name, value))
	}
	if err := c.Refresh(); err != nil {
		return Result{}, err
	}
	d := c.Derived
	carriedAfter := c.Carrying()
	facts = append(facts,
		fmt.Sprintf("totals: hit points %d, damage modifier %s, magic points %d, power points %d",
			d.HitPoints, d.DamageModifier, d.MagicPoints, d.PowerPoints),
		fmt.Sprintf("carried: %d items (advisory — Mythras Imperative has no encumbrance rules)", carriedAfter),
	)
	if carriedBefore != carriedAfter {
		changes = append(changes, change("carried", carriedBefore, carriedAfter))
	}
	a.Changes = changes
	a.addCitation("Mythras Imperative, Characters: Step 8: Passions")
	a.addCitation("Mythras Imperative, Characters: Step 9: Equipment")
	summary := fmt.Sprintf("%s finalized: %d equipment item(s), %d passion(s), HP %d, DM %s",
		c.ID, len(m.Equipment), len(m.Passions), d.HitPoints, d.DamageModifier)
	return newResult(summary, facts, a), nil
}

// UpdateCharacter rewrites the character's free-text fields — concept,
// appearance and notes (tool: update_character). Mechanical changes never
// belong here: characteristics, skills, inventory and conditions move
// through their own moves (golden rule 1).
type UpdateCharacter struct {
	// Actor is the character to update (ID).
	Actor string `json:"actor"`
	// Concept, Appearance and Notes are the free-text fields; nil leaves a
	// field untouched, a non-nil value (including "") replaces it.
	Concept    *string `json:"concept,omitempty"`
	Appearance *string `json:"appearance,omitempty"`
	Notes      *string `json:"notes,omitempty"`
}

// Kind implements [Move].
func (m UpdateCharacter) Kind() string { return "update_character" }

// Apply validates the actor and that at least one field was supplied, then
// replaces only the given fields and reports the before → after diff.
func (m UpdateCharacter) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if m.Concept == nil && m.Appearance == nil && m.Notes == nil {
		return Result{}, fmt.Errorf("%w: no changes supplied (concept, appearance, notes)", ErrInvalidArgument)
	}
	var facts, changes []string
	applyField := func(field string, dst *string, src *string) {
		if src == nil {
			return
		}
		before := *dst
		*dst = *src
		line := change(field, orNone(before), orNone(*src))
		facts = append(facts, line)
		changes = append(changes, line)
	}
	applyField("concept", &c.Concept, m.Concept)
	applyField("appearance", &c.Appearance, m.Appearance)
	applyField("notes", &c.Notes, m.Notes)
	summary := fmt.Sprintf("%s updated (%d field(s))", c.ID, len(facts))
	return newResult(summary, facts, Audit{Changes: changes}), nil
}

// SkillAdvance spends one Experience Roll on a skill or passion, or applies
// the free +1% a fumble grants (tool: skill_advance; Game System › Character
// Improvement).
type SkillAdvance struct {
	// Actor is the character improving (ID).
	Actor string `json:"actor"`
	// Skill is the skill or passion name to improve.
	Skill string `json:"skill"`
	// Free selects the free fumble increase (rules.ApplyFumbleIncrease)
	// instead of spending an Experience Roll (rules.ExperienceRoll).
	Free bool `json:"free,omitempty"`
}

// Kind implements [Move].
func (m SkillAdvance) Kind() string { return "skill_advance" }

// Apply validates the character and the target first; rules.ErrUnknownSkill
// and rules.ErrAlreadyExperienced propagate so a refused advance changes
// nothing. The audit carries the experience d100 (and the improvement die
// when one was rolled) with the Character Improvement citation.
func (m SkillAdvance) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if m.Skill == "" {
		return Result{}, fmt.Errorf("%w: empty skill name", ErrInvalidArgument)
	}
	a := Audit{}
	a.addCitation("Mythras Imperative, Game System: Character Improvement")

	if m.Free {
		granted, err := rules.ApplyFumbleIncrease(c, m.Skill)
		if err != nil {
			return Result{}, err
		}
		if !granted {
			return newResult(fmt.Sprintf("%s: no pending fumble increase for %s", c.ID, m.Skill),
				[]string{m.Skill + ": no free +1% pending"}, a), nil
		}
		facts := []string{m.Skill + ": free fumble increase applied (pending flag cleared)"}
		if value, ok := currentValue(c, m.Skill); ok {
			line := change(m.Skill, value-1, value)
			facts = append(facts, line)
			a.Changes = append(a.Changes, line)
		}
		summary := fmt.Sprintf("%s advances by free fumble increase: %s", c.ID, m.Skill)
		return newResult(summary, facts, a), nil
	}

	gain, err := rules.ExperienceRoll(s.Rand(), c, m.Skill)
	if err != nil {
		return Result{}, err
	}
	facts := []string{fmt.Sprintf("experience roll: %d + INT %d vs %d", gain.Roll, gain.INT, gain.Old+freeOffset(gain))}
	facts = append(facts, change(m.Skill, gain.Old, gain.New))
	if gain.Free {
		facts = append(facts, "pending fumble +1% applied before the roll")
	}
	a.addRoll(d100Roll(gain.Roll))
	if gain.Dice > 1 {
		// The improvement die is only drawn on a hit: 1d4+1 lands in 2..5.
		a.addRoll(dice.Roll{
			Expr:     "1d4+1",
			Dice:     []dice.Die{{Sides: 4, Value: gain.Dice - 1}},
			Modifier: 1,
			Total:    gain.Dice,
		})
	}
	a.Changes = append(a.Changes, change(m.Skill, gain.Old, gain.New))
	summary := fmt.Sprintf("%s advances: %s", c.ID, change(m.Skill, gain.Old, gain.New))
	return newResult(summary, facts, a), nil
}

// currentValue returns the current rating of a skill or Passion by name
// (the sheet first, then Passions — the rules rate both alike), so results
// can quote after-values for either kind of target.
func currentValue(c *char.Character, name string) (int, bool) {
	if sk, ok := c.Skill(name); ok && sk != nil {
		return sk.Value, true
	}
	if p, ok := c.Passions[name]; ok && p != nil {
		return p.Value, true
	}
	return 0, false
}

// freeOffset reports the skill value the experience roll compared against,
// before the free fumble +1% was folded in: Gain.Old already includes it
// when Gain.Free is set, so the comparison line shows the number the rules
// used (roll + INT ≥ value-at-roll-time).
func freeOffset(gain rules.Gain) int {
	if gain.Free {
		return 1
	}
	return 0
}

// orNone renders an empty before-value readably in diffs.
func orNone(v string) string {
	if v == "" {
		return "(none)"
	}
	return v
}
