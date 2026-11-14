---
id: tui-component
name: tui-component
description: Build or alter a Bubbletea UI component in internal/tui — one model per concern, strict Update/View discipline, Lipgloss styles through the 16-colour theme system with adaptive colour profile, no game logic in the UI (only engine Results rendered), Update/View unit tests without a real terminal, and accessibility toggles such as ui.dice_animation. Use when touching chat, dice theatre, HUD, metadata panel, status bar, themes, or keybindings.
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
   renderer the architecture specifies; dice theatre animates (number
   cycling, per-die snap, result hold via `tea.Tick`) only when enabled.
5. **Styling**: Lipgloss styles through the theme system — exactly 16
   colours, built-in themes `mythras`/`obsidian`/`parchment` plus `terminal`
   honouring the user's ANSI 16, adaptive to the light/dark colour profile
   (`[ui] colour mode`, config). No hard-coded hex outside the theme files.
6. **No game logic**: dice animation replays an existing roll's audit; the
   HUD/metadata panel reads state; commands (`save`, `recap`, `roll`,
   `meta`) travel over `Driver.Command` to harness/engine paths instead of
   being computed locally — typed input is always a chat message.
7. **Accessibility**: honour `ui.dice_animation` (off = instant result
   banners) and `metadata_default`; respect the terminal colour profile.
8. **Tests**: unit-test `Update` and `View` with synthetic messages and a
   fixed model — no real terminal, no network, no real timers (tick messages
   are injected directly). Assert transition sequences and rendered strings
   inline; there are no golden files and no `-update` flag.

## Code shape (the dice theatre, `internal/tui/dice.go`)

```go
// The app shell (app.go) is the tea.Model: Update(msg tea.Msg) (tea.Model,
// tea.Cmd). Sub-concerns expose their own Update/View; DiceTheatre's Update
// returns the next step's command plus a done flag.
type DiceTheatre struct {
    theme   Theme      // 16-colour theme; styles come from the theme system
    enabled bool       // config ui.dice_animation; false renders at once
    phase   dicePhase  // idle → rolling → settle → result
    ev      DiceEvent  // last roll to render; never recomputed here
    ...
}

func (d *DiceTheatre) Start(ev DiceEvent) tea.Cmd // nil when disabled: banner at once
func (d *DiceTheatre) Update(msg tea.Msg) (tea.Cmd, bool) // next tick; done = flush banner
func (d *DiceTheatre) View() string                // empty when idle
```

## Checklist — done when all exist

- [ ] Component lives in `internal/tui`, one concern per file, pure
      `Update`/`View`, no game logic — only engine Results and state
      read-throughs rendered.
- [ ] All colour via the theme system: 16 colours, built-in/terminal themes,
      adaptive light/dark profile; no stray hard-coded colours.
- [ ] Accessibility: `ui.dice_animation` (and other `[ui]` toggles)
      respected; degraded path renders the same facts without animation.
- [ ] Unit tests cover `Update` transitions and `View` output without a real
      terminal or timers (inline expectations — no golden files).
- [ ] `go test ./...` passes offline; `just lint` clean; player-facing text
      is British-neutral English, second person, present tense.
