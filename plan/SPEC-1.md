# Spec 1 — reading long documents fast

Status: **in progress** — decisions accepted; phases 0, 1 and 2 done.

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
- text wraps at a column the reader can change, sits at the left edge, and follows live
  resizing;
- a debug bar (`D`) shows the numbers behind the layout;
- the project is named slow, binary `slo`, with its own config directory.

## Phases

Each phase leaves the app working and is committed on its own.

0. **Rename** ([00](00-repo-and-rebrand.md)) — *done*. Module path, binary, config and cache
   directories, help text, man page, status-bar badge, Charm's tagline. A rename commit
   with no behaviour change.
1. **Debug bar** ([04](04-debug-bar.md)) — *done*. Stats struct filled by the renderer and the size
   handler; one line above the status bar, toggled with `D`.
2. **Width and live resize** ([06](06-width-and-resize.md)) — *done*. Reproduce with the debug bar,
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

9. **Width is a centered column** (2026-10-28). Text wraps at 100 columns, or the terminal
   width when that is smaller, and sits in the middle of the screen. `--width N` sets the
   column. While reading, `+`/`=` widen it by 10, `-`/`_` narrow it by 10 (40 to 400) and
   `0` resets it; a status message shows the value, and the reader stays at the same
   place in the document. This replaces "text uses the terminal width" in *What*, and
   changes phase 2 and the sidebar threshold (the column, not the terminal, is the text
   width). *Changed by 10: the column is no longer centered.*
10. **Text is left-aligned again** (2026-10-31). Centering looked broken: it centered the
    whole document on its widest line, and a mermaid diagram wider than the text column
    pushed every line off to one side. The column, its keys and `--width` stay; the text
    starts at the left edge with glamour's margin, as in Glow. This also applies to the
    output printed without the viewer.
11. **Releases are tag-only, work happens on `dev`** (2026-10-31). Answers proposal 8:
    pushing a `v*` tag publishes a release named after the tag; branch pushes only run
    CI. All work and pushes go to the `dev` branch; `main` moves when kinxyo says so.
12. **The pseudo-terminal scripts are in the repo** (2026-10-31), under `scripts/tty/`.
    Saving the column width between sessions stays open (no decision; not built).
13. **Sidebar threshold stays relative** (2026-10-31). kinxyo's full-screen width isn't
    known (the shell here has no terminal to ask), so phase 5 keeps proposal 7: the
    sidebar shows when the terminal has room for an 80 column text plus the sidebar and
    gap (110 columns), and the text then wraps at the smaller of the column and what is
    left. The width is checked against a real screen when phase 5 is reviewed.

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

- **Rendering is still slow on a long document.** Phase 2 took a 1770-line file with 18 mermaid
  blocks from about 550 ms to about 245 ms a render; glamour itself is the rest (mermaid is 4
  ms). A resize that does not change the wrap width now costs about 27 ms and skips glamour,
  but a change of wrap width (`+`/`-` below the terminal width, or a narrow terminal) still
  renders in full, in the background, and the older render is dropped if a newer one was asked
  for. Options if it still feels slow: render only the part near the viewport, or split the
  document at headings and render the parts in parallel (risky at block boundaries).
- **Existing config files keep `width: 80`.** Glow's default config file pinned `width: 80`, and
  slo inherited that until phase 2, which is why text never used the width of the screen. The
  template no longer sets it, but a file that was already written still does, and a width in
  the file counts as an explicit choice. Delete the line, or set `width: 100`.
- **The terminal check is minimal.** `scripts/tty/` drives the real binary in a pseudo-terminal
  with a small screen emulator (`emu.py`). It knows only the escape sequences slo has needed so
  far; a screen that looks stale is more likely a missing sequence than a bug in slo (reverse
  index, `ESC M`, was missing until 2026-10-31).

## As built

_(filled in when the spec closes)_
