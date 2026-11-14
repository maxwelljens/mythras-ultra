---
id: rules-codex
name: rules-codex
description: Change game rule data or the embedded rulebook — keep Mythras-Imperative.md as the source of truth, keep docs/rules/digest.md and docs/rules/tables.md in sync as its implementation mirror, transcribe Go rule tables in internal/engine with citations to rulebook headings, keep internal/codex embedding and lookup in sync, and apply the AMBIGUOUS footnote convention. Use when adding or correcting a rule table, a digest section, a codex lookup, or embedded rulebook content.
---

# rules-codex

`Mythras-Imperative.md` (repo root) is the single source of truth.
`docs/rules/digest.md` and `docs/rules/tables.md` are its implementation-ready
mirror; the Go tables in `internal/engine/rules` and `internal/engine/combat`
are transcriptions of `tables.md`; `internal/codex` embeds the rulebook
(`internal/codex/data/`) for GM lookup. If code and digest disagree, one of
them is a bug — fix it and cite the rulebook section in the commit (AGENTS.md
golden rule 5).

## Recipe

1. **Locate the truth**: find the passage in `Mythras-Imperative.md` and note
   its heading and line range (e.g. "How Combat Works"). Never transcribe from
   memory or from a secondary source.
2. **Update the mirror first**: edit `docs/rules/digest.md` (implementation
   prose: thresholds, grades, procedure steps) and/or
   `docs/rules/tables.md` (machine-readable numeric tables) to reflect the
   rulebook exactly. Both are hand-maintained mirrors of
   `Mythras-Imperative.md` — there is no generator; edit them alongside the
   rulebook passage (ARCHITECTURE.md §12).
3. **AMBIGUOUS convention**: where the rulebook passage admits more than one
   reading, or the text is unclear/contradictory, do not silently pick an
   interpretation. Mark the digest entry with a bracketed paragraph
   `[AMBIGUOUS: <the readings, quoting the source and its location>]`, and
   record the implemented choice in the Go code as an
   `[AMBIGUOUS: … — myul implements <X> because <reason>]` comment citing the
   same section, so the choice stays auditable. Resolve toward the rulebook
   text, and record the choice where code and digest must agree.
4. **Update the Go table**: numeric rule tables live as Go data in
   `internal/engine/rules` or `internal/engine/combat`, transcribed from
   `docs/rules/tables.md`, with a comment citing the rulebook heading. Keep
   the comment and the digest citation identical in spirit — the commit cites
   the rulebook section too.
5. **Keep `internal/codex` in sync**: the embedded copy of the rulebook
   (`internal/codex/data/Mythras-Imperative.md`, byte-identical to the root
   file, via `//go:embed data/*.md`) must carry any rulebook change, and the
   search index must still find the passage. Codex search rankings are pinned
   inline in `TestSearchGolden` — update those expectations if a heading
   moves (there is no `-update` flag).
6. **Tests**: table-driven unit tests in `engine/rules`/`engine/combat` with
   fixed `dice.NewRand(seed)` where rolls are involved; assert on `Facts`
   and audit, not just pass/fail. If a rule change alters an engine outcome,
   update dependent move/tool tests and the scripts in
   `internal/harness/testdata/mocksession/`.
7. **Run `just test` (offline) and `just lint`**; commit scoped
   (`engine: ...` or `docs: ...`) citing the rulebook section.

## Code shape (guidance, not gospel)

```go
// internal/engine/rules/<table>.go — the real citation style:
// luckUses transcribes the three options of Using Luck Points
// (Mythras-Imperative.md, Game System › Luck Points L1259-1279;
// digest L760-770, tables.md §4.1).
var <table> = []struct{ ... }{ ... }
```

```markdown
<!-- docs/rules/digest.md entry -->
### Step N: <Heading> (L<line>)
...implementation prose...

[AMBIGUOUS: <readings — quote the source and its location>.]
```

## Checklist — done when all exist

- [ ] `Mythras-Imperative.md` consulted; section/heading noted and cited.
- [ ] `docs/rules/digest.md` and/or `docs/rules/tables.md` updated to match;
      AMBIGUOUS footnotes added where readings diverge.
- [ ] Go table in `internal/engine/rules` or `engine/combat` updated with a
      comment citing the rulebook heading.
- [ ] `internal/codex` embedding + lookup still serve the changed content;
      `TestSearchGolden` expectations updated if headings moved.
- [ ] Unit tests (seeded, table-driven, asserting facts/audit) cover the new
      behaviour; mock-session scripts updated if outcomes changed.
- [ ] `go test ./...` passes offline; `just lint` clean; commit message cites
      the rulebook section.
