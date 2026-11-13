package world

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidRarity reports a rarity outside the four canonical grades; it
// is returned by [World.CanAcquire] so harness tools can translate it into
// a structured IMPOSSIBLE result for the GM.
var ErrInvalidRarity = errors.New("world: invalid rarity")

// Rarity grades accepted by [World.CanAcquire], from everyday to unique.
const (
	// RarityCommon is an everyday good (rations, a knife, an ordinary
	// horse).
	RarityCommon = "common"
	// RarityUncommon is a specialist good (a metal sword, fine armour, a
	// warhorse).
	RarityUncommon = "uncommon"
	// RarityRare is a hard-to-come-by good (an expert weapon, a rare
	// scroll, foreign steel in quantity).
	RarityRare = "rare"
	// RarityLegendary is a unique or near-unique good (a named blade, a
	// magus's focus).
	RarityLegendary = "legendary"
)

// Decision is the outcome of [World.CanAcquire]: whether the item may be
// obtained, the reasoning in plain words for the GM to narrate, and the
// flags that actually governed the outcome (in evaluation order).
type Decision struct {
	// Allowed is the verdict.
	Allowed bool
	// Reasons explains every contributing flag and ends with the verdict.
	Reasons []string
	// GoverningFlags lists the flags that contributed strain, in the
	// fixed evaluation order of the rule table (never map order).
	GoverningFlags []string
}

// rarityLimits is the availability rule table: acquisition is allowed
// exactly when total strain ≤ the rarity's limit (see [World.CanAcquire]).
var rarityLimits = map[string]int{
	RarityCommon:    3,
	RarityUncommon:  2,
	RarityRare:      1,
	RarityLegendary: 0,
}

// CanAcquire reports whether item of the given rarity can be obtained in
// the given region under the world's current conditions, and why. The
// answer is a pure function of the flag state and rarity — no rolls, no
// wall clock, no map-iteration order — so the same world always answers
// the same way (golden rule 1: determinism is sacred). The item string is
// only consulted for material-scarcity matching and for messages; money
// plays no part, because Mythras Imperative has price lists nowhere (digest
// §8: "Money/equipment: narrative only").
//
// Rarity is trimmed and lower-cased and must be one of [RarityCommon],
// [RarityUncommon], [RarityRare], [RarityLegendary] (otherwise an error
// wrapping [ErrInvalidRarity]). Region is trimmed and lower-cased for flag
// lookup; the empty region skips regional checks.
//
// The rule table — strain contributions, evaluated in this fixed order:
//
//  1. Flag [FlagEconomy]: [EconomyStrained] +1, [EconomyCollapsed] +2
//     ([EconomyStable] +0).
//  2. Flag [FlagWar]: [WarCold] +1, [WarActive] +2 ([WarPeace] +0).
//  3. Flag "region:<region>:blockade" active (value other than "", "false",
//     "0", "no", "off", "clear", "none"): +2. Skipped when region is empty.
//  4. Each active "<material>_scarce" flag whose material appears in the
//     lower-cased item string: +1, evaluated in sorted flag-key order so
//     the result never depends on map iteration.
//
// Unrecognised economy or war values contribute nothing but are called out
// in Reasons rather than being silently ignored. Rarity limits: common ≤ 3,
// uncommon ≤ 2, rare ≤ 1, legendary ≤ 0 — so strain 0 allows everything;
// strain 1 refuses legendary only; strain 2 refuses rare and above; strain
// 3 allows only common; strain 4+ refuses everything.
//
// Errors: [ErrInvalidRarity], an empty item, or an invalid region (see
// [RegionFlagKey]). Nothing is mutated.
//
// shortcut: material matching is a case-insensitive substring test of the
// item name ("iron" matches "ironing board"); tighten to word boundaries
// if false positives ever matter.
func (w *World) CanAcquire(item, rarity, region string) (Decision, error) {
	r := strings.ToLower(strings.TrimSpace(rarity))
	limit, ok := rarityLimits[r]
	if !ok {
		return Decision{}, fmt.Errorf("%w %q (valid: %s, %s, %s, %s)",
			ErrInvalidRarity, rarity, RarityCommon, RarityUncommon, RarityRare, RarityLegendary)
	}
	item = strings.TrimSpace(item)
	if item == "" {
		return Decision{}, errors.New("world: can acquire: item must not be empty")
	}
	region = strings.TrimSpace(region)

	strain := 0
	var reasons, governing []string

	// Rule 1: economy.
	if f, ok := w.Lookup(FlagEconomy); ok {
		switch strings.ToLower(strings.TrimSpace(f.Value)) {
		case EconomyStrained:
			strain++
			reasons = append(reasons, "the economy is strained")
			governing = append(governing, FlagEconomy)
		case EconomyCollapsed:
			strain += 2
			reasons = append(reasons, "the economy has collapsed")
			governing = append(governing, FlagEconomy)
		case EconomyStable:
			// no strain
		default:
			reasons = append(reasons,
				fmt.Sprintf("economy flag value %q is not a recognised state; treated as neutral", f.Value))
		}
	}

	// Rule 2: war.
	if f, ok := w.Lookup(FlagWar); ok {
		switch strings.ToLower(strings.TrimSpace(f.Value)) {
		case WarCold:
			strain++
			reasons = append(reasons, "the world is on a cold-war footing")
			governing = append(governing, FlagWar)
		case WarActive:
			strain += 2
			reasons = append(reasons, "the world is at war")
			governing = append(governing, FlagWar)
		case WarPeace:
			// no strain
		default:
			reasons = append(reasons,
				fmt.Sprintf("war flag value %q is not a recognised state; treated as neutral", f.Value))
		}
	}

	// Rule 3: regional blockade.
	if region != "" {
		key, err := RegionFlagKey(region, AspectBlockade)
		if err != nil {
			return Decision{}, err
		}
		if f, ok := w.Lookup(key); ok && flagActive(f.Value) {
			strain += 2
			reasons = append(reasons, fmt.Sprintf("region %q is blockaded", strings.ToLower(region)))
			governing = append(governing, key)
		}
	}

	// Rule 4: material scarcity. Keys are sorted so evaluation order is
	// deterministic regardless of map iteration.
	var scarceKeys []string
	for k := range w.Flags {
		if strings.HasSuffix(strings.ToLower(k), scarceFlagSuffix) {
			scarceKeys = append(scarceKeys, k)
		}
	}
	slices.Sort(scarceKeys)
	lowerItem := strings.ToLower(item)
	for _, k := range scarceKeys {
		f := w.Flags[k]
		if !flagActive(f.Value) {
			continue
		}
		material := strings.TrimSuffix(strings.ToLower(k), scarceFlagSuffix)
		if material == "" || !strings.Contains(lowerItem, material) {
			continue
		}
		strain++
		reasons = append(reasons, material+" is scarce")
		governing = append(governing, k)
	}

	// Verdict.
	allowed := strain <= limit
	if allowed {
		reasons = append(reasons,
			fmt.Sprintf("%s goods are available (strain %d, limit %d)", r, strain, limit))
	} else {
		reasons = append(reasons,
			fmt.Sprintf("%s goods are unavailable: strain %d exceeds the limit of %d", r, strain, limit))
	}
	return Decision{Allowed: allowed, Reasons: reasons, GoverningFlags: governing}, nil
}
