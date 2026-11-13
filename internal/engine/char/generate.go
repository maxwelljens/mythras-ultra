package char

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// GenMethod selects a characteristic generation method (Step 2 › Calculating
// Characteristics, tables.md §1.1, plus the Larger-Than-Life variants
// §1.15–§1.16).
type GenMethod int

const (
	// GenRoll is "Dice Roll": 3d6 for STR, CON, DEX, POW, CHA; 2d6+6 for
	// SIZ and INT, taken as rolled.
	GenRoll GenMethod = iota
	// GenRollAssign is "Dice Roll, Assign": identical rolls to GenRoll —
	// the rules allow swapping values only within the same dice-range group
	// (STR/CON/DEX/POW/CHA; SIZ/INT), which is the caller's arrangement of
	// the returned Characteristics, not extra dice.
	GenRollAssign
	// GenRollHigh is "Dice Roll, High": one extra die per characteristic
	// (4d6 for the 3d6 group, 3d6+6 for the 2d6+6 group) with the lowest
	// die dropped.
	GenRollHigh
	// GenPoints is "Points Build": 80 points distributed across the seven
	// characteristics, each bounded by its dice range (3–18 for the 3d6
	// group, 8–18 for SIZ/INT).
	GenPoints
	// GenPulpRoll is the Pulp Hero rolled method (tables.md §1.15): its
	// printed rolls are identical to GenRollHigh (4d6 drop lowest ×5;
	// 3d6+6 drop lowest ×2).
	GenPulpRoll
	// GenPulpPoints is the Pulp Hero points build: a 90-point pool with
	// the same bounds (tables.md §1.15).
	GenPulpPoints
	// GenParagonRoll is the Paragon rolled method (tables.md §1.16): six
	// 4d6-drop-lowest rolls, the best five assigned to STR, CON, DEX, POW,
	// CHA; three 3d6+6-drop-lowest rolls, the best two to SIZ and INT.
	GenParagonRoll
	// GenParagonPoints is the Paragon points build: a 100-point pool with
	// the same bounds (tables.md §1.16).
	GenParagonPoints
)

// String names the generation method.
func (m GenMethod) String() string {
	switch m {
	case GenRoll:
		return "Dice Roll"
	case GenRollAssign:
		return "Dice Roll, Assign"
	case GenRollHigh:
		return "Dice Roll, High"
	case GenPoints:
		return "Points Build"
	case GenPulpRoll:
		return "Pulp Hero (rolled)"
	case GenPulpPoints:
		return "Pulp Hero (points)"
	case GenParagonRoll:
		return "Paragon (rolled)"
	case GenParagonPoints:
		return "Paragon (points)"
	}
	return fmt.Sprintf("GenMethod(%d)", int(m))
}

// statOrder is the characteristic order used for every generated roll, so
// the same seed always feeds the same sequence of dice.
var statOrder = [...]stat{statSTR, statCON, statSIZ, statDEX, statINT, statPOW, statCHA}

// Point-pool sizes for the points-build methods (tables.md §1.1,
// §1.15, §1.16).
const (
	pointsBuildPool  = 80
	pulpBuildPool    = 90
	paragonBuildPool = 100
	charMin3d6       = 3  // minimum of the 3d6 range
	charMin2d6plus6  = 8  // minimum of the 2d6+6 range
	charMaximum      = 18 // maximum of both dice ranges
)

// Generate rolls a full set of characteristics for the given method using
// the explicit, seedable dice source — no other randomness is involved, so
// the same seed and method always yield the same characteristics.
//
// For points-build methods Generate deals the pool randomly among
// characteristics within their bounds: the tabletop exercise of distributing
// the points is a player choice with no in-play mechanical consequence, so
// the engine needs only a valid distribution (sum = pool, each
// characteristic inside its dice range).
// [AMBIGUOUS: tables.md §1.1 — "Points Build" bounds are not printed
// explicitly; the natural reading is each characteristic's dice-range
// minimum/maximum (3–18 for 3d6, 8–18 for 2d6+6), which is what the engine
// enforces (digest §9.15).]
// Unknown methods wrap [ErrUnknownGenMethod]; a nil Rand is an error.
func Generate(r *dice.Rand, m GenMethod) (Characteristics, error) {
	if r == nil {
		return Characteristics{}, errors.New("char: nil dice source")
	}
	switch m {
	case GenRoll, GenRollAssign:
		return rollStandard(r, false), nil
	case GenRollHigh, GenPulpRoll:
		return rollStandard(r, true), nil
	case GenParagonRoll:
		return rollParagon(r), nil
	case GenPoints:
		return buildPoints(r, pointsBuildPool)
	case GenPulpPoints:
		return buildPoints(r, pulpBuildPool)
	case GenParagonPoints:
		return buildPoints(r, paragonBuildPool)
	default:
		return Characteristics{}, fmt.Errorf("%w: %d", ErrUnknownGenMethod, int(m))
	}
}

