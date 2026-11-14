package combat

import "strings"

// SpecialEffect is one row of the Special Effects Table: its printed name,
// the side that may pick it (Offensive / Defensive), the printed Special
// Weapon Type and Specific Roll gates, and whether it may be stacked with
// itself inside one exchange.
type SpecialEffect struct {
	// Name is the printed Special Effect name.
	Name string `json:"name"`
	// Offensive reports the X in the Offensive column.
	Offensive bool `json:"offensive"`
	// Defensive reports the X in the Defensive column.
	Defensive bool `json:"defensive"`
	// WeaponType is the printed Special Weapon Type column ("" when the
	// cell is empty).
	WeaponType string `json:"weaponType,omitempty"`
	// SpecificRoll is the printed Specific Roll column ("" when empty).
	SpecificRoll string `json:"specificRoll,omitempty"`
	// Stackable reports the X in the Stackable column.
	Stackable bool `json:"stackable"`
	// Citation names the rulebook heading for the row.
	Citation string `json:"citation"`
}

// citeEffects is the shared citation for the Special Effects Table and its
// descriptions.
const citeEffects = "Mythras-Imperative.md, Combat › Special Effects (table L1616-1651, descriptions L1653-1730); docs/rules/digest.md L916-1036; docs/rules/tables.md §4.6"

// specialEffects transcribes the complete 32-row Special Effects Table
// (tables.md §4.6; Mythras-Imperative.md L1616-1651), in printed order.
//
// The Special Weapon Type and Specific Roll columns are the printed gates
// computeAvailable enforces. Note the sample weapon tables print no damage
// type column, so weapon-type gates that read "Cutting"/"Bludgeoning"
// weapons consult WeaponStats.DamageType — a field the GM or a setting
// supplies (char.WeaponStats): with no type recorded the gate passes and
// the GM can refuse the pick. [AMBIGUOUS: the source never says which
// sample weapons count as cutting or bludgeoning — Combat › Arms and
// Armor vs. Combat › Special Effects.]
var specialEffects = []SpecialEffect{
	{Name: "Accidental Injury", Defensive: true, SpecificRoll: "Attacker Fumbles", Citation: citeEffects},
	{Name: "Arise", Defensive: true, Citation: citeEffects},
	{Name: "Bash", Offensive: true, WeaponType: "Shields or Bludgeoning", Citation: citeEffects},
	{Name: "Bleed", Offensive: true, WeaponType: "Cutting Weapons", SpecificRoll: "Firearms: Attacker Criticals", Citation: citeEffects},
	{Name: "Blind Opponent", Defensive: true, SpecificRoll: "Defender Criticals", Citation: citeEffects},
	{Name: "Bypass Armor", Offensive: true, SpecificRoll: "Attacker Criticals", Stackable: true, Citation: citeEffects},
	{Name: "Choose Location", Offensive: true, SpecificRoll: "See Description", Citation: citeEffects},
	{Name: "Circumvent Cover", Offensive: true, WeaponType: "High Tech Firearms Only", Citation: citeEffects},
	{Name: "Circumvent Parry", Offensive: true, SpecificRoll: "Attacker Criticals", Citation: citeEffects},
	{Name: "Damage Weapon", Offensive: true, Defensive: true, Citation: citeEffects},
	{Name: "Disarm Opponent", Offensive: true, Defensive: true, Citation: citeEffects},
	{Name: "Drop Foe", Offensive: true, WeaponType: "Firearms Only", Citation: citeEffects},
	{Name: "Duck Back", Offensive: true, WeaponType: "Firearms Only", Citation: citeEffects},
	{Name: "Enhance Parry", Defensive: true, SpecificRoll: "Defender Criticals", Citation: citeEffects},
	{Name: "Entangle", Offensive: true, Defensive: true, WeaponType: "Entangling Weapons", Citation: citeEffects},
	{Name: "Force Failure", Offensive: true, Defensive: true, SpecificRoll: "Opponent Fumbles", Citation: citeEffects},
	{Name: "Grip", Offensive: true, WeaponType: "Unarmed", Citation: citeEffects},
	{Name: "Impale", Offensive: true, WeaponType: "Impaling Weapons", Citation: citeEffects},
	{Name: "Marksman", Offensive: true, WeaponType: "Ranged Weapons", Citation: citeEffects},
	{Name: "Maximize Damage", Offensive: true, SpecificRoll: "Attacker Criticals", Stackable: true, Citation: citeEffects},
	{Name: "Overpenetration", Offensive: true, WeaponType: "Firearms Only", SpecificRoll: "Attacker Criticals", Citation: citeEffects},
	{Name: "Pin Down", Offensive: true, WeaponType: "Firearms Only", Stackable: true, Citation: citeEffects},
	{Name: "Prepare Counter", Defensive: true, Stackable: true, Citation: citeEffects},
	{Name: "Rapid Reload", Offensive: true, Stackable: true, Citation: citeEffects},
	{Name: "Scar Foe", Offensive: true, Defensive: true, Citation: citeEffects},
	{Name: "Select Target", Defensive: true, SpecificRoll: "Attacker Fumbles", Citation: citeEffects},
	{Name: "Slip Free", Defensive: true, SpecificRoll: "Defender Criticals", Citation: citeEffects},
	{Name: "Stun Location", Offensive: true, WeaponType: "Bludgeoning Weapons", Citation: citeEffects},
	{Name: "Sunder", Offensive: true, WeaponType: "Two Handed Weapons", Citation: citeEffects},
	{Name: "Trip Opponent", Offensive: true, Defensive: true, Citation: citeEffects},
	{Name: "Weapon Malfunction", Defensive: true, WeaponType: "Firearms Only", SpecificRoll: "Attacker Fumbles", Citation: citeEffects},
	{Name: "Withdraw", Defensive: true, Citation: citeEffects},
}

// SpecialEffects returns a copy of every Special Effects Table row, in
// printed order.
func SpecialEffects() []SpecialEffect {
	out := make([]SpecialEffect, len(specialEffects))
	copy(out, specialEffects)
	return out
}

// EffectByName returns the Special Effects Table row with the given name,
// matched case-insensitively and ignoring surrounding space. The second
// result is false for names the table does not print (callers then wrap
// [ErrUnknownEffect]).
func EffectByName(name string) (SpecialEffect, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, fx := range specialEffects {
		if strings.ToLower(fx.Name) == want {
			return fx, true
		}
	}
	return SpecialEffect{}, false
}
