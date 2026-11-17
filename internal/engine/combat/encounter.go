// Package combat implements Mythras Imperative combat: the encounter state
// machine (Combat Rounds, initiative, Action Points), the complete Combat
// Action table, the atomic attack-resolution pipeline (attack roll, parry or
// dodge reaction, hit location, damage, armour, wound severity and the
// Endurance-opposed wound consequences), the Special Effects matrix, the
// combat situations the rulebook quantifies (charging, knockback, surprise),
// manual damage application and healing.
//
// Determinism contract: every roll flows through an explicit *dice.Rand
// passed by the caller; every operation returns audit-rich results carrying
// each dice.Roll, grade and rule citation, so the session log can replay the
// fight. State mutates only through these operations.
//
// Source of truth: Mythras-Imperative.md (Combat; Game System › Hit Locations
// and Wounds, Healing from Injury), mirrored in docs/rules/digest.md §4–§5
// and docs/rules/tables.md §3–§4. Where the source is ambiguous the choice
// made here carries an [AMBIGUOUS: ...] comment citing the digest §9 list.
package combat

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// RoundSeconds is the real-time length of one Combat Round
// (tables.md §4.1: "Combat Round | 5 seconds of real time").
const RoundSeconds = 5

// Sentinels returned by the encounter operations. Callers (harness tools,
// resolve moves) branch on them with errors.Is to turn a refusal into a
// structured IMPOSSIBLE result for the GM.
var (
	// ErrInvalidEncounter reports a nil or structurally invalid encounter or
	// participant list (nil, empty, or a nil participant).
	ErrInvalidEncounter = errors.New("combat: invalid encounter")

	// ErrDuplicateParticipant reports the same participant offered twice to
	// Start — identity is the pointer itself, or a shared non-empty
	// character ID.
	ErrDuplicateParticipant = errors.New("combat: duplicate participant")

	// ErrNotInEncounter reports a character who is not a participant of the
	// encounter an operation was called on.
	ErrNotInEncounter = errors.New("combat: not a participant in this encounter")

	// ErrNotRunning reports an operation on an encounter that has not
	// started or has already ended.
	ErrNotRunning = errors.New("combat: encounter is not running")

	// ErrNotEnoughAP reports an Action Point cost the actor cannot meet
	// (Combat › Combat Actions: 2 AP per Round, unused AP do not carry over).
	ErrNotEnoughAP = errors.New("combat: not enough action points")

	// ErrCannotAct reports a state that prevents the actor from performing
	// the requested action at all: dead, unconscious, incapacitated,
	// stunned, prone (for actions the rules deny the prone), and so on.
	// The wrapped message names the exact reason.
	ErrCannotAct = errors.New("combat: cannot act")
)

// OrderEntry is one participant's place in the strike order. The order is
// computed at Start from DEX and INT and finalised by RollInitiative's
// 1d10 roll (Combat › Initiative: highest result first, ties broken by DEX,
// then by a die roll; the highest AP value of armour worn penalises the
// roll — Mythras-Imperative.md L1368-1374 and L1588).
type OrderEntry struct {
	// Index is the participant's position in Encounter.Participants.
	Index int `json:"index"`
	// Score is the initiative total: 1d10 + Initiative modifier − armour
	// penalty (−10 while surprised). Before RollInitiative it is the plain
	// Initiative modifier, so Status always shows an order.
	Score int `json:"score"`
	// DEX is the tie-break characteristic (Combat › Initiative).
	DEX int `json:"dex"`
	// Tie is the tie-break die rolled when Score and DEX also tie; 0 until
	// rolled.
	Tie int `json:"tie"`
	// Penalty is the armour penalty subtracted from the roll (the highest
	// AP value of armour worn).
	// [AMBIGUOUS: the rulebook says the highest AP value "acts as a penalty"
	// but prints no scale (Mythras-Imperative.md L1588); the engine reads it
	// flat, subtracting that many points — digest §5, How Combat Works.]
	Penalty int `json:"penalty"`
	// Roll is the initiative 1d10 audit record; the zero value before
	// RollInitiative.
	Roll dice.Roll `json:"roll"`
}

