// Package char models a Mythras Imperative character — player character or
// non-player character — as the deterministic data core of the engine:
// characteristics, derived attributes, skills, passions, combat styles,
// inventory, hit locations, wounds and conditions.
//
// The package is a pure data/Bookkeeping layer: it imports only the standard
// library and internal/engine/dice, it never rolls dice itself except through
// an explicit *dice.Rand passed to Generate, and it never invents rules. Every
// table it carries cites Mythras Imperative (via docs/rules/tables.md and
// docs/rules/digest.md); where the source is ambiguous the choice made here is
// documented with an [AMBIGUOUS: ...] comment, and where a rule does not exist
// in the source (encumbrance, prices, aging penalties) none is implemented.
//
// State mutates only through engine operations; this package provides the
// bookkeeping those operations (engine/resolve) call.
package char

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Sentinel errors. Callers (harness tools, resolve moves) branch on these
// with errors.Is to turn a refusal into a structured IMPOSSIBLE result.
var (
	// ErrUnknownSkill reports a reference to a skill name that is not in the
	// skill catalogue and not acceptable in context (e.g. an empty name).
	// Note that SetSkill deliberately accepts unknown names and flags them
	// Custom; this sentinel is for paths that require a catalogue skill.
	ErrUnknownSkill = errors.New("char: unknown skill")

	// ErrUnknownCulture reports an unknown or absent culture name.
	ErrUnknownCulture = errors.New("char: unknown culture")

	// ErrUnknownCareer reports an unknown or absent career name.
	ErrUnknownCareer = errors.New("char: unknown career")

	// ErrUnknownCatalog reports an unknown hit-location catalog name.
	ErrUnknownCatalog = errors.New("char: unknown hit location catalog")

	// ErrInvalidLocation reports a hit location that is not part of the
	// character's hit-location catalog.
	ErrInvalidLocation = errors.New("char: invalid hit location")

	// ErrNoAmmo reports an ammunition requirement that the inventory cannot
	// meet (no ammunition item at all, or too few rounds to consume).
	ErrNoAmmo = errors.New("char: no ammunition")

	// ErrItemNotFound reports an inventory item ID that does not exist.
	ErrItemNotFound = errors.New("char: item not found")

	// ErrInvalidSlot reports an empty or otherwise invalid equip slot name.
	ErrInvalidSlot = errors.New("char: invalid equip slot")

	// ErrUnknownPool reports an unknown creation point pool name.
	ErrUnknownPool = errors.New("char: unknown point pool")

	// ErrPoolExhausted reports a point-pool spend larger than the points
	// remaining in that pool.
	ErrPoolExhausted = errors.New("char: point pool exhausted")

	// ErrSkillNotInPool reports a skill that is not eligible for the pool
	// being spent (culture/career packages, or the bonus-pool rules).
	ErrSkillNotInPool = errors.New("char: skill not eligible for pool")

	// ErrTooManyPicks reports a spend that would exceed a package's pick
	// limits (up to 3 Professional skills; choice groups pick N).
	ErrTooManyPicks = errors.New("char: too many picks")

	// ErrIncreaseCap reports a bonus-pool spend that would exceed the
	// Maximum Skill Increase of the character's Experience Table band.
	ErrIncreaseCap = errors.New("char: maximum skill increase exceeded")

	// ErrInvalidAmount reports a negative or otherwise unacceptable
	// quantity, point count, damage or heal amount.
	ErrInvalidAmount = errors.New("char: invalid amount")

	// ErrInvalidCharacteristic reports a negative characteristic value
	// (zero is legal — the rules define zero-characteristic effects).
	ErrInvalidCharacteristic = errors.New("char: invalid characteristic")

	// ErrDamageModifierRange reports STR+SIZ beyond the Damage Modifier
	// table (Step 3): the table's "Continue Progression" row beyond 120 is
	// undefined, so no modifier can be computed.
	// [AMBIGUOUS: tables.md §1.2 — "Each 10 points / Continue Progression"
	// gives no terms beyond STR+SIZ 120; the engine refuses to invent them.]
	ErrDamageModifierRange = errors.New("char: STR+SIZ beyond damage modifier table")

	// ErrUnknownGenMethod reports an out-of-range GenMethod.
	ErrUnknownGenMethod = errors.New("char: unknown generation method")

	// ErrUnknownEquipment reports an unknown name in the sample equipment
	// tables (weapons, armor, shields).
	ErrUnknownEquipment = errors.New("char: unknown equipment")

	// ErrInvalidAge reports a negative age passed to ApplyAge.
	ErrInvalidAge = errors.New("char: invalid age")

	// ErrInvalidPassion reports an unknown passion kind or an importance
	// order outside 1–3.
	ErrInvalidPassion = errors.New("char: invalid passion")
)

