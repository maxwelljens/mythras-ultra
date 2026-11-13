package world

// DefaultCultures returns the four human cultures of Mythras Imperative
// (Step 5: Culture, L246–290) as a fresh map on every call — no shared
// mutable state, so callers may freely modify what they receive.
//
// The descriptions are fiction-level, transcribed from the rulebook's
// culture prose (Barbarian, Civilised, Nomadic, Primitive), with Traits and
// Values distilled into behaviour and motivation hints for GM narration.
// Skill data is intentionally absent: the culture skill pool belongs to
// character creation in another package.
//
// Keys are the lower-case names used for lookup by [World.Culture].
func DefaultCultures() map[string]CultureInfo {
	return map[string]CultureInfo{
		"barbarian": {
			Name: "Barbarian",
			Description: "Tribal people of half-tamed wilderness territories who regard towns and cities as " +
				"weak and corrupt. Their own settlements are sedentary but small and close to nature; kinship " +
				"and clan ties are the strong elements of their society, and facility with weapons is a fact " +
				"of daily life against wild creatures and rival tribes.",
			Traits: []string{
				"frank and direct in speech, wary of city manners",
				"territorial about tribal lands, hunting grounds, and grazing",
				"measures people by courage and kin standing",
				"at ease with weapons and the dangers of the wilds",
				"guided by clan elders, custom, and hospitality owed to guests",
			},
			Values: []string{
				"kinship and clan",
				"defence of tribal lands",
				"personal courage",
				"honouring oaths and guests",
			},
		},
		"civilised": {
			Name: "Civilised",
			Description: "People of law and order underpinned by complex social codes and bureaucracy, proud of " +
				"permanent achievements — sprawling cities, imposing temples, monuments, and history recorded in " +
				"books. The veneer is thin: the most civilised city can be more lawless than any frontier " +
				"settlement, but the culture prides itself on its infrastructure, artisans, and reach.",
			Traits: []string{
				"courtesy and social codes as tools of dealing",
				"believes civilisation superior to the frontier",
				"relies on law, records, officials, and contracts",
				"ambitious beneath a polished exterior",
			},
			Values: []string{
				"law and order",
				"civic pride and lasting achievement",
				"social standing",
				"the recorded word",
			},
		},
		"nomadic": {
			Name: "Nomadic",
			Description: "Peoples constantly on the move with no fixed home, whether wandering or cycling through " +
				"camps across the year. They raise few crops and follow the migrations of animals and fish, " +
				"sometimes herding their own beasts, and subsist on what they can quickly take from their " +
				"surroundings, letting nothing go to waste — on foot, by caravan, by raft, or across oceans.",
			Traits: []string{
				"makes camp quickly and moves on before dawn",
				"wastes nothing and improvises from what the land gives",
				"reads weather, tracks, stars, and seasons",
				"counted by the season's journey rather than a birthplace",
			},
			Values: []string{
				"self-reliance",
				"hospitality to travellers on the road",
				"the season's migration",
				"resourcefulness",
			},
		},
		"primitive": {
			Name: "Primitive",
			Description: "Peoples living closest with the land, in tune with its hidden secrets, in extended " +
				"family groups around caves, shelters, or simple huts — typically hunter-gatherers with little " +
				"farming. Cultural beliefs often rest on well-defined superstition, and few have anything like " +
				"a written language: paintings and pictorial symbols carry the message.",
			Traits: []string{
				"reads the land, its signs, and its secrets closely",
				"guided by superstition, omen, and taboo",
				"speaks through story, symbol, and picture rather than writing",
				"practical hunter-gatherer craft, impatient of pretension",
			},
			Values: []string{
				"tradition and custom",
				"the land's favour",
				"extended family",
				"the spoken word",
			},
		},
	}
}
