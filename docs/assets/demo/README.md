# README demo clip

`demo.ts` generates `docs/assets/demo.gif`, the promotional clip embedded in
the project README. It renders an animated, faithful mock-up of the `myul`
TUI (default `mythras` theme, real layout and session content) as SVG frames,
rasterises them with `rsvg-convert`, and assembles a palette-optimised looping
GIF with `ffmpeg`. Everything runs offline and deterministically — no LLM, no
terminal capture, no Node.

## Regenerate

```sh
just demo                          # → docs/assets/demo.gif
# or directly:
deno run -A docs/assets/demo/demo.ts
```

Dependencies: `deno`, `rsvg-convert` (librsvg), `ffmpeg`, and the Fantasque
Sans Mono Nerd Font (falls back to DejaVu Sans Mono).

Useful flags:

- `--out PATH` — output GIF path (default `docs/assets/demo.gif`).
- `--fps N` — frame rate (default 12.5; GIF delays are integer centiseconds,
  so 12.5/10/20/25 play back at exactly the requested rate).
- `--width N` — pixel width; height is 16:10 (default 960).
- `--keep-frames` — keep intermediate SVG/PNG frames in `frames/` for
  inspection (not committed).

The palette, layout metrics and narrative content are mirrored from
`internal/tui` and `internal/harness/testdata/mocksession/`; if the TUI theme
or layout changes, update the constants at the top of `demo.ts`.