// stat indexes a single characteristic. The order matches the declaration
// order of Characteristics and is used by base-formula evaluation.
type stat uint8

const (
	statSTR stat = iota
	statCON
	statSIZ
	statDEX
	statINT
	statPOW
	statCHA
)

// statNames lists the characteristics in stat order (for error messages).
var statNames = [...]string{"STR", "CON", "SIZ", "DEX", "INT", "POW", "CHA"}

// Characteristics holds the seven characteristics (Step 2, tables.md §1.1).
// Zero is a legal value (the rules define zero-characteristic effects, e.g.
// 0 CON = death); negative values are rejected by Derive.
type Characteristics struct {
	STR int `json:"str"`
	CON int `json:"con"`
	SIZ int `json:"siz"`
	DEX int `json:"dex"`
	INT int `json:"int"`
	POW int `json:"pow"`
	CHA int `json:"cha"`
}

// at returns the value of the characteristic indexed by s.
func (ch *Characteristics) at(s stat) int {
	switch s {
	case statSTR:
		return ch.STR
	case statCON:
		return ch.CON
	case statSIZ:
		return ch.SIZ
	case statDEX:
		return ch.DEX
	case statINT:
		return ch.INT
	case statPOW:
		return ch.POW
	case statCHA:
		return ch.CHA
	}
	return 0
}

// ptr returns a pointer to the characteristic indexed by s so generation
// methods can assign in place.
func (ch *Characteristics) ptr(s stat) *int {
	switch s {
	case statSTR:
		return &ch.STR
	case statCON:
		return &ch.CON
	case statSIZ:
		return &ch.SIZ
	case statDEX:
		return &ch.DEX
	case statINT:
		return &ch.INT
	case statPOW:
		return &ch.POW
	case statCHA:
		return &ch.CHA
	}
	return nil
}

// validate reports the first negative characteristic, if any, wrapping
// [ErrInvalidCharacteristic].
func (ch *Characteristics) validate() error {
	for s := statSTR; s <= statCHA; s++ {
		if ch.at(s) < 0 {
			return fmt.Errorf("%w: %s is %d", ErrInvalidCharacteristic, statNames[s], ch.at(s))
		}
	}
	return nil
}

// Skill is one entry of a character's skill map. Base is the characteristic
// formula result (informational — Value already includes it); Value is the
// rating rolled against.
//
// Fumbled and Advanced are the two experience flags the improvement rules
// (Game System › Character Improvement, tables.md §2.6) need: a skill that
// fumbled since the last award gains a free +1%, and each skill may receive
// one Experience Roll per session.
type Skill struct {
	Base     int    `json:"base"`
	Value    int    `json:"value"`
	Fumbled  bool   `json:"fumbled,omitempty"`
	Advanced bool   `json:"advanced,omitempty"`
	Custom   bool   `json:"custom,omitempty"` // not in the rulebook catalogue (GM-added)
	Notes    string `json:"notes,omitempty"`
}

