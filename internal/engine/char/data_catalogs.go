package char

// hitLocationCatalogs holds every hit-location catalog from the rulebook.
// It is immutable package data: read it through Catalog, which returns a
// copy.
//
// Sources: the humanoid catalog transcribes tables.md §3.8 (Game System ›
// Hit Locations, "Humanoid Hit Locations"); the ten creature catalogs
// transcribe tables.md §8.4 (Creatures › Creature Hit Locations, AP/HP per
// 1d20 roll).
var hitLocationCatalogs = map[string][]LocationDef{
	// tables.md §3.8 — 1d20, standard humanoid. HP comes from the Step 3
	// table (§1.6) via HPRow; AP comes from worn armor.
	"humanoid": {
		{Name: "Right Leg", RollRange: [2]int{1, 3}, HPRow: "Leg"},
		{Name: "Left Leg", RollRange: [2]int{4, 6}, HPRow: "Leg"},
		{Name: "Abdomen", RollRange: [2]int{7, 9}, HPRow: "Abdomen"},
		{Name: "Chest", RollRange: [2]int{10, 12}, HPRow: "Chest"},
		{Name: "Right Arm", RollRange: [2]int{13, 15}, HPRow: "Arm"},
		{Name: "Left Arm", RollRange: [2]int{16, 18}, HPRow: "Arm"},
		{Name: "Head", RollRange: [2]int{19, 20}, HPRow: "Head"},
	},

	// tables.md §8.4 — Giant Ant.
	"giant-ant": {
		{Name: "Right Rear Leg", RollRange: [2]int{1, 1}, HP: 6, AP: 4},
		{Name: "Left Rear Leg", RollRange: [2]int{2, 2}, HP: 6, AP: 4},
		{Name: "Right Middle Leg", RollRange: [2]int{3, 3}, HP: 6, AP: 4},
		{Name: "Left Middle Leg", RollRange: [2]int{4, 4}, HP: 6, AP: 4},
		{Name: "Abdomen", RollRange: [2]int{5, 9}, HP: 9, AP: 4},
		{Name: "Thorax", RollRange: [2]int{10, 13}, HP: 6, AP: 4},
		{Name: "Right Front", RollRange: [2]int{14, 14}, HP: 6, AP: 4},
		{Name: "Left Front Leg", RollRange: [2]int{15, 15}, HP: 6, AP: 4},
		{Name: "Head", RollRange: [2]int{16, 20}, HP: 7, AP: 4},
	},

	// tables.md §8.4 — Bear (Grizzly).
	"bear": {
		{Name: "Right Rear Leg", RollRange: [2]int{1, 3}, HP: 10, AP: 3},
		{Name: "Left Rear Leg", RollRange: [2]int{4, 6}, HP: 10, AP: 3},
		{Name: "Hindquarters", RollRange: [2]int{7, 9}, HP: 11, AP: 3},
		{Name: "Forequarters", RollRange: [2]int{10, 12}, HP: 12, AP: 3},
		{Name: "Right Front", RollRange: [2]int{13, 15}, HP: 10, AP: 3},
		{Name: "Left Front Leg", RollRange: [2]int{16, 18}, HP: 10, AP: 3},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 10, AP: 3},
	},

	// tables.md §8.4 — Crocodile/Alligator.
	"crocodile": {
		{Name: "Tail", RollRange: [2]int{1, 3}, HP: 12, AP: 5},
		{Name: "Right Rear Leg", RollRange: [2]int{4, 5}, HP: 12, AP: 5},
		{Name: "Left Rear Leg", RollRange: [2]int{6, 7}, HP: 12, AP: 5},
		{Name: "Hindquarters", RollRange: [2]int{8, 10}, HP: 13, AP: 5},
		{Name: "Forequarters", RollRange: [2]int{11, 14}, HP: 14, AP: 5},
		{Name: "Right Front", RollRange: [2]int{15, 16}, HP: 12, AP: 5},
		{Name: "Left Front Leg", RollRange: [2]int{17, 18}, HP: 12, AP: 5},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 13, AP: 5},
	},

	// tables.md §8.4 — Dragon.
	"dragon": {
		{Name: "Tail", RollRange: [2]int{1, 2}, HP: 17, AP: 8},
		{Name: "Right Rear Leg", RollRange: [2]int{3, 4}, HP: 17, AP: 8},
		{Name: "Left Rear Leg", RollRange: [2]int{5, 6}, HP: 17, AP: 8},
		{Name: "Hindquarters", RollRange: [2]int{7, 8}, HP: 18, AP: 8},
		{Name: "Right Wing", RollRange: [2]int{9, 10}, HP: 16, AP: 8},
		{Name: "Left Wing", RollRange: [2]int{11, 12}, HP: 16, AP: 8},
		{Name: "Forequarters", RollRange: [2]int{13, 14}, HP: 19, AP: 8},
		{Name: "Right Front Leg", RollRange: [2]int{15, 16}, HP: 17, AP: 8},
		{Name: "Left Front Leg", RollRange: [2]int{17, 18}, HP: 17, AP: 8},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 17, AP: 8},
	},

	// tables.md §8.4 — Goblin/Orc.
	"goblin": {
		{Name: "Right Leg", RollRange: [2]int{1, 3}, HP: 5, AP: 1},
		{Name: "Left Leg", RollRange: [2]int{4, 6}, HP: 5, AP: 1},
		{Name: "Abdomen", RollRange: [2]int{7, 9}, HP: 6, AP: 1},
		{Name: "Chest", RollRange: [2]int{10, 12}, HP: 7, AP: 1},
		{Name: "Right Arm", RollRange: [2]int{13, 15}, HP: 4, AP: 1},
		{Name: "Left Arm", RollRange: [2]int{16, 18}, HP: 4, AP: 1},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 5, AP: 1},
	},

	// tables.md §8.4 — Horse.
	"horse": {
		{Name: "Right Hind Leg", RollRange: [2]int{1, 3}, HP: 9, AP: 1},
		{Name: "Left Hind Leg", RollRange: [2]int{4, 6}, HP: 9, AP: 1},
		{Name: "Hindquarters", RollRange: [2]int{7, 9}, HP: 10, AP: 1},
		{Name: "Forequarters", RollRange: [2]int{10, 12}, HP: 11, AP: 1},
		{Name: "Right Front Leg", RollRange: [2]int{13, 14}, HP: 8, AP: 1},
		{Name: "Left Front Leg", RollRange: [2]int{15, 16}, HP: 8, AP: 1},
		{Name: "Head", RollRange: [2]int{17, 20}, HP: 9, AP: 1},
	},

	// tables.md §8.4 — Minotaur (natural AP only on the Head: 3/7).
	"minotaur": {
		{Name: "Right Leg", RollRange: [2]int{1, 3}, HP: 7, AP: 0},
		{Name: "Left Leg", RollRange: [2]int{4, 6}, HP: 7, AP: 0},
		{Name: "Abdomen", RollRange: [2]int{7, 9}, HP: 8, AP: 0},
		{Name: "Chest", RollRange: [2]int{10, 12}, HP: 9, AP: 0},
		{Name: "Right Arm", RollRange: [2]int{13, 14}, HP: 6, AP: 0},
		{Name: "Left Arm", RollRange: [2]int{15, 16}, HP: 6, AP: 0},
		{Name: "Head", RollRange: [2]int{17, 20}, HP: 7, AP: 3},
	},

	// tables.md §8.4 — Shark.
	// [AMBIGUOUS: the printed ranges overlap — "17-18 Left Fin" and
	// "17-20 Head" both cover 17–18 (tables.md §8.4 footnote, digest
	// §9.10). Transcribed exactly as printed; the likely intended Head row
	// is 19-20, but the engine does not silently repair the source.]
	"shark": {
		{Name: "Tail", RollRange: [2]int{1, 3}, HP: 12, AP: 2},
		{Name: "Dorsal Fin", RollRange: [2]int{4, 6}, HP: 11, AP: 2},
		{Name: "Hindquarters", RollRange: [2]int{7, 10}, HP: 13, AP: 2},
		{Name: "Forequarters", RollRange: [2]int{11, 14}, HP: 14, AP: 2},
		{Name: "Right Fin", RollRange: [2]int{15, 16}, HP: 11, AP: 2},
		{Name: "Left Fin", RollRange: [2]int{17, 18}, HP: 11, AP: 2},
		{Name: "Head", RollRange: [2]int{17, 20}, HP: 12, AP: 2},
	},

	// tables.md §8.4 — Skeleton (no natural armor: AP 0).
	"skeleton": {
		{Name: "Right Leg", RollRange: [2]int{1, 3}, HP: 5, AP: 0},
		{Name: "Left Leg", RollRange: [2]int{4, 6}, HP: 5, AP: 0},
		{Name: "Abdomen", RollRange: [2]int{7, 9}, HP: 6, AP: 0},
		{Name: "Chest", RollRange: [2]int{10, 12}, HP: 7, AP: 0},
		{Name: "Right Arm", RollRange: [2]int{13, 15}, HP: 4, AP: 0},
		{Name: "Left Arm", RollRange: [2]int{16, 18}, HP: 4, AP: 0},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 5, AP: 0},
	},

	// tables.md §8.4 — Xenomorphic Alien.
	"xenomorph": {
		{Name: "Tail", RollRange: [2]int{1, 3}, HP: 5, AP: 6},
		{Name: "Right Rear Leg", RollRange: [2]int{4, 5}, HP: 6, AP: 6},
		{Name: "Left Rear Leg", RollRange: [2]int{6, 7}, HP: 6, AP: 6},
		{Name: "Abdomen", RollRange: [2]int{8, 10}, HP: 7, AP: 6},
		{Name: "Chest", RollRange: [2]int{11, 14}, HP: 8, AP: 6},
		{Name: "Right Arm", RollRange: [2]int{15, 16}, HP: 5, AP: 6},
		{Name: "Left Arm", RollRange: [2]int{17, 18}, HP: 5, AP: 6},
		{Name: "Head", RollRange: [2]int{19, 20}, HP: 7, AP: 6},
	},
}

// catalogAliases maps the printed creature names (and common short forms)
// onto catalog keys. It is immutable package data.
var catalogAliases = map[string]string{
	"human":               "humanoid",
	"giant ant":           "giant-ant",
	"bear":                "bear",
	"bear (grizzly)":      "bear",
	"grizzly":             "bear",
	"crocodile":           "crocodile",
	"alligator":           "crocodile",
	"crocodile/alligator": "crocodile",
	"goblin":              "goblin",
	"orc":                 "goblin",
	"goblin/orc":          "goblin",
	"horse":               "horse",
	"minotaur":            "minotaur",
	"shark":               "shark",
	"skeleton":            "skeleton",
	"xenomorphic alien":   "xenomorph",
}