// rollStandard rolls each characteristic in statOrder: 3d6 (2d6+6 for SIZ
// and INT), optionally with one extra die and the lowest dropped.
func rollStandard(r *dice.Rand, high bool) Characteristics {
	var ch Characteristics
	for _, s := range statOrder {
		count := 3
		bonus := 0
		if s == statSIZ || s == statINT {
			count = 2
			bonus = 6
		}
		if high {
			count++
		}
		sum, min := 0, 7
		for i := 0; i < count; i++ {
			v := r.D(6)
			sum += v
			if v < min {
				min = v
			}
		}
		if high {
			sum -= min
		}
		*ch.ptr(s) = sum + bonus
	}
	return ch
}

// rollParagon implements the Paragon rolled method (tables.md §1.16): the
// best five of six 4d6-drop-lowest rolls go to STR, CON, DEX, POW, CHA and
// the best two of three 3d6+6-drop-lowest rolls to SIZ, INT. The rules say
// "assign best of choice"; the engine assigns in descending order into the
// listed characteristic order (an engine choice — no rule depends on which
// value lands where).
func rollParagon(r *dice.Rand) Characteristics {
	fives := make([]int, 0, 6)
	for i := 0; i < 6; i++ {
		fives = append(fives, dropLowest(r, 4))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(fives)))
	twos := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		twos = append(twos, dropLowest(r, 3)+6)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(twos)))

	var ch Characteristics
	fiveStats := [...]stat{statSTR, statCON, statDEX, statPOW, statCHA}
	for i, s := range fiveStats {
		*ch.ptr(s) = fives[i]
	}
	*ch.ptr(statSIZ) = twos[0]
	*ch.ptr(statINT) = twos[1]
	return ch
}

// dropLowest rolls count six-sided dice and returns the sum minus the
// lowest die.
func dropLowest(r *dice.Rand, count int) int {
	sum, min := 0, 7
	for i := 0; i < count; i++ {
		v := r.D(6)
		sum += v
		if v < min {
			min = v
		}
	}
	return sum - min
}

// buildPoints deals an 80/90/100-point pool across the seven
// characteristics: each starts at its dice-range minimum and remaining
// points are dealt one at a time to a random characteristic below the
// maximum of its range. The result always sums to pool.
func buildPoints(r *dice.Rand, pool int) (Characteristics, error) {
	ch := Characteristics{
		STR: charMin3d6, CON: charMin3d6, DEX: charMin3d6, POW: charMin3d6, CHA: charMin3d6,
		SIZ: charMin2d6plus6, INT: charMin2d6plus6,
	}
	used := 5*charMin3d6 + 2*charMin2d6plus6
	remaining := pool - used
	if remaining < 0 {
		return Characteristics{}, fmt.Errorf("%w: pool %d below minimum total %d", ErrInvalidAmount, pool, used)
	}
	for remaining > 0 {
		candidates := make([]stat, 0, len(statOrder))
		for _, s := range statOrder {
			if ch.at(s) < charMaximum {
				candidates = append(candidates, s)
			}
		}
		if len(candidates) == 0 {
			// Unreachable for the printed pools (max total is 126 ≥ 100).
			return Characteristics{}, fmt.Errorf("char: pool %d cannot be dealt within characteristic maxima", pool)
		}
		s := candidates[r.Intn(len(candidates))]
		*ch.ptr(s)++
		remaining--
	}
	return ch, nil
}

// ExperienceBand is one row of the Experience Table (Step 7 › Creating
// Experienced Characters, tables.md §1.13): the age expression printed for
// the category, its starting Bonus Skill Points, and the Maximum Skill
// Increase that caps how many Bonus points may go to any one skill.
type ExperienceBand struct {
	// Category is the printed category name ("Young", ...).
	Category string `json:"category"`
	// AgeFormula is the printed age expression ("10+1d6", ...).
	AgeFormula string `json:"ageFormula"`
	// BonusPoints are the Bonus Skill Points for the band.
	BonusPoints int `json:"bonusPoints"`
	// MaxIncrease caps Bonus points spendable on any one skill.
	MaxIncrease int `json:"maxIncrease"`
	// minAge and maxAge are the inclusive age bounds of AgeFormula
	// (10+1d6 → 10..15, 15+2d6 → 15..27, ...).
	minAge, maxAge int
}