// Passion is one of the character's Passions (Step 8, tables.md §1.17).
// Basis records which Passion Table row produced Value (a characteristic
// formula or one of PassionKinds' keys).
type Passion struct {
	Value int    `json:"value"`
	Basis string `json:"basis,omitempty"`
	Notes string `json:"notes,omitempty"`

	// Fumbled and Advanced are the two experience flags the improvement rules
	// need (passions improve like skills: Mythras-Imperative.md, Character
	// Improvement — "Passions: improved like skills"). Fumbled grants the
	// free +1%; Advanced enforces one Experience Roll per session.
	Fumbled  bool `json:"fumbled,omitempty"`
	Advanced bool `json:"advanced,omitempty"`
}

// CombatStyle is a weapon-package skill (Step 4; Combat › Combat Styles).
// Base is always STR+DEX. Traits are entries of the Combat Style Traits
// table (tables.md §4.3).
type CombatStyle struct {
	Base    int      `json:"base"`
	Value   int      `json:"value"`
	Weapons []string `json:"weapons,omitempty"`
	Traits  []string `json:"traits,omitempty"`
	Notes   string   `json:"notes,omitempty"`
}

// Condition names. The first group transcribes the Conditions Table
// (Game System › Disease and Poison, tables.md §3.3); the second covers the
// combat states named by the wound and combat rules (Damage and Wound
// Levels; Special Effects).
const (
	// CondAgony is the Agony condition (tables.md §3.3).
	CondAgony = "Agony"
	// CondAsphyxiation is the Asphyxiation condition (tables.md §3.3).
	CondAsphyxiation = "Asphyxiation"
	// CondBleeding is the Bleeding condition (tables.md §3.3).
	CondBleeding = "Bleeding"
	// CondContagious is the Contagious condition (tables.md §3.3).
	CondContagious = "Contagious"
	// CondDeath is the Death condition: the victim dies after a number of
	// rounds equal to CON (tables.md §3.3). The round countdown is resolved
	// by combat; char carries the condition only.
	CondDeath = "Death"
	// CondFever is the Fever condition (tables.md §3.3).
	CondFever = "Fever"
	// CondMania is the Mania condition (tables.md §3.3).
	CondMania = "Mania"
	// CondNausea is the Nausea condition (tables.md §3.3).
	CondNausea = "Nausea"
	// CondParalysis is the Paralysis condition (tables.md §3.3).
	CondParalysis = "Paralysis"
	// CondProne is the prone state (Evade, Major Wound, knockback, etc.).
	CondProne = "Prone"
	// CondStunned is the Stunned state (Special Effect: Stun Location).
	CondStunned = "Stunned"
	// CondUnconscious is unconsciousness (Major Wound, failed Endurance
	// rolls — the roll itself is resolved by combat).
	CondUnconscious = "Unconscious"
	// CondIncapacitated is incapacitation (a Major Wound imposes it
	// immediately; the Fatigue table also uses the term).
	CondIncapacitated = "Incapacitated"
)

