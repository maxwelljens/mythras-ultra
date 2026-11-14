// Mock-session tests: the canned GM turns in testdata/mocksession replay a
// full game through llm.ScriptedClient → tools.Registry.Dispatch →
// resolve.Apply-managed state, exactly the way the production orchestrator
// will drive them (session-loop.md). Recording each move in the session log
// through state.Store belongs to that orchestrator and is deliberately not
// exercised here — this test proves the tool surface and the fixtures, not
// persistence.
package harness

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/tools"
)

// mockScript is one testdata/mocksession fixture: the session seed, the
// canned turns in llm.ScriptedTurn's exact JSON shape
// ({"response": {"message": {…}}}), and the replay expectations.
type mockScript struct {
	Seed   int64              `json:"seed"`
	Turns  []llm.ScriptedTurn `json:"turns"`
	Expect mockExpect         `json:"expect"`
}

// mockExpect pins a replay. tools_used must equal the tools actually
// dispatched; impossible lists the tools that must produce at least one
// structured IMPOSSIBLE refusal (their successful calls are allowed too,
// e.g. spend_ap twice with points and a third time without). Every other
// dispatch must be OK.
type mockExpect struct {
	ToolsUsed  []string `json:"tools_used"`
	Impossible []string `json:"impossible"`
}

// scriptPaths are the mock sessions, in execution order.
var scriptPaths = []string{
	"testdata/mocksession/script.json",
	"testdata/mocksession/script_ooc.json",
}

// TestMockSessionCampaign replays the full campaign arc and asserts the
// state it leaves behind: character, inventory, world flags, clock, recap
// and the ended encounter.
func TestMockSessionCampaign(t *testing.T) {
	script, s := replayScript(t, scriptPaths[0], newDecideStub(t))

	if n := len(script.Turns); n != 9 {
		t.Errorf("script.json has %d turns, want 9", n)
	}

	pc, ok := s.Characters["pc-1"]
	if !ok {
		t.Fatalf("pc-1 missing; characters: %v", slices.Sorted(maps.Keys(s.Characters)))
	}
	if !pc.PC || pc.Name != "Wren Ashdown" || pc.Culture != "Civilised" || pc.Career != "Hunter" {
		t.Errorf("pc-1 = %+v, want the scripted sheet", pc)
	}
	if hp := pc.Derived.HitPoints; hp < 25 || hp > 75 {
		t.Errorf("pc-1 total hit points = %d, want the 25–75 band for this seed", hp)
	}

	npc, ok := s.Characters["npc-1"]
	if !ok {
		t.Fatalf("npc-1 missing; characters: %v", slices.Sorted(maps.Keys(s.Characters)))
	}
	if npc.PC || len(npc.Wounds) == 0 {
		t.Errorf("npc-1: PC=%t wounds=%d, want a wounded NPC", npc.PC, len(npc.Wounds))
	}

	if left, err := pc.ArrowsLeft(); err != nil || left != 7 {
		t.Errorf("arrows left = %d, err %v; want 7 after add 10 / remove 3", left, err)
	}

	if v, err := s.World.Get("economy"); err != nil || v != "strained" {
		t.Errorf("economy flag = %q, %v; want strained", v, err)
	}
	if clock := s.World.Clocks["unrest"]; clock == nil || clock.Filled != 2 {
		t.Errorf("unrest clock = %+v, want 2 of 6 segments filled", clock)
	}

	if s.Encounter == nil || !s.Encounter.Ended || s.Encounter.Round != 2 {
		t.Errorf("encounter = %+v, want an ended record at round 2", s.Encounter)
	}
	if len(s.Recaps) != 1 || !strings.Contains(s.Recaps[0], "wayside shrine") {
		t.Errorf("recaps = %v, want the scripted end-of-session summary", s.Recaps)
	}
	if s.Scene.Location != "Wayside shrine on the Salt Roads" {
		t.Errorf("scene location = %q, want the scripted scene", s.Scene.Location)
	}
	if len(s.World.Notes) != 1 {
		t.Errorf("world notes = %d, want 1", len(s.World.Notes))
	}
	var wielded bool
	for _, item := range pc.Inventory {
		if item != nil && item.Name == "Broadsword" && item.Equipped && item.Slot == "main" {
			wielded = true
		}
	}
	if !wielded {
		t.Error("Broadsword not wielded in the main slot")
	}
	if s.EventSeq < 20 {
		t.Errorf("EventSeq = %d, want at least 20 accepted moves", s.EventSeq)
	}
}

// TestMockSessionOutOfCharacter replays the OOC script: clarifications, an
// unavailable decision model, an exhausted Action Point pool, an unknown
// character ID and a malformed call — every refusal structured, the loop
// never crashing.
func TestMockSessionOutOfCharacter(t *testing.T) {
	_, s := replayScript(t, scriptPaths[1], decide.New(decide.Config{Enabled: false}))

	if len(s.Characters) != 1 {
		t.Errorf("characters = %v, want exactly pc-1", slices.Sorted(maps.Keys(s.Characters)))
	}
	if s.Encounter == nil || s.Encounter.Ended {
		t.Fatalf("encounter = %+v, want still running (the OOC script never ends it)", s.Encounter)
	}
	status, err := resolve.CombatStatus(s)
	if err != nil {
		t.Fatalf("CombatStatus: %v", err)
	}
	for _, p := range status.Participants {
		if p.ID == "pc-1" && p.AP != 0 {
			t.Errorf("pc-1 AP = %d, want 0 after two spends and a refused third", p.AP)
		}
	}
}

