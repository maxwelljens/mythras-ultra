package rules

import (
	"errors"
	"fmt"

	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// ErrNoRandomValue reports a CompetenceLevel with an empty Random dice code
// — the zero value, or a hand-built level not taken from Competence.
var ErrNoRandomValue = errors.New("rules: competence level has no random value")

// CompetenceLevel is one row of the Competency Level Table (Mythras-
// Imperative.md, Skills › So, Just How Good Are You? L914-927; digest
// L533-543), transcribed verbatim: the frame of reference that maps a skill
// rating onto a named competence level, its military equivalent, a
// description, and the dice code that rolls a random NPC skill inside the
// band.
type CompetenceLevel struct {
	// Band is the printed Skill Rating range, e.g. "1-30%" or ">110%".
	Band string `json:"band"`
	// Name is the printed Competence Level, e.g. "Novice".
	Name string `json:"name"`
	// Military is the printed Military Equivalent, e.g. "Cannon Fodder".
	Military string `json:"military"`
	// Description is the printed Description.
	Description string `json:"description"`
	// Random is the printed Random Value dice code, e.g. "1d20+10".
	Random string `json:"random"`
}

// competenceTable transcribes the five bounded rows of the Competency Level
// Table (L922-926) in ascending order; max is the top of the printed Skill
// Rating band.
var competenceTable = []struct {
	max   int
	level CompetenceLevel
}{
	{30, CompetenceLevel{
		Band:        "1-30%",
		Name:        "Novice",
		Military:    "Cannon Fodder",
		Description: "Unqualified and learning the basics",
		Random:      "1d20+10",
	}},
	{50, CompetenceLevel{
		Band:        "31-50%",
		Name:        "Competent",
		Military:    "Green",
		Description: "Can perform easy tasks if taking time to be careful",
		Random:      "1d20+30",
	}},
	{70, CompetenceLevel{
		Band:        "51-70%",
		Name:        "Proficient",
		Military:    "Seasoned",
		Description: "Has reached an employable level of competence",
		Random:      "1d20+50",
	}},
	{90, CompetenceLevel{
		Band:        "71-90%",
		Name:        "Expert",
		Military:    "Veteran",
		Description: "Recognised as a professional or subject matter expert in the field",
		Random:      "1d20+70",
	}},
	{110, CompetenceLevel{
		Band:        "91-110%",
		Name:        "Master",
		Military:    "Elite",
		Description: "Considered one of the best in the business",
		Random:      "1d20+90",
	}},
}

// competenceExemplar is the unbounded ">110%" row (L927).
var competenceExemplar = CompetenceLevel{
	Band:        ">110%",
	Name:        "Exemplar",
	Military:    "Heroic",
	Description: "Renowned far and wide for their consummate skill",
	Random:      "1d20+110",
}

// Competence returns the Competency Level Table row for a skill rating:
// Novice for 1-30, Competent for 31-50, Proficient for 51-70, Expert for
// 71-90, Master for 91-110, Exemplar above 110 (L914-927). The table starts
// at 1, so a rating of 0 or less reads as Novice rather than inventing a
// band below it.
func Competence(skill int) CompetenceLevel {
	if skill < 1 {
		skill = 1
	}
	for _, row := range competenceTable {
		if skill <= row.max {
			return row.level
		}
	}
	return competenceExemplar
}

// RandomSkill rolls the level's printed Random Value dice code (the
// Competency Level Table's column for generating a random NPC skill in that
// band) and returns the total, always inside the printed band: for example
// Competence(1).Random is "1d20+10", yielding 11-30 inside 1-30%.
//
// It returns [ErrNoRandomValue] when level carries no dice code (use rows
// from Competence), wraps the dice package's parse error for a malformed
// code, and rejects a nil Rand.
func RandomSkill(r *dice.Rand, level CompetenceLevel) (int, error) {
	if r == nil {
		return 0, errors.New("rules: nil dice source")
	}
	if level.Random == "" {
		return 0, fmt.Errorf("%w: band %q", ErrNoRandomValue, level.Band)
	}
	roll, err := r.Roll(level.Random)
	if err != nil {
		return 0, fmt.Errorf("rules: random skill %q: %w", level.Random, err)
	}
	return roll.Total, nil
}
