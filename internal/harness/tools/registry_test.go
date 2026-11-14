package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// catalogueNames is the authoritative tool list from
// docs/design/harness-surface.md, hard-coded so any drift between the doc,
// NewRegistry and this test fails loudly.
var catalogueNames = []string{
	// Rules & advice
	"rules_lookup", "rules_section", "decide",
	// Dice & tests
	"roll_dice", "skill_test", "opposed_test", "luck_point",
	// Characters & NPCs
	"create_character", "finalize_character", "get_character",
	"update_character", "list_characters", "skill_advance",
	// Inventory & equipment
	"inventory_add", "inventory_remove", "equip", "check_encumbrance",
	// Combat
	"combat_start", "combat_status", "spend_ap", "can_act", "next_round",
	"resolve_attack", "apply_special_effect", "apply_damage", "heal", "combat_end",
	// World & continuity
	"world_get", "world_set_flag", "can_acquire", "add_clock", "advance_clock",
	"advance_time", "set_scene", "note", "session_recap", "end_session_recap",
}

// TestRegistryCompleteness pins the registry to the catalogue: every listed
// tool exists, nothing extra is registered.
func TestRegistryCompleteness(t *testing.T) {
	got := NewRegistry(nil).Names()
	want := slices.Clone(catalogueNames)
	slices.Sort(want)
	have := slices.Clone(got)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Errorf("Names() = %v, want catalogue %v", got, catalogueNames)
	}
	for _, name := range catalogueNames {
		if !slices.Contains(got, name) {
			t.Errorf("tool %q from the catalogue is not registered", name)
		}
	}
	if n := len(catalogueNames); n != 37 {
		t.Errorf("catalogueNames has %d entries, want 37 — update the list with the doc", n)
	}
}

// TestSchemaSanity checks every registered tool's contract: a non-empty
// description, an object schema whose required fields are a subset of its
// properties, and typed properties throughout.
func TestSchemaSanity(t *testing.T) {
	reg := NewRegistry(nil)
	for _, spec := range reg.Tools() {
		if strings.TrimSpace(spec.Description) == "" {
			t.Errorf("tool %q: empty description", spec.Name)
		}
		if spec.Schema["type"] != "object" {
			t.Errorf("tool %q: schema type = %v, want object", spec.Name, spec.Schema["type"])
		}
		props, _ := spec.Schema["properties"].(map[string]any)
		if props == nil {
			t.Errorf("tool %q: schema has no properties object", spec.Name)
			continue
		}
		required, _ := spec.Schema["required"].([]string)
		for _, key := range required {
			if _, ok := props[key]; !ok {
				t.Errorf("tool %q: required field %q not in properties", spec.Name, key)
			}
		}
		for key, prop := range props {
			p, ok := prop.(map[string]any)
			if !ok {
				t.Errorf("tool %q property %q: not a schema object", spec.Name, key)
				continue
			}
			if kind, ok := p["type"].(string); !ok || kind == "" {
				t.Errorf("tool %q property %q: missing type", spec.Name, key)
			}
		}
	}
}

// TestRegisterRejections covers the registration boundary.
func TestRegisterRejections(t *testing.T) {
	base := func() *Registry { return &Registry{byName: map[string]Tool{}} }
	good := Tool{Name: "x", Description: "d", Schema: objSchema(nil), Handler: func(context.Context, *resolve.State, json.RawMessage) (Result, error) { return Result{}, nil }}

	tests := []struct {
		name string
		tool Tool
		want string
	}{
		{"empty name", Tool{Description: "d", Schema: objSchema(nil), Handler: good.Handler}, "name must not be empty"},
		{"empty description", Tool{Name: "x", Schema: objSchema(nil), Handler: good.Handler}, "empty description"},
		{"nil handler", Tool{Name: "x", Description: "d", Schema: objSchema(nil)}, "nil handler"},
		{"nil schema", Tool{Name: "x", Description: "d", Handler: good.Handler}, "object JSON Schema"},
		{"schema not object", Tool{Name: "x", Description: "d", Schema: map[string]any{"type": "array"}, Handler: good.Handler}, "object JSON Schema"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := base().Register(tt.tool)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Register() error = %v, want containing %q", err, tt.want)
			}
		})
	}

	t.Run("duplicate", func(t *testing.T) {
		r := base()
		if err := r.Register(good); err != nil {
			t.Fatalf("first Register: %v", err)
		}
		err := r.Register(good)
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("second Register error = %v, want duplicate rejection", err)
		}
	})
}