// ParticipantStatus is one participant's snapshot in a StatusReport.
type ParticipantStatus struct {
	// Index is the participant's position in Encounter.Participants.
	Index int `json:"index"`
	// ID and Name identify the character (ID preferred; Name fallback).
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// AP is the Action Points left this Round.
	AP int `json:"ap"`
	// Dead reports the death flag (char.Character.Dead).
	Dead bool `json:"dead"`
	// Score is the current initiative score for this Round.
	Score int `json:"score"`
	// Conditions renders the active conditions as "Name (location, N
	// rounds)" strings for the GM's status line.
	Conditions []string `json:"conditions,omitempty"`
	// Locations lists every hit location's current and starting Hit Points
	// with its wound level.
	Locations []LocationStatus `json:"locations,omitempty"`
}

// LocationStatus is one hit location's HP and wound level inside a
// ParticipantStatus.
type LocationStatus struct {
	// Location is the hit location's name.
	Location string `json:"location"`
	// Current is the live Hit Point value.
	Current int `json:"current"`
	// Starting is the location's starting Hit Points.
	Starting int `json:"starting"`
	// Severity is the wound level name (char.Severity.String).
	Severity string `json:"severity"`
	// ArmorPoints is the total protection at the location.
	ArmorPoints int `json:"armorPoints"`
}

// StatusReport is the read-only picture returned by Encounter.Status: round,
// strike order and one entry per participant. Errors hit while deriving a
// character are reported in Errors rather than swallowed.
type StatusReport struct {
	// Round is the current Combat Round (round 1 begins at Start).
	Round int `json:"round"`
	// Started and Ended mirror the encounter flags.
	Started bool `json:"started"`
	Ended   bool `json:"ended"`
	// InitiativeRolled reports whether RollInitiative has run; until then
	// Order shows the preliminary DEX/INT ordering.
	InitiativeRolled bool `json:"initiativeRolled"`
	// Order is the strike order, highest initiative first.
	Order []OrderEntry `json:"order"`
	// Participants holds one entry per participant, in Order.
	Participants []ParticipantStatus `json:"participants"`
	// Errors records derivation failures encountered while building the
	// report; an empty slice means the report is complete.
	Errors []string `json:"errors,omitempty"`
}

// Encounter is a running fight: its participants (pointers — the resolve
// layer owns their lifecycle), their per-Round Action Points, the Round
// counter, the initiative strike order, pending Prepare Counter effects and
// damage-tracked weapon Hit Points. The whole struct round-trips through
// JSON; participants are referenced by index, and character identity for
// lookups is the pointer or (after a load) the character ID, so persisted
// sessions must give participants stable IDs.
//
// The zero value is not a running encounter; create one with Start.
type Encounter struct {
	// Participants are the fighting characters, in registration order.
	Participants []*char.Character `json:"participants"`
	// AP is the Action Points each participant has left this Round,
	// parallel to Participants.
	AP []int `json:"ap"`
	// Round is the current Combat Round; round 1 begins at Start.
	Round int `json:"round"`
	// Order is the strike order (highest initiative first).
	Order []OrderEntry `json:"order"`
	// InitiativeRolled reports whether RollInitiative has replaced the
	// preliminary order with 1d10 rolls.
	InitiativeRolled bool `json:"initiativeRolled"`
	// Counters holds Prepare Counter effects nominated in secret
	// (Special Effect Descriptions: the defender notes a counter in secret;
	// the engine keeps it in encounter state so it survives a save).
	Counters []CounterEntry `json:"counters,omitempty"`
	// WeaponHP tracks damage dealt to weapons by the Damage Weapon effect,
	// keyed "<participant index>:<item ID>". Weapons absent from the map
	// are at their printed Hit Points.
	WeaponHP map[string]int `json:"weaponHP,omitempty"`
	// Started reports that Start has run; Ended reports End.
	Started bool `json:"started"`
	Ended   bool `json:"ended"`
}

