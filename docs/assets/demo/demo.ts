// SPDX-License-Identifier: EUPL-1.2
//
// Promotional GIF generator for the README.
//
// Renders a faithful, animated mock-up of the myul TUI as a sequence of SVG
// frames (Deno), rasterises them with rsvg-convert, and assembles a
// palette-optimised looping GIF with ffmpeg. Fully offline and deterministic.
//
// The palette, layout, spinner frame sets, transcript formats and narrative
// content mirror internal/tui (default `mythras` theme, dark variant) and
// internal/harness/testdata/mocksession/script.json (seed 42, Wren Ashdown).
//
// Usage:
//   deno run -A docs/assets/demo/demo.ts [--out docs/assets/demo.gif]
//       [--fps 12.5] [--width 960] [--keep-frames] [--at 7.2]
//
// Requires: deno, rsvg-convert, ffmpeg (with palettegen/paletteuse).

// ---------------------------------------------------------------------------
// CLI args
// ---------------------------------------------------------------------------

function arg(name: string, fallback: string): string {
  const i = Deno.args.indexOf(`--${name}`);
  return i >= 0 && Deno.args[i + 1] ? Deno.args[i + 1] : fallback;
}
const flag = (name: string) => Deno.args.includes(`--${name}`);

const OUT = arg("out", "docs/assets/demo.gif");
const FPS = Number(arg("fps", "12.5")); // 8cs GIF delay — exact, no muxer rounding
const WIDTH = Number(arg("width", "960"));
const HEIGHT = Math.round(WIDTH * (640 / 960));
const KEEP_FRAMES = flag("keep-frames");
const AT = arg("at", ""); // render a single frame at time t (preview)

// ---------------------------------------------------------------------------
// Theme — internal/tui/theme.go, `mythras` dark variant.
// ---------------------------------------------------------------------------

const C = {
  bg: "#17101f", // SlotBackground
  fg: "#ece4f4", // SlotForeground
  accent: "#d9a441", // SlotAccent
  muted: "#8a7d9b", // SlotMuted
  highlight: "#f2c971", // SlotHighlight
  border: "#3b2b52", // SlotBorder
  header: "#cbb7e6", // SlotHeader
  footer: "#7f7291", // SlotFooter
  panel: "#b9a8d4", // SlotPanel
  speaker: "#e0b354", // SlotSpeaker
  tool: "#7fb8a8", // SlotTool
  dice: "#f0d38a", // SlotDice
  critical: "#ffe08a", // SlotCritical
  success: "#7fd08a", // SlotSuccess
  failure: "#e0954f", // SlotFailure
  fumble: "#e05a78", // SlotFumble
};

// ---------------------------------------------------------------------------
// SVG helpers
// ---------------------------------------------------------------------------

const FONT = "FantasqueSansM Nerd Font Mono, DejaVu Sans Mono, monospace";
const FS = 16;
const ADV = 8.28; // measured Fantasque Sans Mono advance at 16px (0.5175em)
const LH = 22;

const esc = (s: string) =>
  s.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");

interface TextOpts {
  fill?: string;
  bold?: boolean;
  italic?: boolean;
  size?: number;
  opacity?: number;
  anchor?: "start" | "middle" | "end";
  spacing?: number;
  adv?: number; // override advance (panel uses a smaller size)
}

function text(x: number, y: number, s: string, o: TextOpts = {}): string {
  if (s === "") return "";
  const size = o.size ?? FS;
  const attrs = [
    `x="${x.toFixed(2)}"`,
    `y="${y.toFixed(2)}"`,
    `font-family="${FONT}"`,
    `font-size="${size}"`,
    `fill="${o.fill ?? C.fg}"`,
    o.bold ? 'font-weight="bold"' : "",
    o.italic ? 'font-style="italic"' : "",
    o.opacity !== undefined ? `opacity="${o.opacity.toFixed(3)}"` : "",
    o.anchor ? `text-anchor="${o.anchor}"` : "",
    o.spacing ? `letter-spacing="${o.spacing}"` : "",
    'xml:space="preserve"',
  ].filter(Boolean).join(" ");
  return `<text ${attrs}>${esc(s)}</text>`;
}

