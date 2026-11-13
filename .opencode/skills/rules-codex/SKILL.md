---
id: rules-codex
name: rules-codex
description: Change game rule data or the embedded rulebook — keep Mythras-Imperative.md as the source of truth, regenerate docs/rules/digest.md and docs/rules/tables.md as its implementation mirror, transcribe Go rule tables in internal/engine with citations to rulebook headings, keep internal/codex embedding and lookup in sync, and apply the AMBIGUOUS footnote convention. Use when adding or correcting a rule table, a digest section, a codex lookup, or embedded rulebook content.
---

# rules-codex

`Mythras-Imperative.md` (repo root) is the single source of truth.
`docs/rules/digest.md` and `docs/rules/tables.md` are its implementation-ready
mirror; the Go tables in `internal/engine/rules` and `internal/engine/combat`
are transcriptions of `tables.md`; `internal/codex` embeds the rulebook and
digest for GM lookup. If code and digest disagree, one of them is a bug — fix
it and cite the rulebook section in the commit (AGENTS.md golden rule 5).

## Recipe

1. **Locate the truth**: find the passage in `Mythras-Imperative.md` and note
   its heading/section number (e.g. "Combat §3.2"). Never transcribe from
   memory or from a secondary source.
2. **Update the mirror first**: edit `docs/rules/digest.md` (implementation
   prose: thresholds, grades, procedure steps) and/or
   `docs/rules/tables.md` (machine-readable numeric tables) to reflect the
   rulebook exactly. These files are generated/derived from
   `Mythras-Imperative.md` — regenerate them with the existing tooling rather
   than hand-editing when a generator applies (ARCHITECTURE.md §12).
3. **AMBIGUOUS convention**: where the rulebook passage admits more than one
   reading, or the text is unclear/contradictory, do not silently pick an
   interpretation. Footnote the digest entry as `AMBIGUOUS: <the readings> —
   myul implements <X> because <reason>` with the rulebook section cited, so
   the implemented choice is auditable. Resolve toward the rulebook text, and
   record the choice where code and digest must agree.
4. **Update the Go table**: numeric rule tables live as Go data in
   `internal/engine/rules` or `internal/engine/combat`, transcribed from
   `docs/rules/tables.md`, with a comment citing the rulebook heading. Keep
   the comment and the digest citation identical in spirit — the commit cites
   the rulebook section too.
5. **Keep `internal/codex` in sync**: the embedded rulebook + digest must
   include the changed files (check embedding directives/FS and the
   search/lookup index). Add or adjust lookup coverage so the GM's
   `rules_lookup` tool finds the changed passage. Codex search results have
   golden tests — regenerate with `go test ./... -update`.
6. **Tests**: table-driven unit tests in `engine/rules`/`engine/combat` with
   fixed `dice.NewRand(seed)` where rolls are involved; assert on `Facts`
   and audit, not just pass/fail. If a rule change alters an engine outcome,
   update dependent move/tool tests and any `testdata/mocksession/`
   snapshots.
7. **Run `just test` (offline) and `just lint`**; commit scoped
   (`engine: ...` or `docs: ...`) citing the rulebook section.

## Code shape (guidance, not gospel)

```go
// internal/engine/rules/<table>.go
// Transcribed from docs/rules/tables.md — Mythras Imperative, <Heading §N>.
var SuccessGrades = [...]Grade{
    {Name: "Special", Threshold: ...},
    ...
}
```

```markdown
<!-- docs/rules/digest.md entry -->
### <Heading §N> — as per Mythras-Imperative.md
...implementation prose...
[^amb]: AMBIGUOUS: <readings> — myul implements <X> because <reason>.
```

## Checklist — done when all exist

- [ ] `Mythras-Imperative.md` consulted; section/heading noted and cited.
- [ ] `docs/rules/digest.md` and/or `docs/rules/tables.md` regenerated or
      updated to match; AMBIGUOUS footnotes added where readings diverge.
- [ ] Go table in `internal/engine/rules` or `engine/combat` updated with a
      comment citing the rulebook heading.
- [ ] `internal/codex` embedding + lookup still serve the changed content;
      golden tests regenerated with `-update` if needed.
- [ ] Unit tests (seeded, table-driven, asserting facts/audit) cover the new
      behaviour; mock-session snapshots updated if outcomes changed.
- [ ] `go test ./...` passes offline; `just lint` clean; commit message cites
      the rulebook section.