// CounterEntry is one secret Prepare Counter nomination: Index holds the
// effect against Against, and if the opponent applies it the counter
// substitutes Substitute (or simply neutralises it when Substitute is
// empty) — Special Effect Descriptions › Prepare Counter.
type CounterEntry struct {
	// Index is the holder's position in Encounter.Participants.
	Index int `json:"index"`
	// Against is the effect the holder is ready to counter.
	Against string `json:"against"`
	// Substitute replaces Against when it fires; empty means the counter
	// only neutralises the effect.
	Substitute string `json:"substitute,omitempty"`
}

// Start validates the participants, gives everyone their full complement of
// Action Points for round 1, and records the preliminary strike order from
// DEX and INT (Combat › Initiative). Dice are not consumed here: call
// RollInitiative with the session Rand to roll the 1d10 initiative dice and
// finalise the order — resolve does both before the first Turn.
//
// Errors wrap [ErrInvalidEncounter] (nil list or nil participant),
// [ErrDuplicateParticipant] (the same character offered twice — identity is
// the pointer or a shared non-empty ID), or whatever char.Derive returns
// for a participant with invalid characteristics.
func Start(participants []*char.Character) (*Encounter, error) {
	if len(participants) == 0 {
		return nil, fmt.Errorf("%w: no participants", ErrInvalidEncounter)
	}
	e := &Encounter{
		Participants: participants,
		AP:           make([]int, len(participants)),
		Round:        1,
		Started:      true,
	}
	seenPtr := make(map[*char.Character]bool, len(participants))
	seenID := make(map[string]bool, len(participants))
	for i, c := range participants {
		if c == nil {
			return nil, fmt.Errorf("%w: participant %d is nil", ErrInvalidEncounter, i)
		}
		if seenPtr[c] {
			return nil, fmt.Errorf("%w: %s (index %d)", ErrDuplicateParticipant, idOf(c), i)
		}
		seenPtr[c] = true
		if c.ID != "" {
			if seenID[c.ID] {
				return nil, fmt.Errorf("%w: %q (index %d)", ErrDuplicateParticipant, c.ID, i)
			}
			seenID[c.ID] = true
		}
		d, err := char.Derive(c)
		if err != nil {
			return nil, fmt.Errorf("combat: derive %s: %w", idOf(c), err)
		}
		e.AP[i] = d.ActionPoints
		e.Order = append(e.Order, OrderEntry{
			Index: i,
			Score: d.Initiative,
			DEX:   c.Chars.DEX,
		})
	}
	e.sortOrder()
	return e, nil
}

// RollInitiative rolls each participant's 1d10 + Initiative modifier,
// subtracts the armour penalty (the highest AP value of armour worn), breaks
// ties by DEX and then by a die roll, and rewrites the strike order
// (Mythras-Imperative.md, Combat › Initiative L1368-1374; tables.md §4.1).
// The encounter must be running.
func RollInitiative(r *dice.Rand, e *Encounter) error {
	if r == nil {
		return errors.New("combat: nil rand")
	}
	if err := e.running(); err != nil {
		return err
	}
	for i := range e.Order {
		o := &e.Order[i]
		c := e.Participants[o.Index]
		d, err := char.Derive(c)
		if err != nil {
			return fmt.Errorf("combat: derive %s: %w", idOf(c), err)
		}
		roll, err := r.Roll("1d10")
		if err != nil {
			return fmt.Errorf("combat: initiative for %s: %w", idOf(c), err)
		}
		penalty := c.HighestArmorAP()
		o.Roll = roll
		o.Penalty = penalty
		o.Score = roll.Total + d.Initiative - penalty
		o.DEX = c.Chars.DEX
	}
	// Still-tied entries each roll one die, high going first (Combat ›
	// Initiative); rolls happen in registration order so the sequence is
	// replayable.
	for i := 0; i < len(e.Order); i++ {
		j := i
		for j+1 < len(e.Order) && e.Order[j+1].Score == e.Order[i].Score && e.Order[j+1].DEX == e.Order[i].DEX {
			j++
		}
		if j > i {
			for k := i; k <= j; k++ {
				roll, err := r.Roll("1d10")
				if err != nil {
					return fmt.Errorf("combat: initiative tie-break: %w", err)
				}
				e.Order[k].Tie = roll.Total
			}
		}
		i = j
	}
	e.InitiativeRolled = true
	e.sortOrder()
	return nil
}

