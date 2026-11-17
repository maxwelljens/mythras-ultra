# Mythras Ultra (`myul`)

<p align="center">
  <img src="docs/assets/demo.gif" width="720"
       alt="The myul TUI in play: you search a shrine, the GM calls skill_test, an animated d100 lands a critical, and the metadata panel gains a silver torc.">
</p>

A tabletop RPG in your terminal. An LLM plays the game master, a deterministic
Go engine plays the rules, and the ruleset is [Mythras
Imperative](https://thedesignmechanism.com/mythras-imperative-a/); a d100
system with wounds that matter and not a single hallucinated roll.

## What is Mythras Ultra?

`myul` isn't another chatbot pretending to run a campaign. It's a terminal game
with one main job: letting you play a proper d100 tabletop RPG against a game
master that never sleeps, never cancels the session, and, most importantly,
respects what the dice say. Think of it as a GM in a box. You type what you do,
in plain English, and the story answers.

> The salt wind combs the grass flat along the trade road. Two weeks ago the
> baron's riders burned the western tollhouse, and every caravan since has
> travelled under armed guard. What do you do?

That is the opening line of a real session, narrated by the GM from the
embedded Mythras Imperative rulebook. You answer and it narrates. When the
story needs a number, like a skill test, a wound, an item, a clock ticking
toward something bad, the GM has to ask the engine for it.

The table itself is a proper terminal app: markdown chat with an
in-character/out-of-character switch, a `$VISUAL` editor for long messages,
animated dice, a character panel, and four 16-colour themes that adapt to light
and dark terminals.

## What keeps the GM real?

If you have played with an LLM before for any amount of time, you know the
failure mode. It "forgets" basic facts, because it is just a glorified token
guesser. It always agrees that your plan is brilliant. It can even roll a die
in prose and tell you that you succeeded in the most epic way possible, because
you sounded confident.

This is obviously not very good. A game master who can be talked into anything
is not a game master; he is a yes-man, and the game evaporates pretty quickly.

Mythras Ultra takes the numbers away from the LLM and tries to put it into
a deterministic engine, as a program should. Every roll, skill test, attack,
wound, inventory change and world flag goes through a deterministic engine as
a typed move. The GM calls the engine as a tool and narrates from the result it
gets back: the dice it rolled, the facts it changed, and the rulebook section
it applied -- because, yes, it reads from the rulebook. Ask the GM for
a critical success and it can want one for you; it cannot give you one. The
engine rolls a seeded d100, and 04 against Perception 65 is a critical because
the rules say so, not because the LLM's tokens landed on a win. When the GM
tries something the rules forbid, the engine refuses, and the refusal is final.

This is what makes the thing worth playing: stakes survive, a wound you took in
session three is still on your sheet in session nine, because your sheet is
state saved in a file, not context memory that is notoriously unreliable. The
world keeps flags, such as tollhouse staying burnt, the economy staying
strained, and the GM must narrate around facts it cannot edit. When you win,
you won. When you lose, it was also fair.

## Decision models: Force the GM to grade its own homework!

As an option, you can employ [decision
models](https://docs.openclaw.ai/concepts/decision-models) as a second, smaller
helper. For instance, `tev1` through Ollama, running locally, with one job:
answering whether a player action needs a check at all, and which skill it
calls for. The storyteller does not get to skip the roll because a roll would
spoil his scene. The `decide` tool asks the decision model, and the answer goes
to the engine like everything else.

Of course, Mythras Ultra can work without it, in which case the GM falls back
to judging checks itself. Switch it on when you want extra guardrails to make
the experience all the more organised.

## How do I make it mine?

One TOML file. `myul config init` writes a commented default, and most of it can
be changed to suit one's preferences: the LLM provider and model, the theme
(`mythras`, `obsidian`, `parchment` or `terminal`), the dice theatre animation
(there is an accessibility toggle, because watching numbers spin is not
everyone's idea of drama), and the metadata panel. Every command accepts
`--config PATH`, so you can keep several configurations side by side. That is
the whole configuration surface. There is no launcher, no account, no settings
dialog.

## How do I play?

```sh
just build            # static binary → bin/myul (or: go build ./cmd/myul)
myul config init      # write the commented default config
myul doctor           # check the config and probe your API key
myul new              # create a session and start playing
```

Requires Go 1.27 to build. Play needs a configured `[llm]` provider; `myul
doctor` tells you if anything is missing. Sessions save themselves under
`~/.local/share/myul` and resume with the same seed, conversation and play
history, so a campaign can span months of evenings.

| Command | Description |
|---|---|
| `myul` | Session picker (or resume the newest session; `--no-tui` headless). |
| `myul new [name]` | Create a session and play it (`--seed`, `--model`, `--provider`, `--skip-llm`, `--no-tui`). |
| `myul resume [session-id]` | Resume a saved session; picker when the id is omitted. |
| `myul sessions` | List saved sessions, newest first (`--json`). |
| `myul delete <session-id>` | Delete a session and its log (`--force` skips confirmation). |
| `myul export <session-id>` | Write the Markdown play log (`--out FILE`, `--state` for state JSON). |
| `myul config path` | Print the resolved configuration file path. |
| `myul config init` | Write a commented default config file (`--force` overwrites). |
| `myul doctor` | Validate the config and probe the LLM API key and decision model. |
| `myul version` | Print version and build date. |

## How does it work?

For the curious: the engine (`internal/engine`) is pure Go and knows nothing
about LLMs. The harness (`internal/harness`) exposes 37 tools to the GM and
translates engine refusals into `IMPOSSIBLE:` results it must respect. The
session loop (`internal/session`) is the only place the LLM, the engine and the
TUI meet. The rulebook itself is embedded and searchable, so the GM looks up
the rule it is applying instead of recalling it. Every engine package has
seeded unit tests, and a scripted mock session exercises all 37 tools offline.
The design contract is [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Status

Early development. The engine, harness, session loop, TUI and CLI all work and
are covered by the offline test suite, but API may still change.

## Documentation

- [`AGENTS.md`](AGENTS.md) — development guidelines, golden rules, testing
  and commit conventions.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the design contract;
  read it before touching code.
- [`docs/design/harness-surface.md`](docs/design/harness-surface.md) — the
  37-tool catalogue; [`docs/design/session-loop.md`](docs/design/session-loop.md)
  — how a turn runs.
- The promo clip is generated, not recorded: `just demo` rebuilds
  `docs/assets/demo.gif` from [`docs/assets/demo/demo.ts`](docs/assets/demo/demo.ts).
- Project skills (`.opencode/skills/`): `engine-move`, `harness-tool`,
  `tui-component`, `rules-codex`.

## Licence

European Union Public Licence 1.2 (EUPL-1.2) — see [`LICENCE`](LICENCE).
