package char

import (
	"fmt"
	"slices"
	"strings"
)

// CareerInfo is one of the 31 careers (Step 6, tables.md §1.12).
// StandardSkills are granted by the career; ProfessionalSkills is the list
// from which up to 3 are chosen (MaxProfessionalPicks); Choices holds the
// printed alternatives ("either Drive or Ride", "either Astrogation or
// Seamanship"). Description carries the printed alias cell ("Bounty Hunter
// (Bail enforcement agent, …)").
//
// "Combat Style (…)" entries in the printed Standard column are not skills
// and are excluded here — the caller records styles with
// Character.SetCombatStyle.
type CareerInfo struct {
	// Name is the career's canonical name ("Warrior", ...).
	Name string `json:"name"`
	// Description is the printed alias cell of the career.
	Description string `json:"description"`
	// StandardSkills are the Standard skills the career lists.
	StandardSkills []string `json:"standardSkills"`
	// Choices are the career's printed either/or picks.
	Choices []ChoiceGroup `json:"choices,omitempty"`
	// ProfessionalSkills is the list from which up to 3 are chosen.
	ProfessionalSkills []string `json:"professionalSkills"`
}

// careerPackage transcribes the complete career table (Step 6, tables.md
// §1.12; Mythras-Imperative.md L298–332).
//
// [AMBIGUOUS: tables.md §1.12 footnote / digest §9.8 — "Linguistics"
// (Bounty Hunter, Detective) is not a defined skill; the footnote presumes
// Language, and that is transcribed here. "Culture" appears among the
// Mechanic career's Standard skills although Culture is Professional —
// kept as printed (it is a real skill listed in the Standard column, so it
// costs no Professional pick). The Magician row omits "(any)" qualifiers and
// prints "Lore x2" — stored as "Lore" (the speciality count is play-time
// detail). Parenthetical specialities such as "Craft (any)" are stored by
// canonical skill name.]
var careerPackage = []CareerInfo{
	{
		Name:        "Agent",
		Description: "Agent (Agitator, Assassin, Detective, Informer, Spy…)",
		StandardSkills: []string{
			"Conceal", "Deceit", "Evade", "Insight", "Perception", "Stealth",
		},
		ProfessionalSkills: []string{
			"Culture", "Disguise", "Language", "Sleight", "Streetwise", "Survival", "Track",
		},
	},
	{
		Name:        "Beast Handler",
		Description: "Beast Handler (Animal Breeder, Beast Tamer, Menagerie Keeper…)",
		StandardSkills: []string{
			"Drive", "Endurance", "First Aid", "Influence", "Locale", "Ride", "Willpower",
		},
		ProfessionalSkills: []string{
			"Craft", "Commerce", "Healing", "Lore", "Survival", "Teach", "Track",
		},
	},
	{
		Name:        "Bounty Hunter",
		Description: "Bounty Hunter (Bail enforcement agent, Bounty killer, Scalp hunter, Skip tracer...)",
		StandardSkills: []string{
			"Athletics", "Endurance", "Evade", "Insight", "Perception", "Stealth",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Commerce", "Culture", "Language", "Streetwise", "Survival", "Track",
		},
	},
	{
		Name:        "Courtesan",
		Description: "Courtesan (Concubine, Escort, Harem-girl, Houri, Paramour…)",
		StandardSkills: []string{
			"Customs", "Dance", "Deceit", "Influence", "Insight", "Perception", "Sing",
		},
		ProfessionalSkills: []string{
			"Art", "Courtesy", "Culture", "Gambling", "Language", "Musicianship", "Seduction",
		},
	},
	{
		Name:        "Crafter",
		Description: "Crafter, Artificer, Artisan…",
		StandardSkills: []string{
			"Brawn", "Drive", "Influence", "Insight", "Locale", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Art", "Commerce", "Craft", "Engineering", "Mechanisms", "Streetwise",
		},
	},
	{
		Name:        "Detective",
		Description: "Detective Gumshoe, Policeman, Private Eye, Private Investigator, Sleuth...",
		StandardSkills: []string{
			"Customs", "Evade", "Influence", "Insight", "Perception", "Stealth",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Culture", "Disguise", "Language", "Lore", "Research", "Sleight", "Streetwise",
		},
	},
	{
		Name:        "Entertainer",
		Description: "Entertainer, Acrobat, Bard, Dancer, Player, Poet…",
		StandardSkills: []string{
			"Athletics", "Brawn", "Dance", "Deceit", "Influence", "Insight", "Sing",
		},
		ProfessionalSkills: []string{
			"Acrobatics", "Acting", "Oratory", "Musicianship", "Seduction", "Sleight", "Streetwise",
		},
	},
	{
		Name:        "Farmer",
		Description: "Farmer, Estate Owner, Gardener, Ploughman…",
		StandardSkills: []string{
			"Athletics", "Brawn", "Drive", "Endurance", "Locale", "Perception", "Ride",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Lore", "Navigation", "Survival", "Track",
		},
	},
	{
		Name:        "Fisher",
		Description: "Fisher, Beach Comber, Net Tender, Pearl Diver, Whaler…",
		StandardSkills: []string{
			"Athletics", "Boating", "Endurance", "Locale", "Perception", "Stealth", "Swim",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Lore", "Navigation", "Seamanship", "Survival",
		},
	},
	{
		Name:        "Gambler",
		Description: "Gambler, Bookie, Cardsharp, Hustler...",
		StandardSkills: []string{
			"Athletics", "Brawn", "Endurance", "Locale", "Perception", "Willpower",
		},
		Choices: []ChoiceGroup{
			{Description: "either Drive or Ride", Options: []string{"Drive", "Ride"}, Pick: 1},
		},
		ProfessionalSkills: []string{
			"Acting", "Bureaucracy", "Commerce", "Courtesy", "Gambling", "Research", "Sleight", "Streetwise",
		},
	},
	{
		Name:        "Herder",
		Description: "Herder, Breeder, Herdsman, Sacred Goose Tender, Wrangler…",
		StandardSkills: []string{
			"Endurance", "First Aid", "Insight", "Locale", "Perception", "Ride",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Healing", "Navigation", "Musicianship", "Survival", "Track",
		},
	},
	{
		Name:        "Hunter",
		Description: "Hunter, Forester, Poacher, Scavenger, Stalker, Trapper…",
		StandardSkills: []string{
			"Athletics", "Endurance", "Locale", "Perception", "Ride", "Stealth",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Lore", "Mechanisms", "Navigation", "Survival", "Track",
		},
	},
	{
		Name:        "Journalist",
		Description: "Journalist, Commentator, Hack, Newshound, Pressman...",
		StandardSkills: []string{
			"Customs", "Deceit", "Influence", "Insight", "Locale", "Native Tongue", "Perception",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Culture", "Language", "Lore", "Oratory", "Politics", "Streetwise",
		},
	},
	{
		Name:        "Magician",
		Description: "Magician, Mage, Sorcerer, Witch, Wizard",
		StandardSkills: []string{
			"Customs", "Deceit", "Influence", "Insight", "Locale", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Culture", "Magic", "Literacy", "Lore", "Oratory", "Sleight",
		},
	},
	{
		Name:        "Mechanic",
		Description: "Mechanic, Engineer, Grease monkey...",
		StandardSkills: []string{
			"Brawn", "Culture", "Drive", "Endurance", "Influence", "Locale", "Willpower",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Electronics", "Gambling", "Mechanisms", "Streetwise",
		},
	},
	{
		Name:        "Merchant",
		Description: "Merchant, Broker, Hawker, Money Lender, Smuggler, Trader…",
		StandardSkills: []string{
			"Boating", "Drive", "Deceit", "Insight", "Influence", "Locale", "Ride",
		},
		ProfessionalSkills: []string{
			"Commerce", "Courtesy", "Culture", "Language", "Navigation", "Seamanship", "Streetwise",
		},
	},
	{
		Name:        "Miner",
		Description: "Miner, Prospector, Quarryman, Sapper, Well Digger…",
		StandardSkills: []string{
			"Athletics", "Brawn", "Endurance", "Locale", "Perception", "Sing", "Willpower",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Engineering", "Lore", "Mechanisms", "Navigation", "Survival",
		},
	},
	{
		Name:        "Official",
		Description: "Official, Minister, Overseer, Steward, Tax Collector…",
		StandardSkills: []string{
			"Customs", "Deceit", "Influence", "Insight", "Locale", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Commerce", "Courtesy", "Language", "Literacy", "Lore", "Oratory",
		},
	},
	{
		Name:        "Physician",
		Description: "Physician, Doctor, Healer, Medicine Man, Torturer, Vivisectionist…",
		StandardSkills: []string{
			"Dance", "First Aid", "Influence", "Insight", "Locale", "Sing", "Willpower",
		},
		ProfessionalSkills: []string{
			"Commerce", "Craft", "Healing", "Language", "Literacy", "Lore", "Streetwise",
		},
	},
	{
		Name:        "Pilot",
		Description: "Pilot, Aeronaut, Astronaut, Barnstormer, Flyboy, Test Pilot...",
		StandardSkills: []string{
			"Brawn", "Drive", "Endurance", "Evade", "Locale", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Customs", "Electronics", "Mechanisms", "Navigation", "Pilot", "Sensors", "Streetwise",
		},
	},
	{
		Name:        "Politician",
		Description: "Politician, Civil Servant, Mandarin, Minister, Overseer, Steward…",
		StandardSkills: []string{
			"Customs", "Deceit", "Influence", "Insight", "Locale", "Native Tongue", "Perception",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Courtesy", "Culture", "Language", "Lore", "Oratory", "Politics",
		},
	},
	{
		Name:        "Priest",
		Description: "Priest, Arbiter, Cultist, Druid, Mendicant, Prophet…",
		StandardSkills: []string{
			"Customs", "Dance", "Deceit", "Influence", "Insight", "Locale", "Willpower",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Courtesy", "Customs", "Literacy", "Lore", "Oratory", "Politics",
		},
	},
	{
		Name:        "Sailor",
		Description: "Sailor, Captain, Galley Slave, Marine, Pirate, Raft man…",
		StandardSkills: []string{
			"Athletics", "Boating", "Brawn", "Endurance", "Locale", "Swim",
		},
		ProfessionalSkills: []string{
			"Craft", "Culture", "Language", "Lore", "Navigation", "Seamanship", "Survival",
		},
	},
	{
		Name:        "Scholar",
		Description: "Scholar, Annalist, Librarian, Philosopher, Skald, Scribe…",
		StandardSkills: []string{
			"Customs", "Influence", "Insight", "Locale", "Native Tongue", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Culture", "Language", "Literacy", "Lore", "Oratory", "Teach",
		},
	},
	{
		Name:        "Scientist",
		Description: "Scientist, Archivist, Librarian, Philosopher…",
		StandardSkills: []string{
			"Customs", "Influence", "Insight", "Locale", "Native Tongue", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Culture", "Engineering", "Language", "Oratory", "Research", "Science", "Teach",
		},
	},
	{
		Name:        "Scout",
		Description: "Scout, Bounty Hunter, Explorer, Pioneer, Ranger, Wayfarer…",
		StandardSkills: []string{
			"Athletics", "Endurance", "First Aid", "Perception", "Stealth", "Swim",
		},
		ProfessionalSkills: []string{
			"Culture", "Healing", "Language", "Lore", "Navigation", "Survival", "Track",
		},
	},
	{
		Name:        "Servant",
		Description: "Servant, Aide, Butler, Chauffeur, House Maid, Governess, Major Domo, Manservant, Personal Assistant, Secretary...",
		StandardSkills: []string{
			"Customs", "Deceit", "Drive", "Influence", "Insight", "Locale", "Perception",
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Courtesy", "Craft", "Culture", "Language", "Politics", "Streetwise",
		},
	},
	{
		Name:        "Smuggler",
		Description: "Smuggler, Black Marketer, Bootlegger, Gun Runner...",
		StandardSkills: []string{
			"Conceal", "Customs", "Deceit", "Drive", "Influence", "Insight", "Locale",
		},
		Choices: []ChoiceGroup{
			{Description: "and either Astrogation or Seamanship", Options: []string{"Astrogation", "Seamanship"}, Pick: 1},
		},
		ProfessionalSkills: []string{
			"Bureaucracy", "Commerce", "Customs", "Language", "Navigation", "Streetwise",
		},
	},
	{
		Name:        "Technician",
		Description: "Technician, Electrician, Laboratory Assistant...",
		StandardSkills: []string{
			"Brawn", "Drive", "Endurance", "Influence", "Locale", "Perception", "Willpower",
		},
		ProfessionalSkills: []string{
			"Comms", "Craft", "Electronics", "Gambling", "Mechanisms", "Science", "Sensors",
		},
	},
	{
		Name:        "Thief",
		Description: "Thief, Burglar, Conman, Fence, Mugger, Tomb Robber…",
		StandardSkills: []string{
			"Athletics", "Deceit", "Evade", "Insight", "Perception", "Stealth",
		},
		ProfessionalSkills: []string{
			"Acting", "Commerce", "Disguise", "Lockpicking", "Mechanisms", "Sleight", "Streetwise",
		},
	},
	{
		Name:        "Warrior",
		Description: "Warrior, Bodyguard, Mercenary, Pit Fighter, Soldier…",
		StandardSkills: []string{
			"Athletics", "Brawn", "Endurance", "Evade", "Unarmed",
		},
		ProfessionalSkills: []string{
			"Craft", "Engineering", "Gambling", "Lore", "Oratory", "Survival",
		},
	},
}

