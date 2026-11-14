package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// worldTools returns the world and continuity tools, in catalogue order.
func worldTools() []Tool {
	return []Tool{
		{
			Name: "world_get",
			Description: "Read the world summary, or one flag's value when a key is " +
				"given. Use to recall established world facts before narrating or " +
				"checking what the players already know." + factsNote,
			Schema: objSchema(map[string]any{
				"key": strProp("Flag key (e.g. \"economy\"); omit for the whole-world summary."),
			}),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Key string `json:"key"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				block, err := resolve.WorldGet(s, a.Key)
				if err != nil {
					return Result{}, err
				}
				if a.Key != "" {
					return Result{
						OK:      true,
						Summary: fmt.Sprintf("flag %s = %s", a.Key, block),
						Facts:   []string{"key: " + a.Key, "value: " + block},
					}, nil
				}
				return Result{
					OK:      true,
					Summary: "world summary read",
					Facts:   []string{"world:\n" + block},
				}, nil
			},
		},
		{
			Name: "world_set_flag",
			Description: "Write one world fact — economy, war, regional aspects, " +
				"material scarcity or a free-form key — with its reason and old → new " +
				"value. Use when the campaign's state changes so later narration and " +
				"can_acquire stay consistent." + factsNote,
			Schema: objSchema(map[string]any{
				"key":    strProp("Flag key: \"economy\", \"war\", \"region:<region>:<aspect>\", \"<material>_scarce\", or any free-form key."),
				"value":  strProp("New value. Economy: stable/strained/collapsed; war: peace/cold/active; scarcity: true/false."),
				"reason": strProp("Why the flag now holds that value; recorded in its history (recommended)."),
			}, "key", "value"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Key    string `json:"key"`
					Value  string `json:"value"`
					Reason string `json:"reason"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				for _, f := range []struct{ name, value string }{
					{"key", a.Key}, {"value", a.Value},
				} {
					if err := requireField(f.name, f.value); err != nil {
						return Result{}, err
					}
				}
				res, err := resolve.Apply(s, resolve.SetWorldFlag{
					Key: a.Key, Value: a.Value, Reason: a.Reason,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "can_acquire",
			Description: "Check whether an item of the given rarity can be obtained in " +
				"a region under the world's current flags — yes/no with the governing " +
				"flags and reasoning. Use before granting or denying rare gear." + factsNote,
			Schema: objSchema(map[string]any{
				"item":   strProp("Item being sought."),
				"rarity": enumProp("Item rarity.", "common", "uncommon", "rare", "legendary"),
				"region": strProp("Region the acquisition happens in; omit for the world at large."),
			}, "item", "rarity"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Item   string `json:"item"`
					Rarity string `json:"rarity"`
					Region string `json:"region"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("item", a.Item); err != nil {
					return Result{}, err
				}
				if err := requireField("rarity", a.Rarity); err != nil {
					return Result{}, err
				}
				d, err := resolve.CanAcquire(s, a.Item, a.Rarity, a.Region)
				if err != nil {
					return Result{}, err
				}
				verdict := "no"
				if d.Allowed {
					verdict = "yes"
				}
				facts := []string{fmt.Sprintf("allowed: %t", d.Allowed)}
				for _, reason := range d.Reasons {
					facts = append(facts, "reason: "+reason)
				}
				if len(d.GoverningFlags) > 0 {
					facts = append(facts, "governing flags: "+strings.Join(d.GoverningFlags, ", "))
				}
				return Result{
					OK:      true,
					Summary: fmt.Sprintf("can acquire %s (%s): %s", a.Item, a.Rarity, verdict),
					Facts:   facts,
				}, nil
			},
		},
		{
			Name: "add_clock",
			Description: "Create a progress clock — a segmented tension track for " +
				"campaign arcs such as unrest, doom or a closing ritual. Use once " +
				"when the tension first appears, then fill it with advance_clock " +
				"as pressure mounts." + factsNote,
			Schema: objSchema(map[string]any{
				"name":        strProp("The clock's display name; must not already exist."),
				"segments":    intProp("Capacity in segments (positive, fixed at creation).", 1),
				"on_complete": strArrProp("Narration hooks that fire once, when the clock fills; optional."),
			}, "name", "segments"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Name       string   `json:"name"`
					Segments   int      `json:"segments"`
					OnComplete []string `json:"on_complete"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("name", a.Name); err != nil {
					return Result{}, err
				}
				if a.Segments < 1 {
					return Result{}, fmt.Errorf("%w: %q must be a positive integer", ErrInvalidArgs, "segments")
				}
				res, err := resolve.Apply(s, resolve.AddClock{
					Name: a.Name, Segments: a.Segments, OnComplete: a.OnComplete,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "advance_clock",
			Description: "Fill segments of a progress clock and report what the fill " +
				"triggered. Use when campaign tension advances; create the clock " +
				"first with add_clock if it does not already exist." + factsNote,
			Schema: objSchema(map[string]any{
				"clock":    strProp("The clock's name as it exists in the world."),
				"segments": intProp("Segments to fill (positive).", 1),
				"reason":   strProp("Why the clock advances; recorded in its history (recommended)."),
			}, "clock", "segments"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Clock    string `json:"clock"`
					Segments int    `json:"segments"`
					Reason   string `json:"reason"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("clock", a.Clock); err != nil {
					return Result{}, err
				}
				if a.Segments < 1 {
					return Result{}, fmt.Errorf("%w: %q must be a positive integer", ErrInvalidArgs, "segments")
				}
				res, err := resolve.Apply(s, resolve.AdvanceClock{
					Clock: a.Clock, Segments: a.Segments, Reason: a.Reason,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "advance_time",
			Description: "Advance the in-game calendar by days and/or hours. Use when " +
				"time passes off-screen — travel, watch, recovery — and report the " +
				"date and season it lands on." + factsNote,
			Schema: objSchema(map[string]any{
				"days":  intProp("Whole days to advance (0 when only hours are used).", 0),
				"hours": intProp("Hours to advance (24 make one day; 0 when only days are used).", 0),
			}),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Days  int `json:"days"`
					Hours int `json:"hours"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if a.Days < 0 || a.Hours < 0 {
					return Result{}, fmt.Errorf("%w: days and hours must not be negative", ErrInvalidArgs)
				}
				if a.Days == 0 && a.Hours == 0 {
					return Result{}, fmt.Errorf("%w: supply at least one of days or hours", ErrInvalidArgs)
				}
				res, err := resolve.Apply(s, resolve.AdvanceTime{Days: a.Days, Hours: a.Hours})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "set_scene",
			Description: "Establish the narrative frame: location, description and who " +
				"is present. Use at the start of every scene so later narration and the " +
				"metadata panel agree on where play happens." + factsNote,
			Schema: objSchema(map[string]any{
				"location":    strProp("Where play happens (required)."),
				"description": strProp("Scene-setting text."),
				"present":     strArrProp("Who or what is on stage — free text, typically character IDs."),
			}, "location"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Location    string   `json:"location"`
					Description string   `json:"description"`
					Present     []string `json:"present"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("location", a.Location); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.SetScene{
					Location: a.Location, Description: a.Description, Present: a.Present,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "note",
			Description: "Store a persistent GM annotation on the world — clues, NPC " +
				"motives, loose ends — with an engine-generated note ID. Use for facts " +
				"that must survive context compaction and later sessions." + factsNote,
			Schema: objSchema(map[string]any{
				"topic": strProp("What the note is about, e.g. \"clues\", \"NPC motives\"."),
				"text":  strProp("The note's body."),
				"tags":  strArrProp("Free-form labels for grouping."),
			}, "topic", "text"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Topic string   `json:"topic"`
					Text  string   `json:"text"`
					Tags  []string `json:"tags"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("topic", a.Topic); err != nil {
					return Result{}, err
				}
				if err := requireField("text", a.Text); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.Note{
					Topic: a.Topic, Text: a.Text, Tags: a.Tags,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "session_recap",
			Description: "Read the stored end-of-session recaps, oldest first. Use at " +
				"the start of a session to rebuild context before play resumes." + factsNote,
			Schema: objSchema(nil),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct{}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				recaps := resolve.SessionRecap(s)
				if len(recaps) == 0 {
					return Result{
						OK:      true,
						Summary: "no recaps stored yet",
						Facts:   []string{"recaps stored: 0"},
					}, nil
				}
				facts := make([]string, 0, len(recaps)+1)
				facts = append(facts, fmt.Sprintf("recaps stored: %d", len(recaps)))
				for i, recap := range recaps {
					facts = append(facts, fmt.Sprintf("recap %d: %s", i+1, recap))
				}
				return Result{
					OK:      true,
					Summary: fmt.Sprintf("%d recap(s) stored", len(recaps)),
					Facts:   facts,
				}, nil
			},
		},
		{
			Name: "end_session_recap",
			Description: "Store an end-of-session summary for future context rebuilds. " +
				"Use when a session ends: one paragraph covering what happened, what " +
				"changed and what hangs unresolved." + factsNote,
			Schema: objSchema(map[string]any{
				"summary": strProp("The end-of-session summary to store."),
			}, "summary"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Summary string `json:"summary"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("summary", a.Summary); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.EndRecap{Summary: a.Summary})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
	}
}
