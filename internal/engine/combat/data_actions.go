package combat

import "strings"

// ActionType classifies a Combat Action as Proactive, Reactive or Free
// (Mythras-Imperative.md, Combat › Combat Actions L1382-1518).
type ActionType string

const (
	// Proactive actions are attempted only on the actor's own Turn
	// (Combat › Actions in Combat).
	Proactive ActionType = "proactive"
	// Reactive actions answer an imminent threat, any time in the Round,
	// one reaction attempt per threat (Combat › Actions in Combat).
	Reactive ActionType = "reactive"
	// Free actions cost no Action Points (Combat › Free Actions).
	Free ActionType = "free"
)

// Action is one row of the Combat Actions and Costs table: its name, class,
// Action Point cost, the rule that governs it, and the source citation.
type Action struct {
	// Name is the printed action name.
	Name string `json:"name"`
	// Type is Proactive, Reactive or Free.
	Type ActionType `json:"type"`
	// AP is the Action Point cost. "Unless otherwise specified any Combat
	// Action (save for 'Free' Actions) costs one Action Point" (L1387);
	// the two exceptions printed in the table are carried in Mechanics.
	AP int `json:"ap"`
	// Mechanics summarises the action's rule in one line.
	Mechanics string `json:"mechanics"`
	// Citation names the rulebook heading (and digest/tables sections).
	Citation string `json:"citation"`
}

// citeActions is the shared citation for the Combat Actions table.
const citeActions = "Mythras-Imperative.md, Combat › Combat Actions L1382-1518; docs/rules/digest.md L818-852; tables.md §4.2"