function rect(
  x: number,
  y: number,
  w: number,
  h: number,
  fill: string,
  r = 0,
  opacity = 1,
  stroke?: string,
  strokeWidth = 1,
): string {
  if (w <= 0 || h <= 0) return "";
  return `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${r}" fill="${fill}"${opacity < 1 ? ` opacity="${opacity.toFixed(3)}"` : ""}${stroke ? ` stroke="${stroke}" stroke-width="${strokeWidth}"` : ""}/>`;
}

function group(body: string, opacity = 1, transform = ""): string {
  if (opacity <= 0.001) return "";
  const attrs = [
    opacity < 1 ? `opacity="${opacity.toFixed(3)}"` : "",
    transform ? `transform="${transform}"` : "",
  ].filter(Boolean).join(" ");
  return `<g ${attrs}>${body}</g>`;
}

function svgFrame(body: string): string {
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${WIDTH}" height="${HEIGHT}" viewBox="0 0 ${WIDTH} ${HEIGHT}">
<defs>
  <linearGradient id="bgGrad" x1="0" y1="0" x2="0" y2="1">
    <stop offset="0" stop-color="#1d1528"/>
    <stop offset="1" stop-color="#0d0a14"/>
  </linearGradient>
  <radialGradient id="vignette" cx="0.5" cy="0.42" r="0.85">
    <stop offset="0.55" stop-color="#000000" stop-opacity="0"/>
    <stop offset="1" stop-color="#000000" stop-opacity="0.42"/>
  </radialGradient>
  <clipPath id="bodyClip"><rect x="${L.chatX}" y="${L.bodyTop}" width="${L.chatW}" height="${L.bodyH}"/></clipPath>
  <clipPath id="panelClip"><rect x="${L.panelX + 1}" y="${L.bodyTop + 1}" width="${L.panelW - 2}" height="${L.bodyH - 2}"/></clipPath>
</defs>
${body}
</svg>`;
}

// ---------------------------------------------------------------------------
// Easing, timing, PRNG
// ---------------------------------------------------------------------------

const clamp01 = (t: number) => Math.max(0, Math.min(1, t));
const span = (t: number, a: number, b: number) => clamp01((t - a) / (b - a));
const easeOut = (t: number) => 1 - Math.pow(1 - t, 3);
const easeInOut = (t: number) =>
  t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Greedy word wrap at a column count. */
function wrap(s: string, cols: number): string[] {
  const out: string[] = [];
  for (const para of s.split("\n")) {
    let line = "";
    for (const word of para.split(" ")) {
      if (line === "") line = word;
      else if (line.length + 1 + word.length <= cols) line += " " + word;
      else {
        out.push(line);
        line = word;
      }
    }
    out.push(line);
  }
  return out;
}

// ---------------------------------------------------------------------------
// Layout — mirrors App.View: header / body(chat+panel) / status / input /
// footer, inside a terminal window chrome.
// ---------------------------------------------------------------------------

const L = (() => {
  const winX = 20, winY = 18, winW = WIDTH - 40, winH = HEIGHT - 36;
  const titleH = 34;
  const padX = 12;
  const x0 = winX + padX;
  const innerW = winW - padX * 2;
  const y0 = winY + titleH + 12;
  const bottom = winY + winH - 10;
  const footerY = bottom - 4;
  const inputY = footerY - LH;
  const statusY = inputY - LH;
  const bodyTop = y0 + LH + 4;
  const bodyH = statusY - LH + 6 - bodyTop;
  const panelW = Math.round(innerW * 0.3);
  const panelX = x0 + innerW - panelW;
  const chatX = x0;
  const chatW = panelX - chatX - 14;
  return {
    winX, winY, winW, winH, titleH, x0, innerW, y0, bottom,
    footerY, inputY, statusY, bodyTop, bodyH, panelW, panelX, chatX, chatW,
    chatCols: Math.floor(chatW / ADV),
    headerY: y0 + 16,
  };
})();

// Panel uses a slightly smaller metric so all seven sections fit.
const PFS = 14, PADV = PFS * 0.5175, PLH = 18.5;
const PCOLS = Math.floor((L.panelW - 22) / PADV);

// ---------------------------------------------------------------------------
// Script content — authentic formats and mock-session facts.
// ---------------------------------------------------------------------------

const SESSION_NAME = "The Ferry-God's Shrine";
const MODEL_NAME = "qwen3-max";
const PLAYER_LINE = "I search the shrine for offerings.";
const SCENE_LINE =
  "A roofless stone shrine to a forgotten ferry-god, wind-scoured grass around it, the road a pale ribbon below.";
const GM_LINE_1 =
  "You kneel at the altar stone and sweep the grit aside. Your fingers find a seam, then a hollow ring beneath the flagstone — a hiding place, cut and fitted by a careful hand.";
const GM_LINE_2 =
  "The torc is cold as river ice. The wax tablet names the baron's burners — and the price they paid.";
const ANNOTATION_1 =
  "⚙ skill_test → Perception test: 04 vs 65% (standard) — critical";
const ANNOTATION_2 = "⚙ inventory_add → Wren Ashdown gains Silver torc";
const ANNOTATION_3 = "⚙ world_set_flag → shrine_searched: – → true";
const BANNER = "── 1d100 [04] = 04 · CRITICAL ──";

// Timeline (seconds).
const T = {
  titleEnd: 1.5,
  termIn: 1.9,
  typeStart: 2.3,
  typeEnd: 4.0,
  send: 4.15,
  thinkStart: 4.2,
  rollStart: 4.75,
  settle: 5.85,
  result: 6.05,
  file: 6.8,
  stream1Start: 7.0,
  stream1End: 9.6,
  tools2: 9.7,
  hud: 10.1,
  stream2Start: 10.5,
  stream2End: 11.9,
  idle: 12.5,
  outroIn: 13.0,
  outroOut: 15.1,
  loopReady: 15.8,
  duration: 16,
};

// ---------------------------------------------------------------------------
// Terminal pieces
// ---------------------------------------------------------------------------

function windowChrome(): string {
  const { winX, winY, winW, winH, titleH } = L;
  const parts = [
    // soft shadow
    rect(winX + 4, winY + 8, winW, winH, "#000000", 12, 0.35),
    rect(winX, winY, winW, winH, C.bg, 10, 1, C.border, 1.5),
    // title bar
    `<path d="M ${winX} ${winY + 10} q 0 -10 10 -10 h ${winW - 20} q 10 0 10 10 v ${titleH - 10} h ${-winW} z" fill="#1d1528"/>`,
    rect(winX, winY + titleH, winW, 1, C.border),
    rect(winX + 16, winY + 13, 9, 9, C.fumble, 4.5),
    rect(winX + 32, winY + 13, 9, 9, C.failure, 4.5),
    rect(winX + 48, winY + 13, 9, 9, C.success, 4.5),
    text(winX + winW / 2, winY + 22, `myul — ${SESSION_NAME.toLowerCase()}`, {
      fill: C.muted,
      size: 13,
      anchor: "middle",
    }),
  ];
  return parts.join("\n");
}

function headerRow(): string {
  const y = L.headerY;
  const sep = " · ";
  let x = L.x0;
  const put = (s: string, fill: string, bold = false) => {
    const el = text(x, y, s, { fill, bold });
    x += s.length * ADV;
    return el;
  };
  const parts = [
    put(SESSION_NAME, C.header),
    put(sep, C.muted),
    put("[IC]", C.highlight, true),
    put(" ", C.muted),
    put("OOC", C.muted),
    put(sep, C.muted),
    put(MODEL_NAME, C.muted),
    put(sep, C.muted),
    put("save –", C.muted),
    put(sep, C.muted),
    put("decision –", C.muted),
  ];
  return parts.join("");
}

function footerRow(): string {
  const hints = "enter send · ctrl+c quit · ctrl+p commands";
  const usage = "12.4K (6%)";
  return [
    text(L.x0, L.footerY, hints, { fill: C.footer, opacity: 0.8 }),
    text(L.x0 + L.innerW, L.footerY, usage, {
      fill: C.muted,
      anchor: "end",
    }),
  ].join("");
}

const MINI_DOT = ["⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"];
const WRITE_FRAMES = ["▉", "▊", "▋", "▍", "▎", "▏", "▎", "▍", "▋", "▊"];

function statusRow(t: number): string {
  let frame = "", label = "", fill = C.accent;
  if (t >= T.thinkStart && t < T.file) {
    frame = MINI_DOT[Math.floor(t * 8) % MINI_DOT.length];
    label = "Thinking...";
    fill = C.accent;
  } else if (
    (t >= T.file && t < T.tools2) ||
    (t >= T.stream2Start && t < T.stream2End)
  ) {
    frame = WRITE_FRAMES[Math.floor(t * 8) % WRITE_FRAMES.length];
    label = "Thinking...";
    fill = C.success;
  } else if (t >= T.tools2 && t < T.stream2Start) {
    frame = MINI_DOT[Math.floor(t * 8) % MINI_DOT.length];
    label = "Thinking...";
    fill = C.accent;
  }
  if (!frame) return "";
  return text(L.x0, L.statusY, `${frame} ${label}`, { fill });
}

function inputRow(t: number): string {
  const prompt = text(L.x0, L.inputY, "> ", { fill: C.accent, bold: true });
  let typed = "";
  if (t >= T.typeStart && t < T.send) {
    const n = Math.floor(span(t, T.typeStart, T.typeEnd) * PLAYER_LINE.length);
    typed = PLAYER_LINE.slice(0, n);
  }
  const cursorOn = t < T.typeStart || t >= T.send
    ? Math.floor(t * 2) % 2 === 0
    : true;
  const cx = L.x0 + (2 + typed.length) * ADV;
  const cursor = cursorOn
    ? rect(cx, L.inputY - 13, ADV, 17, C.accent, 0, 0.85)
    : "";
  return prompt + text(L.x0 + 2 * ADV, L.inputY, typed, { fill: C.fg }) + cursor;
}

// ---------------------------------------------------------------------------
// Metadata panel
// ---------------------------------------------------------------------------

function panel(t: number): string {
  const { panelX, panelW, bodyTop, bodyH } = L;
  const x = panelX + 11;
  let y = bodyTop + 18;
  const lines: string[] = [
    rect(panelX, bodyTop, panelW, bodyH, "none", 0, 1, C.border, 1),
  ];
  const put = (s: string, fill: string, o: TextOpts = {}) => {
    for (const ln of wrap(s, PCOLS)) {
      lines.push(text(x, y, ln, { fill, size: PFS, ...o }));
      y += PLH;
    }
  };
  const section = (name: string) => {
    y += 2;
    put(name, C.accent, { bold: true });
  };

  put("METADATA", C.header, { bold: true });
  section("CHARACTER");
  put("Wren Ashdown", C.fg, { bold: true });
  put("Civilised · Hunter", C.panel);
  put("HP 34/40", C.panel);
  put("AP 3/4", C.panel);
  put("PP 5/7", C.panel);
  put("Luck 2", C.panel);
  section("LOCATIONS");
  put("—", C.muted);
  section("INVENTORY");
  const torcOn = t >= T.hud;
  const flash = torcOn ? 1 - span(t, T.hud, T.hud + 0.7) : 0;
  if (!torcOn) {
    put("Broadsword · Bow", C.panel);
  } else {
    // One line; the new item flashes highlight before settling to panel.
    const pre = "Broadsword · Bow · ";
    lines.push(text(x, y, pre, { fill: C.panel, size: PFS }));
    lines.push(
      text(x + pre.length * PADV, y, "Silver torc", {
        fill: flash > 0 ? C.highlight : C.panel,
        size: PFS,
        bold: flash > 0.4,
        opacity: 0.55 + 0.45 * span(t, T.hud, T.hud + 0.25),
      }),
    );
    y += PLH;
  }
  section("SCENE");
  put("A roofless stone shrine to a forgotten ferry-god.", C.panel);
  section("CLOCKS");
  put("—", C.muted);
  section("FLAGS");
  if (t < T.hud) put("—", C.muted);
  else {
    put("shrine_searched = true", C.panel, {
      opacity: span(t, T.hud, T.hud + 0.4),
    });
  }
  section("SESSION");
  put(SESSION_NAME, C.panel);
  put("13th of Seedfall", C.panel);

  return group(lines.join("\n"), 1, "");
}

// ---------------------------------------------------------------------------
// Chat transcript
// ---------------------------------------------------------------------------

interface Line {
  s: string;
  fill: string;
  bold?: boolean;
  faint?: boolean;
  center?: boolean;
  size?: number;
  opacity?: number;
}

function transcriptLines(t: number): Line[] {
  const out: Line[] = [];
  const prose = (s: string, faint = false) => {
    for (const ln of wrap(s, L.chatCols)) {
      out.push({ s: ln, fill: C.fg, faint });
    }
    out.push({ s: "", fill: C.fg });
  };
  const tag = (s: string) => out.push({ s, fill: C.speaker, bold: true });

  // Opening scene-setting (already present when the window appears).
  tag("GM");
  prose(SCENE_LINE);

  // Player turn.
  if (t >= T.send) {
    tag("You");
    prose(PLAYER_LINE, true);
  }

  // Dice theatre, spliced at the tail while the roll is on stage.
  if (t >= T.rollStart && t < T.file) {
    const cx = L.chatW / 2;
    if (t < T.settle) {
      const rnd = mulberry32(Math.floor(t * FPS) * 7 + 13);
      const face = String(Math.floor(rnd() * 100)).padStart(2, "0");
      out.push({ s: "⟳ 1d100 rolling…", fill: C.dice, center: true });
      out.push({ s: `[${face}]`, fill: C.dice, center: true, bold: true });
    } else if (t < T.result) {
      out.push({ s: "⟳ 1d100 settling…", fill: C.dice, center: true });
      out.push({ s: "[04]", fill: C.highlight, center: true, bold: true });
    } else {
      out.push({ s: "✦ CRITICAL ✦", fill: C.critical, center: true, bold: true });
      out.push({ s: "1d100 [04] = 04", fill: C.dice, center: true });
    }
    out.push({ s: "", fill: C.fg });
    return out;
  }

  // Filed roll: annotation + banner.
  if (t >= T.file) {
    out.push({ s: ANNOTATION_1, fill: C.tool });
    out.push({ s: BANNER, fill: C.dice, bold: true, center: true });
    out.push({ s: "", fill: C.fg });
  }

  // First GM narrative, streamed word by word.
  if (t >= T.stream1Start) {
    const words = GM_LINE_1.split(" ");
    const n = Math.max(
      1,
      Math.floor(span(t, T.stream1Start, T.stream1End) * words.length),
    );
    tag("GM");
    prose(words.slice(0, n).join(" "));
  }

  // Second tool round.
  if (t >= T.tools2) {
    const o = span(t, T.tools2, T.tools2 + 0.25);
    out.push({ s: ANNOTATION_2, fill: C.tool, opacity: o });
    out.push({ s: ANNOTATION_3, fill: C.tool, opacity: o });
    out.push({ s: "", fill: C.fg });
  }

  // Final GM line.
  if (t >= T.stream2Start) {
    const words = GM_LINE_2.split(" ");
    const n = Math.max(
      1,
      Math.floor(span(t, T.stream2Start, T.stream2End) * words.length),
    );
    tag("GM");
    prose(words.slice(0, n).join(" "));
  }

  return out;
}

function chat(t: number): string {
  const lines = transcriptLines(t);
  const totalH = lines.length * LH;
  const offset = Math.max(0, totalH - L.bodyH + LH * 0.5);
  const els: string[] = [];
  let y = L.bodyTop + 16 - offset;
  for (const ln of lines) {
    if (y > L.bodyTop - LH && y < L.bodyTop + L.bodyH + LH) {
      if (ln.center) {
        // Centre within the chat column; colour the outcome word.
        if (ln.s === BANNER) {
          const base = ln.s.replace("CRITICAL", "");
          const wBase = base.length * ADV;
          const wCrit = "CRITICAL".length * ADV;
          const startX = L.chatX + (L.chatW - (wBase + wCrit)) / 2;
          const [pre, post] = ln.s.split("CRITICAL");
          els.push(text(startX, y, pre, { fill: C.dice, bold: true }));
          els.push(
            text(startX + pre.length * ADV, y, "CRITICAL", {
              fill: C.critical,
              bold: true,
            }),
          );
          els.push(
            text(startX + (pre.length + 8) * ADV, y, post, {
              fill: C.dice,
              bold: true,
            }),
          );
        } else {
          els.push(
            text(L.chatX + L.chatW / 2, y, ln.s, {
              fill: ln.fill,
              bold: ln.bold,
              anchor: "middle",
              opacity: ln.opacity,
            }),
          );
        }
      } else {
        els.push(
          text(L.chatX, y, ln.s, {
            fill: ln.fill,
            bold: ln.bold,
            opacity: ln.faint ? 0.62 : ln.opacity,
          }),
        );
      }
    }
    y += LH;
  }
  return group(els.join("\n"), 1, "");
}

// ---------------------------------------------------------------------------
// Scenes
// ---------------------------------------------------------------------------

function terminal(t: number): string {
  const body = [
    windowChrome(),
    headerRow(),
    `<g clip-path="url(#bodyClip)">${chat(t)}</g>`,
    `<g clip-path="url(#panelClip)">${panel(t)}</g>`,
    statusRow(t),
    inputRow(t),
    footerRow(),
  ].join("\n");
  return body;
}

function titleCard(t: number, opacity: number): string {
  if (opacity <= 0.001) return "";
  const cx = WIDTH / 2;
  const cy = HEIGHT / 2;
  const cursorOn = Math.floor(t * 2) % 2 === 0;
  const els = [
    rect(0, 0, WIDTH, HEIGHT, "url(#bgGrad)"),
    // faint d100 watermark
    text(cx, cy + 150, "d100", {
      fill: C.border,
      size: 220,
      bold: true,
      anchor: "middle",
      opacity: 0.16,
    }),
    rect(cx - 250, cy - 64, 130, 1, C.border),
    rect(cx + 120, cy - 64, 130, 1, C.border),
    text(cx, cy - 24, "MYTHRAS ULTRA", {
      fill: C.accent,
      size: 46,
      bold: true,
      anchor: "middle",
      spacing: 6,
    }),
    text(cx, cy + 16, "An LLM game master. A deterministic d100 engine.", {
      fill: C.fg,
      size: 17,
      anchor: "middle",
      opacity: 0.92,
    }),
    text(cx, cy + 44, "Mythras Imperative · in your terminal", {
      fill: C.muted,
      size: 14,
      anchor: "middle",
    }),
    text(cx - 44, cy + 92, "❯ myul new", { fill: C.accent, size: 16 }),
    cursorOn
      ? rect(cx - 44 + 10 * ADV, cy + 92 - 13, ADV, 17, C.accent, 0, 0.85)
      : "",
    rect(0, 0, WIDTH, HEIGHT, "url(#vignette)"),
  ];
  return group(els.join("\n"), opacity);
}

function outro(t: number, opacity: number): string {
  if (opacity <= 0.001) return "";
  const cx = WIDTH / 2;
  const cy = HEIGHT / 2;
  const cursorOn = Math.floor(t * 2) % 2 === 0;
  const els = [
    rect(0, 0, WIDTH, HEIGHT, C.bg, 0, 0.93 * opacity),
    text(cx, cy - 58, "MYTHRAS ULTRA", {
      fill: C.accent,
      size: 30,
      bold: true,
      anchor: "middle",
      spacing: 4,
    }),
    rect(cx - 190, cy - 34, 380, 1, C.border),
    text(cx, cy + 2, "Every roll audited. Every fact earned.", {
      fill: C.fg,
      size: 19,
      anchor: "middle",
    }),
    text(cx, cy + 34, "dice · combat · inventory · world state — only the engine moves them", {
      fill: C.muted,
      size: 13.5,
      anchor: "middle",
    }),
    text(cx - 44, cy + 84, "❯ myul new", { fill: C.accent, size: 16 }),
    cursorOn
      ? rect(cx - 44 + 10 * ADV, cy + 84 - 13, ADV, 17, C.accent, 0, 0.85)
      : "",
  ];
  return group(els.join("\n"), opacity);
}

// ---------------------------------------------------------------------------
// Compositor
// ---------------------------------------------------------------------------

function renderFrame(t: number): string {
  const parts: string[] = [rect(0, 0, WIDTH, HEIGHT, "url(#bgGrad)")];

  // Terminal visibility: fades in after the title, dims never (outro overlays).
  const termIn = easeOut(span(t, T.titleEnd, T.termIn));
  const termScale = lerp(0.985, 1, termIn);
  const termOp = t < T.titleEnd ? 0 : termIn;
  if (termOp > 0) {
    const tx = (WIDTH / 2) * (1 - termScale);
    const ty = (HEIGHT / 2) * (1 - termScale);
    parts.push(
      group(
        terminal(t),
        termOp,
        `translate(${tx.toFixed(2)} ${ty.toFixed(2)}) scale(${termScale.toFixed(4)})`,
      ),
    );
  }

  // Title card: full at t=0, fades out into the terminal.
  const titleOp = t < T.titleEnd ? 1 : 1 - span(t, T.titleEnd, T.termIn);
  parts.push(titleCard(t, titleOp));

  // Outro: overlays the live TUI, then crossfades back to the title card so
  // the GIF loops seamlessly (frame at t≈16 equals frame at t=0).
  if (t >= T.idle) {
    const inOp = easeInOut(span(t, T.idle, T.outroIn));
    const outOp = 1 - easeInOut(span(t, T.outroOut, T.loopReady));
    parts.push(outro(t, inOp * outOp));
    const backOp = easeInOut(span(t, T.outroOut, T.loopReady));
    parts.push(titleCard(t, backOp));
  }

  return svgFrame(parts.join("\n"));
}

// ---------------------------------------------------------------------------
// Render pipeline
// ---------------------------------------------------------------------------

async function run(cmd: string, args: string[]): Promise<void> {
  const p = new Deno.Command(cmd, { args, stdout: "piped", stderr: "piped" });
  const { code, stderr } = await p.output();
  if (code !== 0) {
    throw new Error(
      `${cmd} ${args.join(" ")} failed (${code}):\n${new TextDecoder().decode(stderr)}`,
    );
  }
}

async function renderPng(pngPath: string, svgPath: string): Promise<void> {
  await run("rsvg-convert", ["-w", String(WIDTH), "-h", String(HEIGHT), "-o", pngPath, svgPath]);
}

async function main(): Promise<void> {
  if (AT !== "") {
    const t = Number(AT);
    const dir = "docs/assets/demo/frames";
    await Deno.mkdir(dir, { recursive: true });
    const svg = `${dir}/at-${t.toFixed(2)}.svg`;
    await Deno.writeTextFile(svg, renderFrame(t));
    await renderPng(`${dir}/at-${t.toFixed(2)}.png`, svg);
    console.log(`Preview: ${dir}/at-${t.toFixed(2)}.png`);
    return;
  }

  const frameDir = KEEP_FRAMES
    ? "docs/assets/demo/frames"
    : await Deno.makeTempDir({ prefix: "myul-demo-" });
  await Deno.mkdir(frameDir, { recursive: true });

  const total = Math.round(T.duration * FPS);
  console.log(`Rendering ${total} frames at ${WIDTH}×${HEIGHT} @ ${FPS}fps → ${OUT}`);

  const BATCH = 8;
  for (let i = 0; i < total; i += BATCH) {
    await Promise.all(
      Array.from({ length: Math.min(BATCH, total - i) }, async (_, j) => {
        const idx = i + j;
        const t = idx / FPS;
        const base = `${frameDir}/f${String(idx).padStart(4, "0")}`;
        await Deno.writeTextFile(`${base}.svg`, renderFrame(t));
        await run("rsvg-convert", ["-w", String(WIDTH), "-h", String(HEIGHT), "-o", `${base}.png`, `${base}.svg`]);
        if (!KEEP_FRAMES) await Deno.remove(`${base}.svg`);
      }),
    );
    if ((i / BATCH) % 5 === 0) console.log(`  frame ${Math.min(i + BATCH, total)}/${total}`);
  }

  const palette = `${frameDir}/palette.png`;
  await run("ffmpeg", [
    "-y", "-framerate", String(FPS), "-i", `${frameDir}/f%04d.png`,
    "-vf", "palettegen=stats_mode=diff:max_colors=128", palette,
  ]);
  await Deno.mkdir(OUT.replace(/\/[^/]+$/, ""), { recursive: true });
  await run("ffmpeg", [
    "-y", "-framerate", String(FPS), "-i", `${frameDir}/f%04d.png`,
    "-i", palette,
    "-lavfi", "paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle",
    "-loop", "0", OUT,
  ]);

  const size = (await Deno.stat(OUT)).size;
  console.log(`Done: ${OUT} (${(size / 1024 / 1024).toFixed(2)} MiB)`);
  if (!KEEP_FRAMES) await Deno.remove(frameDir, { recursive: true });
}

if (import.meta.main) {
  await main();
}