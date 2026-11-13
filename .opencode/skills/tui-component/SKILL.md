---
id: tui-component
name: tui-component
description: Build or alter a Bubbletea UI component in internal/tui — one model per concern, strict Update/View discipline, Lipgloss styles through the 16-colour theme system with adaptive colour profile, no game logic in the UI (only engine Results rendered), Update/View unit tests without a real terminal, and accessibility toggles such as config.dice_animation. Use when touching chat, dice theatre, HUD, metadata panel, status bar, themes, or keybindings.
---

# tui-component

The TUI renders the game; it never plays it. Mechanical facts arrive as
engine `Result`s and state read-throughs — UI code must not roll dice, mutate
state, or invent outcomes (golden rule 1). One concern per file in
`internal/tui`, Bubbletea models with plain `Update`/`View`, Lipgloss styles
fed by the 16-colour theme system (ARCHITECTURE.md §7).

## Recipe

1. **Locate the concern**: chat view, dice theatre, event annotations,
   metadata panel, status bar, themes/palette, or keybindings (ARCHITECTURE.md
   §7). One Bubbletea model per concern; keep files small and single-purpose.
2. **Model**: hold only UI state — viewport position, input line + history,
   IC/OOC channel, animation frames, toggles. Game data enters as messages
   carrying engine `Result`s or state snapshots; never a copy that the UI
   can mutate as if it were truth.
3. **Update**: pattern-match on message type; no side effects beyond
   returning the updated model and commands (`tea.Cmd`). Streaming GM text,
   tool-call strips (⚙ name → outcome), and roll banners are driven by
   messages, not by querying engines mid-update.
4. **View**: pure function of model → string. Markdown chat via the
   renderer the architecture specifies; dice theatre animates (harmonica
   spring settle) only when enabled.
5. **Styling**: Lipgloss styles through the theme system — exactly 16
   colours, built-in themes `mythras`/`obsidian`/`parchment` plus `terminal`
   honouring the user's ANSI 16, adaptive to the light/dark colour profile
   (`[ui] colour mode`, config). No hard-coded hex outside the theme files.
6. **No game logic**: dice animation replays an existing roll's audit; the
   HUD/metadata panel reads state; slash commands (`/roll`, `/save`, ...)
   dispatch to harness/engine paths rather than computing outcomes locally.
7. **Accessibility**: honour `config.dice_animation` (off = instant result
   banners) and `metadata_default`; respect the terminal colour profile.
8. **Tests**: unit-test `Update` and `View` with synthetic messages and a
   fixed model — no real terminal, no network. Assert transition sequences
   and rendered strings (golden files under `testdata/` where useful;
   regenerate with `go test ./... -update`).

## Code shape (guidance, not gospel)

```go
// internal/tui/<concern>.go — names illustrative; follow existing models.
type diceModel struct {
    enabled bool        // config.dice_animation
    roll    engine.Result // last roll to render; never recomputed here
    frame   int
}

func (m diceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case rollMsg:   // engine Result arrives; start animation if enabled
    case tickMsg:   // advance frames; settle → banner
    }
    return m, nil
}

func (m diceModel) View() string {
    return theme.BannerStyle.Render(m.roll.Summary) // 16-colour theme only
}
```

## Checklist — done when all exist

- [ ] Component lives in `internal/tui`, one concern per file, pure
      `Update`/`View`, no game logic — only engine Results and state
      read-throughs rendered.
- [ ] All colour via the theme system: 16 colours, built-in/terminal themes,
      adaptive light/dark profile; no stray hard-coded colours.
- [ ] Accessibility: `config.dice_animation` (and other `[ui]` toggles)
      respected; degraded path renders the same facts without animation.
- [ ] Unit tests cover `Update` transitions and `View` output without a real
      terminal (golden files regenerated via `-update` where used).
- [ ] `go test ./...` passes offline; `just lint` clean; player-facing text
      is British-neutral English, second person, present tense.
