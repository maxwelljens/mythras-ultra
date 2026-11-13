---
id: harness-tool
name: harness-tool
description: Expose a new GM-facing tool in internal/harness/tools — Tool struct with name/description/JSON schema, handler translating an engine Result into {ok, summary, facts, audit} and sentinel errors into structured IMPOSSIBLE: results, registry wiring, mock-session coverage in testdata/mocksession, and prompt updates in internal/harness/prompts when tool policy changes. Use when adding or changing any tool the LLM game master can call.
---

# harness-tool

Tools are the *only* way mechanics happen for the GM (ARCHITECTURE.md §3.2).
A tool is a thin, pure adapter: parse/validate arguments → call a `resolve`
Move → project the `Result` into `{ok, summary, facts, audit}` → on engine
sentinel errors, return a structured refusal the GM can narrate. Every
registered tool must be exercised at least once by the mock session — the
coverage check walks the registry.

## Recipe

1. **Write (or reuse) the engine Move first.** The handler never implements
   mechanics; see the `engine-move` skill. If the Move already exists, skip
   to step 2.
2. **Define the `Tool`** in `internal/harness/tools`: `Name` (snake_case,
   matching the family conventions in ARCHITECTURE.md §6.1), `Description`
   (what the GM may use it for, and when not to), and `JSONSchema` for the
   argument object. Schemas are what the LLM sees — keep them tight and
   typed.
3. **Write the handler**: unmarshal/validate args → dispatch the Move → map
   the outcome:
   - success → `{ ok: true, summary, facts[], audit }` exactly as the
     architecture specifies;
   - sentinel error (`ErrNotEnoughAP`, `ErrNoEncounter`, ...) →
     `ok: false` with a structured `IMPOSSIBLE: <reason>` message so the GM
     refuses narratively instead of retrying blindly;
   - unexpected error → wrapped with `%w`, surfaced as a tool failure, never
     swallowed.
4. **Register it** in the tool registry (`internal/harness/tools`) so the
   schema reaches `harness/llm` conversation assembly. Unregistered tools do
   not exist for the GM.
5. **Add a scripted turn** to `testdata/mocksession/script.json` so the
   `ScriptedClient` replays a GM turn calling this tool, and extend the
   `state.json` snapshot assertion in `internal/harness/harness_test.go` for
   the resulting facts. The registry coverage check must pass.
6. **Update prompts** in `internal/harness/prompts` if the tool changes GM
   policy (new capability, new refusal mode, ordering rules). Preserve the
   house voice: long-form, intrigue-first, never inventing mechanical
   outcomes (AGENTS.md "House style"). Prompt assembly has golden tests —
   regenerate with `go test ./... -update` if they change.
7. Run `just test` (offline, fakes only) and `just lint`.

## Code shape (guidance, not gospel)

```go
// internal/harness/tools — names illustrative; follow existing tools.
type Tool struct {
    Name        string
    Description string
    JSONSchema  string // or a typed schema value, as the registry expects
    Handler     func(ctx context.Context, args json.RawMessage, g *Game) (ToolResult, error)
}

type ToolResult struct {
    OK      bool     `json:"ok"`
    Summary string   `json:"summary"`
    Facts   []string `json:"facts"`
    Audit   []string `json:"audit,omitempty"`
}

// handler skeleton
res, err := resolve.Execute(g.State, move, g.Rand)
if err != nil {
    if errors.Is(err, engine.ErrNotEnoughAP) {
        return ToolResult{OK: false, Summary: "IMPOSSIBLE: not enough action points"}, nil
    }
    return ToolResult{}, fmt.Errorf("tool %s: %w", name, err)
}
```

## Checklist — done when all exist

- [ ] `Tool` with name, description, and JSON schema registered in
      `internal/harness/tools`.
- [ ] Handler calls a `resolve` Move; no mechanics in the handler.
- [ ] Success maps to `{ok, summary, facts, audit}`; engine sentinel errors
      map to structured `IMPOSSIBLE:` results; other errors wrapped `%w`.
- [ ] A scripted turn in `testdata/mocksession/script.json` calls the tool;
      `internal/harness/harness_test.go` asserts the snapshot effect; the
      registry coverage check passes.
- [ ] `internal/harness/prompts` updated iff tool policy changed; golden
      tests regenerated.
- [ ] `go test ./...` passes offline; `just lint` clean.
