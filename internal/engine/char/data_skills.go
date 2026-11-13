package char

import (
	"fmt"
	"sort"
	"strings"
)

// SkillClass distinguishes the two skill classes (Step 4; Skills chapter).
type SkillClass int

const (
	// StandardSkill is a skill every character has (22 of them — tables.md
	// §1.8).
	StandardSkill SkillClass = iota
	// ProfessionalSkill is a trained skill (39 in the Skills chapter plus
	// Fly — tables.md §1.9).
	ProfessionalSkill
)

// String renders the class name.
func (s SkillClass) String() string {
	switch s {
	case StandardSkill:
		return "Standard"
	case ProfessionalSkill:
		return "Professional"
	}
	return "Unknown"
}

// baseFormula evaluates a skill's base percentage: either (x + y) or
// (2 × x), plus a static bonus. It renders back to the printed formula
// text with text().
type baseFormula struct {
	x, y   stat
	double bool
	bonus  int
}

// eval computes the base percentage for the given characteristics.
func (f baseFormula) eval(ch Characteristics) int {
	v := ch.at(f.x)
	if f.double {
		v *= 2
	} else {
		v += ch.at(f.y)
	}
	return v + f.bonus
}

// text renders the formula exactly as printed in the rules tables
// (e.g. "STR+DEX", "INT x2", "INT+CHA, +40").
func (f baseFormula) text() string {
	var b strings.Builder
	b.WriteString(statNames[f.x])
	if f.double {
		b.WriteString(" x2")
	} else {
		b.WriteString("+")
		b.WriteString(statNames[f.y])
	}
	if f.bonus != 0 {
		fmt.Fprintf(&b, ", +%d", f.bonus)
	}
	return b.String()
}

// skillDef is one row of the skill tables: name, class and base formula.
type skillDef struct {
	name  string
	class SkillClass
	base  baseFormula
}