// NextRound advances the encounter to the next Combat Round: it refreshes
// every participant's Action Points to their full value (unused AP do not
// carry over — tables.md §4.1), ticks round-limited conditions (Stun
// Location, Blinded, the Serious-Wound attack ban, Surprised, and so on all
// carry Round durations), and lets the untreated-Major-Wound death timer
// run out if it expires. The encounter must be running.
func (e *Encounter) NextRound() error {
	if err := e.running(); err != nil {
		return err
	}
	e.Round++
	for i, c := range e.Participants {
		if c == nil {
			return fmt.Errorf("%w: participant %d is nil", ErrInvalidEncounter, i)
		}
		d, err := char.Derive(c)
		if err != nil {
			return fmt.Errorf("combat: derive %s: %w", idOf(c), err)
		}
		e.AP[i] = d.ActionPoints
		tickConditions(c)
	}
	// Blood Loss: "At the start of each Combat Round, the recipient loses
	// one level of Fatigue, until they collapse and possibly die" (Special
	// Effect Descriptions › Bleed; Game System › Blood Loss).
	for i, c := range e.Participants {
		if c.HasCondition(char.CondBleeding) {
			if _, err := accrueBleeding(c); err != nil {
				return fmt.Errorf("combat: bleed round %d, participant %d: %w", e.Round, i, err)
			}
		}
	}
	return nil
}

// End closes the encounter: further mechanical operations refuse with
// [ErrNotRunning]. Ending twice is harmless.
func (e *Encounter) End() {
	if e == nil {
		return
	}
	e.Ended = true
}

// Status returns the current picture: round, strike order and one entry per
// participant with AP, conditions and per-location Hit Points. It never
// returns an error; derivation failures are reported in StatusReport.Errors.
func (e *Encounter) Status() StatusReport {
	if e == nil {
		return StatusReport{}
	}
	rep := StatusReport{
		Round:            e.Round,
		Started:          e.Started,
		Ended:            e.Ended,
		InitiativeRolled: e.InitiativeRolled,
		Order:            append([]OrderEntry(nil), e.Order...),
	}
	for i, c := range e.Participants {
		ps := ParticipantStatus{
			Index: i,
			AP:    safeAP(e, i),
		}
		if c == nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("participant %d is nil", i))
			rep.Participants = append(rep.Participants, ps)
			continue
		}
		ps.ID, ps.Name = c.ID, c.Name
		ps.Dead = c.Dead
		for _, o := range e.Order {
			if o.Index == i {
				ps.Score = o.Score
				break
			}
		}
		for _, cond := range c.Conditions {
			ps.Conditions = append(ps.Conditions, renderCondition(cond))
		}
		d, err := char.Derive(c)
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", idOf(c), err))
			rep.Participants = append(rep.Participants, ps)
			continue
		}
		for _, loc := range d.Locations {
			cur, ok := c.CurrentHP[loc.Name]
			if !ok {
				cur = d.LocationHP[loc.Name]
			}
			sev, err := c.SeverityAt(loc.Name)
			if err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", idOf(c), err))
				continue
			}
			ap, err := c.ArmorAt(loc.Name)
			if err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", idOf(c), err))
				continue
			}
			ps.Locations = append(ps.Locations, LocationStatus{
				Location:    loc.Name,
				Current:     cur,
				Starting:    d.LocationHP[loc.Name],
				Severity:    sev.String(),
				ArmorPoints: ap,
			})
		}
		rep.Participants = append(rep.Participants, ps)
	}
	return rep
}

// running reports whether mechanical operations are allowed: the encounter
// must have started and must not have ended.
func (e *Encounter) running() error {
	if e == nil {
		return fmt.Errorf("%w: nil encounter", ErrInvalidEncounter)
	}
	if !e.Started {
		return fmt.Errorf("%w: not started", ErrNotRunning)
	}
	if e.Ended {
		return fmt.Errorf("%w: ended in round %d", ErrNotRunning, e.Round)
	}
	return nil
}