// combatActions transcribes the complete Combat Actions and Costs table
// (tables.md §4.2; digest L818-852): 19 Proactive, 4 Reactive and 6 Free
// actions. Every action except the Free column costs 1 AP; the "2 Actions"
// cost of retrieving a dropped object is a stated variant of Ready Weapon
// and is carried in its Mechanics line rather than invented as an action
// the table does not print.
var combatActions = []Action{
	{Name: "Attack", Type: Proactive, AP: 1, Mechanics: "Strike in melee or with a ranged weapon; resolved by ResolveAttack.", Citation: citeActions},
	{Name: "Brace", Type: Proactive, AP: 1, Mechanics: "SIZ treated as 50% bigger vs Knockback and Leaping Attacks; doubled vs Bash (records the Braced condition).", Citation: citeActions},
	{Name: "Cast Magic/Use Power", Type: Proactive, AP: 1, Mechanics: "Cast a spell or invoke a power; complex magics take several Turns; release before the next Turn or Hold.", Citation: citeActions},
	{Name: "Change Range", Type: Proactive, AP: 1, Mechanics: "Close with or retreat from an opponent; not while engaged or pinned by fire.", Citation: citeActions},
	{Name: "Charge", Type: Proactive, AP: 1, Mechanics: "Run/sprint into engagement; the attack is Hard, weapon Size +1 step, Damage Modifier +1 step (+2 quadruped) — pass AttackParams.Charging.", Citation: citeActions},
	{Name: "Delay", Type: Proactive, AP: 1, Mechanics: "Reserve actions for later Reactions/Interrupts; if unused by the next Turn they are lost (Dither).", Citation: citeActions},
	{Name: "Dither", Type: Proactive, AP: 1, Mechanics: "Do nothing — the default action.", Citation: citeActions},
	{Name: "Hold Magic/Power", Type: Proactive, AP: 1, Mechanics: "Hold a cast spell each Turn; lets enemies Counter Spell.", Citation: citeActions},
	{Name: "Mount", Type: Proactive, AP: 1, Mechanics: "Mount or dismount; large mounts take several Turns.", Citation: citeActions},
	{Name: "Move", Type: Proactive, AP: 1, Mechanics: "Move the Base Movement Rate if unengaged (prone: crawl, max 2 metres per Turn).", Citation: citeActions},
	{Name: "Outmanoeuvre", Type: Proactive, AP: 1, Mechanics: "Group opposed Evade (all participants spend 1 AP); those who fail cannot attack you this Round.", Citation: citeActions},
	{Name: "Ready Weapon", Type: Proactive, AP: 1, Mechanics: "Draw, sheathe, retrieve or reload; retrieving a dropped object takes 2 Actions (move down, then back up).", Citation: citeActions},
	{Name: "Reload", Type: Proactive, AP: 1, Mechanics: "Reload a firearm, as Ready Weapon; time varies by weapon (Rapid Reload reduces).", Citation: citeActions},
	{Name: "Regain Footing", Type: Proactive, AP: 1, Mechanics: "Stand when unengaged (automatic); if engaged, win an Opposed Brawn/Athletics — the contest itself costs no AP.", Citation: citeActions},
	{Name: "Sneak Peek", Type: Proactive, AP: 1, Mechanics: "One Perception look from cover; a Fumble exposes the shooter.", Citation: citeActions},
	{Name: "Steady Weapon", Type: Proactive, AP: 1, Mechanics: "Firearm posture: reduces the Burst/Full-Auto recoil penalty one step until the character moves.", Citation: citeActions},
	{Name: "Struggle", Type: Proactive, AP: 1, Mechanics: "Escape grips, entanglements and similar effects.", Citation: citeActions},
	{Name: "Switch Fire Mode", Type: Proactive, AP: 1, Mechanics: "Change fire mode or ammunition; Free for sufficiently advanced firearms.", Citation: citeActions},
	{Name: "Take Cover", Type: Proactive, AP: 1, Mechanics: "Duck behind cover (not prone); cover quality per the Inanimate Objects table.", Citation: citeActions},
	{Name: "Counter Spell", Type: Reactive, AP: 1, Mechanics: "Negate an incoming spell outright; needs a 1-Turn casting time or a pre-Held spell.", Citation: citeActions},
	{Name: "Evade", Type: Reactive, AP: 1, Mechanics: "Dive clear with the Evade skill (opposed); leaves the character prone unless mitigated — ReactionDodge in ResolveAttack.", Citation: citeActions},
	{Name: "Interrupt", Type: Reactive, AP: 1, Mechanics: "Delaying characters only: cut into an opponent's Turn or strike those passing within reach.", Citation: citeActions},
	{Name: "Parry", Type: Reactive, AP: 1, Mechanics: "Defend with a Combat Style — ReactionParry in ResolveAttack.", Citation: citeActions},
	{Name: "Assess Situation", Type: Free, AP: 0, Mechanics: "Perception if unengaged; Running = Hard, Sprinting = Formidable.", Citation: citeActions},
	{Name: "Drop Item", Type: Free, AP: 0, Mechanics: "Drop a carried item.", Citation: citeActions},
	{Name: "Signal", Type: Free, AP: 0, Mechanics: "Gestures or signals, if unengaged and perceivable.", Citation: citeActions},
	{Name: "Speak", Type: Free, AP: 0, Mechanics: "A phrase of five seconds or less.", Citation: citeActions},
	{Name: "Use Luck Point", Type: Free, AP: 0, Mechanics: "Spend one Luck Point (Cheat Fate, Desperate Effort or Mitigate Damage).", Citation: citeActions},
	{Name: "Ward Location", Type: Free, AP: 0, Mechanics: "Dedicate a weapon to passively block one location (shields cover more); ends if the weapon attacks or parries.", Citation: citeActions},
}

// Actions returns a copy of every Combat Action row, in table order.
func Actions() []Action {
	out := make([]Action, len(combatActions))
	copy(out, combatActions)
	return out
}

// ActionByName returns the Combat Action with the given name, matched
// case-insensitively and ignoring surrounding space. The second result is
// false for names the table does not print (callers then wrap
// ErrUnknownAction).
func ActionByName(name string) (Action, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, a := range combatActions {
		if strings.ToLower(a.Name) == want {
			return a, true
		}
	}
	return Action{}, false
}