// standardSkillDefs transcribes the Standard Skills — Basic Percentage table
// (Step 4, tables.md §1.8; Mythras-Imperative.md L217–244). The +40 on
// Customs and Native Tongue is a static bonus (Native Tongue is Common for
// humans).
var standardSkillDefs = []skillDef{
	{"Athletics", StandardSkill, baseFormula{statSTR, statDEX, false, 0}},
	{"Boating", StandardSkill, baseFormula{statSTR, statCON, false, 0}},
	{"Brawn", StandardSkill, baseFormula{statSTR, statSIZ, false, 0}},
	{"Conceal", StandardSkill, baseFormula{statDEX, statPOW, false, 0}},
	{"Customs", StandardSkill, baseFormula{statINT, statINT, true, 40}},
	{"Dance", StandardSkill, baseFormula{statDEX, statCHA, false, 0}},
	{"Deceit", StandardSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Drive", StandardSkill, baseFormula{statDEX, statPOW, false, 0}},
	{"Endurance", StandardSkill, baseFormula{statCON, statCON, true, 0}},
	{"Evade", StandardSkill, baseFormula{statDEX, statDEX, true, 0}},
	{"First Aid", StandardSkill, baseFormula{statINT, statDEX, false, 0}},
	{"Influence", StandardSkill, baseFormula{statCHA, statCHA, true, 0}},
	{"Insight", StandardSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Locale", StandardSkill, baseFormula{statINT, statINT, true, 0}},
	{"Native Tongue", StandardSkill, baseFormula{statINT, statCHA, false, 40}},
	{"Perception", StandardSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Ride", StandardSkill, baseFormula{statDEX, statPOW, false, 0}},
	{"Sing", StandardSkill, baseFormula{statCHA, statPOW, false, 0}},
	{"Stealth", StandardSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Swim", StandardSkill, baseFormula{statSTR, statCON, false, 0}},
	{"Unarmed", StandardSkill, baseFormula{statSTR, statDEX, false, 0}},
	{"Willpower", StandardSkill, baseFormula{statPOW, statPOW, true, 0}},
}

// professionalSkillDefs transcribes the Professional Skills — Base
// Percentage table (Skills › Professional Skills, tables.md §1.9).
// [AMBIGUOUS: the printed name "Literacy (Specific Language)" is stored as
// "Literacy" — the parenthetical is the speciality mechanic (Language caps
// dependant skills), not part of the name.]
// Fly is not in the §1.9 table but is referenced by Creatures › Creature
// Abilities › Flying (base STR+DEX) and is transcribed here so flying
// creatures can be built.
var professionalSkillDefs = []skillDef{
	{"Acting", ProfessionalSkill, baseFormula{statCHA, statCHA, true, 0}},
	{"Acrobatics", ProfessionalSkill, baseFormula{statSTR, statDEX, false, 0}},
	{"Astrogation", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Art", ProfessionalSkill, baseFormula{statPOW, statCHA, false, 0}},
	{"Bureaucracy", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Commerce", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Comms", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Computers", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Courtesy", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Craft", ProfessionalSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Culture", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Demolitions", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Disguise", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Electronics", ProfessionalSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Engineering", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Forgery", ProfessionalSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Gambling", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Healing", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Language", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Literacy", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Lockpicking", ProfessionalSkill, baseFormula{statDEX, statDEX, true, 0}},
	{"Lore", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Magic", ProfessionalSkill, baseFormula{statPOW, statCHA, false, 0}},
	{"Mechanisms", ProfessionalSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Musicianship", ProfessionalSkill, baseFormula{statDEX, statCHA, false, 0}},
	{"Navigation", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Oratory", ProfessionalSkill, baseFormula{statPOW, statCHA, false, 0}},
	{"Pilot", ProfessionalSkill, baseFormula{statDEX, statINT, false, 0}},
	{"Politics", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Research", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Science", ProfessionalSkill, baseFormula{statINT, statINT, true, 0}},
	{"Seamanship", ProfessionalSkill, baseFormula{statINT, statCON, false, 0}},
	{"Seduction", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Sensors", ProfessionalSkill, baseFormula{statINT, statPOW, false, 0}},
	{"Sleight", ProfessionalSkill, baseFormula{statDEX, statCHA, false, 0}},
	{"Streetwise", ProfessionalSkill, baseFormula{statPOW, statCHA, false, 0}},
	{"Survival", ProfessionalSkill, baseFormula{statCON, statPOW, false, 0}},
	{"Teach", ProfessionalSkill, baseFormula{statINT, statCHA, false, 0}},
	{"Track", ProfessionalSkill, baseFormula{statINT, statCON, false, 0}},
	{"Fly", ProfessionalSkill, baseFormula{statSTR, statDEX, false, 0}},
}

// skillCatalogue indexes every skill definition by its canonical name.
// It is immutable package data: read it through the exported accessors.
var skillCatalogue = buildSkillCatalogue()

func buildSkillCatalogue() map[string]skillDef {
	defs := make(map[string]skillDef, len(standardSkillDefs)+len(professionalSkillDefs))
	for _, d := range standardSkillDefs {
		defs[d.name] = d
	}
	for _, d := range professionalSkillDefs {
		defs[d.name] = d
	}
	return defs
}

// BaseSkill computes the base percentage of a known skill from the given
// characteristics (Step 4 standard bases, tables.md §1.8; professional
// bases, §1.9). The second result is false for names outside the catalogue
// (including empty names) — those are custom skills with no base formula.
func BaseSkill(name string, ch Characteristics) (int, bool) {
	def, ok := skillCatalogue[name]
	if !ok {
		return 0, false
	}
	return def.base.eval(ch), true
}

// BaseSkillFormula returns a known skill's base-percentage formula exactly
// as printed in the rules (e.g. "STR+DEX", "INT x2, +40"). The second
// result is false for unknown skills.
func BaseSkillFormula(name string) (string, bool) {
	def, ok := skillCatalogue[name]
	if !ok {
		return "", false
	}
	return def.base.text(), true
}

// AllSkills returns a fresh map of every canonical skill name to its printed
// base-percentage formula (tables.md §1.8 and §1.9). The returned map is a
// copy — mutating it cannot corrupt the catalogue.
func AllSkills() map[string]string {
	out := make(map[string]string, len(skillCatalogue))
	for name, def := range skillCatalogue {
		out[name] = def.base.text()
	}
	return out
}

// IsStandard reports whether the named skill is one of the 22 Standard
// skills everyone has.
func IsStandard(name string) bool {
	def, ok := skillCatalogue[name]
	return ok && def.class == StandardSkill
}

// IsProfessional reports whether the named skill is a Professional skill.
func IsProfessional(name string) bool {
	def, ok := skillCatalogue[name]
	return ok && def.class == ProfessionalSkill
}

// StandardSkills returns the names of all Standard skills, sorted.
func StandardSkills() []string {
	return skillNamesOf(StandardSkill)
}

// ProfessionalSkills returns the names of all Professional skills (including
// Fly), sorted.
func ProfessionalSkills() []string {
	return skillNamesOf(ProfessionalSkill)
}

// skillNamesOf returns the sorted names of every skill of the given class.
func skillNamesOf(class SkillClass) []string {
	names := make([]string, 0, len(skillCatalogue))
	for name, def := range skillCatalogue {
		if def.class == class {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