// experienceTable transcribes the Experience Table (Step 7, tables.md
// §1.13). minAge/maxAge are parsed from the printed Age column.
//
// [AMBIGUOUS: the printed age ranges overlap at 15, 25, 40 and 60 — the
// table does not say which category wins. Conservative reading (digest
// step 7): the first row in table order that reaches the age, so an overlap
// age keeps the younger category and its smaller Bonus; ages below the
// first row clamp to Young and ages beyond the last row (90) clamp to Old.
// There are no aging penalties anywhere in the rules — age only sizes the
// starting experience pool.]
var experienceTable = []ExperienceBand{
	{Category: "Young", AgeFormula: "10+1d6", BonusPoints: 100, MaxIncrease: 10, minAge: 10, maxAge: 15},
	{Category: "Adult", AgeFormula: "15+2d6", BonusPoints: 150, MaxIncrease: 15, minAge: 15, maxAge: 27},
	{Category: "Middle Aged", AgeFormula: "25+3d6", BonusPoints: 200, MaxIncrease: 20, minAge: 25, maxAge: 43},
	{Category: "Senior", AgeFormula: "40+4d6", BonusPoints: 250, MaxIncrease: 25, minAge: 40, maxAge: 64},
	{Category: "Old", AgeFormula: "60+5d6", BonusPoints: 300, MaxIncrease: 30, minAge: 60, maxAge: 90},
}

// ExperienceBands returns copies of the Experience Table rows in table
// order.
func ExperienceBands() []ExperienceBand {
	out := make([]ExperienceBand, len(experienceTable))
	copy(out, experienceTable)
	return out
}

// ExperienceForAge returns the Experience Table band for an actual age,
// applying the conservative first-match rule documented on
// experienceTable. Negative ages wrap [ErrInvalidAge].
func ExperienceForAge(age int) (ExperienceBand, error) {
	if age < 0 {
		return ExperienceBand{}, fmt.Errorf("%w: %d", ErrInvalidAge, age)
	}
	for _, row := range experienceTable {
		if age <= row.maxAge {
			return row, nil
		}
	}
	return experienceTable[len(experienceTable)-1], nil
}

// ApplyAge sets the character's age and sizes the Bonus point pool from the
// Experience Table: Pools.Bonus becomes the band's Bonus Skill Points and
// Pools.MaxIncrease its per-skill cap. Culture 100 and Career 100 are
// retained.
//
// [AMBIGUOUS: the table does not say outright that the age-band Bonus
// replaces the default 150-point Bonus pool (digest §9.16); the natural
// reading of "Bonus Skill Points determine how many points the character
// must spend" is that it does, so the remaining Bonus pool is reset to the
// band value. Call ApplyAge before spending Bonus points.]
func (c *Character) ApplyAge(age int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	band, err := ExperienceForAge(age)
	if err != nil {
		return err
	}
	c.Age = age
	c.ensurePools()
	c.Pools.Bonus = band.BonusPoints
	c.Pools.MaxIncrease = band.MaxIncrease
	return nil
}

// SkillPicks returns the Skill Pyramid allocations (Step 7 › The Skill
// Pyramid, tables.md §1.14): 1×50%, 2×40%, 3×30%, 4×20%, 5×10% — 15
// allocations totalling 350 points, the alternative to the flat 350-point
// pools. Each call returns a fresh slice.
func SkillPicks() []int {
	return []int{50, 40, 40, 30, 30, 30, 20, 20, 20, 20, 10, 10, 10, 10, 10}
}

// Creation point pool sizes (Step 4, tables.md §1.10: Culture 100, Career
// 100, Bonus 150 — 350 total; 1 point = +1% to a skill).
const (
	CulturePoolPoints = 100
	CareerPoolPoints  = 100
	BonusPoolPoints   = 150
)

// Point pool names accepted by Character.SpendPool.
const (
	// PoolCulture is the 100-point Culture pool: spent on the culture's
	// Standard skills, its choice options and up to 3 of its Professional
	// skills.
	PoolCulture = "culture"
	// PoolCareer is the 100-point Career pool, with the career's package
	// and pick limits.
	PoolCareer = "career"
	// PoolBonus is the Bonus pool (150 by default, or the age band's value
	// — Step 7): any Standard skill, or a Professional skill granted by the
	// character's culture or career. When set, MaxIncrease caps points per
	// skill (Experience Table).
	PoolBonus = "bonus"
)

