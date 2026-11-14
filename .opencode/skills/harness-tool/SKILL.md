---
id: harness-tool
name: harness-tool
description: Expose a new GM-facing tool in internal/harness/tools — Tool struct with name/description/JSON schema, handler translating an engine Result into {ok, summary, facts, audit} and sentinel errors into structured IMPOSSIBLE: results, registry wiring, mock-session coverage in internal/harness/testdata/mocksession, and prompt updates in internal/harness/prompts when tool policy changes. Use when adding or changing any tool the LLM game master can call.
---

# harness-tool

Tools are the *only* way mechanics happen for the GM (ARCHITECTURE.md §3.2).
A tool is a thin, pure adapter: parse/validate arguments → call a `resolve`
Move → project the `Result` into `{ok, summary, facts, audit}`; engine
sentinel errors surface as structured `IMPOSSIBLE:` refusals (`Registry.Dispatch`
does the translation). Every registered tool must be exercised at least once
by the mock session — the coverage check walks the registry.

## Recipe

1. **Write (or reuse) the engine Move first.** The handler never implements
   mechanics; see the `engine-move` skill. If the Move already exists, skip
   to step 2.
2. **Define the `Tool`** in `internal/harness/tools`: `Name` (snake_case,
   matching the family conventions in ARCHITECTURE.md §6.1), `Description`
   (what the GM may use it for, and when not to), and `Schema` for the
   argument object (an object JSON Schema). Schemas are what the LLM sees —
   keep them tight and typed.
3. **Write the handler**: unmarshal/validate args → dispatch the Move → map
   the outcome:
   - success → `{ ok: true, summary, facts[], audit }` exactly as the
     architecture specifies;
   - engine sentinel error (`ErrNotEnoughAP`, `ErrNoEncounter`, ...) →
     return it: `Registry.Dispatch` turns it into `ok: false` with a
     structured `IMPOSSIBLE: <reason>` the GM refuses narratively instead
     of retrying blindly;
   - unusable arguments → return an error wrapping `ErrInvalidArgs`;
     anything else is a harness fault (`ErrHarnessFault`), wrapped `%w`
     and never swallowed.
4. **Register it** in the tool registry (`internal/harness/tools`) so the
   schema reaches `harness/llm` conversation assembly. Unregistered tools do
   not exist for the GM.
5. **Add a scripted turn** to `internal/harness/testdata/mocksession/script.json`
   so `llm.ScriptedClient` replays a GM turn calling this tool, and extend the
   state assertions in `internal/harness/mocksession_test.go` for the
   resulting facts. The registry coverage check must pass.
6. **Update prompts** in `internal/harness/prompts` if the tool changes GM
   policy (new capability, new refusal mode, ordering rules). Preserve the
   house voice: long-form, intrigue-first, never inventing mechanical
   outcomes (AGENTS.md "House style"). Prompt assembly is pinned by inline
   content assertions in `prompt_test.go` — update them with the text (there
   is no `-update` flag).
7. Run `just test` (offline, fakes only) and `just lint`.

## Code shape (the contract — `internal/harness/tools/tool.go`)

```go
// internal/harness/tools — the real contract (tool.go).
type Tool struct {
    Name        string
    Description string
    Schema      map[string]any
    Handler     func(ctx context.Context, s *resolve.State, args json.RawMessage) (Result, error)
}

type Result struct {
    OK      bool           `json:"ok"`
    Summary string         `json:"summary"`
    Facts   []string       `json:"facts"`
    Audit   *resolve.Audit `json:"audit,omitempty"`
}

// handler skeleton — handlers return errors, never translate them:
// Registry.Dispatch maps ErrInvalidArgs and engine sentinels to
// structured "IMPOSSIBLE: ..." results.
res, err := resolve.Apply(s, move)
if err != nil {
    return Result{}, err
}
return fromMove(res), nil
```

## Checklist — done when all exist

- [ ] `Tool` with name, description, and JSON schema registered in
      `internal/harness/tools`.
- [ ] Handler calls a `resolve` Move; no mechanics in the handler.
- [ ] Success maps to `{ok, summary, facts, audit}`; engine sentinel errors
      map to structured `IMPOSSIBLE:` results; other errors wrapped `%w`.
- [ ] A scripted turn in `internal/harness/testdata/mocksession/script.json`
      calls the tool; `internal/harness/mocksession_test.go` asserts the
      resulting state; the registry coverage check passes.
- [ ] `internal/harness/prompts` updated iff tool policy changed; the inline
      prompt assertions updated with the text.
- [ ] `go test ./...` passes offline; `just lint` clean.
