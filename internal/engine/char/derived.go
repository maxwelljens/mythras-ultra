package char

import (
	"errors"
	"fmt"
)

// Derived is the cache of the attributes derived from characteristics
// (Step 3, tables.md §1.2–§1.7). It is a pure function of a Character's
// Characteristics and hit-location catalog: never edit it by hand, always
// regenerate it with [Derive] (or [Character.Refresh]).
//
// No field here is rolled — every value in the digest's Step 3 tables is a
// deterministic function of characteristics, so Derive consumes no dice and
// a session seed is never needed to reproduce it.
type Derived struct {
	// ActionPoints is 2 for every character (tables.md §1.7).
	ActionPoints int `json:"actionPoints"`
	// DamageModifier is the dice expression to add to (or subtract from)
	// weapon damage, e.g. "1d6" or "-1d4"; roll it with the dice package.
	DamageModifier string `json:"damageModifier"`
	// DamageModifierBand is the STR+SIZ range label the expression was
	// taken from, e.g. "36-40" (tables.md §1.2).
	DamageModifierBand string `json:"damageModifierBand"`
	// ExperienceModifier adjusts Experience Rolls received, from CHA
	// (tables.md §1.3). Also called the Experience Bonus.
	ExperienceModifier int `json:"experienceModifier"`
	// HealingRate is HP recovered per day/week/month per wound severity,
	// from CON (tables.md §1.4).
	HealingRate int `json:"healingRate"`
	// HitPoints is the total starting Hit Points across every hit location
	// (the rules track HP per location; this total is their sum).
	HitPoints int `json:"hitPoints"`
	// LocationHP is the starting Hit Points of each hit location
	// (tables.md §1.6 for the humanoid catalog; fixed stat-block values for
	// creature catalogs). The character's live values are
	// Character.CurrentHP.
	LocationHP map[string]int `json:"locationHP"`
	// LuckPoints comes from POW (tables.md §1.5).
	LuckPoints int `json:"luckPoints"`
	// MagicPoints equals POW (tables.md §1.7).
	MagicPoints int `json:"magicPoints"`
	// PowerPoints equals POW for superhero-style powers (Magic and Powers ›
	// How Powers Work: the energy pool is POW). Kept alongside MagicPoints
	// because the rules use both names for the same pool.
	PowerPoints int `json:"powerPoints"`
	// Initiative is the average of DEX and INT — the Initiative Modifier
	// added to 1d10 in combat (tables.md §1.7).
	Initiative int `json:"initiative"`
	// MovementRate is a species default, not a characteristic formula
	// (tables.md §1.7: humans = 6 metres). This model has no species field,
	// so Derive yields the human default; non-human movement belongs to
	// creature construction data.
	MovementRate int `json:"movementRate"`
	// Locations is the hit-location catalog the snapshot was computed
	// against; see HitLocations.
	Locations []LocationDef `json:"locations"`
}

// HitLocations returns the hit-location definitions the derived snapshot was
// computed against. The result is a copy — mutating it cannot corrupt the
// cache.
func (d Derived) HitLocations() []LocationDef {
	out := make([]LocationDef, len(d.Locations))
	copy(out, d.Locations)
	return out
}

// Derive computes the derived attributes for a character. It is pure: no
// mutation, no dice, and on error nothing is consumed, so a failed Derive
// leaves a replayable state untouched. Errors wrap
// [ErrInvalidCharacteristic] (a negative characteristic),
// [ErrDamageModifierRange] (STR+SIZ beyond the printed Damage Modifier
// table) or [ErrUnknownCatalog].
func Derive(c *Character) (Derived, error) {
	if c == nil {
		return Derived{}, errors.New("char: nil character")
	}
	if err := c.Chars.validate(); err != nil {
		return Derived{}, err
	}

	d := Derived{
		ActionPoints:       2, // tables.md §1.7: 2 for all characters
		MagicPoints:        c.Chars.POW,
		PowerPoints:        c.Chars.POW,
		Initiative:         (c.Chars.DEX + c.Chars.INT + 1) / 2, // AMBIGUOUS: rounding of the average is not stated; the Introduction's round-up rule gives ceil((DEX+INT)/2) — digest §9.2, Step 3 › Initiative.
		MovementRate:       6,                                   // humans (tables.md §1.7)
		ExperienceModifier: band6(c.Chars.CHA, 1, 0, -1),        // tables.md §1.3
		HealingRate:        band6(c.Chars.CON, 3, 2, 1),         // tables.md §1.4
		LuckPoints:         band6(c.Chars.POW, 3, 2, 1),         // tables.md §1.5
	}

	expr, band, err := damageModifier(c.Chars.STR + c.Chars.SIZ)
	if err != nil {
		return Derived{}, err
	}
	d.DamageModifier = expr
	d.DamageModifierBand = band

	locations, err := Catalog(c.HitLocationCatalog)
	if err != nil {
		return Derived{}, err
	}
	d.Locations = locations
	d.LocationHP = make(map[string]int, len(locations))
	conSiz := c.Chars.CON + c.Chars.SIZ
	for _, loc := range locations {
		hp, err := startingHP(loc, conSiz)
		if err != nil {
			return Derived{}, err
		}
		d.LocationHP[loc.Name] = hp
		d.HitPoints += hp
	}
	return d, nil
}