// TestDispatchUnknownTool: an unknown name is a Go error listing the known
// tools — never a silent no-op.
func TestDispatchUnknownTool(t *testing.T) {
	reg := NewRegistry(nil)
	_, err := reg.Dispatch(t.Context(), resolve.NewState("t", 1), "frobnicate", nil)
	if err == nil {
		t.Fatal("Dispatch(unknown) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "unknown tool") || !strings.Contains(err.Error(), "rules_lookup") {
		t.Errorf("error = %v, want unknown tool with known list", err)
	}
}

// TestDispatchNilState: a nil state is a harness fault, not IMPOSSIBLE.
func TestDispatchNilState(t *testing.T) {
	reg := NewRegistry(nil)
	_, err := reg.Dispatch(t.Context(), nil, "roll_dice", json.RawMessage(`{"expr":"1d6","reason":"t"}`))
	if !errors.Is(err, ErrHarnessFault) {
		t.Errorf("Dispatch(nil state) error = %v, want ErrHarnessFault", err)
	}
}

// TestDispatchHarnessFault: handler errors wrapping ErrHarnessFault
// propagate as Go errors instead of IMPOSSIBLE results.
func TestDispatchHarnessFault(t *testing.T) {
	r := &Registry{byName: map[string]Tool{}}
	tool := Tool{
		Name: "broken", Description: "d", Schema: objSchema(nil),
		Handler: func(context.Context, *resolve.State, json.RawMessage) (Result, error) {
			return Result{}, fmt.Errorf("%w: something broke", ErrHarnessFault)
		},
	}
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err := r.Dispatch(t.Context(), resolve.NewState("t", 1), "broken", nil)
	if !errors.Is(err, ErrHarnessFault) {
		t.Errorf("Dispatch harness fault = %v, want ErrHarnessFault", err)
	}
}

// TestDispatchInvalidArgs: argument failures become narratable IMPOSSIBLE
// results naming the tool and the missing/bad field — never Go errors.
func TestDispatchInvalidArgs(t *testing.T) {
	reg := NewRegistry(nil)
	s := resolve.NewState("t", 1)
	tests := []struct {
		name string
		tool string
		args string
		want string
	}{
		{"malformed json", "skill_test", `{"actor": }`, "invalid arguments for skill_test"},
		{"missing required field", "skill_test", `{"skill":"Perception"}`, `missing required field "actor"`},
		{"missing pc", "create_character", `{"name":"X","culture":"Civilised","career":"Hunter"}`, `missing required field "pc"`},
		{"unknown field", "roll_dice", `{"expr":"1d6","reason":"t","extra":1}`, "invalid arguments for roll_dice"},
		{"wrong type", "roll_dice", `{"expr":42,"reason":"t"}`, "invalid arguments for roll_dice"},
		{"empty list", "combat_start", `{"participants":[]}`, `missing required field "participants"`},
		{"zero amount", "apply_damage", `{"actor":"pc-1","amount":0}`, `"amount" must be a positive integer`},
		{"zero clock segments", "add_clock", `{"name":"doom","segments":0}`, `"segments" must be a positive integer`},
		{"zero duration", "advance_time", `{}`, "at least one of days or hours"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := reg.Dispatch(t.Context(), s, tt.tool, json.RawMessage(tt.args))
			if err != nil {
				t.Fatalf("Dispatch returned Go error %v, want IMPOSSIBLE result", err)
			}
			if res.OK {
				t.Fatalf("OK = true, want refusal")
			}
			if !strings.HasPrefix(res.Summary, "IMPOSSIBLE: invalid arguments for "+tt.tool+": ") {
				t.Errorf("Summary = %q, want invalid-arguments refusal for %s", res.Summary, tt.tool)
			}
			if !strings.Contains(res.Summary, tt.want) {
				t.Errorf("Summary = %q, want containing %q", res.Summary, tt.want)
			}
		})
	}
}

// TestDispatchSentinelTranslation: an engine refusal keeps its own message
// (minus the package prefix) — ErrUnknownCharacter lists the known IDs, so
// the GM can correct the call.
func TestDispatchSentinelTranslation(t *testing.T) {
	reg := NewRegistry(nil)
	s := resolve.NewState("t", 1)
	mustOK(t, reg, s, "create_character", map[string]any{
		"name": "Wren", "pc": true, "culture": "Civilised", "career": "Hunter",
	})

	t.Run("unknown character", func(t *testing.T) {
		res, err := reg.Dispatch(t.Context(), s, "get_character", json.RawMessage(`{"actor":"npc-99"}`))
		if err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		if res.OK {
			t.Fatal("OK = true, want refusal")
		}
		if !strings.HasPrefix(res.Summary, "IMPOSSIBLE: unknown character") {
			t.Errorf("Summary = %q, want engine message with prefix stripped", res.Summary)
		}
		if !strings.Contains(res.Summary, "known: pc-1") {
			t.Errorf("Summary = %q, want known ID list", res.Summary)
		}
	})

	t.Run("no encounter", func(t *testing.T) {
		res, err := reg.Dispatch(t.Context(), s, "combat_status", nil)
		if err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		if res.OK || !strings.HasPrefix(res.Summary, "IMPOSSIBLE: no encounter running") {
			t.Errorf("Summary = %q, want no-encounter refusal", res.Summary)
		}
	})

	t.Run("unknown tool is the only Go error", func(t *testing.T) {
		if _, err := reg.Dispatch(t.Context(), s, "nope", nil); err == nil {
			t.Error("want error for unknown tool")
		}
	})
}

// mustOK dispatches args and fails the test unless the result is OK.
func mustOK(t *testing.T, r *Registry, s *resolve.State, name string, args any) Result {
	t.Helper()
	res := dispatch(t, r, s, name, args)
	if !res.OK {
		t.Fatalf("%s: not OK: %s", name, res.Summary)
	}
	if strings.TrimSpace(res.Summary) == "" {
		t.Fatalf("%s: empty summary", name)
	}
	return res
}

// dispatch marshals args and runs one registry call, failing on harness
// faults (a Go error) but accepting IMPOSSIBLE results.
func dispatch(t *testing.T, r *Registry, s *resolve.State, name string, args any) Result {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		marshalled, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("marshal %s args: %v", name, err)
		}
		raw = marshalled
	}
	res, err := r.Dispatch(t.Context(), s, name, raw)
	if err != nil {
		t.Fatalf("%s: harness fault: %v", name, err)
	}
	return res
}