// indexOf returns the participant index for c. Identity is the pointer
// first, then a shared non-empty character ID (which is what survives a JSON
// round trip of the encounter). Errors wrap [ErrNotInEncounter].
func (e *Encounter) indexOf(c *char.Character) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: nil character", ErrNotInEncounter)
	}
	for i, p := range e.Participants {
		if p == c {
			return i, nil
		}
	}
	if c.ID != "" {
		for i, p := range e.Participants {
			if p != nil && p.ID == c.ID {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrNotInEncounter, idOf(c))
}

// spend deducts cost Action Points from the participant. The caller must
// have validated sufficiency; errors wrap [ErrNotInEncounter] or
// [ErrNotEnoughAP].
func (e *Encounter) spend(c *char.Character, cost int) error {
	i, err := e.indexOf(c)
	if err != nil {
		return err
	}
	if i >= len(e.AP) || e.AP[i] < cost {
		return fmt.Errorf("%w: %s has %d, needs %d", ErrNotEnoughAP, idOf(c), safeAP(e, i), cost)
	}
	e.AP[i] -= cost
	return nil
}

// adjustInitiative shifts one participant's initiative score by delta and
// re-sorts the strike order (Surprise applies −10).
func (e *Encounter) adjustInitiative(index, delta int) {
	for i := range e.Order {
		if e.Order[i].Index == index {
			e.Order[i].Score += delta
			break
		}
	}
	e.sortOrder()
}

// sortOrder orders the strike entries: score descending, then DEX, then the
// tie-break die, then registration order. Entries always reference
// Participants by index, so the sort never invalidates them.
func (e *Encounter) sortOrder() {
	sort.SliceStable(e.Order, func(a, b int) bool {
		x, y := e.Order[a], e.Order[b]
		if x.Score != y.Score {
			return x.Score > y.Score
		}
		if x.DEX != y.DEX {
			return x.DEX > y.DEX
		}
		if x.Tie != y.Tie {
			return x.Tie > y.Tie
		}
		return x.Index < y.Index
	})
}

// tickConditions decrements every round-limited condition and removes the
// expired ones. Rapid Reload reuses Rounds as its stack counter (it is a
// flag, not a duration), so it is exempt. An expiring Untreated Major Wound
// condition kills the character: "survivors die unless treated within
// Combat Rounds = 2 × Healing Rate" (tables.md §3.9).
func tickConditions(c *char.Character) {
	if len(c.Conditions) == 0 {
		return
	}
	kept := c.Conditions[:0]
	for _, cond := range c.Conditions {
		if cond.Rounds > 0 && !tickExempt(cond.Name) {
			if cond.Rounds--; cond.Rounds == 0 {
				if cond.Name == CondUntreatedMajorWound {
					c.Dead = true
				}
				continue
			}
		}
		kept = append(kept, cond)
	}
	c.Conditions = kept
}

// tickExempt reports whether a condition name is exempt from round ticking
// (its Rounds field counts stacks, not time).
func tickExempt(name string) bool { return name == CondRapidReload }

// renderCondition formats a condition for the status line: "Stunned (Right
// Arm, 3 rounds)".
func renderCondition(cond char.Condition) string {
	var b strings.Builder
	b.WriteString(cond.Name)
	var parts []string
	if cond.Location != "" {
		parts = append(parts, cond.Location)
	}
	if cond.Rounds > 0 && !tickExempt(cond.Name) {
		parts = append(parts, fmt.Sprintf("%d rounds", cond.Rounds))
	}
	if cond.Notes != "" && len(parts) == 0 {
		parts = append(parts, cond.Notes)
	}
	if len(parts) > 0 {
		b.WriteString(" (" + strings.Join(parts, ", ") + ")")
	}
	return b.String()
}

// safeAP returns the AP ledger value at i, or 0 when the ledger is short.
func safeAP(e *Encounter, i int) int {
	if i < 0 || i >= len(e.AP) {
		return 0
	}
	return e.AP[i]
}

// idOf identifies a character for audit lines: the ID when set, otherwise
// the name, otherwise "unknown".
func idOf(c *char.Character) string {
	if c == nil {
		return "unknown"
	}
	if c.ID != "" {
		return c.ID
	}
	if c.Name != "" {
		return c.Name
	}
	return "unknown"
}