// Pools holds the Step 4–7 creation point pools while a character is being
// built: remaining points per pool, the per-skill Bonus cap from the
// Experience Table (0 = no cap until ApplyAge), and Spent — points recorded
// per pool and skill, which is what the eligibility and pick limits are
// enforced against. Pools is nil on finished characters unless a caller
// keeps it around; exhausted pools can simply be dropped.
type Pools struct {
	// Culture is the remaining Culture pool (CulturePoolPoints at start).
	Culture int `json:"culture"`
	// Career is the remaining Career pool (CareerPoolPoints at start).
	Career int `json:"career"`
	// Bonus is the remaining Bonus pool (BonusPoolPoints at start, or the
	// age band's value after ApplyAge).
	Bonus int `json:"bonus"`
	// MaxIncrease caps Bonus points per skill (Experience Table band);
	// 0 means no cap.
	MaxIncrease int `json:"maxIncrease,omitempty"`
	// Spent maps pool name → skill name → points spent from that pool.
	// It stays nil until the first spend.
	Spent map[string]map[string]int `json:"spent,omitempty"`
}

// NewPools returns fresh creation point pools with the default sizes
// (Culture 100, Career 100, Bonus 150; no per-skill cap until ApplyAge).
func NewPools() *Pools {
	return &Pools{
		Culture: CulturePoolPoints,
		Career:  CareerPoolPoints,
		Bonus:   BonusPoolPoints,
	}
}

// ensurePools lazily creates the character's creation point pools.
func (c *Character) ensurePools() {
	if c.Pools == nil {
		c.Pools = NewPools()
	}
}

// poolRemaining returns the remaining points of the named pool (and reports
// whether the pool name is valid).
func (p *Pools) poolRemaining(pool string) (int, bool) {
	switch pool {
	case PoolCulture:
		return p.Culture, true
	case PoolCareer:
		return p.Career, true
	case PoolBonus:
		return p.Bonus, true
	}
	return 0, false
}

// poolDeduct subtracts points from the named pool. The caller must have
// validated the pool name first.
func (p *Pools) poolDeduct(pool string, points int) {
	switch pool {
	case PoolCulture:
		p.Culture -= points
	case PoolCareer:
		p.Career -= points
	case PoolBonus:
		p.Bonus -= points
	}
}

// SpendPool spends points from a creation point pool into a skill: 1 point
// = +1% (tables.md §1.10). Eligibility follows the digest:
//
//   - PoolCulture: the culture's listed Standard skills, its choice-group
//     options, and at most 3 of its Professional skills (Step 5);
//   - PoolCareer: the career's Standard skills, choice options, and at most
//     3 of its Professional skills (Step 6);
//   - PoolBonus: any Standard skill, or a Professional skill granted by the
//     character's culture or career, capped at MaxIncrease points per skill
//     when the Experience Table band is set (Step 7).
//
// Choice groups additionally cap how many of their options may receive pool
// points (e.g. "either Boating or Ride" funds at most one). The skill's
// value rises by the spent points; a missing skill is created.
//
// Errors wrap [ErrUnknownPool], [ErrUnknownCulture], [ErrUnknownCareer],
// [ErrSkillNotInPool], [ErrTooManyPicks], [ErrIncreaseCap],
// [ErrPoolExhausted], [ErrInvalidAmount] or [ErrUnknownSkill].
func (c *Character) SpendPool(pool, skill string, points int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if points <= 0 {
		return fmt.Errorf("%w: %d points from pool %q", ErrInvalidAmount, points, pool)
	}
	if skill == "" {
		return fmt.Errorf("%w: empty skill name", ErrUnknownSkill)
	}
	c.ensurePools()
	p := c.Pools
	remaining, ok := p.poolRemaining(pool)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownPool, pool)
	}

	var (
		groups []ChoiceGroup
		std    []string
		pros   []string
	)
	switch pool {
	case PoolCulture:
		ci, found := lookupCulture(c.Culture)
		if !found {
			return fmt.Errorf("%w: character has no culture", ErrUnknownCulture)
		}
		if !packageEligible(ci.StandardSkills, ci.Choices, ci.ProfessionalSkills, skill) {
			return fmt.Errorf("%w: %q is not in culture %q package", ErrSkillNotInPool, skill, ci.Name)
		}
		groups, std, pros = ci.Choices, ci.StandardSkills, ci.ProfessionalSkills
	case PoolCareer:
		ki, found := lookupCareer(c.Career)
		if !found {
			return fmt.Errorf("%w: character has no career", ErrUnknownCareer)
		}
		if !packageEligible(ki.StandardSkills, ki.Choices, ki.ProfessionalSkills, skill) {
			return fmt.Errorf("%w: %q is not in career %q package", ErrSkillNotInPool, skill, ki.Name)
		}
		groups, std, pros = ki.Choices, ki.StandardSkills, ki.ProfessionalSkills
	case PoolBonus:
		if !IsStandard(skill) && !c.grantedProfessional(skill) {
			return fmt.Errorf("%w: %q is not a Standard skill nor a Professional skill granted by culture or career", ErrSkillNotInPool, skill)
		}
		if p.MaxIncrease > 0 && p.Spent[pool][skill]+points > p.MaxIncrease {
			return fmt.Errorf("%w: %q would take %d bonus points (max %d)", ErrIncreaseCap, skill, p.Spent[pool][skill]+points, p.MaxIncrease)
		}
	}

	if pool == PoolCulture || pool == PoolCareer {
		if err := checkChoicePicks(pool, groups, p, skill, points); err != nil {
			return err
		}
		if err := checkProfessionalPicks(pool, std, pros, p, skill, points); err != nil {
			return err
		}
	}
	if remaining < points {
		return fmt.Errorf("%w: pool %q has %d points left, need %d", ErrPoolExhausted, pool, remaining, points)
	}

	p.poolDeduct(pool, points)
	if p.Spent == nil {
		p.Spent = make(map[string]map[string]int, 3)
	}
	if p.Spent[pool] == nil {
		p.Spent[pool] = make(map[string]int)
	}
	p.Spent[pool][skill] += points

	value := points
	if sk, ok := c.Skill(skill); ok {
		value = sk.Value + points
	} else if base, known := BaseSkill(skill, c.Chars); known {
		value = base + points
	}
	return c.SetSkill(skill, value)
}

