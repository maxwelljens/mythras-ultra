# Mythras Imperative — Machine-Implementable Rule Tables

Transcribed **exactly** from `Mythras-Imperative.md` (the Mythras Imperative System
Resource Document). Every numeric table in the source appears here, headed by the
source heading it comes under (line numbers refer to `Mythras-Imperative.md`).
Nothing is paraphrased; where the source is ambiguous or inconsistent, a footnote
`[AMBIGUOUS: ...]` is attached instead of guessing.

Dice conventions used below: `d2` (odd = 1, even = 2), `d3` (1–2 = 1, 3–4 = 2,
5–6 = 3), plus d4, d6, d8, d10, d12, d20, d100. All division results round **up**
(Introduction › Rounding of Numbers and Results).

---

## 1. Character Generation

### 1.1 Characteristic Dice Ranges (Step 2 › Calculating Characteristics)

| Method | STR, CON, DEX, POW, CHA | SIZ, INT |
| --- | --- | --- |
| Dice Roll | 3d6 | 2d6+6 |
| Dice Roll, Assign | 3d6 (values swappable within this group only) | 2d6+6 (values swappable within this group only) |
| Dice Roll, High | roll one extra die per characteristic, drop the lowest | roll one extra die per characteristic, drop the lowest |
| Points Build | pool of 80 points; each characteristic bounded by min/max of its dice range (3–18 for 3d6, 8–18 for 2d6+6) | same |

