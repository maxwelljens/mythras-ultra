package char

import (
	"fmt"
	"slices"
	"strings"
)

// ChoiceGroup is a "pick N of …" line inside a culture or career skill
// package (Step 5 and Step 6, tables.md §1.11–§1.12), e.g. the Barbarian
// culture's "either Boating or Ride".
type ChoiceGroup struct {
	// Description is the printed wording of the choice.
	Description string `json:"description"`
	// Options are the skills to choose from.
	Options []string `json:"options"`
	// Pick is how many options may be chosen.
	Pick int `json:"pick"`
}

// CultureInfo is one of the four human cultures (Step 5, tables.md §1.11).
// StandardSkills are granted unconditionally; ProfessionalSkills is the list
// from which up to 3 are chosen (see MaxProfessionalPicks); Choices holds
// the printed alternative picks. The cultural Combat Style mentioned by the
// table is not named in the source, so it is not stored here — the caller
// records it with Character.SetCombatStyle.
type CultureInfo struct {
	// Name is the culture's canonical name ("Barbarian", ...).
	Name string `json:"name"`
	// Description is a one-line summary of the culture's flavour.
	Description string `json:"description"`
	// StandardSkills are the Standard skills the culture grants.
	StandardSkills []string `json:"standardSkills"`
	// Choices are the "either/or" and "two of" picks the culture offers.
	Choices []ChoiceGroup `json:"choices,omitempty"`
	// ProfessionalSkills is the list from which up to 3 are chosen.
	ProfessionalSkills []string `json:"professionalSkills"`
}

// MaxProfessionalPicks is the number of Professional skills a culture or
// career grants: "up to 3" (Step 5 and Step 6, tables.md §1.11–§1.12).
const MaxProfessionalPicks = 3

// culturePackage transcribes the Culture Skill Packages table (Step 5,
// tables.md §1.11; Mythras-Imperative.md L246–290). Parenthetical
// specialities such as "Craft (any)" are stored by canonical skill name —
// the speciality is chosen at play time, not part of the skill's identity.
// Every culture also grants one cultural Combat Style, which the table does
// not name.
var culturePackage = []CultureInfo{
	{
		Name:        "Barbarian",
		Description: "A non-urban, tribal or frontier background.",
		StandardSkills: []string{
			"Athletics", "Brawn", "Endurance", "First Aid", "Locale", "Perception",
		},
		Choices: []ChoiceGroup{
			{Description: "either Boating or Ride", Options: []string{"Boating", "Ride"}, Pick: 1},
		},
		ProfessionalSkills: []string{
			"Craft", "Healing", "Lore", "Musicianship", "Navigation", "Seamanship", "Survival", "Track",
		},
	},
	{
		Name:        "Civilised",
		Description: "An urban, literate, law-bound background.",
		StandardSkills: []string{
			"Conceal", "Deceit", "Drive", "Influence", "Insight", "Locale", "Willpower",
		},
		ProfessionalSkills: []string{
			"Art", "Commerce", "Craft", "Courtesy", "Language", "Lore", "Musicianship", "Streetwise",
		},
	},
	{
		Name:        "Nomadic",
		Description: "A migratory way of life, travel by the primary mode of transport.",
		StandardSkills: []string{
			"Endurance", "First Aid", "Locale", "Perception", "Stealth",
		},
		Choices: []ChoiceGroup{
			{
				Description: "two of Athletics, Boating, Swim, Drive, Ride (per primary mode of travel)",
				Options:     []string{"Athletics", "Boating", "Swim", "Drive", "Ride"},
				Pick:        2,
			},
		},
		ProfessionalSkills: []string{
			"Craft", "Culture", "Language", "Lore", "Musicianship", "Navigation", "Survival", "Track",
		},
	},
	{
		Name:        "Primitive",
		Description: "A subsistence, pre-urban or isolated background.",
		StandardSkills: []string{
			"Brawn", "Endurance", "Evade", "Locale", "Perception", "Stealth",
		},
		Choices: []ChoiceGroup{
			{Description: "one of Athletics, Boating, Swim", Options: []string{"Athletics", "Boating", "Swim"}, Pick: 1},
		},
		ProfessionalSkills: []string{
			"Craft", "Healing", "Lore", "Musicianship", "Navigation", "Survival", "Track",
		},
	},
}

// Cultures returns copies of all four culture packages, in table order.
func Cultures() []CultureInfo {
	out := make([]CultureInfo, 0, len(culturePackage))
	for _, ci := range culturePackage {
		out = append(out, copyCulture(ci))
	}
	return out
}

// copyCulture deep-copies a culture so callers cannot mutate the table.
func copyCulture(ci CultureInfo) CultureInfo {
	ci.StandardSkills = slices.Clone(ci.StandardSkills)
	ci.ProfessionalSkills = slices.Clone(ci.ProfessionalSkills)
	groups := make([]ChoiceGroup, len(ci.Choices))
	for i, g := range ci.Choices {
		g.Options = slices.Clone(g.Options)
		groups[i] = g
	}
	ci.Choices = groups
	return ci
}

// lookupCulture finds a culture by name, case-insensitively.
func lookupCulture(name string) (CultureInfo, bool) {
	want := strings.TrimSpace(name)
	for _, ci := range culturePackage {
		if strings.EqualFold(want, ci.Name) {
			return ci, true
		}
	}
	return CultureInfo{}, false
}

// ApplyCulture adopts the named culture on the character: it records the
// culture, initialises the creation point pools (tables.md §1.10) if the
// character has none, and refreshes the character — which grants every
// Standard skill at its characteristic base (Step 4: everyone has them) and
// rebuilds the cached Derived snapshot.
//
// The digest leaves the distribution of the 100 Culture points and the
// choice of up to 3 Professional skills to the player, so this function
// spends nothing: use SpendPool with PoolCulture for that. Unknown names
// wrap [ErrUnknownCulture].
func ApplyCulture(c *Character, name string) error {
	if c == nil {
		return fmt.Errorf("%w: nil character", ErrUnknownCulture)
	}
	ci, ok := lookupCulture(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownCulture, name)
	}
	c.Culture = ci.Name
	c.ensurePools()
	return c.Refresh()
}