// Careers returns copies of all 31 careers, in table order.
func Careers() []CareerInfo {
	out := make([]CareerInfo, 0, len(careerPackage))
	for _, ki := range careerPackage {
		out = append(out, copyCareer(ki))
	}
	return out
}

// copyCareer deep-copies a career so callers cannot mutate the table.
func copyCareer(ki CareerInfo) CareerInfo {
	ki.StandardSkills = slices.Clone(ki.StandardSkills)
	ki.ProfessionalSkills = slices.Clone(ki.ProfessionalSkills)
	groups := make([]ChoiceGroup, len(ki.Choices))
	for i, g := range ki.Choices {
		g.Options = slices.Clone(g.Options)
		groups[i] = g
	}
	ki.Choices = groups
	return ki
}

// lookupCareer finds a career by name, case-insensitively.
func lookupCareer(name string) (CareerInfo, bool) {
	want := strings.TrimSpace(name)
	for _, ki := range careerPackage {
		if strings.EqualFold(want, ki.Name) {
			return ki, true
		}
	}
	return CareerInfo{}, false
}

// ApplyCareer adopts the named career on the character: it records the
// career, initialises the creation point pools (tables.md §1.10) if the
// character has none, and refreshes the character (Standard skills at base,
// cached Derived rebuilt).
//
// As with ApplyCulture, the 100 Career points and the choice of up to 3
// Professional skills are spent by the player through SpendPool with
// PoolCareer. Unknown names wrap [ErrUnknownCareer].
func ApplyCareer(c *Character, name string) error {
	if c == nil {
		return fmt.Errorf("%w: nil character", ErrUnknownCareer)
	}
	ki, ok := lookupCareer(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownCareer, name)
	}
	c.Career = ki.Name
	c.ensurePools()
	return c.Refresh()
}