// Condition is one active condition on a character. Rounds is the remaining
// number of Combat Rounds for timed conditions; 0 means the condition lasts
// until explicitly removed (or a duration the rules express in longer units,
// carried in Notes). Location optionally scopes the condition to one hit
// location (e.g. a stunned arm).
type Condition struct {
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
	Rounds   int    `json:"rounds,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// Character is a PC or NPC: identity, characteristics, the cached Derived
// snapshot, skills, passions, combat styles, inventory, per-location hit
// points, wounds, conditions and life flags. All exported fields round-trip
// through JSON cleanly for state.json.
//
// Dead is a flag rather than Alive so the zero value means "alive" — a fresh
// Character literal is a living character. Unconscious, prone, stunned and
// similar states are Conditions, not fields, because the rules give them
// durations and locations.
type Character struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	PC      bool   `json:"pc,omitempty"`
	Culture string `json:"culture,omitempty"`
	Career  string `json:"career,omitempty"`
	Age     int    `json:"age,omitempty"`
	// Concept, Appearance and Notes are free text (Step 1: concept is
	// narrative only and has no mechanics).
	Concept    string `json:"concept,omitempty"`
	Appearance string `json:"appearance,omitempty"`
	Notes      string `json:"notes,omitempty"`

	Chars Characteristics `json:"characteristics"`

	Skills       map[string]*Skill       `json:"skills,omitempty"`
	Passions     map[string]*Passion     `json:"passions,omitempty"`
	CombatStyles map[string]*CombatStyle `json:"combatStyles,omitempty"`
	Inventory    []*Item                 `json:"inventory,omitempty"`

	// CurrentHP holds the character's current Hit Points per hit location;
	// it is initialised and topped up for missing entries by Refresh.
	// The starting values live in Derived.LocationHP.
	CurrentHP map[string]int `json:"currentHP,omitempty"`
	// Wounds is the injury log: one record per applied wound. Severities
	// reflect the moment of injury; current severity is recomputed from
	// CurrentHP whenever needed.
	Wounds     []Wound     `json:"wounds,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`

	// HitLocationCatalog selects the catalog used for hit locations; empty
	// means DefaultCatalog ("humanoid").
	HitLocationCatalog string `json:"hitLocationCatalog,omitempty"`

	// Pools holds the Step 4–7 creation point pools while the character is
	// being built; nil after creation (or for NPCs built without pools).
	Pools *Pools `json:"pools,omitempty"`

	// Derived is a cached snapshot — regenerate via Derive/Refresh; never
	// edit it by hand.
	Derived Derived `json:"derived"`

	// Dead reports death. Set (never cleared) by Refresh when CON reaches
	// 0 (Step 2: 0 CON = death); Major-Wound death timers are resolved by
	// combat, which then sets this flag.
	Dead bool `json:"dead,omitempty"`
}

// NextID returns the next unused engine identifier of the "prefix-N" form
// given the identifiers already in use: the largest N found plus one. IDs
// are never reused — a deleted "npc-3" is not handed out again, so the
// append-only session log can never show two different entities under one
// ID. Strings that do not match "prefix-<positive integer>" are ignored; an
// empty prefix is treated as "id".
func NextID(prefix string, existing []string) string {
	if prefix == "" {
		prefix = "id"
	}
	sep := prefix + "-"
	max := 0
	for _, id := range existing {
		if !strings.HasPrefix(id, sep) {
			continue
		}
		n, err := strconv.Atoi(id[len(sep):])
		if err != nil || n <= 0 || n <= max {
			continue
		}
		max = n
	}
	return prefix + "-" + strconv.Itoa(max+1)
}

// Refresh synchronises the character with its characteristics and catalog:
//
//  1. it recomputes the cached Derived snapshot (Derive);
//  2. it creates every missing Standard skill (Step 4: everyone has them)
//     and refreshes Base for every catalogue skill the character has,
//     keeping Value at least Base (a skill cannot fall below its base —
//     the rules only ever raise skills);
//  3. it initialises any hit location that has no current HP yet, without
//     touching existing values, so damage persists;
//  4. it sets Dead when CON is 0 (Step 2: 0 CON = death). It never clears
//     the flag — resurrection, if any, is the caller's explicit action.
//
// Refresh returns an error wrapping [ErrInvalidCharacteristic],
// [ErrDamageModifierRange] or [ErrUnknownCatalog] when Derive does.
func (c *Character) Refresh() error {
	if c == nil {
		return errors.New("char: nil character")
	}
	d, err := Derive(c)
	if err != nil {
		return err
	}
	c.Derived = d
	c.syncSkills()
	if c.CurrentHP == nil {
		c.CurrentHP = make(map[string]int, len(d.LocationHP))
	}
	for loc, hp := range d.LocationHP {
		if _, ok := c.CurrentHP[loc]; !ok {
			c.CurrentHP[loc] = hp
		}
	}
	if c.Chars.CON <= 0 {
		c.Dead = true
	}
	return nil
}

