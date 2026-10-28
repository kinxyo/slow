# Spec 1 — reading long documents fast

Status: **in progress** — decisions accepted; phase 0 done.

This is a sprint: one batch of related work, built phase by phase, each phase about a day
and leaving slow working. Process: [PROCESS.md](PROCESS.md) (local copy, git-ignored).
Item-level design notes are in the numbered files of this folder; this spec orders them
and fixes what "done" means.

## Why

Reviewing a long document in slow today means hundreds of single-line keypresses to reach
a section, no way to see or jump to the headings, text that doesn't use the width of a
large terminal, and no way to see why. Everything in this sprint attacks that.

## What

By the end of the sprint, on a long markdown file:

- holding a scroll key or spinning the wheel speeds up; one tap is still one line;
- `]`/`[` jump between headings, `}`/`{` between top-level ones;
- a contents sidebar lists the `#` and `##` headings when the terminal is wide enough, and
  follows the scroll position;
- text uses the terminal width, follows live resizing, and is centered when narrower;
- a debug bar (`D`) shows the numbers behind the layout;
- the project is named slow, binary `slo`, with its own config directory.

## Phases

Each phase leaves the app working and is committed on its own.

0. **Rename** ([00](00-repo-and-rebrand.md)) — *done*. Module path, binary, config and cache
   directories, help text, man page, status-bar badge, Charm's tagline. A rename commit
   with no behaviour change.
1. **Debug bar** ([04](04-debug-bar.md)). Stats struct filled by the renderer and the size
   handler; one line above the status bar, toggled with `D`.
2. **Width and live resize** ([06](06-width-and-resize.md)). Reproduce with the debug bar,
   name the cause, fix it, add a resize test. Includes the pty harness fix (answer the
   start-up terminal queries).
3. **Scroll acceleration** ([02](02-scroll-and-sections.md), part A). Pure ramp function,
   wired to keys and wheel.
4. **Heading list and section jumping** ([02](02-scroll-and-sections.md), part B).
   Extraction from the source, mapping to rendered lines, the four keys.
5. **Contents sidebar** ([03](03-contents-sidebar.md)). Layout, highlight, focus and
   jump, hidden below the width threshold.
6. **Tidy.** Triage the rough edges, split files over ~500 lines or doing two jobs, share
   logic repeated three times, tests for what the sprint added, take compatible
   dependency updates. No behaviour change, committed separately.

## Out of scope

- More mermaid ([05](05-mermaid.md)): `LR`, nested subgraphs, other diagram types.
- Search inside a document, bookmarks, themes beyond glamour's styles.
- Upstream pull requests to Charm.
- AUR or Homebrew packaging.

## Open questions

Each has a proposed answer; saying "go" accepts them all.

1. **When does the rename happen?** Proposal: phase 0, so everything after it is written
   under the final module path and no later diff has to touch every import.
2. **Which inputs accelerate?** Proposal: `j`/`k`, the arrow keys and the mouse wheel.
   `pgup`/`pgdn`, `d`/`u`, `g`/`G` keep their fixed steps.
3. **How fast?** Proposal: the step grows from 1 to 12 lines after about a second of
   holding (events less than 120 ms apart), the wheel from 1 to 6; both constants in one
   place so they can be tuned by feel in phase 3.
4. **Which keys jump?** Proposal: `]`/`[` next/previous heading of any level, `}`/`{`
   next/previous `#`/`##` heading. Setext headings (underlined) are ignored at first.
5. **Where is the sidebar threshold?** Proposal: shown from 110 columns up (text 80,
   sidebar 28, gap 2); `t` toggles it when it fits, `tab` moves focus into it.
6. **Debug bar key.** Proposal: `D` (capital), because `d` and `ctrl+d` already scroll.
7. **Does the debug bar ship?** Proposal: yes, hidden by default, so it is available
   whenever a layout looks wrong.
8. **Release cadence.** `release.yaml` publishes `v0.1.<run>` on every push to `main`
   that touches more than docs. Proposal: keep that during the sprint, and move to
   tagged releases when the rename is done.

## Decisions

All eight proposals above were accepted ("lets proceed ahead", 2026-10-28).

## Checks before a commit

The Go version of PROCESS.md's list:

- `gofmt -l .` prints nothing; `go vet ./...`; `go test -race -count=1 ./...`;
  `golangci-lint run ./...` shows nothing new in files this phase touched.
- New logic gets a test, preferably a pure function (the scroll ramp, heading extraction,
  the sidebar visibility rule, the stats formatting).
- UI changes get a model-level check: send `tea.WindowSizeMsg` and key messages to the
  model, assert on `View()` and on widths, and look at the output.
- Mermaid output stays pinned by the golden files; a phase that changes it regenerates
  them with `-update` and the diff is read.
- Anything that needs a real terminal (how acceleration feels, the sidebar at a real
  full-screen size, resizing by dragging) is listed as unchecked in the summary.

## Rough edges

_(none yet)_

## As built

_(filled in when the spec closes)_
