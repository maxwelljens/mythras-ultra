# Mythras Imperative — Rules Digest for Engine Implementation

Implementation-oriented digest of **`Mythras-Imperative.md`** (repo root, ~260 KB,
3,409 lines) — the *Mythras Imperative System Resource Document* (ORC-licensed
Mythras ruleset). This file is the implementation mirror of the rulebook; the pure
numeric data lives in [`tables.md`](tables.md). Where this digest and the rulebook
disagree, the rulebook wins.

Source-citation convention: every section below names the rulebook heading (and
source line numbers) it transcribes. `[AMBIGUOUS: ...]` footnotes flag passages
where the source admits more than one reading — no interpretation is invented.

**Caveats about the source**: it is a working draft. It contains a stray editorial
note at line 4 ("XXX- EDIT to see if I would be restoring errors from the previous
edit..."), broken "page XX" cross-references, occasional duplicated words from
editing (e.g. "metrefive-metre"), and minor internal inconsistencies (noted below).
None of these change the rules; transcriptions follow the evident intent and flag
doubts.

---

## 0. Document Structure (source headings, with line numbers)

```
L1    # Mythras Imperative System Resource Document
L6    # Introduction
        L10  ## Rounding of Numbers and Results
        L14  ## Dice Used
L22   # Characters
        L34  ## Step 1: Character Concept
        L38  ## Step 2: Characteristics (STR, CON, SIZ, DEX, INT, POW, CHA)
        L80  ## Calculating Characteristics
        L92  ## Step 3: Attributes (Action Points, Damage Modifier, Experience
              Modifier, Healing Rate, Hit Points, Initiative, Luck Points,
              Magic Points, Movement Rate)
        L211 ## Step 4: Skills
        L246 ## Step 5: Culture (Barbarian L250, Civilised L260, Nomadic L270,
              Primitive L280)
        L292 ## Step 6: Career (31 careers, L298–332)
        L334 ## Step 7: Bonus Skill Points (Experience Table L344, Skill
              Pyramid L359, Larger-Than-Life Heroics L393: Pulp Hero L401,
              Paragon L431)
        L463 ## Step 8: Passions (Using Passions L484, Deepening and Waning L504)
        L520 ## Step 9: Equipment
L534  # Skills
        L538 ## How Skills Work (Criticals and Fumbles L554, Automatic
              Successes L563, Modifying Skills L567)
        L590 ## Standard Skills (22 skills, L592–688)
        L690 ## Professional Skills (39 skills, L694–852)
        L854 ## Skill Rules for Different Circumstances and Situations
              (Reattempting L858, Opposed Rolls L864, Differential Rolls L893,
              Opposed Skills Over 100% L908, So Just How Good Are You? L914)
L929  # Game System
        L935 ## Asphyxiation
        L950 ## Character Improvement (Experience Rolls L959, Increasing
              Existing Skills L967, Increasing Passions L979, Learning New
              Skills L985)
        L991 ## Disease and Poison (Traits L997, Conditions Table L1023,
              Sample Disease: Red Pox L1037)
        L1051 ## Falling (Falling Objects L1066)
        L1070 ## Fatigue (Recovering L1082, Fatigue Levels L1088)
        L1103 ## Fires (Fire Intensity Table L1109)
        L1121 ## Game Time (Combat Rounds L1127, Skill Time L1131, Narrative
              Time L1135)
        L1139 ## Hit Locations (Humanoid Hit Locations L1147; Damage and Wound
              Levels L1161: Minor L1180, Serious L1184, Major L1194, Blood Loss
              L1202, Heroic Last Actions L1206)
        L1210 ## Healing from Injury
        L1227 ## Inanimate Objects (Weapons vs Inanimate Objects L1233)
        L1259 ## Luck Points (Cheat Fate L1269, Desperate Effort L1273,
              Mitigate Damage L1277)
L1281 # Combat
        L1285–1307 intro terms (Combat Style, Combat Rounds, Combat Actions,
              Weapon Size, Engagement, Special Effects)
        L1309 ## Combat Styles (Combat Style Traits table L1321)
        L1337 ## Running Combat
        L1341 ## Firearms (design notes)
        L1358 ## Combat Rounds and Initiative (Initiative L1368, Actions in
              Combat L1374)
        L1382 ## Combat Actions (Proactive L1390: Attack, Brace, Cast Magic/
              Use Power, Change Range, Charge, Delay, Dither, Hold Magic/Power,
              Mount, Move, Outmanoeuvre, Ready Weapon, Reload, Regain Footing,
              Sneak Peek, Steady Weapon, Struggle, Switch Fire Mode, Take Cover;
              Reactive L1470: Counter Spell, Evade, Interrupt, Parry; Free L1490:
              Assess Situation, Drop Item, Signal, Speak, Use Luck Point, Ward
              Location)
        L1520 ## Movement (Situational Movement Rates L1532: Climb, Crawl,
              Jump, Swim)
        L1552 ## How Combat Works (Attacks and Parries L1556, Weapon Size L1570,
              Damage Reduction L1574, Armor L1584, Parrying a Missed Attack L1590,
              Unsuccessful Rolls and Fumbles L1598)
        L1602 ## Special Effects (Special Effects Table L1616; Special Effect
              Descriptions L1653–1730)
        L1732 ## Close Combat (Situational Modifiers L1736, Engagement L1758,
              Charging L1762, Cover L1770, Evading L1778, Knockback L1784,
              Leaping Attacks L1788, Outmanoeuvring L1796, Passive Blocking
              L1804, Surprise L1810, Sweep Attacks L1821)
        L1827 ## Ranged Combat (Force/DM/Range/Load/Impale Size L1833–1851,
              Situational Modifiers L1853, Aiming L1881, Automatic Fire L1885,
              Dual Firearms L1911, Firing into a Crowd L1915, Fumbling with
              Firearms L1923, Parrying Weapon Fire with Shields L1927,
              Telescopic Sights L1933)
        L1939 ## Arms and Armor (armor/shields/melee/ranged tables L1943–2003)
        L2005 ## Vehicles (Statistics L2011, Hull and Structure L2040, Speed
              L2067, Systems L2091, Traits L2099, Handling and Manoeuvres L2248,
              Shields L2274, Weapons L2282, Sample Vehicles L2356–2554)
L2556 # Magic and Powers
        L2560 ## How Powers Work (Casting Spells L2568, Using Superpowers L2572)
        L2576 ## Beginning Powers and Learning New Ones (Magicians L2578,
              Superheroes L2584)
        L2602 ## Magic (roll outcomes L2606)
        L2615 ## Limits to Magic Capacity
        L2619 ## Spells (Duration L2623, Magic Traits L2627)
        L2637 ## Spell Descriptions (24 spells, L2639–2793)
        L2795 ## Superpowers (Powers L2803: 17 powers with Boosts; Limits L2918)
L2948 # Creatures
        L2952 ## Sample Creatures (Giant Ant, Bear, Crocodile, Dragon,
              Goblin/Orc, Horse, Minotaur, Shark, Skeleton, Xenomorphic Alien)
        L3251 ## Creature Abilities (≈35 abilities, L3257–3409)
```

---

## 1. Conventions (Introduction, L6–20)

- **Rounding**: whenever a division creates a fraction, **round up** to the whole
  number (e.g. 1/10th of 63% = 6.3 → 7).
- **Dice**: d4, d6, d8, d10, d12, d20, d100; **d2** = odd 1 / even 2 (or coin);
  **d3** = d6: 1–2 = 1, 3–4 = 2, 5–6 = 3.
- The document is compatible with Mythras core and other ORC percentile systems;
  *Classic Fantasy Imperative* is its companion.

---

## 2. Character Creation (Characters, L22–532)

### Step 1: Character Concept (L34)
Narrative only — guides Culture and Career choices. No mechanics.

### Step 2: Characteristics (L38–90)

Seven characteristics: **STR** (lift/hit; 0 STR = cannot move/lift), **CON**
(health; 0 CON = death), **SIZ** (mass; feeds HP and Damage Modifier), **DEX**
(agility; feeds AP and Initiative; 0 DEX = functional paralysis), **INT**
(cognition; feeds AP and Initiative; 0 INT = mindless), **POW** (soul/magic; feeds
Magic Points and Luck Points; 0 POW = no independent will), **CHA** (presence;
feeds Experience Modifier; 0 CHA = cannot socially interact).

**Generation methods** (L80–90):

| Method | Rule |
| --- | --- |
| Dice Roll | 3d6 for STR, CON, DEX, POW, CHA; 2d6+6 for SIZ, INT. Take results as rolled. |
| Dice Roll, Assign | As above, but swap values **within the same dice-range group** only (STR/CON/DEX/POW/CHA group; SIZ/INT group). Never across groups. |
| Dice Roll, High | Roll one extra die per characteristic (per its formula) and drop the lowest. |
| Points Build | Assign from a pool of **80 points**; each characteristic bounded by the min/max of its dice range. |

[AMBIGUOUS: "Points Build" bounds ("cannot be lower than the minimum rolled value,
nor higher than the maximum") most naturally mean the dice-range minima/maxima
(3–18 for 3d6, 8–18 for 2d6+6), but no explicit bound table is printed.
Source also references "the number in parentheses after each dice score is the
racial average" — no such parenthetical values appear anywhere in the document.]

### Step 3: Attributes (L92–209)

| Attribute | Rule / Formula (exact) |
| --- | --- |
| **Action Points** | **2** for all Mythras Imperative characters (L98). |
| **Damage Modifier** | Add STR+SIZ, consult table below. Added as an extra die roll to weapon/tool damage (or subtracted). If a negative DM takes damage to zero or below, **no damage at all** is inflicted. |
| **Experience Modifier** | From CHA (table below); adjusts Experience Rolls received. |
| **Healing Rate** | From CON (table below); HP recovered per day/week/month depending on wound severity. |
| **Hit Points** | Per location. Add CON+SIZ, consult table below. |
| **Initiative** | "Initiative is the **average of the DEX and INT** Characteristics" (L180). Used as the Initiative Modifier added to 1d10 in combat. |
| **Luck Points** | From POW (table below). Replenish to normal value at the start of each session. |
| **Magic Points** | **Equal to POW** (L205). |
| **Movement Rate** | Not calculated from characteristics; species default. **Humans: 6 metres** (L209). |

**Damage Modifier** (L106–127):

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

[AMBIGUOUS: the "Continue Progression" row (beyond STR+SIZ 120) does not define
the next terms — 2d10+2d2? +1d4? — the source gives no answer. Step 3, Damage
Modifier.]

**Experience Modifier** (L135–142): CHA 6 or less = **-1**; 7–12 = **+0**;
13–18 = **+1**; **each 6 points** thereafter = **+1**.

**Healing Rate** (L148–155): CON 6 or less = **1**; 7–12 = **2**; 13–18 = **3**;
**each 6 points** thereafter = **+1**.

**Luck Points** (L192–199): POW 6 or less = **1**; 7–12 = **2**; 13–18 = **3**;
**each 6 points** thereafter = **+1**.

**Hit Points per Location** (L165–174) — cross-reference CON+SIZ:

| Location | 1–5 | 6–10 | 11–15 | 16–20 | 21–25 | 26–30 | 31–35 | 36–40 | +5 |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Head | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | +1 |
| Chest | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | +1 |
| Abdomen | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | +1 |
| Each Arm | 1 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | +1 |
| Each Leg | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | +1 |

("+5" column: +1 HP per location per additional 5 points of CON+SIZ beyond 40.)
[AMBIGUOUS: rounding of the Initiative "average" of DEX and INT is not stated;
the Introduction's round-up rule implies ceil((DEX+INT)/2). Step 3, Initiative.]

### Step 4: Skills (L211–244)

Two classes: **Standard** (everyone has them) and **Professional** (trained).
Three point pools: **Culture 100**, **Career 100**, **Bonus 150** (350 total);
1 point = +1% to a skill; only into skills the Culture/Career grants (Bonus pool:
any Standard skill, or Professional skills from Culture/Career). GM may cap
per-skill spend. **Combat Style** is listed in Cultural descriptions and has a
Basic Percentage of **STR+DEX** (L215).

**Standard Skills — Basic Percentage** (L217–244):

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

(The +40 on Customs and Native Tongue is a static bonus; Native Tongue is Common
for humans.) Full rules text for each skill: Skills chapter, L590–852.

### Step 5: Culture (L246–290)

Four human cultures. Each grants listed Standard skills + **up to 3** Professional
skills chosen from a list + a cultural Combat Style. Distribute the 100 Culture
points among the listed skills.

| Culture | Standard Skills | Professional Skills (pick ≤3) |
| --- | --- | --- |
| **Barbarian** | Athletics, Brawn, Endurance, First Aid, Locale, Perception; **and either Boating or Ride**; + cultural Combat Style | Craft (any), Healing, Lore (any), Musicianship, Navigation, Seamanship, Survival, Track |
| **Civilised** | Conceal, Deceit, Drive, Influence, Insight, Locale, Willpower; + cultural Combat Style | Art (any), Commerce, Craft (any), Courtesy, Language (any), Lore (any), Musicianship, Streetwise |
| **Nomadic** | Endurance, First Aid, Locale, Perception, Stealth; **and two of** Athletics, Boating, Swim, Drive, Ride (per primary mode of travel); + cultural Combat Style | Craft (any), Culture (any), Language (any), Lore (any), Musicianship, Navigation, Survival, Track |
| **Primitive** | Brawn, Endurance, Evade, Locale, Perception, Stealth; **and one of** Athletics, Boating, Swim; + cultural Combat Style | Craft (any), Healing, Lore (any), Musicianship, Navigation, Survival, Track |

### Step 6: Career (L292–332)

31 careers (also in `tables.md` §1.12). Each grants listed Standard skills +
**up to 3** Professional skills from its list. Distribute the 100 Career points
among them. Specific Professional skills (e.g. Pilot) may not exist in a given
setting. Complete table (L298–332):

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

[AMBIGUOUS: career lists contain items that are not defined skills — "Linguistics"
(Bounty Hunter, Detective; presumably Language), "Culture" among Mechanic's
Standard skills (Culture is Professional), "Customs (any)" (Pilot), and the
Magician row omits "(any)" qualifiers. Step 6, Careers.]

### Step 7: Bonus Skill Points (L334–461)

- **150 bonus points** to round out; any Standard skill or Culture/Career
  Professional skills; GM may cap.
- **Creating Experienced Characters** — **Experience Table** (L344–352):

| Category | Age | Bonus Skill Points | Maximum Skill Increase |
| :-- | :-: | :-: | :-: |
| Young | 10+1d6 | 100 | +10 |
| Adult | 15+2d6 | 150 | +15 |
| Middle Aged | 25+3d6 | 200 | +20 |
| Senior | 40+4d6 | 250 | +25 |
| Old | 60+5d6 | 300 | +30 |

Bonus Skill Points = points spent on skills; Maximum Skill Increase = cap on how
many Bonus Points may go to any one skill. The Age Bonus is approximate;
converting an existing character substitutes its actual age.

[AMBIGUOUS: the source does not say outright that the age-category Bonus replaces
the default 150-point Bonus pool while Culture 100 + Career 100 are kept — that is
the natural reading of "Bonus Skill Points determine how many points the character
must spend on developing skills". Step 7, Experience Table Notes.]
**There are no aging penalty rules** — age exists only to size the starting
experience pool.

- **The Skill Pyramid** (L359–391) — alternative to the 350 points:
  1 skill at 50%, 2 at 40%, 3 at 30%, 4 at 20%, 5 at 10% (total 350).
  Rules: all points allocated; at most one allocation per skill; Customs and
  Native Language keep their +40; Passions keep their values; no skill may exceed
  **80%** from an allocation; **six** values must go to Professional Skills;
  a Combat Style allocation is recommended but optional; GM has final say.
- **Larger-Than-Life Heroics** (L393–461):

| | Pulp Hero (L401) | Paragon (L431) |
| --- | --- | --- |
| Characteristics | Roll 4d6 drop lowest ×5 → STR, CON, DEX, POW, CHA; 3d6+6 drop lowest ×2 → SIZ, INT | Roll 4d6 drop lowest ×6, assign best 5 → STR, CON, DEX, POW, CHA; 3d6+6 drop lowest ×3, assign best 2 → SIZ, INT |
| Points Build pool | 90 | 100 |
| Advantages (no stacking) | choose **2**: +1 AP; +1 Luck Point; +1 HP per Hit Location; Endurance / Stealth / Willpower rolls one Grade easier | choose **3**: +1 AP; +2 Luck Points; +2 HP per Hit Location; Endurance / Stealth / Willpower rolls one Grade easier |
| Healing | double Healing Rate for Minor/Serious Wounds (Major at normal rate) | same |
| Extra skill points | +50 Bonus | +100 Bonus |

### Step 8: Passions (L463–518)

Passions measure deep commitments (Desire, Hate, Loyalty to, Uphold, …). Rated
1–100+ like skills. Beginning characters: **up to 3 Passions**, each built on two
characteristics per the Passion Table, plus **+40 (first), +30 (second), +20
(third)** — player sets the order of importance.

| Object of Passion | Starting Percentage |
| :-- | --- |
| A person (romantic/familial) | POW+CHA |
| An organization or group | POW+INT |
| A race or species | POW x2 |
| A place | POW+INT |
| An object or substance | POW x2 |
| A personality trait | POW+CHA |

**Uses** (L484–502): (1) augment a thematically relevant skill with **+20% of the
Passion's value**; (2) standalone roll — Success = act per the Passion, Failure =
free action; (3) opposed Passion vs. Passion; (4) measure of commitment; (5)
substitute for **Willpower** in an Opposed Roll vs. psychological manipulation or
magical domination. New in-play Passions cost no Experience Rolls; established via
improvement they cost 1 Experience Roll at base value.

**Deepening and Waning** (L504–518): change triggered by play; GM decides reduce
or reverse (reversal keeps the same percentage):

| Depth of Change | Passion Modified by |
| --- | --- |
| Weak | 1d10 |
| Moderate | 1d10+5 |
| Strong | 1d10+10 |

### Step 9: Equipment (L520–532)

Starting gear: appropriate clothing; **pocket money** ("enough to last a little
while without hardship"); a personal heirloom/trinket; trade tools/transportable
equipment if appropriate; a personal weapon for the character's Combat Style; other
GM-approved items.

**Explicitly absent**: currency system, price lists, equipment costs, encumbrance
rules. (No money table exists anywhere in the document.)

---

## 3. Skill System (Skills, L534–927)

### How Skills Work (L538–552)

A skill's Base Value = two characteristics summed, or one ×2. No upper limit;
skills routinely exceed 100%. Test: roll **1d100**:

| Outcome | Rule |
| --- | --- |
| Success | roll **equal to or less than** the (possibly modified) skill value |
| Failure | roll greater than the skill value |
| Always Success | any roll of **01–05** (irrespective of skill) |
| Always Failure | any roll of **96–00** (irrespective of skill) |

**Criticals and Fumbles** (L554–561):
- **Critical Success** = roll ≤ **one-tenth of the skill's value** (of the
  *modified* value; fractions round up). Example: skill modified 33% → 66% ⇒
  Critical range 07 or less (6.6 → 7).
- **Fumble** = roll of **99 or 00**. Skills **over 100%** fumble **only on 00**.
- Critical/Fumble consequences are GM-determined.

**There are exactly four success levels: Critical Success, Success, Failure,
Fumble. Mythras Imperative has no "Exceptional Success" tier.**

**Automatic Successes** (L563–565): trivial or time-unconstrained tasks need no
roll (gentle canter on open ground; unhurried craftwork; picking a simple lock
undisturbed).

**Modifying Skills — Difficulty Grade Table** (L567–588). Two alternative systems
(use one consistently):

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

When several penalties apply, the **hardest grade takes precedence** (grades are
not stacked). Out of combat the GM assigns grades ad hoc.

[AMBIGUOUS: the Ranged Combat Situational Modifiers table uses a grade named
"Impossible" (completely invisible target) that this table does not define; its
footnote says treat as Herculean if the target's general location is known.
Skills › Modifying Skills / Combat › Ranged Combat.]

### Skill Lists

**Standard skills** (bases in §2 Step 4 above): Athletics, Boating, Brawn, Conceal,
Customs, Dance, Deceit, Drive, Endurance, Evade, First Aid, Influence, Insight,
Locale, Native Tongue, Perception, Ride, Sing, Stealth, Swim, Unarmed, Willpower.
Plus **Combat Style** (STR+DEX). Notable per-skill mechanics:

- **Evade** (L632): dodge ranged attacks/traps/hazards; usually leaves you **prone**;
  Daredevil trait avoids that. Resistance roll vs. some magic.
- **First Aid** (L636): once per specific injury; heals **1d3** points.
- **Native Tongue** (L652): not rolled; static fluency measure.
- **Unarmed** (L678): universal combat skill; combat Criticals/Fumbles use combat rules.
- **Willpower** (L684): mental counterpart to Endurance; a **Critical** on a
  Standard test means no further attempts to influence/shake the character work
  (vs. magic: no further mentally afflicting spells from that opponent for the
  rest of the encounter).

**Professional skills** (39; bases in `tables.md` §1.9): Acting, Acrobatics,
Astrogation, Art, Bureaucracy, Commerce, Comms, Computers, Courtesy, Craft,
Culture, Demolitions, Disguise, Electronics, Engineering, Forgery, Gambling,
Healing, Language, Literacy (Specific Language), Lockpicking, Lore, Magic,
Mechanisms, Musicianship, Navigation, Oratory, Pilot, Politics, Research, Science,
Seamanship, Seduction, Sensors, Sleight, Streetwise, Survival, Teach, Track.
Notable mechanics:

- **Acrobatics** (L698): halves fall damage on success (and avoids prone if no
  Serious/Major Wound); may substitute for Evade and keeps footing.
- **Language** (L770): static bands 1–25 basic words / 26–50 simple sentences /
  51–75 fluent / 76+ eloquent; caps dependent skills (e.g. Bureaucracy ≤ Language).
- **Healing** (L766): treats serious injuries (typically HP at zero or below);
  sets bones, sutures; requires resources.
- **Magic** (L786): casts the spells of the Magic and Powers chapter.

### Circumstance Rules (L854–927)

- **Reattempting Skills** (L858): one follow-up attempt allowed (GM discretion),
  one Difficulty Grade harder; a second failure means no further attempts "for the
  foreseeable future".
- **Opposed Rolls** (L864): both roll; better Level of Success wins; equal levels →
  the **highest dice roll still within the success range** wins. Both fail →
  re-roll after narrating, or GM explains a fitting outcome. Used for wound
  resistance, poison/disease, magic. Standard pairings (L877–891):

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

- **Differential Rolls** (L893): both roll and suffer their own results; the
  higher Level of Success gains an advantage equal to the difference in success
  levels (needs at least one Standard success). Equal levels = no effect.
  Complete results matrix (L899–906):

| Result of Rolls | Antagonist Critical | Antagonist Success | Antagonist Failure | Antagonist Fumble |
| --- | --- | --- | --- | --- |
| Protagonist Critical | No Benefit | Protagonist Wins 1 Level of Success | Protagonist Wins 2 Levels of Success | Protagonist Wins 3 Levels of Success |
| Protagonist Success | Antagonist Wins 1 Level of Success | No Benefit | Protagonist Wins 1 Level of Success | Protagonist Wins 2 Levels of Success |
| Protagonist Failure | Antagonist Wins 2 Levels of Success | Antagonist Wins 1 Level of Success | No Benefit | No Benefit |
| Protagonist Fumble | Antagonist Wins 3 Levels of Success | Antagonist Wins 2 Levels of Success | No Benefit | No Benefit |

- **Skills Over 100% in Opposed/Differential Rolls** (L908): the highest-skilled
  participant (after modifiers) subtracts (skill − 100) **from every participant's
  skill including their own**.
- **Competency Level Table** (L914–927) — complete (Random Value = dice code for a
  random NPC skill in that band):

| Skill Rating | Random Value | Competence Level | Military Equivalent | Description |
| --- | --- | --- | --- | --- |
| 1-30% | 1d20+10 | Novice | Cannon Fodder | Unqualified and learning the basics |
| 31-50% | 1d20+30 | Competent | Green | Can perform easy tasks if taking time to be careful |
| 51-70% | 1d20+50 | Proficient | Seasoned | Has reached an employable level of competence |
| 71-90% | 1d20+70 | Expert | Veteran | Recognised as a professional or subject matter expert in the field |
| 91-110% | 1d20+90 | Master | Elite | Considered one of the best in the business |
| >110% | 1d20+110 | Exemplar | Heroic | Renowned far and wide for their consummate skill |

---

## 4. Game System (Game System, L929–1279)

### Asphyxiation (L935)
Hold breath = **Endurance skill in seconds** (prepared); halved if unprepared and
passive; **one-fifth** if in strenuous activity. Then Endurance roll: **Critical** =
no deterioration; **Success** = +1 Fatigue level; **Fail** = 1d2 extra levels that
Round; **Fumble** = 1d3 extra levels that Round. Recovery: 1 Fatigue level per
minute once asphyxiation ends; GM may impose an enduring level (smoke/water).

### Character Improvement (L950–989)
- **Experience Rolls**: GM awards at scenario end or every 2–3 sessions of a long
  story; rule of thumb **1–3 Experience Rolls per session** since last award.
- **Increasing a skill** (1 Experience Roll): roll **1d100, add INT**, compare to
  current skill: **≥ skill → +1d4+1%**; **< skill → +1%**. A skill **Fumbled
  since the last award gains a free +1%** (applied before any Experience Roll;
  multiple fumbles don't stack). **One Experience Roll per skill per session.**
- **Passions**: improved like skills (GM may allow reduction).
- **Learning a new Professional skill**: find a teacher/source, **one month** of
  study, **3 Experience Rolls** + in-game costs → skill at Base Level.

### Disease and Poison (L991–1049)
Traits: **Application** (Ingestion / Inhaled / Contact / Injected), **Potency**
(opposed vs. resisting skill — usually Endurance or Willpower; win = shrug off;
lose = suffer all Conditions), **Resistance** (Endurance or Willpower; may be
rolled at exposure or deferred to Onset Time), **Onset Time**, **Duration**,
**Conditions**, **Antidote/Cure**.

**Conditions Table** (complete, L1023–1035):

| Condition | Effects |
| --- | --- |
| Agony | Any skill roll involving use of the affected area must also be ≤ the character's Willpower, otherwise the attempt fails and they moan or scream in pain. |
| Asphyxiation | Victim collapses incapacitated, unable to breathe (Asphyxiation rules apply). May be asthmatic (shortness of breath) or complete respiratory failure (death); in the latter case the victim can be kept alive by winning an opposed First Aid test against the Potency of the disease or poison. |
| Bleeding | Internal bleeding or surface haemorrhaging → the Blood Loss effects. |
| Contagious | The victim can transfer the poison or disease by touch. |
| Death | The victim collapses incapacitated and dies after a number of rounds equal to their CON characteristic. |
| Fever | All skills suffer a difficulty grade of Hard. |
| Mania | Driven to follow a compulsion (e.g. avoiding water, paranoia, self-mutilation); the mania is noted in the description. |
| Nausea | Cannot eat; Endurance roll each time a stressful physical action is performed to avoid being sick — vomiting lasts 1d3 rounds, during which they cannot act. Long durations may cause starvation. |
| Paralysis | Unable to move: the affected area (if a location) cannot be used for the duration; whole-body paralysis means no movement at all. |

**Sample disease — Red Pox** (L1037–1049):

| Field | Value |
| --- | --- |
| Application | Contact |
| Potency | 50 |
| Resistance | Endurance |
| Onset time | 1d6+6 hours |
| Duration | 1d6+3 days |
| Conditions | On manifestation: Contagious. 1 day later: Fever and Bleeding. Each day the pustules weep blood, lose one level of Fatigue, which does not recover until the duration of the disease completes. |
| Antidote/Cure | Can be cured by the Healing skill. |

Object contagion persists 1d3 days after contact.

### Falling (L1051–1068)

| Distance Fallen | Damage Taken |
| --- | --- |
| 1 metre or less | No damage |
| 2-5 metres | 1d6 to one random location |
| 6-10 metres | 2d6 to two random locations |
| 11-15 metres | 3d6 to three random locations |
| 16-20 metres | 4d6 to four random locations |
| Each +5 metres | +1d6 damage |

Armor Points do **not** reduce falling damage; the faller's Damage Modifier
**applies** to damage suffered. **Falling objects**: 1d6 per 6 points of SIZ (or
fraction) **plus** the Falling Distance damage.

### Fatigue (L1070–1101)
Accrual: failed Athletics/Brawn/Endurance roll per strenuous activity (GM
discretion), + Asphyxiation/Blood Loss/etc. Each level penalises skills, movement,
Initiative, AP:

| Fatigue Level | Skill Grade | Movement | Initiative | Action Points | Recovery Period |
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

Recovery (physical exertion Fatigue): complete rest for **Recovery Period ÷
Healing Rate** per level. At Incapacitated the character is conscious but nearly
helpless; beyond that, no action (levels reserved for suffocation, blood loss,
starvation, exposure).

### Fires (L1103–1119)

| Intensity | Examples | Time to Ignite (Rounds) | Damage (per Round) |
| --- | --- | --- | --- |
| 1 | Candle | 1d4 | 1d2 |
| 2 | Torch | 1d3 | 1d4 |
| 3 | Campfire | 1d2 | 1d6 |
| 4 | Room filling conflagration | 1d2 | 2d6 |
| 5 | Volcanic lava | Instant | 3d6 |

Intensity 1–2: one Hit Location; 3–4: nearest 1d4+1 locations; 5: all locations.
Fire attacks ignite flammables at **damage rolled ×5%** (electrical ×1%). Burning:
**1d3-1 damage per Round** (ignores AP on materials; flesh beneath takes it;
natural armor protects). Smother: drop prone + Athletics (1 AP, once per Turn);
assistants or self-extinguished locations each make the roll one grade easier; a
zero damage roll means the fire died in that location. Uncontrolled spread: to
Intensity-many locations per Round.

### Game Time (L1121–1137)
**Combat Round** = 5 seconds. **Skill Time** = minutes to hours (definite effort).
**Narrative Time** = real-time roleplay / heavily compressed travel.

### Hit Locations and Wounds (L1139–1208)

Random location: **1d20** on the Humanoid Hit Locations table:

| 1d20 | Hit Location |
| --- | --- |
| 1-3 | Right Leg |
| 4-6 | Left Leg |
| 7-9 | Abdomen |
| 10-12 | Chest |
| 13-15 | Right Arm |
| 16-18 | Left Arm |
| 19-20 | Head |

(Non-humanoids have their own tables in their stat blocks; Special Effects such as
Choose Location can select the location. Some attacks — breath weapons, grenades —
strike several locations at once.)

**Damage application order** (L1165–1170):
1. Apply the attacker's **Damage Modifier**.
2. Modify damage from weapon-enhancing (or reducing) **magic**.
3. If parried, reduce per comparative **weapon sizes**.
4. Reduce by the location's **Armor Points**.

If damage > 0, subtract it from the location's HP immediately and note the wound
category:

| Wound | Threshold |
| --- | --- |
| **Minor Wound** | location still has positive HP |
| **Serious Wound** | location at zero HP or below |
| **Major Wound** | location at negative HP **equal to or greater than its starting HP** |

**Serious Wound** (L1184–1192): victim **cannot attack or start to cast spells for
1d3 Turns** (may Parry/Evade). **Limb**: opposed Endurance vs. the enemy's attack
roll — fail = limb useless until back to positive HP (leg: drop prone; arm: drop
held object unless strapped). **Abdomen/Chest/Head**: same opposed Endurance —
fail = **unconscious for minutes equal to the damage** of the wounding attack
(First Aid/Healing can rouse). GM may impose one ongoing Difficulty Grade on tasks
using the location until it returns to Minor.

**Major Wound** (L1194–1200): victim **immediately Incapacitated**, drops prone;
a limb is **severed/transfixed/shattered/ripped off**. Opposed Endurance vs. the
attack roll — fail = unconscious. Untreated severed/punctured/ripped limb: **death
after minutes = 5 × Healing Rate**. **Torso/Head Major Wound**: drop unconscious,
totally Incapacitated; opposed Endurance vs. attack roll — fail = **instant death**;
survivors die unless treated within **Combat Rounds = 2 × Healing Rate**. Major
Wounds need surgery or significant magic to heal and may maim the location.

**Blood Loss** (L1202–1204): tracked as **Fatigue levels** (like Asphyxiation) but
enduring — recover **1 Fatigue level per day**, starting the day after bleeding
stops. Unrestrained bleeding kills without medical attention.

**Heroic Last Actions** (L1206–1208): after a Major Wound, a key character may burn
**1 Luck Point** for one final action (needs applicable working limbs), then
collapses.

### Healing from Injury (L1210–1225)

| Wound | Natural healing |
| --- | --- |
| Minor | Healing Rate HP per **day** |
| Serious | Healing Rate HP per **week** until above zero HP, then per day |
| Major | Healing Rate HP per **month** |

Example (verbatim logic): Healing Rate 3, a location taken to −3 (Serious) heals
3 HP/week until above zero, then 3 HP/day. Restrictions: strenuous activity reduces
Healing Rate by **1d3**; Major Wounds do not heal at all until a **successful
Healing roll** or healing magic; **non-dismembering Major Wounds untreated for days
= 1/20th of the Healing skill value become maimed** (permanent HP loss).
**First Aid**: once per injury, heals **1d3**.

### Inanimate Objects (L1227–1257)
Objects have AP and HP (AP reduce damage first; 0 HP = destroyed). Breakage: Brawn,
Unarmed, or Combat Style roll to damage an item. Weapon vs. object whose AP ≥ the
weapon's AP damages **both** (GM discretion; purpose-built tools like chisels take
no damage from their intended work). Complete table (L1237–1257):

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

### Luck Points (L1259–1279)
Replenish to normal value at the start of each session. **One Luck Point per
action**; **cannot** be spent on Experience or Training rolls. Each use costs 1:

| Use | Effect |
| --- | --- |
| **Cheat Fate** | re-roll **or swap digits** (75 → 57) any dice roll the character makes (skill, damage, anything with an effect); can also force an opponent to re-roll an attack/damage roll against them |
| **Desperate Effort** | gain one extra Action Point after AP are exhausted |
| **Mitigate Damage** | downgrade a Major Wound to Serious; damage reduced to 1 HP less than the Major threshold |

There are **no Hero Points** in this document — Luck Points fill that role.

---

## 5. Combat (Combat, L1281–2354)

### Overview (L1281–1356)
Terms: **Combat Style** (weapon-package skill), **Combat Rounds** (5-second
segments), **Combat Actions** (AP-costed acts), **Weapon Size** (parry difficulty),
**Engagement** (melee reach relationship), **Special Effects** (manoeuvres won
from success-level differences). **Combat Styles** (L1309–1335) bundle multiple
weapons, singly and combined; using a weapon outside the style costs one or more
difficulty grades. Styles may carry **Combat Style Traits** (table below).
The **Firearms** subsection (L1341) is design guidance: gunfire is near-impossible
to parry; cover, suppression and armour dominate; a hit exposes the victim to
Special Effects; the AP economy shifts toward all-out attacks.

**Combat Style Traits** (L1321–1335):

| Trait | Effect |
| --- | --- |
| Beast-back Lancer | mounted charge with the style: no one-step hit penalty |
| Blind Fighting | ignore poor-lighting/blinding penalties |
| Daredevil | Evade melee blows without ending prone; ranged: prone only on failure |
| Defensive Minded | weapon Size +1 step when parrying (no offensive action that round) |
| Formation Fighting | unflanked group of 3+ in close formation: each engaged foe −1 Action Point |
| Knockout Blow | surprise attacks: Stun Location lasts minutes instead of turns |
| Mounted Combat | ignore the Ride skill cap on combat rolls |
| Ranged Marksman | shift a random Hit Location roll to an adjoining location |
| Skirmishing | ranged attacks while walking or running |
| Throw Weapons | any style melee weapon throwable at no penalty; thrown damage halved |
| Unarmed Prowess | Unarmed blocks/parries count as Medium size |

### Combat Rounds and Initiative (L1358–1380)

- **Initiative**: each participant rolls **1d10 + Initiative Modifier** (Initiative
  = average of DEX and INT); highest first. Ties: higher DEX first; still tied:
  each rolls a die, high goes first. Initiative persists until something forces a
  re-roll. The **highest AP value of armor worn penalises the Initiative roll**.
- **Turns**: per Round each participant takes one or more Turns (per AP); a Turn =
  one Combat Action (declaration, movement, rolls incl. Reactions, resolution).
  Iterate Initiative order while any participant has unspent AP (or is holding AP
  for Reactions); then the next Round begins.
- **Action Points**: **2 per Round** for all characters. **Proactive** actions only
  on your own Turn; **Reactive** actions answer an imminent threat (one reaction
  per threat); **Free** actions cost nothing. **Unused AP do not carry over.**
  Unless specified, every Combat Action costs **1 AP**.

### Combat Actions (L1382–1518) — complete list with costs

**Proactive** (1 AP each): **Attack** (melee/ranged strike); **Brace** (SIZ +50%
vs. Knockback and Leaping Attacks; SIZ doubled vs. Bash); **Cast Magic/Use Power**
(complex magics take several Turns; release before your next Turn or Hold);
**Change Range** (close/retreat; not while engaged or pinned by fire); **Charge**
(see Charging); **Delay** (reserve actions for later Reactions/Interrupt; if unused
by your next Turn they are lost — Dither); **Dither** (do nothing — default);
**Hold Magic/Power** (hold a cast spell each Turn; lets enemies Counter Spell);
**Mount** (mount/dismount; large mounts several Turns); **Move** (Base Movement
Rate if unengaged); **Outmanoeuvre** (group opposed Evade — see below); **Ready
Weapon** (retrieve/draw/sheathe/reload; retrieving a dropped object = 2 Actions);
**Reload (Firearms)**; **Regain Footing** (auto if unengaged; if engaged, win an
Opposed Brawn/Athletics — the contest costs no AP); **Sneak Peek (Firearms)** (one
Perception look from cover; Fumble exposes you); **Steady Weapon (Firearms)**
(recoil penalty for Burst/Full-Auto −1 step until you move); **Struggle** (escape
grips/effects); **Switch Fire Mode (Firearms)** (Free for sufficiently advanced
weapons); **Take Cover** (duck behind cover — not prone; cover quality per Inanimate
Objects table, e.g. thin wooden wall = 4 AP).

**Reactive** (1 AP each): **Counter Spell** (negate an incoming spell outright;
needs 1-Turn casting time or pre-Held); **Evade** (opposed Evade; leaves prone
unless mitigated); **Interrupt** (Delaying characters only — cut into an
opponent's Turn, or attack of opportunity on anyone passing within reach; if the
opponent can't fulfil their declaration their AP is wasted); **Parry** (defend with
Combat Style).

**Free** (0 AP): **Assess Situation** (Perception if unengaged; Running = Hard,
Sprinting = Formidable); **Drop Item**; **Signal** (if unengaged); **Speak** (≤5
seconds of speech); **Use Luck Point**; **Ward Location** (dedicate a weapon to
passively block one location — shields can cover multiple; the block applies as a
parry-size reduction to any blow landing there; ends if the weapon attacks or
actively parries; must be declared before the attack roll; shields may cover
multiple locations; crouching behind a shield doubles the count — see Passive
Blocking).

### Movement (L1520–1550)

- Move Action = **Base Movement Rate** (humans 6 m). Only unengaged characters
  move (exceptions: Charge, Outmanoeuvre). Entering an opponent's engagement zone
  engages both (charging through contact excepted). Engaged characters cannot
  leave without **Withdrawing** (e.g. Withdraw Special Effect, Outmanoeuvre).
  Passing an unengaged Delaying foe lets them strike or block you.
- **Climb**: at Base Movement Rate (must not have moved faster than Walk; subtract
  movement already used); worn armor hinders by the AP of the heaviest item worn.
- **Crawl**: prone, max **2 metres per Turn** instead of a normal Move.
- **Jump**: successful Athletics: up to **2× own height** horizontal, **½ own
  height** vertical (with a 5-metre run-up); **halved** from standing.
- **Swim**: Base Movement Rate minus movement already used (Swim skill conditions
  apply).

[AMBIGUOUS: Walk/Run/Sprint speeds are referenced ("the rules for Walk, Run, and
Sprint set forth above") but never defined in this document — only the Move action
(Base Movement Rate) and Charge (Running/Sprint) are quantified. Combat ›
Movement.]

### How Combat Works — resolution procedure (L1552–1600)

Close combat, step by step (verbatim order):
1. Attacker spends 1 AP, rolls Combat Style, notes result.
2. If desired, defender spends 1 AP to Parry, rolls Combat Style.
3. Compare success levels **as a Differential Roll**.
4. Difference grants the superior roller one or more **Special Effects**.
5. If attacker achieved Success or Critical: roll weapon damage + Damage Modifier;
   determine Hit Location if applicable.
6. If defender achieved Success or Critical: reduce damage by comparative weapon
   Sizes.
7. Reduce remaining damage by Armor Points (natural + worn, which stack).

Special Effects are **independent of whether damage is inflicted** — a defender can
win the exchange and still be injured. Effects are selected **before** damage and
Hit Location are rolled; Endurance rolls for wounds come **after** applying effects.

**Parrying a missed attack** (L1590): a defender may still parry an attack that
would miss — winning Special Effects off the differential. A defender **unable or
unwilling** to parry is treated as having automatically **Failed** (granting a
successful attacker its Special Effects). If both combatants Fail, the exchange
ends.

**Weapon Size** (L1570): Small, Medium, Large, Huge, Enormous — mass, leverage and
stability; governs parry capacity. Creature natural weapons use the same
categories.

**Damage Reduction by parry** (L1574–1582):

| Parry vs. attack | Damage |
| --- | --- |
| Parrying weapon/shield **equal or greater Size** | deflects **all** damage |
| **one Size less** | deflects **half** damage |
| **two or more Sizes less** | deflects **none** |

(Example in source: great axe (Huge) — kite shield (Huge) blocks all; longsword
(Large) halves; shortsword (Medium) stops none.)

**Armor** (L1584–1588): remaining damage reduced by AP of natural and/or worn
protection (they stack). The **highest AP value worn penalises the Initiative
roll**.

### Special Effects (L1602–1730)

Number of Special Effects = Levels of Success won (Differential Roll Results
above). Effects may be mixed; some stack (e.g. Maximise Damage twice). Complete
matrix (L1616–1651) — X marks applicability:

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

Key mechanics of each effect (exact numbers, L1653–1730):

- **Accidental Injury** (attacker fumbles): attacker rolls weapon damage against
  self, random location; unarmed = internal tear/break ignoring armor.
- **Arise**: stand up from prone.
- **Bash** (shields/bludgeons): shields knock back **1 m per 2 damage rolled**
  (pre-reductions); bludgeons **1 m per 3**. Creatures up to **2× attacker's SIZ**.
  Into an obstacle: Hard Athletics or Acrobatics or fall/trip.
- **Bleed** (cutting; firearms = Critical only): overcome AP and injure → opposed
  Endurance vs. the attack roll; fail = **1 Fatigue level at the start of every
  Combat Round** until collapse/death. First Aid staunches (no strenuous action or
  it reopens).
- **Blind Opponent** (defender Criticals): attacker opposed Evade (or shield weapon
  skill) vs. the parry roll; fail = Hard or Formidable for **1d3 Turns**.
- **Bypass Armor** (attacker Criticals, stackable): pick worn or natural armor to
  bypass (magic protection counts as worn).
- **Choose Location**: melee — freely pick a reachable location; ranged/firearms —
  Critical only unless target is within **Close Range** and stationary or unaware.
- **Circumvent Parry** (Critical): ignore a successful parry.
- **Damage Weapon**: apply damage to the parrying weapon (uses its own AP; 0 HP =
  break). Firearms: Critical only (shields easier).
- **Disarm Opponent**: opposed Combat Style vs. your roll; weapon flung a distance
  equal to **the disarmer's Damage Modifier roll in metres** (drops at feet if no
  DM). Each Size step your weapon is larger: opponent's resistance one grade
  harder (smaller: easier). Up to **2× attacker's STR**.
- **Drop Foe** (firearms): minor wound+ → opposed Endurance vs. hit roll; fail =
  incapacitated for **1 hour ÷ target's Healing Rate**.
- **Duck Back** (firearms): immediately return to cover.
- **Enhance Parry** (defender Criticals): deflect entire force regardless of Size.
- **Entangle** (entangling weapons): immobilise the location struck (arm: can't
  use held item; leg: no movement; torso/head: all skill rolls one grade harder);
  wielder's next Turn: 1 AP → automatic **Trip** attempt. Escape: opposed Brawn,
  or win Damage Weapon / Disarm / Slip Free.
- **Force Failure** (opponent fumbles): combine with another opposed-roll effect —
  the opponent auto-fails the resistance.
- **Grip** (Unarmed): hold the opponent (no disengaging); escape: opposed Brawn or
  Unarmed vs. the gripper's choice.
- **Impale** (impaling weapons): **roll weapon damage twice, take either result**.
  May leave the weapon in: victim suffers a Difficulty Grade (Impale Effects
  Table: penalty by creature SIZ vs. weapon Size — e.g. SIZ 21–30 vs. Large =
  Formidable) until withdrawn. Withdraw (Ready Weapon; unopposed Brawn, opposed if
  resisted): extra damage to the same location = **half the normal damage roll,
  no Damage Modifier, armor does not reduce it** (barbed weapons: normal damage).
  While impaled the weapon cannot parry; further impalements of the same Size add
  no further penalty. **Impale Effects Table** (L1689–1698) — Difficulty Grade
  imposed on the victim's future skill attempts while the weapon remains:

| Creature SIZ | Small Weapons | Medium Weapons | Large Weapons | Huge Weapons | Enormous Weapons |
| --- | --- | --- | --- | --- | --- |
| 1-10 | Formidable | Herculean | Incapacitated | Incapacitated | Incapacitated |
| 11-20 | Hard | Formidable | Herculean | Incapacitated | Incapacitated |
| 21-30 | No Effect | Hard | Formidable | Herculean | Incapacitated |
| 31-40 | No Effect | No Effect | Hard | Formidable | Herculean |
| 41-50 | No Effect | No Effect | No Effect | Hard | Formidable |
| Each +10 | Follow table progression | | | | |
- **Marksman** (ranged): move the hit location one step to an adjoining area.
- **Maximise Damage** (attacker Criticals, stackable): one damage die becomes its
  full value (1d6 → 6; 2d6 → 1d6+6); Damage Modifier still rolled.
- **Over-penetration** (firearms Critical): shot passes through to a second target
  behind (armor must be overcome); second victim takes **half damage**; effects on
  the first don't carry.
- **Pin Down** (firearms, stackable): opposed Willpower vs. hit roll; fail = no
  return fire next Turn (crawl/reload/talk allowed). Works with zero damage.
- **Prepare Counter** (stackable): secretly nominate an effect; if the opponent
  uses it, substitute your own automatic effect.
- **Rapid Reload** (stackable): next reload time −1.
- **Scar Foe**: cosmetic but lasting scar.
- **Select Target** (attacker fumbles): the blow hits an adjacent bystander
  automatically (no effect on them; they cannot avoid it).
- **Slip Free** (defender Criticals): auto-escape Entangle/Grip/Pin.
- **Stun Location** (bludgeoning): overcome AP and injure → opposed Endurance vs.
  attack roll; fail = location **Incapacitated for turns = damage inflicted**;
  torso = staggered (defend only); head = insensible.
- **Sunder** (two-handed weapons): damage (after parry/magic) vs. armor AP; surplus
  **reduces the armor's AP**; any remainder carries to the location's HP.
- **Trip Opponent**: opposed Brawn/Evade/Acrobatics vs. your roll; fail = prone.
  Quadrupeds: Athletics for Evade, one grade easier.
- **Weapon Malfunction** (attacker fumbles, firearms): weapon useless until
  field-stripped/cleaned.
- **Withdraw**: defender breaks engagement automatically.

### Close Combat Situational Modifiers (L1736–1756)

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
| Fighting in pitch black conditions | Herculean |
| Blinded or loss of primary perceptive sense | Herculean |

Use the **most severe** when several apply. Skill **caps** instead of grades:
mounted fighting caps Combat Style at **Ride**; fighting in water caps at
**Swim**.

### Close Combat Situations (L1758–1825)

- **Engagement**: engaged if within melee reach of an opponent (one-sided reach is
  enough). Leaving requires Outmanoeuvre, the Withdraw effect, etc.
- **Charging** (L1762): Charge action at Run/Sprint; attack is **Hard**; attacker's
  weapon Size **+1 step** and Damage Modifier **+1 step** (bipeds) or **+2 steps**
  (quadrupeds+); riders may use the mount's DM. The recipient may counterattack
  instead of parrying/evading — **longer weapon strikes first**; a set spear uses
  the **charger's** Damage Modifier.
- **Cover** (L1770): attacks landing on obscured locations are blocked by the
  cover's protection; Choose Location can aim at visible parts, or strike through
  penetrable cover. Blind fire through total cover: **one grade harder**, no
  Choose Location.
- **Evading** (L1778): opposed **Evade vs. the attacker's pertinent roll**
  (anything: Drive of a ramming tank, casting skill, etc.). Winner: full effect or
  full avoidance; surplus Levels of Success grant Special Effects. Evading leaves
  the defender **prone** (Arise excepted).
- **Knockback** (L1784): damage (pre-parry/armor) **> recipient's SIZ** → knockback:
  Easy Acrobatics or Standard Athletics to avoid falling prone; pushed **1 m per 5
  points** (or fraction) of damage over SIZ. Brace mitigates.
- **Leaping Attacks** (L1788): opposed leaper Athletics vs. Brawn/Evade
  (quadruped target: one grade harder). Win → target prone, attacker astride (no
  damage; target can't rise until the attacker leaves or wins Arise). Up to
  **2× attacker's SIZ**. Brace helps if aware.
- **Outmanoeuvring** (L1796): group Opposed Evade (everyone spends 1 AP); foes who
  fail to beat your roll cannot attack you this Round; beating everyone lets you
  engage a single foe safely or Withdraw entirely.
- **Passive Blocking** (L1804): hold a weapon/shield to cover chosen location(s)
  — automatic parry-size damage reduction; cannot actively parry with it. Choose
  Location cannot bypass it in melee (other locations remain targetable).
  Crouching behind a shield doubles covered locations.
- **Surprise** (L1810): target −10 Initiative; flat-footed (cannot defend) until
  their Initiative; first successful attack gains a **bonus Special Effect**; no
  offensive actions for the rest of the Round.
- **Sweep Attacks** (L1821): huge weapons/creatures strike a clumped group: one
  attack roll applies to every target in the path; each defends separately.

### Ranged Combat (L1827–1937)

Resolved like close combat; **only shields normally parry ranged fire** (others
use cover or Evade). Weapon attributes:

| Attribute | Meaning |
| --- | --- |
| **Force** | ranged equivalent of Size (S/M/L/H/E) — determines shield-parry interaction |
| **Damage Modifier** | Y/N — attacker's DM adds (self-drawn bows and thrown weapons: generally Y) |
| **Range** | Close / Effective / Long (metres). At Long Range damage is **halved** and **Force −1 step**; Close Range allows Choose Location vs. stationary/unaware targets |
| **Load** | Turns to load/reload (Rapid Reload reduces) |
| **Impale Size** | the impaled weapon's actual Size for the Impale Effects Table |

**Ranged Situational Modifiers**:

| Situation | Difficulty Grade |
| --- | --- |
| Light Wind* | Hard |
| Moderate Wind* | Formidable |
| Strong Wind* | Herculean |
| Gale, Storm, or Worse* | Hopeless |
| Target is Running | Hard |
| Target is Sprinting | Formidable |
| Target obscured by mist / partial darkness | Hard |
| Target obscured by thick smoke / darkness | Formidable |
| Target completely obscured | Herculean |
| Target completely invisible | Impossible** |
| Target prone | Formidable |
| Attacker prone*** | Herculean |
| Attacker on unstable ground | Hard |

\* wind-vulnerable ammunition; supersedes normal wind penalties. \*\* unless the
target's general location is known → Herculean. \*\*\* negated by crossbow/firearm
from a prepared position.

- **Aiming** (L1881): a whole Combat Round of aiming reduces **one** Range or
  Situational Modifier by one grade (no stacking).
- **Automatic Fire** (L1885–1909):
  - **Semi-Automatic**: default; one shot per trigger pull.
  - **Burst**: **Hard** penalty; if the attack hits, roll the burst dice (e.g. 1d3
    for a 3-round burst) for rounds that strike; **separate location and damage
    per round**.
  - **Full-Automatic**: shooter fires N rounds distributed **evenly across targets
    in the arc** (spare rounds lost); **separate attack roll per target at
    Formidable**; hits per Burst. Only the **first** shot of a burst/spray may
    benefit from Special Effects.
- **Dual Firearms** (L1911): pistol + pistol = Hard; carbine/SMG + carbine/SMG =
  Formidable.
- **Firing into a Crowd** (L1915): target at melee edge = Hard; through the melee =
  Formidable. A failure that would have succeeded without the penalty hits a
  **random adjacent victim** instead (who may Evade/Parry; effects apply only to
  the intended target).
- **Fumbling with Firearms** (L1923): since there is no opposing roll, the
  **intended target** picks a fumble-specific Special Effect (e.g. Weapon
  Malfunction) for the shooter to suffer.
- **Parrying fire with shields** (L1927): Passive Blocking only. GM may instead
  give modern shields an AP value (wooden 4, Kevlar tactical 8, force field 15).
- **Telescopic Sights** (L1933): make rolls one or more grades easier by sight
  sophistication; best attainable grade is **Very Easy** (never Automatic).

### Arms and Armor (L1939–2003) — full tables

**Sample Armor** (AP by era; AP is per-item and the highest worn AP penalises
Initiative):

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

**Sample Shields**:

| Shield Type | Damage | Size | AP/HP | Locations for Passive Blocking |
| --- | --- | --- | --- | --- |
| Buckler | 1d3 | M | 6/9 | 2 |
| Heater | 1d4 | L | 6/12 | 3 |
| Kite | 1d4 | H | 4/15 | 4 |
| Scutum | 1d4 | H | 4/18 | 5 |
| Viking/Riot Shield | 1d4 | L | 4/12 | 4 |

**Sample Melee Weapons**:

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
| 1H Spear | 1d8+1 | M | 4/5 | — |
| 2H Spear | 1d10+1 | L | 4/10 | Can be set to receive a charge |
| War hammer | 1d8+1 | M | 3/8 | Two handed |
| Whip | 1d3 | M | 2/8 | Entangling |

**Sample Ranged Weapons** (Range = Close/Effective/Long metres; Load in Turns):

| Weapon | Damage | DM? | Force | Range | Load | Impale Size | AP/HP |
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

### Vehicles (L2005–2554) — mechanics and key tables

Semi-abstract vehicles: **Hull** (armor), **Structure** (HP), **Speed** (abstract
rating), **Systems** (hit on penetrating damage: chance % = penetrating damage),
**Traits**, **Shields** (ablative). Every hull-penetrating hit has a percentage
chance (equal to the penetrating damage) of affecting a System; at 0 Structure the
vehicle is destroyed/scrapped. Weapon systems supported = 1/10th of Structure
Points. Complete core tables:

**Vehicle Size / System Hits / Max Traits** (L2031–2038, L2107–2116):

| Size | System Hits | Max Traits |
| --- | --- | --- |
| Small | 1 | 1 |
| Medium | 2 | 2 |
| Large | 3 | 3 |
| Huge | 4 | 4 |
| Enormous | 5 | 5 |
| Colossal | 6 | 6 |

**Terrestrial Hulls** (L2046–2054):

| **Size** | **Structure** | **Civilian Hull Value** | **Military Hull Value** | **Example** |
| --- | --- | --- | --- | --- |
| Small | 1-20 | 1-3 | 4-6 | Motorcycle, motorcycle with sidecar, compact or subcompact car, canoe, kayak, rowboat |
| Medium | 21-40 | 1-6 | 7-9 | Sedan, panel van, pickup, luxury car, luxury sports car, muscle car, limousine, lifeboat, speedboat, hang glider, autogyro |
| Large | 41-60 | 4-9 | 10-12 | Lorry/truck, stretch limo, SUV, bus, people carrier, personal aircraft, glider, pleasure cruiser, tank, helicopter, submarine |
| Huge | 61-80 | 7-12 | 13-15 | Semi, airliner, jet, coastguard vessel, yacht |
| Enormous | 81-100 | 10-15 | 16-18 | Train, cruise liner, naval vessel |

**Starship Hulls** (L2056–2065):

| Size | Structure | Civilian Hull Value | Military Hull Value | Example |
| --- | --- | --- | --- | --- |
| Small | 10-50 | 4-9 | 10-12 | Single seat fighter |
| Medium | 51-150 | 7-12 | 13-15 | Light Freighter, Shuttle craft |
| Large | 151-450 | 10-15 | 16-18 | Heavy Freighter, Cruiser |
| Huge | 451-1350 | 13-18 | 19-21 | Escort Frigate, Light Battle Cruiser |
| Enormous | 1351-4050 | 16-21 | 21-24 | Generation Ship, Heavy Battle Cruiser |
| Colossal | 4050-12,110 | 19-24 | 24-30 | Orbital Habitat, Planet-Busting Battle Station |

**Speed Rating Table** (L2077–2089):

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

Pushing performance: +1 step on a successful Drive roll, sustainable 1d6 minutes
(Critical: 1d12 minutes); +2 steps needs a Herculean Drive roll, max 1d3 minutes
(Critical does not extend). Handling defaults: Large and below = Standard,
Huge = Formidable, Enormous = Herculean (Superior Handling trait = Easy; Walker =
capped at Gentle speed; Rails = +3 speed steps; Cargo laden = −2 steps; Enhanced
Performance = +1 step above size maximum).

**FTL Speed Rating Table** (L2162–2174) — factor time to cross 1 parsec (3.26
light-years), factor unit is GM-set (e.g. 1 Day):

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

FTL cannot activate within 1.5 AU of a star (~225 million km).

**Loss of Control Table** (terrestrial, L2260–2272):

| 1d100 | Result |
| --- | --- |
| 01-25 | Swerve. The loss of control is temporary. Vehicle drops its speed by 1 step for 5 seconds. |
| 26-40 | Skid. Driver must fight to keep the vehicle under control. Vehicle drops its speed by 2 steps for 10 seconds. |
| 41-50 | Severe Skid. Vehicle ends-up facing in the wrong direction and at a standstill for 15 seconds. |
| 51-60 | Roll. Vehicle skids and rolls, sustaining 3d10 damage to its Structure. Occupants must make successful Endurance rolls or sustain 1d10 damage to 1d3 Hit Locations. |
| 61-70 | Severe Roll. As above, but vehicle sustains 3d10+10 damage and Occupants receive 1d10 damage even if the Endurance roll succeeds, and 2d10 if it fails. |
| 71-80 | Write-Off. As above but the vehicle is reduced to 0 Structure. Occupants sustain same damage as for Severe Roll. |
| 81-90 | Explosion. As above but vehicle's fuel system ignites and explodes within 1d20+10 seconds. If unable to get clear, occupants suffer a further 1d6 burn damage to 1d6 locations |
| 91-98 | Immediate Explosion. As above but the explosion is immediate. |
| 99-00 | Catastrophic Crash. Occupants must successfully roll Endurance or be killed instantly. Damage as for Write-Off is sustained regardless. |

**System Damage Table** (L2296–2305):

| Size | System Hits | Loss per Hit |
| --- | --- | --- |
| Small | 1 | 1 Grade or 50% |
| Medium | 2 | 1 Grade or 33% |
| Large | 3 | 1 Grade or 25% |
| Huge | 4 | 1 Grade or 20% |
| Enormous | 5 | 1 Grade or 16% |
| Colossal | 6 | 1 Grade or 10% |

**System Component Damage Table** (L2309–2321):

| 1d10 | System | Damaged Result | Destroyed Result |
| --- | --- | --- | --- |
| 1 | Cargo | An amount of possessions or equipment stored in the hold, proportional to the damage, are destroyed. | All cargo is destroyed. |
| 2 | Comms | Comms rolls suffer one additional Difficulty Grade of difficulty each time they are damaged. | Vehicle can no longer communicate or spoof enemy sensors. |
| 3 | Controls | Boating, Drive, or Pilot rolls suffer one additional Difficulty Grade each time they are damaged; immediate Control roll required. | Vehicle can no longer be steered or change course. |
| 4 | Drive | Speed reduced by an amount proportional to damage. | Vehicle stopped dead; aircraft crash. |
| 5 | Crew | A number of passengers proportional to the damage are rendered casualties. Those affected suffer a Major Wound and must succeed in an Endurance roll or be killed instantly. | Vehicle occupants die. |
| 6 | Engine / Fuel | Maximum Speed is halved; electronic systems one Difficulty Grade more difficult to use. | Vehicle is destroyed in a disastrous explosion. |
| 7 | Sensors | Sensor, Navigation and Weapon attack rolls suffer one additional Difficulty Grade each time they are damaged. | Vehicle is rendered blind. |
| 8 | Weapons | A number of weapon systems proportional to the damage become inoperative. | Vehicle can no longer fire weapons. |
| 9-0 | None | No systems struck; just Structure damage. | N/A |

**Weapons by Size Table** (L2325–2334):

| Size | Terrestrial | Spacecraft |
| --- | --- | --- |
| Small | Light Weapon (e.g. Assault Rifle) | Medium Weapon (e.g. Machine Gun, Rocket Launcher) |
| Medium | Medium Weapon (e.g. Machine Gun, Rocket Launcher) | Turreted/Heavy Weapon (e.g. Heavy Linked Machine Guns, Mini-Guns) |
| Large | Turreted/Heavy Weapon (e.g. Heavy Linked Machine Guns, Mini-Guns) | Weapon Battery (e.g. Battlefield Artillery) |
| Huge | Weapon Battery (e.g. Battlefield Artillery) | Advanced Weapons (e.g. Missile Systems) |
| Enormous | Advanced Weapons (e.g. Missile Systems) | Spinal Mounted Weapons (one only) |
| Colossal | NA | Spinal Mounted Weapons (up to 3) |

**Sample Vehicle Weapon Statistics** (L2336–2354):

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

Ranges in metres/km; Firing Rate = single/semi-auto/full-auto; Ammo = shots before
reload; Load = reload time in Combat Rounds with a full crew. **Deflector
shields** (L2274–2280): damage ≤ shield value fully blocked; excess over the value
reduces shield strength by that amount; at 0 the shield collapses and remaining
damage carries to the hull. **Handling and Manoeuvres** (L2248–2256): evasive
manoeuvres need a Drive/Pilot roll whose base grade is the vehicle's Handling,
further graded by the manoeuvre's ambition; failure → Loss of Control roll.

13 sample vehicles (Interceptor … General Contact Unit, L2356–2554) — full stat
blocks in `tables.md` §6.12.

[AMBIGUOUS: sample-vehicle "Systems" counts don't always match the Size/System
Hits table (e.g. Medium Interceptor lists Systems 3 vs. 2 System Hits); some listed
traits ("Unobtrusive", "Watercraft", "Waterborne") are not in the Traits list; the
Yacht trait line is garbled in the source. Combat › Vehicles.]

---

## 6. Magic and Powers (Magic and Powers, L2556–2946)

### How Powers Work (L2560–2574)
Energy pool = **POW** (magicians: **Magic Points**; superheroes: **Power Points**).
Magic Points recover at **Healing Rate per hour** with full rest (GM may slow this
if fatigued/injured), up to POW. Superhero Power Points recover **1 per full minute
of rest**; or spend **1 Luck Point → regain 1d4+1** immediately.

**Casting spells** (L2568): requires the **Magic skill** (without it, no magic);
**1 Action + successful Magic roll**; failure = spell fizzles (try later).
**Superpowers** (L2572): always active, no roll or PP cost — unless **Boosting**
(PP cost).

### Magic (casting results, L2602–2613)

| Magic roll | Magic Point cost | Effect |
| --- | --- | --- |
| Critical Success | **0** | spell works |
| Success | **1** | spell works |
| Failure | **1** | spell does not work |
| Fumble | **1d3** | spell fails |

All spells have default **Intensity and Magnitude of 1**. No limit on spells known
(L2615). Casting takes a single turn.

### Beginning Powers (L2576–2600)
- **Magicians**: begin with **1d4+1 spells**; new spell = **3 Experience Rolls +
  one week** of study with a teacher/grimoire.
- **Superheroes**: core powers = table of **POW + (CON for altered physiology or
  INT for technology/skill-based powers)**: ≤12 → 1; 13–20 → 2; 21–30 → 3;
  31–40 → 4; 40+ → 5. A core power trades for **2 Boosts**; up to **two Limits**
  (one → +1 Boost; two → +1 core power). [AMBIGUOUS: "31-40" and "40+" overlap at
  exactly 40. Magic and Powers › Beginning Powers.]

### Spells (L2619–2793)
**Duration**: entire scene/action unless Concentration, Instant, or custom.
**Magic Traits**: **Concentration** (sustained; distraction ends it), **Instant**,
**Ranged** (up to caster's "Magic score" in metres [AMBIGUOUS: "Magic score" —
presumably the Magic skill value; the attribute is Magic Points/POW. Spells › Magic
Traits]; unseen target: one grade harder), **Resist (Endurance/Evade/Willpower)**
(opposed vs. the casting result; Evade costs the target 1 AP), **Touch**.

24 spells (full effects in `tables.md` §7.2): **Alarm** (area/object trigger),
**Avert** (dismiss spell; Counter Magic reaction), **Befuddle** (no constructive
action; attack breaks it), **Bladesharp** (damage dice +1 step, edged/piercing;
1d4→1d6→1d8→1d10→2d6→2d8→2d10), **Bludgeon** (as Bladesharp, blunt), **Breath**
(hold breath ½ caster's POW in minutes), **Calm**, **Chill** (hand-sized objects to
ice-water temperature), **Darkness** (POW square metres of dim glow), **Disruption**
(1d3 damage, ignores armor), **Extinguish**, **Find (X)** (6 listed variants;
blocked by metal or ≥1 m earth/stone), **Firearrow** (+1d3; ignition chance =
"Folk Magic" skill % [AMBIGUOUS: "Folk Magic" is not a skill defined in this
document — presumably the Magic skill. Spell Descriptions › Firearrow]),
**Fireblade** (+1d3 melee), **Glue** (Brawn 5× caster's POW to break), **Heal**
(minor ailments cured; Minor Wound: full HP restore; Serious/Major: stabilise
only), **Ignite**, **Knock** (one mundane lock), **Light** (counters Darkness),
**Lock** (1 MP held until dismissed), **Phantasm**, **Sleep** (half caster's POW in
hours; 1d3 Rounds to work; no effect on SIZ > caster's POW; auto-fails in combat),
**Vigour** (ignore labour Fatigue), **Witchsight**.

### Superpowers (L2795–2946)
17 core powers, each with a **Boost** (PP cost) — Animation, Combat Expert,
Durability, Elemental Control, Energy Blast, Enhanced Reactions, Enhanced Speed,
Enhanced Strength, Flight, Heightened Sense, Inherent Armor, Life Support, Lucky,
Regeneration, Savant, Teleport, Vaporous Form (full mechanics in `tables.md`
§7.4). Embedded numeric tables, complete (L2805–2916):

**Super Powers Table** (core powers from POW + CON or INT — see above, L2592–2600):

| POW+ Characteristic | Core Powers |
| --- | --- |
| 12 or less | 1 |
| 13-20 | 2 |
| 21-30 | 3 |
| 31-40 | 4 |
| 40+ | 5 |

**Animation — construct Armor Values / Power Point Cost** (L2809–2814):

| Armor Values | Power Point Cost |
| --- | --- |
| Plastic or Soft Wood | 2 |
| Concrete or Hard Wood | 4 |
| Soft Metal or Stone | 6 |
| Hard Metal | 8 |

Constructs: 2 m³ of non-living material within POW metres; base armor = the
material's value; STR 2d6+16 (23), SIZ 1d6+12 (15), DEX 1d6+8 (11); HP as normal;
caster directs (their skills); Serious Wound to chest/abdomen: opposed Endurance
vs. the attack roll or the construct collapses.

**Elemental Control — damage and force/size by POW** (L2834):

| POW | Damage | Force/Size |
| --- | --- | --- |
| 8 or less | 1d4 | T |
| 9-12 | 1d6 | S |
| 13-15 | 1d8 | M |
| 16-18 | 1d10 | M |
| 19+ | 1d12 | L |

(Melee reach Very Long; ranged 8/24/100; two Combat Effects chosen at creation;
control volume ½POW m³ within POW metres.)

**Energy Blast — damage and force by POW** (L2840):

| POW | Damage | Force |
| --- | --- | --- |
| 8 or less | 1d6 | S |
| 9-12 | 1d8 | M |
| 13-15 | 1d10 | M |
| 16-18 | 1d12 | L |
| 19+ | 2d6 | L |

(Range 10/30/120; two Combat Effects; hit via Combat Style or Unarmed.)

**Enhanced Speed — movement multiplier by DEX+POW** (L2854):

| DEX+POW | Multiplier |
| --- | --- |
| 12 or less | x2 |
| 13-24 | x3 |
| 25-36 | x4 |
| 37+ | x5 |

(Max out-of-combat speed = ten times modified movement in km/h.)

**Enhanced Strength — maximum lift by STR+POW** (L2860):

| STR+POW | Max lift |
| --- | --- |
| 12 or less | 1 tonne |
| 13-24 | 2 tonnes |
| 25-36 | 3 tonnes |
| 37+ | 5 tonnes |

(Damage bonus from STR+SIZ+POW; unarmed attacks count as Large; melee/thrown
weapons' size or force +1.)

Key remaining formulas: Durability HP = **CON+SIZ+POW**, Healing Rate = **CON +
½POW (round up)**; Inherent Armor = **⅓CON (round up)** (may shift up to 2 points
between locations); Lucky = Luck from **CHA + ½POW (round up)**; Flight = POW m;
Teleport = POW m (Boost: ½POW m as free action); Vaporous Form fly = ⅓POW m.

**Limits** (take up to two): **Activation Cost** (2 PP to activate / +2 PP per
boost / 1 PP per turn to maintain), **External Power Source**, **Fatal Flaw** (4
hours without a substance: 1 Fatigue; Endurance to stave off, +1 grade per extra 4
hours), **Limited Control** (fumble range 90–00), **Limited Power**, **Physical
Limitation**, **Vulnerability** (double damage from one attack form; or Hard
Endurance per turn within 5 m of a substance).

**What this document does NOT contain**: no Folk Magic, Psychic Powers, Theism,
Animism, Sorcery or cult/religion subsystem — only the generic **Magic skill +
24 "petty" spells** (Intensity/Magnitude 1 by default) and the **superpower** rules.
There are no cults, religious hierarchies, or divine magic mechanics (the Priest
career and Lore (Religion) exist, but nothing mechanical attaches to them).

---

## 7. Creatures (Creatures, L2948–3409)

10 sample creatures with full stat lines (SIZ, Action Points, Damage Modifier,
Magic Points, Initiative Bonus, Move), skills, Combat Style, natural weapons
(Size + damage incl. DM), and **per-creature 1d20 Hit Location tables with
AP/HP** — all transcribed verbatim in `tables.md` §8: Giant Ant, Bear (Grizzly),
Crocodile/Alligator, Dragon, Goblin/Orc, Horse, Minotaur, Shark, Skeleton,
Xenomorphic Alien. Highlights:

- **Dragon**: SIZ 50, **4 AP**, +2d8 DM, 19 MP, +19 Initiative, 12 m/18 m flying;
  Flame Breath 4d6 (cone CON m long, ¼CON m wide; ignites, melts un-enchanted
  metal); Magic 78% with 8 spells (5 of which are undefined in this document).
- **Skeleton**: blunt weapons +1 damage dice step; slashing −1 step; thrusting
  −2 steps. Undead trait (immune to Serious Wound effects; bound location = head
  or chest).
- **Xenomorphic Alien**: acidic blood — when injured, 1d3 damage for 1d3 rounds to
  a random location on a hand-to-hand attacker, eating armor first.

**Creature Abilities** (~35; full text L3257–3409, key numbers in `tables.md`
§8.5): Adhering, Aquatic, Blood Sense (1d6+6 km), Breathe Flame (cone CON m; once
per hour/day, extras cost Endurance or Fatigue), Burrower, Camouflaged (Perception
two grades harder), Characteristic Drain, Cold-Blooded (below 15 °C: −6 Initiative
and −1 Combat Action; below 5 °C: torpid), Dark Sight, Death Sense (½ INT km),
Disease Immunity, Diving Strike (charge-equivalent: Size and DM +1 step), Earth
Sense, Echolocation (Stealth two grades harder), Engulfing (≤ half own SIZ),
Flying (Fly skill STR+DEX), Formidable Natural Weapons (may parry with natural
weapons), Frenzy (CON rounds; immune to Serious Wound effects; then Exhausted),
Gaze Attack, Grappler, Hold Breath (CON minutes), Immunity, Intimidate, Leaper,
Life Sense, Magic Sense, Multi-Headed (extra Combat Action per head), Multi-Limbed,
Night Sight, Poison Immunity, Regeneration, Swimmer, Terrifying (Willpower; fail =
flee, fumble = collapse), Trample (≤ half own SIZ; 2× Damage Modifier; Size +1
step), Undead, Vampiric, Venomous, Wing Buffet (damage = damage bonus, 3 m).
Rule of thumb: **up to three Abilities per creature** (more for exceptional ones).

---

## 8. GM Guidance and Cross-Cutting Rules

- **Difficulty grades** (§3 above) are the universal adjudication tool; outside
  combat the GM assigns them freely ("the hardest Difficulty Grade takes
  precedence"; skill caps replace grades in some combat situations).
- **Luck Points** are the only meta-currency (replenish per session; GM may award
  impromptu). There are no Hero Points.
- **Experience**: no per-roll experience checks; GM-awarded **Experience Rolls**
  (1–3 per session) spent on skills (1d100+INT vs. skill), Passions, new skills
  (1 month + 3 rolls), new spells (1 week + 3 rolls). Fumbled skills earn a free
  +1%.
- **Time**: 5-second Combat Rounds; Skill Time; Narrative Time. No travel or
  exploration subsystem (only the Movement rates and Survival/Navigation skills).
- **Money/equipment**: narrative only ("pocket money"); **no price lists**.
- **Encumbrance**: **no rules** (armor's only weight effects: Initiative penalty
  from highest AP, climbing hindrance, and Difficulty Grades like "fighting while
  prone").
- **Cults/religions**: **none** (Priest is just a career).
- **Aging**: **none** (age categories size starting experience only).
- **Rounding**: always up on fractions (Critical ranges, half damage is left to
  the same rule where not otherwise specified).
- **Setting scope**: the rules are deliberately generic — historical, fantasy,
  pulp, and sci-fi all appear (firearms, vehicles, starships, superpowers); the GM
  decides which subsystems are in play.

---

## 9. Engine-Relevant Inconsistencies (do not "fix" silently)

These are places where the source contradicts itself or omits a definition. Per
house convention, implement a documented default only at the engine layer with an
`AMBIGUOUS`-style citation; this list is the audit trail:

1. **Damage Modifier progression >120** — "Continue Progression" undefined.
   (Step 3 › Damage Modifier.)
2. **Initiative average rounding** — implied ceil by the global round-up rule.
   (Step 3 › Initiative; Introduction › Rounding.)
3. **"Impossible" difficulty grade** — used once (invisible ranged target), never
   defined; footnote says Herculean if location known. (Combat › Ranged Combat.)
4. **Walk/Run/Sprint rates** — referenced but never defined. (Combat › Movement.)
5. **"1d3 Turns" stun on Serious Wounds** — "Turn" is a per-character action slot
   in the Combat chapter; context implies Rounds. (Game System › Serious Wound.)
6. **"Folk Magic skill"** in Firearrow — the skill is "Magic" here. (Spells.)
7. **"Magic score"** for spell range — skill or POW not specified. (Magic Traits.)
8. **Career skill slips** — "Linguistics", "Customs (any)", "Culture" listed as
   Standard, Magician row unqualified. (Step 6 › Careers.)
9. **Super Powers Table band overlap at 40** ("31-40: 4 / 40+: 5"). (Beginning
   Powers.)
10. **Shark hit-location ranges overlap** (17–18 both Left Fin and Head).
    (Creatures › Shark.)
11. **Dragon Trample "Size: C"** — not a defined Size category. (Creatures ›
    Dragon.)
12. **Dragon's 5 undefined spells** (Cleanse, Coordination, Find Treasure,
    Mindspeech, Spiritshield). (Creatures › Dragon.)
13. **Vehicle stat-block drift** — Systems counts vs. Size table; undefined traits
    ("Unobtrusive", "Watercraft", "Waterborne"); garbled Yacht line. (Vehicles.)
14. **Starship Military Hull ranges overlap** at band boundaries. (Vehicles.)
15. **Points Build bounds** — min/max of dice ranges implied, not stated.
    (Step 2 › Calculating Characteristics.)
16. **Experience Table vs. 150-point Bonus pool** — replacement implied, not
    stated. (Step 7.)
17. **Life Support Boost PP cost** not given. (Superpowers › Life Support.)
18. **"Any weapon for the character's Combat Style"** — starting equipment grants
    free weapons with no economy behind it. (Step 9.)