// syncSkills creates the Standard skills the character is missing at their
// characteristic base and refreshes Base (and the Value floor) for every
// catalogue skill present. Custom skills are left untouched.
func (c *Character) syncSkills() {
	if c.Skills == nil {
		c.Skills = make(map[string]*Skill, len(skillCatalogue))
	}
	for _, name := range StandardSkills() {
		if _, ok := c.Skills[name]; !ok {
			base, _ := BaseSkill(name, c.Chars)
			c.Skills[name] = &Skill{Base: base, Value: base}
		}
	}
	for name, sk := range c.Skills {
		if sk == nil {
			continue
		}
		base, known := BaseSkill(name, c.Chars)
		if !known {
			continue
		}
		sk.Base = base
		if sk.Value < base {
			sk.Value = base
		}
	}
}

// Skill returns the named skill from the character's skill map.
func (c *Character) Skill(name string) (*Skill, bool) {
	if c == nil || c.Skills == nil {
		return nil, false
	}
	sk, ok := c.Skills[name]
	if !ok || sk == nil {
		return nil, false
	}
	return sk, true
}

// SetSkill sets the character's rating for the named skill, creating the
// skill if necessary and refreshing its Base from the current
// characteristics. Names outside the rulebook catalogue are allowed and
// flagged Custom — the GM may add skills the rulebook does not list. For
// catalogue skills the value is floored at Base (a skill can never be below
// its characteristic base). A negative value is rejected wrapping
// [ErrInvalidAmount]; an empty name is rejected wrapping
// [ErrUnknownSkill].
func (c *Character) SetSkill(name string, value int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if name == "" {
		return fmt.Errorf("%w: empty skill name", ErrUnknownSkill)
	}
	if value < 0 {
		return fmt.Errorf("%w: skill %q value %d", ErrInvalidAmount, name, value)
	}
	if c.Skills == nil {
		c.Skills = make(map[string]*Skill, 1)
	}
	sk, ok := c.Skills[name]
	if !ok || sk == nil {
		sk = &Skill{}
		c.Skills[name] = sk
	}
	base, known := BaseSkill(name, c.Chars)
	sk.Custom = !known
	if known {
		sk.Base = base
		if value < base {
			value = base
		}
	}
	sk.Value = value
	return nil
}

// SetCombatStyle sets the value of the named combat style, creating it if
// necessary. Its Base is always STR+DEX (Step 4).
func (c *Character) SetCombatStyle(name string, value int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if name == "" {
		return errors.New("char: empty combat style name")
	}
	if value < 0 {
		return fmt.Errorf("%w: combat style %q value %d", ErrInvalidAmount, name, value)
	}
	if c.CombatStyles == nil {
		c.CombatStyles = make(map[string]*CombatStyle, 1)
	}
	cs, ok := c.CombatStyles[name]
	if !ok || cs == nil {
		cs = &CombatStyle{}
		c.CombatStyles[name] = cs
	}
	cs.Base = c.Chars.STR + c.Chars.DEX
	cs.Value = value
	return nil
}

// passionBasisTable transcribes the Passion Table (Step 8 › Passions,
// tables.md §1.17: "Object of Passion / Starting Percentage").
var passionBasisTable = []struct {
	key    string
	object string
	base   baseFormula
}{
	{"person", "A person, in a romantic or familial context", baseFormula{statPOW, statCHA, false, 0}},
	{"organization", "An organization or group of people", baseFormula{statPOW, statINT, false, 0}},
	{"race", "A race or species", baseFormula{statPOW, statPOW, true, 0}},
	{"place", "A place", baseFormula{statPOW, statINT, false, 0}},
	{"object", "An object or substance", baseFormula{statPOW, statPOW, true, 0}},
	{"trait", "A personality trait", baseFormula{statPOW, statCHA, false, 0}},
}