// TestMockSessionToolCoverage enforces the coverage rule from
// harness-surface.md: every registered tool must appear in expect.tools_used
// of at least one script (each script test additionally proves the declared
// list equals what was actually dispatched).
func TestMockSessionToolCoverage(t *testing.T) {
	registered := tools.NewRegistry(nil).Names()
	covered := map[string]bool{}
	for _, path := range scriptPaths {
		script := loadScript(t, path)
		for _, name := range script.Expect.ToolsUsed {
			covered[name] = true
		}
	}
	var unexercised []string
	for _, name := range registered {
		if !covered[name] {
			unexercised = append(unexercised, name)
		}
	}
	if len(unexercised) > 0 {
		t.Errorf("tools not exercised by any mock session script: %s", strings.Join(unexercised, ", "))
	}
	for name := range covered {
		if !slices.Contains(registered, name) {
			t.Errorf("script declares tool %q which is not registered", name)
		}
	}
}

// loadScript reads and parses one fixture, failing the test on bad JSON.
func loadScript(t *testing.T, path string) mockScript {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var script mockScript
	if err := json.Unmarshal(raw, &script); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return script
}

// replayScript drives one fixture through the real loop shape: Chat →
// dispatch each tool call against the live state → append RoleTool results →
// Chat again, until ScriptedClient reports the script spent. It returns the
// parsed fixture and the final state.
//
// Per dispatch it enforces the fixture's expect block: tools listed in
// impossible must refuse at least once with an "IMPOSSIBLE: " summary (their
// successful calls are allowed), every other tool must succeed with facts.
// A harness fault (a Go error from Dispatch) fails the replay — only the
// model's bad calls may be refused, never the harness.
func replayScript(t *testing.T, path string, decisionClient *decide.Client) (mockScript, *resolve.State) {
	t.Helper()
	script := loadScript(t, path)
	client := llm.NewScripted(script.Turns...)
	reg := tools.NewRegistry(decisionClient)

	s := resolve.NewState("Mock Session", script.Seed)

	history := []llm.Message{{
		Role:    llm.RoleSystem,
		Content: "You are the Mythras Ultra game master. Narrate from tool facts.",
	}}
	counts := map[string]int{}
	refusals := map[string]int{}
	for turn := 0; ; turn++ {
		if turn > 500 {
			t.Fatalf("%s: replay exceeded 500 assistant turns", path)
		}
		resp, err := client.Chat(t.Context(), llm.ChatRequest{Messages: history, Tools: reg.Tools()})
		if errors.Is(err, llm.ErrScriptExhausted) {
			break
		}
		if err != nil {
			t.Fatalf("%s: chat turn %d: %v", path, turn, err)
		}
		msg := resp.Message
		history = append(history, msg)
		for _, tc := range msg.ToolCalls {
			res, err := reg.Dispatch(t.Context(), s, tc.Name, tc.Args)
			if err != nil {
				t.Fatalf("%s: turn %d: %s: harness fault: %v", path, turn, tc.Name, err)
			}
			t.Logf("turn %d: %s → OK=%t | %s | facts: %s", turn, tc.Name, res.OK, res.Summary, strings.Join(res.Facts, " / "))
			payload, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("%s: marshal %s result: %v", path, tc.Name, err)
			}
			history = append(history, llm.Message{
				Role:       llm.RoleTool,
				Content:    string(payload),
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
			counts[tc.Name]++
			mayRefuse := slices.Contains(script.Expect.Impossible, tc.Name)
			switch {
			case mayRefuse && !res.OK:
				if !strings.HasPrefix(res.Summary, "IMPOSSIBLE: ") {
					t.Errorf("%s: turn %d: %s: refusal %q lacks the IMPOSSIBLE prefix", path, turn, tc.Name, res.Summary)
				}
				refusals[tc.Name]++
			case mayRefuse && res.OK:
				// Allowed: a tool may both succeed and refuse across a
				// script; the post-loop check demands at least one refusal.
			case !mayRefuse && !res.OK:
				t.Errorf("%s: turn %d: %s: expected OK, got refusal %q", path, turn, tc.Name, res.Summary)
			case !mayRefuse && res.OK:
				if len(res.Facts) == 0 {
					t.Errorf("%s: turn %d: %s: OK result carries no facts", path, turn, tc.Name)
				}
			}
		}
	}
	for _, name := range script.Expect.Impossible {
		if refusals[name] == 0 {
			t.Errorf("%s: tool %q: expected at least one IMPOSSIBLE result, got none", path, name)
		}
	}
	declared := slices.Clone(script.Expect.ToolsUsed)
	slices.Sort(declared)
	actual := slices.Sorted(maps.Keys(counts))
	if !slices.Equal(declared, actual) {
		t.Errorf("%s: expect.tools_used = %v, but the replay dispatched %v", path, declared, actual)
	}
	return script, s
}

// newDecideStub serves a minimal /v1/systemone endpoint that answers every
// choice question with its lexicographically first option, so the decide
// tool runs end-to-end offline (docs/research/llm-stack.md §3 documents the
// endpoint shape).
func newDecideStub(t *testing.T) *decide.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Model     string `json:"model"`
			State     string `json:"state"`
			Questions map[string]struct {
				Type         string             `json:"type"`
				Instructions string             `json:"instructions"`
				Criteria     map[string]*string `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Questions) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		answers := make(map[string]any, len(req.Questions))
		for label, q := range req.Questions {
			options := slices.Sorted(maps.Keys(q.Criteria))
			if len(options) == 0 {
				http.Error(w, "no options", http.StatusBadRequest)
				return
			}
			pick := options[0]
			answers[label] = map[string]any{
				"type":          "choice",
				"choice":        pick,
				"probabilities": map[string]float64{pick: 1},
				"confidence":    0.9,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"model": "tev1:4b", "answers": answers}); err != nil {
			t.Errorf("encode /v1/systemone response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return decide.New(decide.Config{Enabled: true, URL: srv.URL, Model: "tev1:4b"})
}