[AMBIGUOUS: "Points Build" says "Characteristics cannot be lower than the minimum
rolled value, nor higher than the maximum" — the min/max of the applicable dice
range (3d6: 3–18; 2d6+6: 8–18) is the natural reading, but no explicit table is
given. The source also mentions "the number in parentheses after each dice score is
the racial average" but supplies no such parenthetical values.]

### 1.2 Damage Modifier (Step 3 › Damage Modifier; STR+SIZ)

| STR+SIZ | Damage Modifier |
| --- | --- |
| 5 or less | –1d8 |
| 6–10 | –1d6 |
| 11–15 | –1d4 |
| 16–20 | –1d2 |
| 21–25 | +0 |
| 26–30 | +1d2 |
| 31–35 | +1d4 |
| 36–40 | +1d6 |
| 41–45 | +1d8 |
| 46–50 | +1d10 |
| 51–60 | +1d12 |
| 61–70 | +2d6 |
| 71–80 | +1d8+1d6 |
| 81–90 | +2d8 |
| 91–100 | +1d10+1d8 |
| 101–110 | +2d10 |
| 111–120 | +2d10+1d2 |
| Each 10 points | Continue Progression |

[AMBIGUOUS: "Continue Progression" beyond 120 is not defined; the row says only
"Continue Progression".]

A negative Damage Modifier subtracted from a weapon's damage that reaches zero or
below inflicts **no damage at all** (Step 3 › Damage Modifier).

### 1.3 Experience Modifier (Step 3; CHA)

| CHA | Exp. Modifier |
| --- | --- |
| 6 or less | -1 |
| 7-12 | +0 |
| 13-18 | +1 |
| Each 6 points | +1 |

### 1.4 Healing Rate (Step 3; CON)

| CON | Healing Rate |
| --- | --- |
| 6 or less | 1 |
| 7-12 | 2 |
| 13-18 | 3 |
| Each 6 points | +1 |

### 1.5 Luck Points (Step 3; POW)

| POW | Luck Points |
| :-: | :-: |
| 6 or less | 1 |
| 7-12 | 2 |
| 13-18 | 3 |
| Each 6 points | +1 |

### 1.6 Hit Points per Location (Step 3; CON+SIZ)

| Location | 1–5 | 6–10 | 11–15 | 16–20 | 21–25 | 26–30 | 31–35 | 36–40 | +5 |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Head | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | +1 |
| Chest | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | +1 |
| Abdomen | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | +1 |
| Each Arm | 1 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | +1 |
| Each Leg | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | +1 |

("+5" column: +1 HP to each location per additional 5 points of CON+SIZ above 40.)

### 1.7 Other Derived Attributes (Step 3)

| Attribute | Formula / Value |
| --- | --- |
| Action Points | 2 (all characters) |
| Magic Points | equal to POW |
| Initiative | average of DEX and INT |
| Movement Rate | not derived from characteristics; species default; humans = 6 metres |

[AMBIGUOUS: rounding of Initiative "average" is not stated explicitly; the general
rounding rule (round fractions up) would give ceil((DEX+INT)/2).]

### 1.8 Standard Skills — Basic Percentage (Step 4)

| Skill | Basic Percentage |
| :-- | :-: |
| Athletics | STR+DEX |
| Boating | STR+CON |
| Brawn | STR+SIZ |
| Conceal | DEX+POW |
| Customs | INT x2, +40 |
| Dance | DEX+CHA |
| Deceit | INT+CHA |
| Drive | DEX+POW |
| Endurance | CON x2 |
| Evade | DEX x2 |
| First Aid | INT+DEX |
| Influence | CHA x2 |
| Insight | INT+POW |
| Locale | INT x2 |
| Native Tongue | INT+CHA, +40 |
| Perception | INT+POW |
| Ride | DEX+POW |
| Sing | CHA+POW |
| Stealth | DEX+INT |
| Swim | STR+CON |
| Unarmed | STR+DEX |
| Willpower | POW x2 |

Plus **Combat Style**: Basic Percentage = STR+DEX (Step 4; one per culture/career).

Footnote in source: Customs and Native Tongue include a static +40% bonus (Common
language for humans).

### 1.9 Professional Skills — Base Percentage (Skills › Professional Skills)

| Skill | Base Percentage |
| :-- | :-: |
| Acting | CHA x2 |
| Acrobatics | STR+DEX |
| Astrogation | INT x2 |
| Art | POW+CHA |
| Bureaucracy | INT x2 |
| Commerce | INT+CHA |
| Comms | INT x2 |
| Computers | INT x2 |
| Courtesy | INT+CHA |
| Craft | DEX+INT |
| Culture | INT x2 |
| Demolitions | INT+POW |
| Disguise | INT+CHA |
| Electronics | DEX+INT |
| Engineering | INT x2 |
| Forgery | DEX+INT |
| Gambling | INT+POW |
| Healing | INT+POW |
| Language | INT+CHA |
| Literacy (Specific Language) | INT x2 |
| Lockpicking | DEX x2 |
| Lore | INT x2 |
| Magic | POW+CHA |
| Mechanisms | DEX+INT |
| Musicianship | DEX+CHA |
| Navigation | INT+POW |
| Oratory | POW+CHA |
| Pilot | DEX+INT |
| Politics | INT+CHA |
| Research | INT+POW |
| Science | INT x2 |
| Seamanship | INT+CON |
| Seduction | INT+CHA |
| Sensors | INT+POW |
| Sleight | DEX+CHA |
| Streetwise | POW+CHA |
| Survival | CON+POW |
| Teach | INT+CHA |
| Track | INT+CON |

Also referenced (Creatures › Creature Abilities › Flying): **Fly** skill, base
STR+DEX (used by flying creatures).

### 1.10 Skill Point Pools (Steps 4–7)

| Pool | Points | Spend on |
| --- | --- | --- |
| Culture | 100 | culture's listed Standard skills + up to 3 chosen Professional skills |
| Career | 100 | career's listed Standard skills + up to 3 chosen Professional skills |
| Bonus | 150 | any Standard skill, or Professional skills gained from Culture/Career |

1 point = +1% to a skill. GM may cap points per skill. Total = 350.

### 1.11 Culture Skill Packages (Step 5)

| Culture | Standard Skills | Professional Skills (pick up to 3) |
| --- | --- | --- |
| Barbarian | Athletics, Brawn, Endurance, First Aid, Locale, Perception; and either Boating or Ride; plus one cultural Combat Style | Craft (any), Healing, Lore (any), Musicianship, Navigation, Seamanship, Survival, Track |
| Civilised | Conceal, Deceit, Drive, Influence, Insight, Locale, Willpower; plus one cultural Combat Style | Art (any), Commerce, Craft (any), Courtesy, Language (any), Lore (any), Musicianship, Streetwise |
| Nomadic | Endurance, First Aid, Locale, Perception, Stealth; and two of: Athletics, Boating, Swim, Drive, Ride (per primary mode of travel); plus one cultural Combat Style | Craft (any), Culture (any), Language (any), Lore (any), Musicianship, Navigation, Survival, Track |
| Primitive | Brawn, Endurance, Evade, Locale, Perception, Stealth; and one of: Athletics, Boating, Swim; plus one cultural Combat Style | Craft (any), Healing, Lore (any), Musicianship, Navigation, Survival, Track |

### 1.12 Careers (Step 6) — Standard and Professional Skills

| Career Name | Standard Skills | Professional Skills |
| :-- | :-- | :-- |
| Agent (Agitator, Assassin, Detective, Informer, Spy…) | Conceal, Deceit, Evade, Insight, Perception, Stealth; Combat Style (Specific Agent or Cultural Style) | Culture (any), Disguise, Language (any), Sleight, Streetwise, Survival, Track |
| Beast Handler (Animal Breeder, Beast Tamer, Menagerie Keeper…) | Drive, Endurance, First Aid, Influence, Locale, Ride, Willpower | Craft (Animal Husbandry), Commerce, Healing (Specific Species), Lore (Specific Species), Survival, Teach (Specific Species), Track |
| Bounty Hunter (Bail enforcement agent, Bounty killer, Scalp hunter, Skip tracer...) | Athletics, Endurance, Evade, Insight, Perception, Stealth; Combat Style (any) | Bureaucracy, Commerce, Culture (any), Linguistics, Streetwise, Survival (any), Track |
| Courtesan (Concubine, Escort, Harem-girl, Houri, Paramour…) | Customs, Dance, Deceit, Influence, Insight, Perception, Sing | Art (any), Courtesy, Culture (any), Gambling, Language (any), Musicianship, Seduction |
| Crafter, Artificer, Artisan… | Brawn, Drive, Influence, Insight, Locale, Perception, Willpower | Art (any), Commerce, Craft (Primary), Craft (Secondary), Engineering, Mechanisms, Streetwise |
| Detective Gumshoe, Policeman, Private Eye, Private Investigator, Sleuth... | Customs, Evade, Influence, Insight, Perception, Stealth; Combat Style (Unarmed or Side Arms style) | Bureaucracy, Culture (any), Disguise, Linguistics, Lore (any), Research, Sleight, Streetwise |
| Entertainer, Acrobat, Bard, Dancer, Player, Poet… | Athletics, Brawn, Dance, Deceit, Influence, Insight, Sing | Acrobatics, Acting, Oratory, Musicianship, Seduction, Sleight, Streetwise |
| Farmer, Estate Owner, Gardener, Ploughman… | Athletics, Brawn, Drive, Endurance, Locale, Perception, Ride | Commerce, Craft (any), Lore (Agriculture), Lore (Animal Husbandry), Navigation, Survival, Track |
| Fisher, Beach Comber, Net Tender, Pearl Diver, Whaler… | Athletics, Boating, Endurance, Locale, Perception, Stealth, Swim | Commerce, Craft (Any), Lore (Primary Catch), Lore (Secondary Catch), Navigation, Seamanship, Survival |
| Gambler, Bookie, Cardsharp, Hustler... | Athletics, Brawn, Endurance, Locale, Perception, Willpower; and either Drive or Ride | Acting, Bureaucracy, Commerce, Courtesy, Gambling, Research, Sleight, Streetwise |
| Herder, Breeder, Herdsman, Sacred Goose Tender, Wrangler… | Endurance, First Aid, Insight, Locale, Perception, Ride; Combat Style (Specific Herding or Cultural Style) | Commerce, Craft (Animal Husbandry), Healing (Specific Species), Navigation, Musicianship, Survival, Track |
| Hunter, Forester, Poacher, Scavenger, Stalker, Trapper… | Athletics, Endurance, Locale, Perception, Ride, Stealth; Combat Style (Specific Hunting or Cultural Style) | Commerce, Craft (Hunting Related), Lore (Regional or Specific Species), Mechanisms, Navigation, Survival, Track |
| Journalist, Commentator, Hack, Newshound, Pressman... | Customs, Deceit, Influence, Insight, Locale, Native Tongue, Perception | Bureaucracy, Culture (any), Language (any), Lore (any), Oratory, Politics, Streetwise |
| Magician, Mage, Sorcerer, Witch, Wizard | Customs, Deceit, Influence, insight, Locale, Perception, Willpower | Culture, Magic, Literacy, Lore x2, Oratory, Sleight |
| Mechanic, Engineer, Grease monkey... | Brawn, Culture, Drive, Endurance, Influence, Locale, Willpower | Commerce, Craft (primary), Craft (secondary), Electronics, Gambling, Mechanisms, Streetwise |
| Merchant, Broker, Hawker, Money Lender, Smuggler, Trader… | Boating, Drive, Deceit, Insight, Influence, Locale, Ride | Commerce, Courtesy, Culture (any), Language (any), Navigation, Seamanship, Streetwise |
| Miner, Prospector, Quarryman, Sapper, Well Digger… | Athletics, Brawn, Endurance, Locale, Perception, Sing, Willpower | Commerce, Craft (Mining), Engineering, Lore (Minerals), Mechanisms, Navigation (Underground), Survival |
| Official, Minister, Overseer, Steward, Tax Collector… | Customs, Deceit, Influence, Insight, Locale, Perception, Willpower | Bureaucracy, Commerce, Courtesy, Language (any), Literacy, Lore (any), Oratory |
| Physician, Doctor, Healer, Medicine Man, Torturer, Vivisectionist… | Dance, First Aid, Influence, Insight, Locale, Sing, Willpower | Commerce, Craft (Specific Physiological Speciality), Healing, Language (any), Literacy (any), Lore (any), Streetwise |
| Pilot, Aeronaut, Astronaut, Barnstormer, Flyboy, Test Pilot... | Brawn, Drive, Endurance, Evade, Locale, Perception, Willpower | Customs (any), Electronics, Mechanisms, Navigation, Pilot, Sensors, Streetwise |
| Politician, Civil Servant, Mandarin, Minister, Overseer, Steward… | Customs, Deceit, Influence, Insight, Locale, Native Tongue, Perception | Bureaucracy, Courtesy, Culture (any), Language (any), Lore (any), Oratory, Politics |
| Priest, Arbiter, Cultist, Druid, Mendicant, Prophet… | Customs, Dance, Deceit, Influence, Insight, Locale, Willpower | Bureaucracy, Courtesy, Customs, Literacy (any), Lore (any), Oratory, Politics |
| Sailor, Captain, Galley Slave, Marine, Pirate, Raft man… | Athletics, Boating, Brawn, Endurance, Locale, Swim; Combat Style (Specific Sailor or Cultural Style) | Craft (Specific Shipboard Speciality), Culture (any), Language (any), Lore (any), Navigation, Seamanship, Survival |
| Scholar, Annalist, Librarian, Philosopher, Skald, Scribe… | Customs, Influence, Insight, Locale, Native Tongue, Perception, Willpower | Culture (any), Language (any), Literacy (any), Lore (Primary), Lore (Secondary), Oratory, Teach |
| Scientist, Archivist, Librarian, Philosopher… | Customs, Influence, Insight, Locale, Native Tongue, Perception, Willpower | Culture (any), Engineering, Language (any), Oratory, Research, Science (any), Teach |
| Scout, Bounty Hunter, Explorer, Pioneer, Ranger, Wayfarer… | Athletics, Endurance, First Aid, Perception, Stealth, Swim; Combat Style (Specific Hunting or Cultural Style) | Culture (any), Healing, Language (any), Lore (any), Navigation, Survival, Track |
| Servant, Aide, Butler, Chauffeur, House Maid, Governess, Major Domo, Manservant, Personal Assistant, Secretary... | Customs, Deceit, Drive, Influence, Insight, Locale, Perception | Bureaucracy, Courtesy, Craft (service related), Culture (any), Language (any), Politics, Streetwise |
| Smuggler, Black Marketer, Bootlegger, Gun Runner... | Conceal, Customs, Deceit, Drive, Influence, Insight, Locale | Bureaucracy, Commerce, Customs (any), Language (any), Navigation, Streetwise; and either Astrogation or Seamanship |
| Technician, Electrician, Laboratory Assistant... | Brawn, Drive, Endurance, Influence, Locale, Perception, Willpower | Comms, Craft (any), Electronics, Gambling, Mechanisms, Science (any), Sensors |
| Thief, Burglar, Conman, Fence, Mugger, Tomb Robber… | Athletics, Deceit, Evade, Insight, Perception, Stealth; Combat Style (Specific Thief or Cultural Style) | Acting, Commerce, Disguise, Lockpicking, Mechanisms, Sleight, Streetwise |
| Warrior, Bodyguard, Mercenary, Pit Fighter, Soldier… | Athletics, Brawn, Endurance, Evade, Unarmed; Combat Style (Military Style), Combat Style (Cultural Style) | Craft (any), Engineering, Gambling, Lore (Military History), Lore (Strategy and Tactics), Oratory, Survival |

[AMBIGUOUS: "Linguistics" (Bounty Hunter, Detective careers) is not a defined
skill in the Skills chapter — presumed to mean Language. "Culture" appears among
the Mechanic career's Standard skills although Culture is a Professional skill.
The Magician career lists "Culture, Magic, Literacy, Lore x2, Oratory, Sleight"
without "(any)" qualifiers.]

### 1.13 Experience Table (Step 7 › Creating Experienced Characters)

| **Category** | **Age** | **Bonus Skill Points** | **Maximum Skill Increase** |
| :-- | :-: | :-: | :-: |
| Young | 10+1d6 | 100 | +10 |
| Adult | 15+2d6 | 150 | +15 |
| Middle Aged | 25+3d6 | 200 | +20 |
| Senior | 40+4d6 | 250 | +25 |
| Old | 60+5d6 | 300 | +30 |

Notes: Bonus Skill Points are how many points the character spends on skills;
Maximum Skill Increase caps how many Bonus Points may go to any one skill.

[AMBIGUOUS: it is not stated explicitly that the age-category Bonus Skill Points
replace the default 150-point Bonus pool (with Culture 100 + Career 100 retained);
the text says "the Age Bonus should be treated as approximate" and "when converting
an existing character, simply substitute the actual age for that noted on the
table".]

**There are no aging penalty rules in this document** — age categories exist only
to calibrate starting experience.

### 1.14 The Skill Pyramid (Step 7 › The Skill Pyramid) — alternative to the 350 points

| Allocation | Count | Subtotal |
| --- | --- | --- |
| 50% | 1 skill | 50 |
| 40% | 2 skills | 80 |
| 30% | 3 skills | 90 |
| 20% | 4 skills | 80 |
| 10% | 5 skills | 50 |
| **Total** | **15 allocations** | **350** |

Rules: all points allocated; at most one allocation per skill; Customs and Native
Language still gain +40; Passions still gain their values; no skill may exceed 80%
from an allocation (base + allocation); six of the values must go to Professional
Skills; recommended (not mandatory) one value to a Combat Style.

### 1.15 Pulp Hero Characters (Step 7 › Larger-Than-Life Heroics)

| Element | Value |
| --- | --- |
| Characteristics (Dice Roll, Assign) | roll 4d6 drop lowest, five times → assign to STR, CON, DEX, POW, CHA; roll 3d6+6 drop lowest, twice → assign to SIZ, INT |
| Points Build pool | 90 |
| Advantages (choose 2, no stacking) | +1 Action Point; +1 Luck Point; +1 Hit Point to each Hit Location; Endurance rolls one Grade easier; Stealth rolls one Grade easier; Willpower rolls one Grade easier |
| Healing | double Healing Rate for Minor and Serious Wounds (Major Wounds at standard rate) |
| Bonus Skill Points | +50 in addition |

### 1.16 Paragon Characters (Step 7 › Larger-Than-Life Heroics)

| Element | Value |
| --- | --- |
| Characteristics (Dice Roll, Assign) | roll 4d6 drop lowest, six times → assign five results of choice to STR, CON, DEX, POW, CHA; roll 3d6+6 drop lowest, three times → assign two results of choice to SIZ, INT |
| Points Build pool | 100 |
| Advantages (choose 3, no stacking) | +1 Action Point; +2 Luck Points; +2 Hit Points to each Hit Location; Endurance rolls one Grade easier; Stealth rolls one Grade easier; Willpower rolls one Grade easier |
| Healing | double Healing Rate for Minor and Serious Wounds (Major Wounds at standard rate) |
| Bonus Skill Points | +100 in addition |

### 1.17 Passions (Step 8)

| **Object of Passion** | **Starting Percentage** |
| :-- | --- |
| A person, in a romantic or familial context | POW+CHA |
| An organization or group of people | POW+INT |
| A race or species | Character's POW x2 |
| A place | Character's POW+INT |
| An object or substance | Character's POW x2 |
| A personality trait | Character's POW+CHA |

Starting characters: up to three Passions, each based on two Characteristics, with
a bonus of +40 to the first, +30 to the second, +20 to the third (player chooses
order of importance). Passion values range 1–100+ (treated like skills).
Augmentation use: a Passion adds **20% of its value** to a skill it augments.

**Deepening and Waning** (Step 8 › Deepening and Waning):

| **Depth of Change** | **Passion Modified by** |
| --- | --- |
| Weak | 1d10 |
| Moderate | 1d10+5 |
| Strong | 1d10+10 |

(Change may instead reverse the Passion at the same percentage, GM's call.)

### 1.18 Starting Equipment (Step 9)

- Clothing appropriate to profession and setting.
- Pocket money ("enough to last a little while without hardship"). [AMBIGUOUS: no
  currency system or price list exists in this document.]
- A personal heirloom/keepsake/trinket of little relative value.
- Trade tools or transportable equipment suitable to the profession, if appropriate.
- Any personal weapon for the character's Combat Style.
- Other items as appropriate, subject to GM approval.

---

## 2. Skill Mechanics

### 2.1 Success Levels (Skills › How Skills Work; Criticals and Fumbles)

The only four success levels are: **Critical Success, Success, Failure, Fumble**
(there is no "Exceptional" tier in Mythras Imperative).

| Outcome | Rule |
| --- | --- |
| Success | d100 ≤ skill value |
| Failure | d100 > skill value |
| Always Success | any roll of 01–05, irrespective of skill |
| Always Failure | any roll of 96–00, irrespective of skill |
| Critical Success | roll ≤ 1/10th of (modified) skill value; fractions round up |
| Fumble | roll of 99 or 00 (skills >100% fumble only on 00) |

Example in source: skill modified 33% → 66% gives Critical range 07 or less
(6.6 rounds up to 7).

### 2.2 Difficulty Grade Table (Skills › Modifying Skills)

| Difficulty Grade | Skill Modifier | Simplified Skill Modifier |
| :-- | :-- | :-- |
| Automatic | No need to roll | No need to roll |
| Very Easy | Double the skill value | +40% |
| Easy | Add half again to the skill value | +20% |
| Standard | No adjustment | None |
| Hard | Reduce skill value by one-third | -20% |
| Formidable | Reduce the skill value by half | -40% |
| Herculean | Reduce the skill value to one-fifth | -80% |
| Hopeless | No attempt can be made | No attempt can be made |

When multiple penalties apply, the hardest Difficulty Grade takes precedence. The
two modifier columns are alternative systems; apply one consistently.

[AMBIGUOUS: the Ranged Combat Situational Modifiers table lists "Impossible" for a
completely invisible target — a grade not defined in this table (treated as one
step beyond Hopeless; footnote says treat as Herculean if the general location is
known).]

### 2.3 Differential Roll Results (Skills › Differential Rolls)

| Result of Rolls | Antagonist Critical | Antagonist Success | Antagonist Failure | Antagonist Fumble |
| --- | --- | --- | --- | --- |
| Protagonist Critical | No Benefit | Protagonist Wins 1 Level of Success | Protagonist Wins 2 Levels of Success | Protagonist Wins 3 Levels of Success |
| Protagonist Success | Antagonist Wins 1 Level of Success | No Benefit | Protagonist Wins 1 Level of Success | Protagonist Wins 2 Levels of Success |
| Protagonist Failure | Antagonist Wins 2 Levels of Success | Antagonist Wins 1 Level of Success | No Benefit | No Benefit |
| Protagonist Fumble | Antagonist Wins 3 Levels of Success | Antagonist Wins 2 Levels of Success | No Benefit | No Benefit |

### 2.4 Opposed Roll Examples (Skills › Opposed Rolls)

| Activity | Opposed Skill Examples |
| --- | --- |
| Bargaining/Haggling | Commerce vs. Commerce, Influence vs. Commerce |
| Chasing/Racing | Athletics vs. Athletics, Athletics vs. Endurance, Ride vs. Ride |
| Deceiving or Lying | Deceit vs. Insight, Sleight vs. Perception |
| Displaying Loyalty | Passion vs. Passion, Passion vs. Insight, Passion vs. Influence |
| Getting out of the Way | Evade vs. Athletics, Evade vs. Mechanisms, Evade vs. Ride |
| Hiding or Sneaking | Stealth vs. Perception; Stealth vs. Track |
| Pulling Rank | Influence vs. Willpower, Deceit vs. Willpower |
| Staying the Course | Endurance vs. Endurance, Endurance vs. Brawn |
| Swaying a Crowd | Oratory vs. Willpower |
| Test of Strength | Brawn vs. Brawn, Brawn vs. Endurance |
| Test of Wills | Willpower vs. Willpower, Seduction vs. Willpower |

Opposed-roll tiebreak: same Level of Success → the higher dice roll still within
the success range wins. If both Fail: re-roll the contest (after narrative) or the
GM invents a fitting outcome. For skills >100%: the highest-skilled participant
(after modifiers) subtracts (skill − 100) from every participant's skill,
including their own.

### 2.5 Competency Level Table (Skills › So, Just How Good Are You?)

| Skill Rating | Random Value | Competence Level | Military Equivalent | Description |
| --- | --- | --- | --- | --- |
| 1-30% | 1d20+10 | Novice | Cannon Fodder | Unqualified and learning the basics |
| 31-50% | 1d20+30 | Competent | Green | Can perform easy tasks if taking time to be careful |
| 51-70% | 1d20+50 | Proficient | Seasoned | Has reached an employable level of competence |
| 71-90% | 1d20+70 | Expert | Veteran | Recognised as a professional or subject matter expert in the field |
| 91-110% | 1d20+90 | Master | Elite | Considered one of the best in the business |
| >110% | 1d20+110 | Exemplar | Heroic | Renowned far and wide for their consummate skill |

### 2.6 Experience Roll Results (Game System › Character Improvement)

| Roll (1d100 + INT vs. current skill) | Skill increase |
| --- | --- |
| ≥ skill being improved | +1d4+1% |
| < skill being improved | +1% |
| Skill Fumbled since last Experience Rolls | free +1% (applied before any Experience Roll; Fumbles of the same skill do not stack) |

Constraints: one Experience Roll per skill per session. GM awards 1–3 Experience
Rolls per session (rule of thumb). Learning a new Professional Skill: one month of
study + 3 Experience Rolls (+ in-game costs) → skill at Base Level. New Passion at
improvement: 1 Experience Roll → base value. (In-play Passions are free.)

### 2.7 Language Fluency Bands (Skills › Language)

| Skill value | Fluency |
| --- | --- |
| 1-25% | a few basic words |
| 26-50% | simple sentences |
| 51-75% | fluent for general conversation |
| 76+% | able to converse eloquently |

Language is static (not rolled); it caps dependent skills (e.g. Bureaucracy cannot
exceed the Language value used). Native Tongue works the same way and is likewise
not rolled against directly.

---

## 3. Game System Tables

### 3.1 Asphyxiation (Game System › Asphyxiation)

Hold breath: seconds equal to Endurance skill (prepared); halved if unprepared in a
passive situation; one-fifth if engaged in strenuous activity. Thereafter, Endurance
roll:

| Endurance roll | Effect |
| --- | --- |
| Critical Success | no further deterioration |
| Success | accrue 1 extra level of Fatigue |
| Failure | 1d2 extra levels of Fatigue that Round |
| Fumble | 1d3 extra levels of Fatigue that Round |

Recovery after asphyxiation ends: 1 Fatigue level per minute. GM may impose an
enduring Fatigue level (smoke/water inhalation).

### 3.2 Disease and Poison Traits (Game System › Disease and Poison Traits)

| Trait | Meaning |
| --- | --- |
| Application | Ingestion / Inhaled / Contact / Injected |
| Potency | set against resisting skill (usually Endurance or Willpower) in an Opposed Roll; win = shrug off; lose = suffer all Conditions |
| Resistance | Endurance or Willpower (possibly exotic); may be rolled at exposure or deferred to Onset Time |
| Onset Time | delay (seconds → longer) before effects |
| Duration | how long Conditions last |
| Conditions | one or more from the Conditions Table |
| Antidote/Cure | treatment if any; successful treatment prevents further conditions |

### 3.3 Conditions Table (Game System › Disease and Poison Traits)

| **Condition** | **Effects** |
| --- | --- |
| Agony | Any skill roll involving the affected area must also be ≤ the character's Willpower, else the attempt fails (victim moans/screams). |
| Asphyxiation | Victim collapses, unable to breathe; use Asphyxiation rules. May be asthmatic (shortness of breath) or complete respiratory failure (death); kept alive by winning opposed First Aid vs. Potency. |
| Bleeding | Internal bleeding or surface haemorrhaging → Blood Loss effects. |
| Contagious | Victim can transfer the poison or disease by touch. |
| Death | Victim collapses incapacitated, dies after a number of rounds equal to their CON. |
| Fever | All skills suffer a difficulty grade of Hard. |
| Mania | Driven to follow a compulsion (as noted in description). |
| Nausea | Cannot eat; Endurance roll each stressful physical action to avoid vomiting (1d3 rounds, cannot act); long duration may cause starvation. |
| Paralysis | Affected area (or whole body) unable to move for the duration. |

### 3.4 Sample Disease: Red Pox (Game System)

| Field | Value |
| --- | --- |
| Application | Contact |
| Potency | 50 |
| Resistance | Endurance |
| Onset time | 1d6+6 hours |
| Duration | 1d6+3 days |
| Conditions | On manifestation: Contagious. 1 day later: Fever and Bleeding. Each day the pustules weep blood, lose one level of Fatigue, not recovered until duration completes. |
| Antidote/Cure | Can be cured by the Healing skill. |

Object contagion persists 1d3 days after contact.

### 3.5 Falling Distance Table (Game System › Falling)

| **Distance Fallen** | **Damage Taken** |
| --- | --- |
| **1 metre or less** | No damage. |
| **2-5 metres** | 1d6 points of damage to one random location. |
| **6-10 metres** | 2d6 points of damage to two random locations. |
| **11-15 metres** | 3d6 points of damage to three random locations. |
| **16-20 metres** | 4d6 points of damage to four random locations. |
| **Each +5 metres** | +1d6 damage. |

Armor Points do not reduce falling damage; the falling character's/creature's
Damage Modifier applies to damage suffered.

**Falling Objects**: 1d6 damage per 6 points of SIZ (or fraction thereof), plus the
Falling Distance Table damage.

### 3.6 Fatigue Levels (Game System › Fatigue)

| **Fatigue Level** | **Skill Grade** | **Movement** | **Initiative** | **Action Points** | **Recovery Period** |
| --- | --- | --- | --- | --- | --- |
| Fresh | – | – | No Penalties | – | – |
| Winded | Hard | No Penalties | No Penalties | No Penalties | 15 minutes |
| Tired | Hard | -1 metre | No Penalties | No Penalties | 3 hours |
| Wearied | Formidable | -2 metres | -2 | No Penalties | 6 hours |
| Exhausted | Formidable | Halved | -4 | -1 | 12 hours |
| Debilitated | Herculean | Halved | -6 | -2 | 18 hours |
| Incapacitated | Herculean | Immobile | -8 | -3 | 24 hours |
| Semi-Conscious | Hopeless | No Activities Possible | No Activities Possible | No Activities Possible | 36 hours |
| Comatose | No Activities Possible | No Activities Possible | No Activities Possible | No Activities Possible | 48 hours |
| Dead | Dead | – | – | – | Never |

Recovery: complete rest for Recovery Period ÷ Healing Rate per accrued level
(physical exertion Fatigue; asphyxiation recovers faster, blood loss slower).
Fatigue accrues one level per failed Athletics/Brawn/Endurance roll (GM discretion)
and via Asphyxiation, Blood Loss, etc.

### 3.7 Fire Intensity Table (Game System › Fires)

| Intensity | Examples | Time to Ignite | Damage |
| --- | --- | --- | --- |
| 1 | Candle | 1d4 | 1d2 |
| 2 | Torch | 1d3 | 1d4 |
| 3 | Campfire | 1d2 | 1d6 |
| 4 | Room filling conflagration | 1d2 | 2d6 |
| 5 | Volcanic lava | Instant | 3d6 |

"Time to Ignite" units: Rounds. Intensity 1–2 damage hits one location; 3–4 hit
nearest 1d4+1 locations; 5 hits all locations. Fire-based attacks ignite flammables
with chance = damage rolled ×5% (electrical: ×1%). Burning materials take 1d3-1
damage per Round (ignores AP for materials; flesh below takes it too; natural armor
protects). Smother: drop prone + Athletics (1 AP, once per Turn); assistance or a
location that self-extinguished makes the roll one grade easier; zero damage rolled
means the fire went out in that location.

### 3.8 Humanoid Hit Locations (Game System › Hit Locations)

| 1d20 | Hit Location |
| --- | --- |
| 1-3 | Right Leg |
| 4-6 | Left Leg |
| 7-9 | Abdomen |
| 10-12 | Chest |
| 13-15 | Right Arm |
| 16-18 | Left Arm |
| 19-20 | Head |

### 3.9 Wound Levels (Game System › Damage and Wound Levels)

| Wound Level | Threshold (after damage application) |
| --- | --- |
| Minor Wound | Hit Location still has positive Hit Points |
| Serious Wound | Hit Location reduced to zero Hit Points or below |
| Major Wound | Hit Location reduced to a negative score ≥ its starting Hit Points |

Damage application order: (1) attacker's Damage Modifier; (2) weapon-enhancing/
reducing magic; (3) parry reduction (comparative weapon sizes); (4) Armor Points.

**Serious Wound** effects: victim cannot attack or start to cast spells for 1d3
Turns (can Parry/Evade). Limb hit: opposed Endurance vs. the enemy's attack roll —
fail = limb useless until location back to positive HP (leg: drops prone; arm:
drops held object unless strapped). Abdomen/Chest/Head: opposed Endurance vs. attack
roll — fail = unconscious for minutes equal to the damage of the wounding attack
(First Aid/Healing can revive). Ongoing: GM may impose one Difficulty Grade on
tasks using that location until reduced to Minor.

[AMBIGUOUS: "the next 1d3 Turns" — the Combat chapter defines a Turn as one
participant's action slot within a 5-second Round; context implies Rounds.]

**Major Wound** effects: victim immediately Incapacitated, drops prone. Limb is
severed/transfixed/shattered/ripped off. Opposed Endurance vs. attack roll — fail =
unconscious. Untreated severed/ripped location: death after minutes = 5 × Healing
Rate (blood loss/shock). Torso/Head Major Wound: drops unconscious, totally
Incapacitated; opposed Endurance vs. attack roll — fail = instant death; if
survived, death unless treated within Combat Rounds = 2 × Healing Rate.

**Blood Loss**: measured as Fatigue levels (like Asphyxiation) but enduring;
recover 1 Fatigue level per day, starting the day after bleeding ceases.

**Heroic Last Actions**: after a Major Wound, a key character may burn 1 Luck Point
for one final action, then collapse.

### 3.10 Healing from Injury (Game System)

| Wound level | Natural healing rate |
| --- | --- |
| Minor Wounds | Healing Rate in HP per **day** |
| Serious Wounds | Healing Rate in HP per **week** (until location above zero, then per day) |
| Major Wounds | Healing Rate in HP per **month** |

Restrictions: strenuous activity reduces Healing Rate by 1d3; Major Wounds do not
begin healing until a successful Healing roll or healing magic; non-dismembering
Major Wounds untreated for days = 1/20th of the Healing **skill** value become
maimed (permanent HP loss to the location). First Aid: once per injury, heals 1d3.

### 3.11 Inanimate Objects (Game System › Inanimate Objects)

| Object | Armor Points/Hit Points |
| --- | --- |
| Castle Gate | 8/150 |
| Castle Wall | 10/500 |
| Chain/Shackle | 8/8 |
| Iron Post | 8/15 |
| Glass Window | 1/4 |
| Iron Door | 12/30 |
| Rope | 6/3 |
| Wooden Chair | 4/15 |
| Brick Wall | 6/30 |
| Concrete Wall | 8/40 |
| Automobile | 6/30 |
| Bulletproof Glass | 8/25 |
| Steel Beam | 12/50 |
| Armored Personnel Carrier | 10/50 |
| Battle Tank | 15/100 |
| Truck/Excavator | 8/40 |
| Tree Trunk | 6/50 |

Breakage: Brawn, Unarmed, or Combat Style roll (as appropriate) to damage an item.
Weapon vs. object with AP ≥ weapon's AP damages both object and weapon.

### 3.12 Luck Points (Game System › Luck Points)

| Use (1 Luck Point each) | Effect |
| --- | --- |
| Cheat Fate | Re-roll or swap digits (75 → 57) any dice roll the character makes (skill, damage, etc.); can force an opponent to re-roll an attack/damage roll against them |
| Desperate Effort | Gain one additional Action Point after AP are exhausted |
| Mitigate Damage | Downgrade a Major Wound to Serious; damage reduced to 1 HP less than the Major threshold |

Constraints: only one Luck Point per action; cannot be spent on Experience or
Training rolls; pool replenishes to normal value at the start of each session.

---

## 4. Combat Tables

### 4.1 Combat Round Structure (Combat › Combat Rounds and Initiative)

| Term | Rule |
| --- | --- |
| Combat Round | 5 seconds of real time |
| Action Points | 2 per character per Round (all characters); unused AP do not carry over |
| Initiative roll | 1d10 + Initiative Modifier (higher acts first; ties → higher DEX; still tied → opposed die roll) |
| Turn | one Combat Action per Turn, per available AP; proactive actions only on own Turn |
| Reactions | one reaction attempt per threat; costs 1 AP; any time in the Round |
| Free Actions | cost 0 AP |

Armor penalty: the highest Armor Point value of armor worn penalises the
Initiative roll.

### 4.2 Combat Actions and Costs (Combat › Combat Actions)

All actions cost 1 Action Point unless marked Free.

| Action | Type | Cost | Key mechanics |
| --- | --- | --- | --- |
| Attack | Proactive | 1 AP | melee or ranged strike |
| Brace | Proactive | 1 AP | SIZ +50% vs Knockback/Leaping Attacks; SIZ doubled vs Bash |
| Cast Magic/Use Power | Proactive | 1 AP (+ more for complex) | release any time until next Turn; then needs Hold |
| Change Range | Proactive | 1 AP | close/retreat; not while engaged or pinned by fire |
| Charge | Proactive | 1 AP | run/sprint into engagement; see Charging |
| Delay | Proactive | 1 AP (spent on the delayed acts) | reserve actions for Reactions/Interrupt; unused = Dither |
| Dither | Proactive | 1 AP | do nothing (default) |
| Hold Magic/Power | Proactive | 1 AP/turn | hold cast spell; allows enemy Counter Spell |
| Mount | Proactive | 1 AP (large mounts: several Turns) | mount/dismount |
| Move | Proactive | 1 AP | move Base Movement Rate (unengaged) |
| Outmanoeuvre | Proactive | 1 AP | group opposed Evade; losers can't attack this Round |
| Ready Weapon | Proactive | 1 AP | draw/sheathe/retrieve/reload; retrieving a dropped object = 2 Actions |
| Reload (Firearms) | Proactive | 1 AP | as Ready Weapon; times vary by weapon |
| Regain Footing | Proactive | 1 AP | automatic if unengaged; if engaged, unopposed-cost Opposed Brawn/Athletics vs. opponent (0 AP to resolve) |
| Sneak Peek (Firearms) | Proactive | 1 AP | one Perception look from cover; Fumble exposes shooter |
| Steady Weapon (Firearms) | Proactive | 1 AP | reduces Burst/Full-Auto recoil penalty one step; lasts until moved |
| Struggle | Proactive | 1 AP | escape grips/effects |
| Switch Fire Mode (Firearms) | Proactive | 1 AP (Free for high-tech) | change fire mode/ammo |
| Take Cover | Proactive | 1 AP | duck behind cover; not prone |
| Counter Spell | Reactive | 1 AP | negate incoming spell (1-Turn casting time or pre-held) |
| Evade | Reactive | 1 AP | opposed Evade; leaves prone (unless mitigated) |
| Interrupt | Reactive | 1 AP | delaying characters only; cut in on opponent's Turn / attack of opportunity |
| Parry | Reactive | 1 AP | oppose attack with Combat Style |
| Assess Situation | Free | 0 | Perception if unengaged (Running: Hard, Sprinting: Formidable) |
| Drop Item | Free | 0 | — |
| Signal | Free | 0 | if unengaged |
| Speak | Free | 0 | short phrase (≤5 seconds) |
| Use Luck Point | Free | 0 | — |
| Ward Location | Free | 0 | passively block one location (shield: multiple); must be set before attack roll; ends if weapon attacks/parries |

### 4.3 Combat Style Traits (Combat › Combat Styles)

| Trait | Description |
| --- | --- |
| Beast-back Lancer | Mounted charge with this style does not incur the one step difficulty penalty to hit. |
| Blind Fighting | Ignore penalties from poor lighting or temporary blinding. |
| Daredevil | May use Evade to dodge hand-to-hand blows without ending up prone; vs. ranged, prone only on failure. |
| Defensive Minded | Weapon Size increases one step when parrying, if no offensive action taken that round. |
| Formation Fighting | Unflanked group of 3+ warriors in close formation reduces each engaged foe's Action Points by one. |
| Knockout Blow | When attacking with surprise, treat any Stun Location as lasting minutes instead of turns. |
| Mounted Combat | Ignore the skill cap placed on combat rolls by the Ride skill. |
| Ranged Marksman | Shift a random Hit Location roll to an adjoining body location. |
| Skirmishing | May launch ranged attacks whilst walking or running. |
| Throw Weapons | Any melee weapon in the style can be thrown at no skill penalty; damage roll halved when thrown. |
| Unarmed Prowess | Unarmed blocks/parries count as 'Medium' size. |

### 4.4 Close Combat Situational Modifiers (Combat › Close Combat)

| Situation | Difficulty Grade |
| --- | --- |
| Attacking a helpless target | Automatic |
| Attacking in a confined situation | Hard |
| Defending while on lower ground or against mounted foe | Hard |
| Fighting while on unstable ground | Hard |
| Fighting while crouching or from one knee | Hard |
| Fighting in poor visibility (thick fog, snowstorm) | Hard |
| Defending against an attack from behind | Formidable |
| Fighting while prone | Formidable |
| Fighting in partial darkness (dim illumination) | Formidable |
| Fighting in pitch black conditions (no illumination at all) | Herculean |
| Blinded or loss of primary perceptive sense | Herculean |

Skill caps (not grades): mounted combat caps Combat Style at Ride; fighting in
water caps at Swim. Use the most severe modifier when several apply.

### 4.5 Damage Reduction by Parry (Combat › Damage Reduction)

| Parrying weapon/shield Size vs. attacking weapon Size | Damage deflected |
| --- | --- |
| Equal or greater Size | All damage |
| One Size less | Half damage |
| Two or more Sizes less | None |

Size categories: Small (S), Medium (M), Large (L), Huge (H), Enormous (E).

### 4.6 Special Effects Table (Combat › Special Effects)

| Special Effect | Offensive | Defensive | Special Weapon Type | Specific Roll | Stackable |
| --- | --- | --- | --- | --- | --- |
| Accidental Injury | | X | | Attacker Fumbles | |
| Arise | | X | | | |
| Bash | X | | Shields or Bludgeoning | | |
| Bleed | X | | Cutting Weapons | Firearms: Attacker Criticals | |
| Blind Opponent | | X | | Defender Criticals | |
| Bypass Armor | X | | | Attacker Criticals | X |
| Choose Location | X | | | See Description | |
| Circumvent Cover | X | | High Tech Firearms Only | | |
| Circumvent Parry | X | | | Attacker Criticals | |
| Damage Weapon | X | X | | | |
| Disarm Opponent | X | X | | | |
| Drop Foe | X | | Firearms Only | | |
| Duck Back | X | | Firearms Only | | |
| Enhance Parry | | X | | Defender Criticals | |
| Entangle | X | X | Entangling Weapons | | |
| Force Failure | X | X | | Opponent Fumbles | |
| Grip | X | | Unarmed | | |
| Impale | X | | Impaling Weapons | | |
| Marksman | X | | Ranged Weapons | | |
| Maximize Damage | X | | | Attacker Criticals | X |
| Overpenetration | X | | Firearms Only | Attacker Criticals | |
| Pin Down | X | | Firearms Only | | X |
| Prepare Counter | | X | | | X |
| Rapid Reload | X | | | | X |
| Scar Foe | X | X | | | |
| Select Target | | X | | Attacker Fumbles | |
| Slip Free | | X | | Defender Criticals | |
| Stun Location | X | | Bludgeoning Weapons | | |
| Sunder | X | | Two Handed Weapons | | |
| Trip Opponent | X | X | | | |
| Weapon Malfunction | | X | Firearms Only | Attacker Fumbles | |
| Withdraw | | X | | | |

Special Effects are chosen **before** Damage and Hit Location are rolled; number of
effects = Levels of Success won (per Differential Roll Results table); effects are
independent of whether damage is inflicted.

### 4.7 Impale Effects Table (Combat › Special Effects › Impale)

Penalty grade imposed on victim's future skill attempts while the impaling weapon is
left in the wound:

| Creature SIZ | Small Weapons | Medium Weapons | Large Weapons | Huge Weapons | Enormous Weapons |
| --- | --- | --- | --- | --- | --- |
| 1-10 | Formidable | Herculean | Incapacitated | Incapacitated | Incapacitated |
| 11-20 | Hard | Formidable | Herculean | Incapacitated | Incapacitated |
| 21-30 | No Effect | Hard | Formidable | Herculean | Incapacitated |
| 31-40 | No Effect | No Effect | Hard | Formidable | Herculean |
| 41-50 | No Effect | No Effect | No Effect | Hard | Formidable |
| Each +10 | Follow table progression |  |  |  |  |

Impale: roll weapon damage twice, attacker picks one result. Withdrawal (Ready
Weapon action; Brawn roll, opposed if resisted): extra damage to same location =
half normal damage roll, no Damage Modifier, armor does not reduce it; barbed
weapons inflict normal damage on withdrawal. Impaled weapon cannot parry.

### 4.8 Special Effect Key Numbers (Combat › Special Effect Descriptions)

| Effect | Key numbers |
| --- | --- |
| Bash | Shields: knockback 1m per 2 points of damage rolled (pre-reductions); bludgeons: 1m per 3 points. Works on creatures up to 2× attacker's SIZ. Into obstacle: Hard Athletics or Acrobatics or fall/trip. |
| Bleed | Opposed Endurance vs. attack roll; fail = lose 1 Fatigue level at start of each Combat Round until collapse/death; First Aid pass staunches (no strenuous action or it reopens). |
| Blind Opponent | Attacker: opposed Evade (or weapon skill with shield) vs. defender's parry roll; fail = Hard or Formidable for 1d3 Turns. |
| Choose Location | Melee: free choice of reachable location. Ranged/firearms: Critical only, unless target within Close Range and stationary or unaware. |
| Disarm Opponent | Opposed Combat Style vs. original roll; weapon flung Damage-Modifier-roll metres (or drops at feet if no DM). Each Size step the disarming weapon is larger: opponent's roll one grade harder (smaller: one grade easier). Works up to 2× attacker's STR. |
| Drop Foe (firearms) | Min. minor wound required; opposed Endurance vs. hit roll; fail = incapacitated for 1 hour ÷ target's Healing Rate (First Aid/boosters can recover). |
| Entangle | Immobilises location struck (arm: drops use; leg: no movement; head/chest/abdomen: all skill rolls one grade harder). Wielder: next turn 1 AP → automatic Trip attempt. Escape: opposed Brawn, or win Damage Weapon/Disarm/Slip Free. |
| Force Failure | Combined with another opposed-roll effect; opponent auto-fails resistance. |
| Grip | Prevents disengaging; escape: opposed Brawn or Unarmed vs. gripper's choice of the same. |
| Maximize Damage | Substitutes one damage die for its full value (stackable); Damage Modifier still rolled. |
| Overpenetration (firearms) | Second target behind first takes half damage; effects on first not applied to second. |
| Pin Down (firearms) | Opposed Willpower vs. hit roll; fail = no return fire next Turn (other non-exposing actions OK). Works even with no damage. |
| Rapid Reload | Next reload time reduced by one (stackable). |
| Select Target | On attacker Fumble: blow hits adjacent bystander automatically (no Special Effect on them). |
| Stun Location | Opposed Endurance vs. attack roll; fail = location Incapacitated for turns equal to damage inflicted (torso: staggered, defend only; head: insensible). |
| Sunder | Damage (after parry/magic) vs. armor AP; surplus reduces AP; leftover carries to location HP. |
| Trip Opponent | Opposed Brawn/Evade/Acrobatics vs. original roll; fail = prone. Quadrupeds: Athletics for Evade, one grade easier. |
| Weapon Malfunction | Firearm useless until field-stripped/cleaned. |

### 4.9 Charging (Combat › Close Combat › Charging)

| Parameter | Value |
| --- | --- |
| Attack roll | one Difficulty Grade harder (Hard) |
| Attacker weapon Size | +1 step |
| Attacker Damage Modifier | +1 step (biped); +2 steps (quadruped or more legs) |
| Mounted attacker | may substitute mount's Damage Modifier |
| Counter-charge | recipient may attack instead of parry/evade; longer weapon strikes first; set spear (e.g.) uses charger's Damage Modifier |

### 4.10 Knockback (Combat › Close Combat)

Trigger: damage (before parry/armor reductions) exceeds recipient's SIZ. Avoid
falling: Easy Acrobatics or Standard Athletics. Distance: 1 metre per 5 points of
damage (or fraction) over SIZ. Brace mitigates/negates.

### 4.11 Leaping Attacks (Combat › Close Combat)

Opposed: leaper's Athletics vs. defender's Brawn or Evade. Quadruped targets: attack
one grade harder. Win → defender prone, attacker astride (no damage); loser cannot
regain footing until attacker leaves or they win an Arise effect. Works up to 2×
attacker's SIZ. Brace helps if the target is aware.

### 4.12 Surprise (Combat › Close Combat)

| Effect | Value |
| --- | --- |
| Initiative | –10 penalty |
| Until their Initiative | flat-footed, cannot defend |
| First successful attack | gains one bonus Special Effect |
| Rest of the Round | no offensive Actions |

### 4.13 Ranged Combat Attributes (Combat › Ranged Combat)

| Attribute | Rule |
| --- | --- |
| Force | ranged equivalent of Size (Small…Enormous); determines shield-parry overcoming |
| Damage Modifier | Y/N — whether attacker's DM adds (self-drawn bows and thrown weapons generally Y) |
| Range | Close / Effective / Long (metres). Long: damage halved and Force reduced one step. Close: Choose Location usable vs. stationary/unaware target |
| Load | Turns to load/reload (Rapid Reload reduces) |
| Impale Size | actual Size of impaled weapon for Impale Effects Table |

### 4.14 Ranged Combat Situational Modifiers (Combat › Ranged Combat)

| Situation | Difficulty Grade |
| --- | --- |
| Light Wind* | Hard |
| Moderate Wind* | Formidable |
| Strong Wind* | Herculean |
| Gale, Storm, or Worse* | Hopeless |
| Target is Running | Hard |
| Target is Sprinting | Formidable |
| Target obscured by mist or is in partial darkness | Hard |
| Target obscured by thick smoke or is in darkness | Formidable |
| Target completely obscured | Herculean |
| Target completely invisible | Impossible** |
| Target prone | Formidable |
| Attacker prone*** | Herculean |
| Attacker is on unstable ground | Hard |

\* Assumes ammunition vulnerable to crosswinds; supersedes normal wind penalties.
\*\* Unless target's general location is known, then Herculean.
\*\*\* Negated with crossbow/firearm from a prepared position.

**Aiming**: a full Combat Round of aiming reduces a Range or Situational Modifier by
one grade (no stacking over multiple Rounds). **Telescopic sights**: one or more
grades easier (best attainable grade: Very Easy).

### 4.15 Automatic Fire (Combat › Ranged Combat › Automatic Fire)

| Mode | Penalty | Hits |
| --- | --- | --- |
| Semi-Automatic | none (default) | 1 shot per trigger pull |
| Burst | Hard to hit | if hit, roll weapon's burst dice (e.g. 1d3 for 3-round burst) for rounds striking; separate location + damage per round |
| Full-Automatic | Formidable per target | rounds distributed evenly among targets in arc (spare rounds lost); separate attack roll per target; hits per Burst rule |

Only the **first** shot in a burst/spray may benefit from Special Effects.

**Dual firearms**: pistols both hands = Hard; carbine/SMG both hands = Formidable.
**Firing into a crowd**: target at melee edge = Hard; firing through = Formidable;
a failure that would have succeeded without the penalty hits a random adjacent
victim instead (who may Evade/Parry; effects only apply to the intended target).
**Firearm Fumble**: the intended target selects a fumble-specific Special Effect for
the shooter to suffer. **Shields vs. gunfire**: Passive Blocking only; GM may assign
AP instead of Size (e.g. wooden shield 4 AP, Kevlar tactical shield 8 AP, force
field 15 AP).

### 4.16 Movement (Combat › Movement)

| Action/rate | Value |
| --- | --- |
| Move Action | Base Movement Rate (humans: 6 metres) |
| Climb | Base Movement Rate; must not have moved faster than Walk; armor hinders by AP of heaviest item worn |
| Crawl | max 2 metres per Turn (prone) |
| Jump | successful Athletics: up to 2× own height horizontal, ½ own height vertical (with 5-metre run-up); halved from standing |
| Swim | Base Movement Rate minus movement already used |

Engagement: movement only by Move or Charge actions; unengaged characters only;
entering an opponent's engagement zone engages both; engaged characters must
Withdraw (e.g. via Special Effect or Outmanoeuvre) to leave; passing an unengaged
Delaying foe provokes a strike/block.

[AMBIGUOUS: Walk/Run/Sprint movement multipliers are referenced ("rules for Walk,
Run, and Sprint set forth above") but never defined in this document.]

---

## 5. Arms and Armor (Combat › Arms and Armor)

### 5.1 Sample Armor Table

| Ancient/Medieval | Modern | Futuristic | Armor Points |
| --- | --- | --- | --- |
| Furs/Hides | Bike Leathers | | 1 |
| Aketon, Gambeson | Sports Gear | | 2 |
| Linothorax | Ballistic Cloth | | 3 |
| Brigandine | Flak Jacket | Liquid Body Armor | 4 |
| Hoplite Plate | Riot Gear | | 5 |
| Mail | Type I Bulletproof Vest | Adaptive Mesh | 6 |
| Plated Mail | Type II Bulletproof Vest | | 7 |
| | Type III Bulletproof Vest | Light Assault Armor | 8 |
| | Type IV Bulletproof Vest | | 10 |
| | | Full Assault Armor | 12 |

### 5.2 Sample Shield Table

| Shield Type | Damage | Size | AP/HP | Number of Locations for Passive Blocking |
| --- | --- | --- | --- | --- |
| Buckler | 1d3 | M | 6/9 | 2 |
| Heater | 1d4 | L | 6/12 | 3 |
| Kite | 1d4 | H | 4/15 | 4 |
| Scutum | 1d4 | H | 4/18 | 5 |
| Viking/Riot Shield | 1d4 | L | 4/12 | 4 |

### 5.3 Sample Melee Weapons

| Weapon | Damage | Size | AP/HP | Notes |
| --- | --- | --- | --- | --- |
| Axe | 1d6+1 | M | 4/8 | Can Sunder Armor |
| Broadsword | 1d8 | M | 6/10 | — |
| Chain | 1d4 | M | 8/6 | Can entangle |
| Club | 1d6 | M | 4/4 | — |
| Dagger | 1d4+1 | S | 6/8 | Can be thrown |
| Fist/Kick | 1d3 | S | — | Human Unarmed Combat Damage |
| Longsword | 1d8 | M | 6/12 | Two handed |
| Mace | 1d8 | M | 6/6 | — |
| Net | 1d4 | S | 2/20 | Entrapping, Thrown |
| Shortsword | 1d6 | M | 6/8 | — |
| 1H Spear | 1d8+1 | M | 4/5 | (no note) |
| 2H Spear | 1d10+1 | L | 4/10 | Can be set to receive a charge |
| War hammer | 1d8+1 | M | 3/8 | Two handed |
| Whip | 1d3 | M | 2/8 | Entangling |

### 5.4 Sample Ranged Weapons

| Weapon | Damage | Damage Modifier | Force | Range (Metres) | Load | Impale Size | AP/HP |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Bolas | 1d4 | N | — | 10/25/50 | — | — | 2/2 |
| Bow | 1d8 | Y | L | 15/100/200 | 1 | S | 4/4 |
| Dagger | 1d4 | Y | S | 5/10/20 | — | S | 4/8 |
| Javelin | 1d8+1 | Y | H | 10/20/50 | — | M | 3/8 |
| Sling | 1d8 | N | L | 10/150/300 | 2 | — | 1/2 |
| Stone/Rock | 1d3 | Y | S | 5/10/20 | — | — | — |
| Pistol | 1d6 | N | L | 50/100/200 | 2 | — | — |
| Rifle | 2d6 | N | H | 100/300/2000 | 2 | — | — |
| Shotgun | 3d6 | N | M | 20/50/200 | 3 | — | — |
| Laser Rifle | 1d10+2 | N | — | 40/120/480 | 3 | — | — |
| Plasma Rifle | 2d6+4 | N | E | 30/100/300 | 3 | — | — |
| Gauss Rifle | 2d8+2 | N | E | 150/500/5000 | 3 | — | — |

---

## 6. Vehicles (Combat › Vehicles)

### 6.1 Vehicle Size / System Hits / Trait Allocation

| Size | System Hits | Max Traits |
| --- | --- | --- |
| Small | 1 | 1 |
| Medium | 2 | 2 |
| Large | 3 | 3 |
| Huge | 4 | 4 |
| Enormous | 5 | 5 |
| Colossal | 6 | 6 |

### 6.2 Terrestrial Hulls

| **Size** | **Structure** | **Civilian Hull Value** | **Military Hull Value** | **Example** |
| --- | --- | --- | --- | --- |
| Small | 1-20 | 1-3 | 4-6 | Motorcycle, motorcycle with sidecar, compact or subcompact car, canoe, kayak, rowboat |
| Medium | 21-40 | 1-6 | 7-9 | Sedan, panel van, pickup, luxury car, luxury sports car, muscle car, limousine, lifeboat, speedboat, hang glider, autogyro |
| Large | 41-60 | 4-9 | 10-12 | Lorry/truck, stretch limo, SUV, bus, people carrier, personal aircraft, glider, pleasure cruiser, tank, helicopter, submarine |
| Huge | 61-80 | 7-12 | 13-15 | Semi, airliner, jet, coastguard vessel, yacht |
| Enormous | 81-100 | 10-15 | 16-18 | Train, cruise liner, naval vessel |

### 6.3 Starship Hulls

| Size | Structure | Civilian Hull Value | Military Hull Value | Example |
| --- | --- | --- | --- | --- |
| Small | 10-50 | 4-9 | 10-12 | Single seat fighter |
| Medium | 51-150 | 7-12 | 13-15 | Light Freighter, Shuttle craft |
| Large | 151-450 | 10-15 | 16-18 | Heavy Freighter, Cruiser |
| Huge | 451-1350 | 13-18 | 19-21 | Escort Frigate, Light Battle Cruiser |
| Enormous | 1351-4050 | 16-21 | 21-24 | Generation Ship, Heavy Battle Cruiser |
| Colossal | 4050-12,110 | 19-24 | 24-30 | Orbital Habitat, Planet-Busting Battle Station |

[AMBIGUOUS: Enormous starship Military Hull Value "21-24" overlaps the Huge row's
"19-21" and the Colossal row's "24-30" at their boundaries; transcribed as printed.]

### 6.4 Speed Rating Table

| Speed Rating | Max Speed For |
| --- | --- |
| Ponderous | Enormous |
| Sluggish |  |
| Slow | Huge |
| Mediocre |  |
| Gentle | Large |
| Moderate |  |
| Rapid | Medium |
| Fast |  |
| Fleet | Small |

[AMBIGUOUS: the "Max Speed For" column places a Size label only on every other row
(Ponderous→Enormous, Slow→Huge, Gentle→Large, Rapid→Medium, Fleet→Small); the
blank rows are left blank as printed — the interleaved speeds presumably apply
between those sizes.]
Emergency speed: +1 step on successful Drive, sustainable 1d6 minutes (Critical:
1d12 minutes); +2 steps: Herculean Drive, max 1d3 minutes. Default Handling:
Standard for Large and below; Huge = Formidable; Enormous = Herculean. Walker
vehicles capped at Speed Gentle. Rails: +3 speed steps (track-bound). Cargo laden:
−2 speed steps. Enhanced Performance trait: +1 step above size maximum.

### 6.5 FTL Speed Rating Table

| Speed Rating | Factor to Traverse 1 Parsec |
| --- | --- |
| Ponderous | 10 |
| Sluggish | 8 |
| Slow | 7 |
| Mediocre | 6 |
| Gentle | 5 |
| Moderate | 4 |
| Rapid | 3 |
| Fast | 2 |
| Fleet | 1 |

Factor time is GM-set (e.g. 1 Day base). 1 parsec = 3.26 light-years. FTL cannot
activate within 1.5 AU of a star.

### 6.6 Loss of Control Table (terrestrial vehicles)

| 1d100 | Result |
| --- | --- |
| 01-25 | Swerve. Temporary; speed −1 step for 5 seconds. |
| 26-40 | Skid. Speed −2 steps for 10 seconds. |
| 41-50 | Severe Skid. Faces wrong direction, standstill for 15 seconds. |
| 51-60 | Roll. 3d10 damage to Structure. Occupants: Endurance or 1d10 damage to 1d3 Hit Locations. |
| 61-70 | Severe Roll. 3d10+10 damage; occupants take 1d10 even on success, 2d10 on failure. |
| 71-80 | Write-Off. Vehicle to 0 Structure; occupant damage as Severe Roll. |
| 81-90 | Explosion. As Write-Off; fuel ignites in 1d20+10 seconds; if not clear, +1d6 burn damage to 1d6 locations. |
| 91-98 | Immediate Explosion. As above but immediate. |
| 99-00 | Catastrophic Crash. Occupants: Endurance or killed instantly; Write-Off damage regardless. |

### 6.7 System Damage Table

| Size | System Hits | Loss per Hit |
| --- | --- | --- |
| Small | 1 | 1 Grade or 50% |
| Medium | 2 | 1 Grade or 33% |
| Large | 3 | 1 Grade or 25% |
| Huge | 4 | 1 Grade or 20% |
| Enormous | 5 | 1 Grade or 16% |
| Colossal | 6 | 1 Grade or 10% |

Every hull-penetrating hit has a percentage chance (equal to penetrating damage) of
affecting a System. Weapon systems supported: 1/10th of Structure Points (rounded
down is implied but not stated).

### 6.8 System Component Damage Table

| 1d10 | System | Damaged Result | Destroyed Result |
| --- | --- | --- | --- |
| 1 | Cargo | Possessions/equipment proportional to damage destroyed. | All cargo destroyed. |
| 2 | Comms | Comms rolls +1 Difficulty Grade per hit. | No communication or sensor spoofing. |
| 3 | Controls | Boating/Drive/Pilot +1 Grade per hit; immediate Control roll. | Cannot be steered/change course. |
| 4 | Drive | Speed reduced proportionally. | Stopped dead; aircraft crash. |
| 5 | Crew | Passengers proportional to damage suffer Major Wound + Endurance or die. | Occupants die. |
| 6 | Engine / Fuel | Max Speed halved; electronics +1 Grade. | Destroyed in disastrous explosion. |
| 7 | Sensors | Sensor/Navigation/Weapon attack rolls +1 Grade per hit. | Vehicle blind. |
| 8 | Weapons | Weapon systems proportional to damage inoperative. | Cannot fire weapons. |
| 9-0 | None | Structure damage only. | N/A |

### 6.9 Weapons by Size Table

| Size | Terrestrial | Spacecraft |
| --- | --- | --- |
| Small | Light Weapon (e.g. Assault Rifle) | Medium Weapon (e.g. Machine Gun, Rocket Launcher) |
| Medium | Medium Weapon (e.g. Machine Gun, Rocket Launcher) | Turreted/Heavy Weapon (e.g. Heavy Linked Machine Guns, Mini-Guns) |
| Large | Turreted/Heavy Weapon (e.g. Heavy Linked Machine Guns, Mini-Guns) | Weapon Battery (e.g. Battlefield Artillery) |
| Huge | Weapon Battery (e.g. Battlefield Artillery) | Advanced Weapons (e.g. Missile Systems) |
| Enormous | Advanced Weapons (e.g. Missile Systems) | Spinal Mounted Weapons (one only) |
| Colossal | NA | Spinal Mounted Weapons (up to 3) |

### 6.10 Sample Vehicle Weapon Statistics

| Weapon | Damage | Range | Firing Rate | Ammo/Load |
| --- | --- | --- | --- | --- |
| 50-inch Cannon | 3d6+3 | 1000/2000/6500 | 1/10/50 | 1/5 |
| Mini Gun/Gatling Gun | 3d6 | 300/1000/2000 | -/20/100 | 200/4 |
| Rocket | 5d6 | 5000/20km/60km | 1 | 1/5 |
| Medium Missile/Torpedo | 6d10 | Max range of 200km | 1 | 1/5 |
| Large/Cruise Missile | 6d10+10 | Max range of 1000km | 1 | 1/10 |
| Laser/Plasma Cannon | 4d6 | 300/1000/2000 | -/5/25 | 500/5 |
| Heavy Laser/Plasma Cannon | 5d6 | 1000/3000/6000 | -/5/25 | 300/5 |
| Artillery Battery | 5d6+6 | Max range of 50km | 1 | 1/5 |
| Spinal Mount Battery | 10d6+6 | Max range of 20,000km | 1 | 1/10 |

Ranges in metres/kilometres. Firing rates = single/semi-auto/full-auto. Ammo =
shots before reload; Load = reload time in Combat Rounds with full crew.

### 6.11 Vehicle Deflector Shields

Ablative: damage ≤ shield value = fully blocked; excess over shield value reduces
shield strength by that amount; at 0 the shield collapses and remaining damage
carries to hull.

### 6.12 Sample Vehicles

| Vehicle | Hull (size class) | Structure | Speed | Systems | Traits | Weapons |
| --- | --- | --- | --- | --- | --- | --- |
| Interceptor | 6 (Medium Military) | 40 | Fast | 3 | Ejector Seat, Enhanced Performance, Superior Handling, Surveillance Suite, Tough, Weaponised | Oil slick dispenser, smoke dispenser, twin linked medium machine guns |
| Pursuit Motorcycle | 2 (Small Civilian) | 12 | Fast | 3 | Superior Handling, Weaponised | Two Rockets or twin linked Light Machine Guns |
| Sedan Car | 4 (Medium Civilian) | 25 | Rapid | 3 | Unobtrusive, Surveillance Suite | None |
| Submersible Car | 6 (Medium Civilian) | 25 | Fast | 3 | Enhanced Performance, Submersible, Watercraft, Weaponised | Rocket, Torpedo |
| Surveillance Van | 9 (Large Civilian) | 60 | Gentle | 4 | Surveillance Suite, Tough | None |
| Tactical Support Vehicle | 6 (Medium Civilian) | 30 | Rapid | 3 | All Terrain, Weaponised | Mounted Medium Machine Gun |
| Speedboat | 3 (Medium Civilian) | 24 | Fast | 3 | Enhanced Performance, Waterborne, Weaponised | Mine Dispenser |
| Yacht | 10 (Huge Civilian) | 75 | Slow | 4 | Luxurious, (Surveillance Suite), Waterborne | None |
| Dirigible (Civilian Airship Liner) | 10 (Enormous Civilian) | 85 | Ponderous | 5 | Airborne, Luxurious, VTOL | None (Shields: None) |
| Assault Helicopter | 11 (Large Military) | 50 | Mediocre | 3 | Airborne, Enhanced Performance, Stealth, Superior Handling, Surveillance Suite, VTOL | 1 Gatling gun, 4 rockets (Shields: None) |
| Multi-Role Fighter | 10 (Large Military) | 45 | Mediocre | 3 | Airborne, Enhanced Performance | 4 nose-mounted Gatling guns (fore), 1 bomb bay (2× 250 lb bombs = medium missiles range 0, or 1 torpedo) (Shields: None) |
| Interstellar Freighter | 10 (Medium, Civilian Spaceship) | 80 | Rapid (Parsec Factor 3) | 2 | Airborne, Cargo, FTL, Space Craft, VTOL | x2 quad-mounted laser cannon turrets (Shields: 8) |
| General Contact Unit | 20 (Colossal, Civilian Spaceship) | 10,000 | Moderate (Parsec Factor 4; factor = 1 hour at this tech level) | 30 | Airborne, Cargo, Carrier, Construction, FTL, Space Craft, Luxurious, Superior Handling, Tractor Beam | None (Shields: 30) |

[AMBIGUOUS: some vehicle traits used in stat blocks ("Unobtrusive", "Watercraft",
"Waterborne") are not in the Vehicle Traits list; "Waterborne" appears only inside
the Yacht entry, where the source text itself is garbled ("Luxurious, (Surveillance
Suite),Waterborne), Waterborne"). Systems counts in sample vehicles do not always
match the Vehicle Size/System Hits table (e.g. Interceptor is Medium with Systems 3
but Medium = 2 System Hits); Systems in stat blocks may denote system count rather
than hits.]

---

## 7. Magic and Powers (Magic and Powers)

### 7.1 Magic Roll Costs (Magic and Powers › Magic)

| Magic roll result | Magic Point cost | Effect |
| --- | --- | --- |
| Critical Success | 0 | spell works |
| Success | 1 | spell works |
| Failure | 1 | spell does not work |
| Fumble | 1d3 | spell fails |

Casting: 1 Action + successful Magic skill roll. All spells have default Intensity
and Magnitude of 1. Duration: entire scene/action unless Concentration or Instant
(or custom). Magic Points = POW; recovery = Healing Rate per hour (full rest).
Superhero Power Points: 1 per full minute of rest; or spend 1 Luck Point → regain
1d4+1 immediately.

Magicians begin with 1d4+1 spells; new spell = 3 Experience Rolls + one week of
study with a teacher/source. No limit on spells known.

### 7.2 Spell List (Magic and Powers › Spell Descriptions)

| Spell | Traits | Effect summary (key numbers) |
| --- | --- | --- |
| Alarm | Special Duration | alerts caster when living creature SIZ >1 enters area / object is touched |
| Avert | Instant, Ranged | dismisses another spell in range; usable as Counter Magic reaction |
| Befuddle | Ranged, Resist (Willpower) | target cannot initiate constructive activity; attack/threat breaks spell |
| Bladesharp | Touch | +1 damage dice step for edged/piercing melee weapons (1d4→1d6→1d8→1d10→2d6→2d8→2d10) |
| Bludgeon | Touch | as Bladesharp for blunt weapons/tools |
| Breath | Touch | hold breath up to half caster's POW in minutes; speaking ends it |
| Calm | Ranged, Resist (Willpower) | dampens passions; target may still defend/attack calmly |
| Chill | Instant, Touch | cools objects no larger than caster's hand to ice-water temperature; no damage |
| Darkness | Concentration, Ranged | area of POW square metres; all non-magical light reduced to dim glow |
| Disruption | Instant, Ranged, Resist (Endurance) | 1d3 damage to one random location or object HP; ignores armor |
| Extinguish | Instant, Ranged | quenches small/modest fires; not magical or large fires |
| Find (X) | Concentration, Ranged, Resist (Special) | variants: Arrows, Flaw, Livestock (Willpower), Loot, Object, Sickness; blocked by metal or 1m+ earth/stone |
| Firearrow | Touch | missiles +1d3 damage, extinguished if they impale flesh; ignition chance = caster's "Folk Magic" skill % [AMBIGUOUS: "Folk Magic" is not a skill in this document; presumably Magic] |
| Fireblade | Touch | melee weapons/tools +1d3 damage; ignition if held to materials several rounds |
| Glue | Touch | bonds two inanimate objects; Brawn equal to 5× caster's POW to wrench apart (opposed) |
| Heal | Instant, Touch | cures minor complaints; Minor Wound: restores all lost HP instantly; Serious/Major: no HP but stabilises, stops bleeding |
| Ignite | Instant, Ranged | ignites flammable inorganic matter (small object/hand-sized area) |
| Knock | Instant, Touch | unfastens one mundane bar/lock (not magical); one fastening per casting |
| Light | Concentration, Ranged | lantern-light on an object; counters Darkness (both consumed) |
| Lock | Special Duration, Touch | secures one existing lock; only caster can open; 1 MP held until dismissed |
| Phantasm | Concentration, Ranged | ghostly shapes from insubstantial material; no other effect |
| Sleep | Resist (Endurance), Touch | sleep for half caster's POW in hours; takes 1d3 Rounds; no effect on SIZ > caster's POW; auto-fails in combat |
| Vigour | Touch | ignores Fatigue from labour for the duration |
| Witchsight | Ranged, Resist (Willpower) | see magic/invisible (shadowy); pierce illusions; hidden beings: opposed Willpower vs. cast roll |

### 7.3 Super Powers Table (Magic and Powers › Beginning Powers)

| POW + Characteristic¹ | Core Powers |
| --- | --- |
| 12 or less | 1 |
| 13-20 | 2 |
| 21-30 | 3 |
| 31-40 | 4 |
| 40+ | 5 |

¹ Second Characteristic = CON for altered physiology (mutation, alien species,
experimentation) or INT for technology/skill-based powers (cybernetics, battle
suits, intense training). [AMBIGUOUS: "31-40" and "40+" overlap at exactly 40.]

A core power can be exchanged for two Boosts (applied to any core power). Up to two
Limits: one → bonus Boost; two → bonus core power.

### 7.4 Powers and Boosts (Magic and Powers › Powers)

| Power | Core effect (key numbers) | Boost (Power Point cost) |
| --- | --- | --- |
| Animation | animate 2 m³ non-living material within POW m; PP cost = armor value of highest-value material (= construct's base AP); construct STR 2d6+16 (23), SIZ 1d6+12 (15), DEX 1d6+8 (11); caster directs with own skills; Serious Wound chest/abdomen: opposed Endurance or collapse | 2 PP: move construct to same material within POW m |
| Combat Expert | pick 1 expertise (+1 per boost): Melee = +1 defensive AP, weapon size +1 step for attacks/parries; Ranged = aiming reduces situational modifiers 2 grades, reload −1 | 2 PP: +1 AP usable only for Attack or Ready Weapon |
| Durability | HP from CON+SIZ+POW; Healing Rate from CON+½POW (round up) | 2 PP (free action): regain Healing Rate in HP or 1 Fatigue level |
| Elemental Control | control ½POW m³ of chosen element within POW m; Craft (element) for complex shapes; melee reach Very Long or ranged 8/24/100; pick 2 Combat Effects; damage/force by POW: ≤8: 1d4/T, 9–12: 1d6/S, 13–15: 1d8/M, 16–18: 1d10/M, 19+: 1d12/L | 3 PP: sheathe in element for scene (solid: AP = ⅕POW round up; energy: unarmed +base damage, unarmed attackers take same damage to limb) |
| Energy Blast | ranged 10/30/120; pick 2 Combat Effects; damage/force by POW: ≤8: 1d6/S, 9–12: 1d8/M, 13–15: 1d10/M, 16–18: 1d12/L, 19+: 2d6/L; hit with Combat Style or Unarmed | 2 PP + action: +1d6 damage on next blast |
| Enhanced Reactions | initiative bonus = Athletics critical range; never prone after Evade | 1 PP: next Athletics/Acrobatics one grade easier |
| Enhanced Speed | movement multiplier by DEX+POW: ≤12 ×2, 13–24 ×3, 25–36 ×4, 37+ ×5; max out-of-combat speed = 10× modified movement in km/h | 2 PP: +1 AP only for Change Range, Move, Outmanoeuvre |
| Enhanced Strength | damage bonus from STR+SIZ+POW; max lift by STR+POW: ≤12: 1 tonne, 13–24: 2 tonnes, 25–36: 3 tonnes, 37+: 5 tonnes; unarmed = Large; melee/thrown size or force +1 | 3 PP: double max lift for one action/task |
| Flight | fly POW m; max out-of-combat = 10× movement in km/h | 3 PP: charge attack during aerial Move |
| Heightened Sense | chosen sense: Perception one grade easier | 2 PP: use core power on a different sense |
| Inherent armor | AP to all locations = ⅓CON (round up); may shift up to 2 points between locations | 1 PP per +1 AP vs. the attack when struck |
| Life Support | immunity to two environmental conditions/biological requirements | (text: "Gain immunity to two additional environments or requirements" — cost not stated) [AMBIGUOUS: no PP cost given for this Boost] |
| Lucky | Luck Points from CHA + ½POW (round up) | 3 PP (on rolling doubles): temporary Luck Point, must be used by end of scene |
| Regeneration | regain 1 HP to all wounded locations each round (not Major-Wounded locations) | 1 PP/hour trance: regain 1 HP to a Major-Wounded location |
| Savant | 3 skills sharing one characteristic: one grade easier | 3 PP: gain a missing Craft/Science speciality at Savant level for one task |
| Teleport | teleport to visible point within POW m as move action | 2 PP: teleport up to ½POW m (round up) as free action |
| Vaporous Form | become vapour: immune to physical attacks, fly ⅓POW m, pass any opening; energy attacks deal half damage to PP; at 0 PP: solid + Hard Endurance or unconscious 1d6+1 minutes | 3 PP: solidify one hit location for one action |

### 7.5 Limits (Magic and Powers › Limits)

| Limit | Effect |
| --- | --- |
| Activation Cost | 2 PP to activate a core power, or +2 PP to a boost; continuous power: 1 PP/turn to maintain |
| External Power Source | powers from an artefact/device; small: vulnerable to Disarm/Take Weapon; large: ≥1 minute to don/activate |
| Fatal Flaw | must consume/contact a substance every 4 hours or suffer 1 Fatigue (Endurance to stave off; +1 grade per additional 4 hours) |
| Limited Control | critical failure range increases to 90-00 for actions involving the power/boost |
| Limited Power | works only under specific conditions / reduced effect (GM-defined) |
| Physical Limitation | persistent condition (blindness, permanent HP loss, reduced mobility) |
| Vulnerability | double damage from a specific attack form; or Hard Endurance each turn within 5 m of a substance or suffer 1 Fatigue |

### 7.6 Animation Construct Armor Values / PP Cost

| Armor Values | Power Point Cost |
| --- | --- |
| Plastic or Soft Wood | 2 |
| Concrete or Hard Wood | 4 |
| Soft Metal or Stone | 6 |
| Hard Metal | 8 |

---

## 8. Creatures (Creatures)

All sample creature stat lines: `SIZ, Action Points, Damage Modifier, Magic Points,
Initiative Bonus, Move`. (Full STR/CON/DEX/INT/POW/CHA are not given.)

### 8.1 Sample Creature Summary

| Creature | SIZ | AP | Damage Modifier | MP | Initiative Bonus | Move | Abilities | Combat Style |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Giant Ant | 14 | 2 | +1d2 | 4 | +11 | 12m | Formidable Natural Weapons | Ant Attack (Mandibles and Sting) 67% |
| Bear (Grizzly) | 34 | 3 | +1d12 | 7 | +13 | 8m | Intimidate, Night Sight | Ursine Fury (Bite and Claws) 78% |
| Crocodile/Alligator | 37 | 2 | +2d6 | 7 | +13 | 6m | Camouflaged, Cold Blooded, Swimmer | Lurking Death (Bite and Tail) 67% |
| Dragon | 50 | 4 | +2d8 | 19 | +19 | 12m / 18m Flying | Breathe Flame, Cold Blooded, Dark Sight, Diving Strike, Engulfing, Flying, Formidable Natural Weapons, Immunity (Fire), Terrifying, Trample | Death and Destruction (Bite, Claws, Tail Sweep) 90%; Flame Breath 80% |
| Goblin/Orc | 11 | 2 | 0 | 11 | +11 | 6m | Night Sight | Warrior (Shortsword, Shield, Spear, Sling) 62% |
| Horse | 31 | 2 | +1d12 | 7 | +11 | 12m | Trample | Rear and Plunge (Hoofs) 62% |
| Minotaur | 22 | 2 | +1d6 | 11 | +12 | 8m | Intimidate | Minotaur Warrior (2H Axe, 2H spear one-handed, Hoplite Shield, Gore) 67% |
| Shark | 42 | 3 | +2d6 | 7 | +13 | 12m (swim) | Blood Sense, Frenzy, Swimmer | Jaws (Bite) 81% |
| Skeleton | 13 | 2 | +0 | 16 | +14 | 6m | Undead | Skeletal Warrior (Sword, Spear, Peltast Shield) 68%, Formation Fighting |
| Xenomorphic Alien | 16 | 3 | +1d6 | 11 | +18 | 8m | Camouflaged, Frenzy | Parasitic Alien Horror (Bite, Claw and Tail) 77% |

### 8.2 Creature Skills

| Creature | Skills |
| --- | --- |
| Giant Ant | Athletics 67%, Brawn 68%, Endurance 74%, Evade 56%, Perception 53%, Track 66%, Willpower 48% |
| Bear (Grizzly) | Athletics 68%, Brawn 79%, Endurance 66%, Evade 46%, Perception 60%, Stealth 66%, Survival 60%, Swim 68%, Track 66%, Willpower 44% |
| Crocodile/Alligator | Athletics 47%, Brawn 99%, Endurance 78%, Evade 32%, Perception 49%, Swim 80%, Willpower 44% |
| Dragon | Athletics 80%, Brawn 120%, Deceit 78%, Endurance 92%, Evade 68%, Fly 90%, Insight 78%, Locale 68%, Lore (Dragon) 68%, Perception 78%, Teach 58%, Willpower 78%; Magic 78% (Befuddle, Cleanse, Coordination, Find Treasure, Mindspeech, Spiritshield, Vigour, Witchsight) |
| Goblin/Orc | Athletics 52%, Brawn 42%, Deceit 57%, Endurance 48%, Evade 62%, Perception 61%, Unarmed 52%, Willpower 42% |
| Horse | Athletics 52%, Brawn 83%, Endurance 66%, Evade 40%, Perception 48%, Willpower 34% |
| Minotaur | Athletics 57%, Brawn 79%, Customs 66%, Endurance 66%, Evade 40%, Insight 44%, Locale 56%, Lore (Herding) 66%, Perception 54%, Unarmed 57%, Willpower 42% |
| Shark | Athletics 87%, Brawn 87%, Endurance 66%, Evade 55%, Perception 57%, Swim 91%, Willpower 34% |
| Skeleton | Athletics 68%, Brawn 45%, Endurance 44%, Evade 72%, Perception 57%, Unarmed 68%, Willpower 72% |
| Xenomorphic Alien | Athletics 77%, Brawn 68%, Endurance 56%, Evade 80%, Perception 62%, Stealth 76%, Track 64%, Willpower 52% |

[AMBIGUOUS: the Dragon's spell list includes Cleanse, Coordination, Find Treasure,
Mindspeech, Spiritshield — none are defined in the Spell Descriptions section.]

### 8.3 Creature Natural Weapons

| Creature | Weapon | Size | Damage |
| --- | --- | --- | --- |
| Giant Ant | Bite | M | 1d6+1d2 |
| Giant Ant | Sting | M | 1d4+1d2 |
| Bear | Bite | L | 1d8+1d12 |
| Bear | Claw | H | 1d8+1d12 |
| Crocodile | Bite | H | 1d10+2d6 |
| Crocodile | Tail (Sweep attack) | H | 1d8+2d6 |
| Dragon | Bite | E | 1d12+2d8 |
| Dragon | Tail (Sweep attack) | E | 1d10+2d8 |
| Dragon | Claw | E | 1d10+2d8 |
| Dragon | Flame Breath | — | 4d6 (cone: CON metres long, ¼CON metres wide; ignites flammables, melts un-enchanted metal) |
| Dragon | Trample | C | 4d8 |
| Goblin/Orc | Shortsword | M | 1d6 |
| Goblin/Orc | Spear | M | 1d8+1 |
| Goblin/Orc | Shield (3 locations passive) | L | 1d4 |
| Horse | Hoof | H | 1d6+1d12 |
| Minotaur | Axe (2H) | E | 2d8+2+1d6 |
| Minotaur | Spear (2H used 1H) | H | 1d8+1+1d6 |
| Minotaur | Shield | E | 1d8+1d6 |
| Minotaur | Gore | L | 1d8+1d6 |
| Shark | Bite | E | 1d10+2d6 |
| Skeleton | Sword | M | 1d6 |
| Skeleton | Spear | H | 1d8+1 |
| Skeleton | Shield | L | 1d4 |
| Xenomorphic Alien | Bite | M | 1d4+1d6 |
| Xenomorphic Alien | Claw | M | 1d4+1+1d6 |
| Xenomorphic Alien | Tail (Sweep attack) | L | 1d6+1d6 |

[AMBIGUOUS: Dragon Trample "Size: C" — C is not a Size category (S/M/L/H/E);
presumably Colossal or Enormous. The Minotaur shield "Damage 1d8+1d6" appears to
have the Damage Modifier folded into the damage line, as do creature weapons
generally in these stat blocks.]

### 8.4 Creature Hit Locations (AP/HP per 1d20 roll)

**Giant Ant**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1 | Right Rear Leg | 4/6 |
| 2 | Left Rear Leg | 4/6 |
| 3 | Right Middle Leg | 4/6 |
| 4 | Left Middle Leg | 4/6 |
| 5-9 | Abdomen | 4/9 |
| 10-13 | Thorax | 4/6 |
| 14 | Right Front | 4/6 |
| 15 | Left Front Leg | 4/6 |
| 16-20 | Head | 4/7 |

**Bear (Grizzly)**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Right Rear Leg | 3/10 |
| 4-6 | Left Rear Leg | 3/10 |
| 7-9 | Hindquarters | 3/11 |
| 10-12 | Forequarters | 3/12 |
| 13-15 | Right Front | 3/10 |
| 16-18 | Left Front Leg | 3/10 |
| 19-20 | Head | 3/10 |

**Crocodile/Alligator**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Tail | 5/12 |
| 4-5 | Right Rear Leg | 5/12 |
| 6-7 | Left Rear Leg | 5/12 |
| 8-10 | Hindquarters | 5/13 |
| 11-14 | Forequarters | 5/14 |
| 15-16 | Right Front | 5/12 |
| 17-18 | Left Front Leg | 5/12 |
| 19-20 | Head | 5/13 |

**Dragon**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-2 | Tail | 8/17 |
| 3-4 | Right Rear Leg | 8/17 |
| 5-6 | Left Rear Leg | 8/17 |
| 7-8 | Hindquarters | 8/18 |
| 9-10 | Right Wing | 8/16 |
| 11-12 | Left Wing | 8/16 |
| 13-14 | Forequarters | 8/19 |
| 15-16 | Right Front Leg | 8/17 |
| 17-18 | Left Front Leg | 8/17 |
| 19-20 | Head | 8/17 |

**Goblin/Orc**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Right Leg | 1/5 |
| 4-6 | Left Leg | 1/5 |
| 7-9 | Abdomen | 1/6 |
| 10-12 | Chest | 1/7 |
| 13-15 | Right Arm | 1/4 |
| 16-18 | Left Arm | 1/4 |
| 19-20 | Head | 1/5 |

**Horse**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Right Hind Leg | 1/9 |
| 4-6 | Left Hind Leg | 1/9 |
| 7-9 | Hindquarters | 1/10 |
| 10-12 | Forequarters | 1/11 |
| 13-14 | Right Front Leg | 1/8 |
| 15-16 | Left Front Leg | 1/8 |
| 17-20 | Head | 1/9 |

**Minotaur**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Right Leg | 0/7 |
| 4-6 | Left Leg | 0/7 |
| 7-9 | Abdomen | 0/8 |
| 10-12 | Chest | 0/9 |
| 13-14 | Right Arm | 0/6 |
| 15-16 | Left Arm | 0/6 |
| 17-20 | Head | 3/7 |

**Shark**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Tail | 2/12 |
| 4-6 | Dorsal Fin | 2/11 |
| 7-10 | Hindquarters | 2/13 |
| 11-14 | Forequarters | 2/14 |
| 15-16 | Right Fin | 2/11 |
| 17-18 | Left Fin | 2/11 |
| 17-20 | Head | 2/12 |

[AMBIGUOUS: Shark ranges overlap — "17-18 Left Fin" and "17-20 Head" both cover
17-18; transcribed exactly as printed (likely Head should be 19-20).]

**Skeleton**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Right Leg | 0/5 |
| 4-6 | Left Leg | 0/5 |
| 7-9 | Abdomen | 0/6 |
| 10-12 | Chest | 0/7 |
| 13-15 | Right Arm | 0/4 |
| 16-18 | Left Arm | 0/4 |
| 19-20 | Head | 0/5 |

**Xenomorphic Alien**

| 1d20 | Location | AP/HP |
| --- | --- | --- |
| 1-3 | Tail | 6/5 |
| 4-5 | Right Rear Leg | 6/6 |
| 6-7 | Left Rear Leg | 6/6 |
| 8-10 | Abdomen | 6/7 |
| 11-14 | Chest | 6/8 |
| 15-16 | Right Arm | 6/5 |
| 17-18 | Left Arm | 6/5 |
| 19-20 | Head | 6/7 |

Xenomorph special rule: when injured, acidic blood sprays an attacker in
hand-to-hand range — 1d3 damage for 1d3 rounds to a random Hit Location; acid eats
through armor first.

Skeleton special rule: bludgeoning weapons +1 damage dice step vs. skeletons;
slashing weapons −1 step; thrusting weapons −2 steps.

### 8.5 Creature Abilities — Key Numbers (Creatures › Creature Abilities)

| Ability | Key numbers |
| --- | --- |
| Adhering | move on vertical/overhead surfaces at half Movement rate |
| Aquatic | suffocate after CON minutes out of water (passive), half if moving/fighting |
| Blood Sense | sense blood at up to 1d6+6 kilometres |
| Breathe Flame | cone CON metres long, ¼CON wide at far end; all hit locations; Evade halves; once per hour/day, extra uses: Endurance +1 grade each or 1 Fatigue |
| Burrower | tunnel at normal Movement rate |
| Camouflaged | spotting via Perception two grades harder |
| Characteristic Drain | temporary Characteristic drain (per creature description) |
| Cold-Blooded | 1 meal/week; below 15 °C: −6 Initiative and −1 Combat Action/round; below 5 °C: torpid/helpless |
| Dark Sight | see normally with no light |
| Death Sense | sense death/dead flesh up to half INT in kilometres |
| Disease Immunity | immune (automatic for creatures without SIZ) |
| Diving Strike | as charge: attack Size and Damage Modifier +1 step; once per round; needs one round of Movement height/depth advantage |
| Earth Sense | fight/Perceive with no penalty up to "INS"[sic] metres; halved above ground |
| Echolocation | Stealth vs. it two grades harder |
| Engulfing | swallow up to half own SIZ; bite damage first; then asphyxiation |
| Flying | Fly skill (STR+DEX); may substitute Fly for Evade while aloft |
| Formidable Natural Weapons | may actively parry/deflect with natural weapons |
| Frenzy | Willpower or frenzy CON rounds: only attack/move actions; immune to pain/fatigue/mental control/Serious Wound effects (Major still incapacitates); afterwards Exhausted |
| Gaze Attack | active (1 AP) or passive (per description) |
| Grappler | successful strike seizes (Grip); parried exchange grants Grip on limb or Pin Weapon; resists with Brawn |
| Hold Breath | CON minutes prepared/static; halved if active |
| Immunity | immune to one damage source (named per creature) |
| Intimidate | unopposed Willpower to hold ground; fail = retreat a round; Fumble = flee at max speed; Critical = immune to it/herd for the encounter |
| Leaper | leaping attack + natural weapon damage on win (only Passive Blocking parries) |
| Life Sense | Perception → injuries/Fatigue/poison/disease by touch; life within Willpower-skill metres |
| Magic Sense | as Life Sense for magic points/enchantments/active spells |
| Multi-Headed | +1 Combat Action per extra head (lost with each head); separate saves per head |
| Multi-Limbed | +1 Combat Action per extra limb pair brought to bear (limbs used for movement don't count) |
| Night Sight | partial darkness = lit; darkness = partial darkness |
| Poison Immunity | immune (automatic for creatures without SIZ) |
| Regeneration | HP regen per creature; no limb regrowth; severing head/chest/abdomen still kills |
| Swimmer | auto-succeed routine swimming; may substitute Swim for Athletics/Evade in water |
| Terrifying | unopposed Willpower: Success = shaken 1 round, no offense; Failure = flee; Fumble = collapse unconscious; Critical = unhindered; no further checks that combat |
| Trample | creatures of ≤ half own SIZ; Athletics to attack; damage = 2× base Damage Modifier, Size +1 step; 1 AP if engaged vs. prone target, Free if moving/charging over them |
| Undead | immune to fatigue and Serious Wound effects; Major Wounds don't incapacitate but sever/shatter; bound location (head or chest): Major Wound destroys outright; contiguous severed parts stop functioning |
| Vampiric | bite damage required to drain blood → Fatigue levels (rate per creature) |
| Venomous | venomous bite/sting (per description) |
| Wing Buffet | Attack Action; damage = damage bonus to all within 3 m of front or sides |

Rule of thumb: creatures have up to three Abilities (exceptional creatures more).

---

## 9. Tables the Document Does NOT Contain

For engine implementers: the following commonly expected tables are **absent from
Mythras Imperative** and must not be invented from other Mythras products:

- Currency / price lists / equipment costs (equipment is narrative: "pocket money").
- Encumbrance or carried-weight rules.
- Aging effects (only age categories for starting experience exist).
- Characteristic-by-characteristic effect tables (only zero-characteristic effects
  and the derived-attribute tables above).
- Walk/Run/Sprint movement multipliers (referenced but never defined).
- Hit Location tables for non-humanoid creatures other than the ten sample
  creatures above.
- Cults, religions, Folk Magic, Psychic Powers, Theism, Animism, Sorcery (only the
  generic "Magic" skill + 24 spells and the superpower system).
- Hero Points (only Luck Points).
- Experience "checks" BRP-style (only Experience Rolls spent after sessions).