// startingHP returns a location's starting Hit Points: the fixed stat-block
// value for creature catalogs, or the Step 3 table row evaluated at CON+SIZ
// for humanoid-style locations.
func startingHP(loc LocationDef, conSiz int) (int, error) {
	if loc.HP > 0 {
		return loc.HP, nil
	}
	if loc.HPRow == "" {
		return 0, fmt.Errorf("%w: location %q has neither fixed HP nor an HP table row", ErrUnknownCatalog, loc.Name)
	}
	hp, ok := hitPointsFor(loc.HPRow, conSiz)
	if !ok {
		return 0, fmt.Errorf("%w: location %q references unknown HP row %q", ErrUnknownCatalog, loc.Name, loc.HPRow)
	}
	return hp, nil
}

// hpRows transcribes the Hit Points per Location table (Step 3, tables.md
// §1.6): HP for CON+SIZ bands 1–5, 6–10, …, 36–40; the "+5" column adds
// +1 to every location per additional 5 points of CON+SIZ beyond 40.
// Rows are keyed by the location classes "Head", "Chest", "Abdomen", "Arm"
// (each arm) and "Leg" (each leg).
var hpRows = map[string][8]int{
	"Head":    {1, 2, 3, 4, 5, 6, 7, 8},
	"Chest":   {3, 4, 5, 6, 7, 8, 9, 10},
	"Abdomen": {2, 3, 4, 5, 6, 7, 8, 9},
	"Arm":     {1, 1, 2, 3, 4, 5, 6, 7},
	"Leg":     {1, 2, 3, 4, 5, 6, 7, 8},
}

// hitPointsFor evaluates an HP table row at the given CON+SIZ. The second
// result is false for an unknown row name.
func hitPointsFor(row string, conSiz int) (int, bool) {
	vals, ok := hpRows[row]
	if !ok {
		return 0, false
	}
	if conSiz <= 40 {
		i := 0
		if conSiz > 1 {
			i = (conSiz - 1) / 5
		}
		if i > 7 {
			i = 7
		}
		return vals[i], true
	}
	// +1 per additional 5 points beyond 40 (41–45 → 1, 46–50 → 2, …).
	return vals[7] + 1 + (conSiz-41)/5, true
}

// damageModifierTable transcribes the Damage Modifier table (Step 3,
// tables.md §1.2: STR+SIZ → modifier). Expressions are stored as ASCII
// dice expressions without the table's decorative "+" sign so the dice
// package parses them directly.
var damageModifierTable = []struct {
	max  int
	expr string
	band string
}{
	{5, "-1d8", "5 or less"},
	{10, "-1d6", "6-10"},
	{15, "-1d4", "11-15"},
	{20, "-1d2", "16-20"},
	{25, "0", "21-25"},
	{30, "1d2", "26-30"},
	{35, "1d4", "31-35"},
	{40, "1d6", "36-40"},
	{45, "1d8", "41-45"},
	{50, "1d10", "46-50"},
	{60, "1d12", "51-60"},
	{70, "2d6", "61-70"},
	{80, "1d8+1d6", "71-80"},
	{90, "2d8", "81-90"},
	{100, "1d10+1d8", "91-100"},
	{110, "2d10", "101-110"},
	{120, "2d10+1d2", "111-120"},
	// [AMBIGUOUS: the table's next row is only "Each 10 points / Continue
	// Progression" — no terms are printed for STR+SIZ beyond 120 (Step 3 ›
	// Damage Modifier, digest §9.1). Rather than invent dice, Derive fails
	// with ErrDamageModifierRange. Ordinary characters cannot reach it
	// (max STR+SIZ from standard generation is 36).]
}

// damageModifier returns the Damage Modifier dice expression and the
// STR+SIZ band label for the given STR+SIZ sum.
func damageModifier(sum int) (expr, band string, err error) {
	for _, row := range damageModifierTable {
		if sum <= row.max {
			return row.expr, row.band, nil
		}
	}
	return "", "", fmt.Errorf("%w: STR+SIZ %d", ErrDamageModifierRange, sum)
}

// band6 implements the three identical "6 or less / 7–12 / 13–18 / each 6
// points thereafter" tables: Experience Modifier (tables.md §1.3), Healing
// Rate (§1.4) and Luck Points (§1.5).
func band6(ch, at13, at7, at6 int) int {
	switch {
	case ch <= 6:
		return at6
	case ch <= 12:
		return at7
	default:
		return at13 + (ch-13)/6
	}
}