// PassionKind is one row of the Passion Table (Step 8, tables.md §1.17).
type PassionKind struct {
	// Key is the stable identifier accepted by PassionBase.
	Key string `json:"key"`
	// Object is the printed "Object of Passion" text.
	Object string `json:"object"`
	// Formula is the printed starting-percentage formula.
	Formula string `json:"formula"`
}

// PassionKinds returns copies of the Passion Table rows (tables.md §1.17).
func PassionKinds() []PassionKind {
	out := make([]PassionKind, 0, len(passionBasisTable))
	for _, row := range passionBasisTable {
		out = append(out, PassionKind{Key: row.key, Object: row.object, Formula: row.base.text()})
	}
	return out
}

// PassionBase returns the starting percentage of a passion of the given kind
// (a PassionKinds key) before the importance bonus, computed from the
// character's characteristics. The second result is false for unknown kinds.
func PassionBase(kind string, ch Characteristics) (int, bool) {
	for _, row := range passionBasisTable {
		if row.key == kind {
			return row.base.eval(ch), true
		}
	}
	return 0, false
}

// PassionBonus returns the starting bonus for a passion of the given
// importance: +40 to the first, +30 to the second, +20 to the third
// (Step 8, tables.md §1.17). The second result is false for orders outside
// 1–3.
func PassionBonus(order int) (int, bool) {
	switch order {
	case 1:
		return 40, true
	case 2:
		return 30, true
	case 3:
		return 20, true
	}
	return 0, false
}

// AddPassion records a starting Passion for the character: value = PassionBase
// + PassionBonus(order), with order 1–3 the passion's importance (Step 8).
// An existing passion of the same name is replaced. It wraps
// [ErrInvalidPassion] for an unknown kind or an importance outside 1–3.
func (c *Character) AddPassion(name, kind string, order int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if name == "" {
		return fmt.Errorf("%w: empty passion name", ErrInvalidPassion)
	}
	base, ok := PassionBase(kind, c.Chars)
	if !ok {
		return fmt.Errorf("%w: unknown passion kind %q", ErrInvalidPassion, kind)
	}
	bonus, ok := PassionBonus(order)
	if !ok {
		return fmt.Errorf("%w: importance %d outside 1–3", ErrInvalidPassion, order)
	}
	if c.Passions == nil {
		c.Passions = make(map[string]*Passion, 1)
	}
	c.Passions[name] = &Passion{Value: base + bonus, Basis: kind}
	return nil
}

// HasCondition reports whether the character carries a condition with the
// given name.
func (c *Character) HasCondition(name string) bool {
	if c == nil {
		return false
	}
	for _, cond := range c.Conditions {
		if cond.Name == name {
			return true
		}
	}
	return false
}

// AddCondition adds the condition, or replaces the existing one with the
// same name and location (durations and notes refresh with it).
func (c *Character) AddCondition(cond Condition) {
	if c == nil {
		return
	}
	for i, existing := range c.Conditions {
		if existing.Name == cond.Name && existing.Location == cond.Location {
			c.Conditions[i] = cond
			return
		}
	}
	c.Conditions = append(c.Conditions, cond)
}

// RemoveCondition removes every condition with the given name.
func (c *Character) RemoveCondition(name string) {
	if c == nil {
		return
	}
	kept := c.Conditions[:0]
	for _, cond := range c.Conditions {
		if cond.Name != name {
			kept = append(kept, cond)
		}
	}
	c.Conditions = kept
}

// IsAlive reports whether the character is not dead. The zero value of
// Character is alive.
func (c *Character) IsAlive() bool {
	return c != nil && !c.Dead
}

// IsUnconscious reports whether the character carries the Unconscious
// condition (a Major Wound imposes it immediately; Endurance-opposed
// unconsciousness is added by combat after its roll).
func (c *Character) IsUnconscious() bool {
	return c.HasCondition(CondUnconscious)
}