// packageEligible reports whether a skill may receive points from a
// culture/career package: it is in the standard list, a choice option, or
// the professional list.
func packageEligible(std []string, groups []ChoiceGroup, pros []string, skill string) bool {
	if slices.Contains(std, skill) || slices.Contains(pros, skill) {
		return true
	}
	for _, g := range groups {
		if slices.Contains(g.Options, skill) {
			return true
		}
	}
	return false
}

// grantedProfessional reports whether a Professional skill is granted by
// the character's culture or career (the Bonus pool's rule, Step 7).
func (c *Character) grantedProfessional(skill string) bool {
	if ci, ok := lookupCulture(c.Culture); ok && slices.Contains(ci.ProfessionalSkills, skill) {
		return true
	}
	if ki, ok := lookupCareer(c.Career); ok && slices.Contains(ki.ProfessionalSkills, skill) {
		return true
	}
	return false
}

// checkChoicePicks enforces a choice group's Pick limit: after this spend,
// at most g.Pick distinct options of the group may have received points
// from this pool. It returns an error wrapping [ErrTooManyPicks].
func checkChoicePicks(pool string, groups []ChoiceGroup, p *Pools, skill string, points int) error {
	for _, g := range groups {
		if !slices.Contains(g.Options, skill) {
			continue
		}
		funded := 0
		for _, opt := range g.Options {
			n := p.Spent[pool][opt]
			if opt == skill && n == 0 {
				n = points
			}
			if n > 0 {
				funded++
			}
		}
		if funded > g.Pick {
			return fmt.Errorf("%w: group %q would fund %d of %d picks", ErrTooManyPicks, g.Description, funded, g.Pick)
		}
	}
	return nil
}

// checkProfessionalPicks enforces "up to 3 Professional skills" for a
// culture/career pool. Skills printed in the package's Standard column are
// grants, not picks (e.g. the Mechanic's Culture, the Priest's Customs),
// and are not counted.
func checkProfessionalPicks(pool string, std, pros []string, p *Pools, skill string, points int) error {
	if !slices.Contains(pros, skill) || slices.Contains(std, skill) {
		return nil
	}
	funded := 0
	for _, s := range pros {
		if slices.Contains(std, s) {
			continue
		}
		n := p.Spent[pool][s]
		if s == skill && n == 0 {
			n = points
		}
		if n > 0 {
			funded++
		}
	}
	if funded > MaxProfessionalPicks {
		return fmt.Errorf("%w: %q would be the %dth Professional skill (max %d)", ErrTooManyPicks, skill, funded, MaxProfessionalPicks)
	}
	return nil
}
