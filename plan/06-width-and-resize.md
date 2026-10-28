# 06 — width and live resize

## Symptom

In the interactive viewer, the text "still doesn't really take the full width" of a
large terminal. Rendered output is now centered, but the underlying width behaviour is
not understood. Live resize is wanted: the layout should follow the terminal as it is
dragged.

## What the code does today (read, not yet reproduced)

- `main.go` `validateOptions`: when `--width` is not set and stdout is a terminal, `width`
  is the terminal width, no cap any more. `widthAuto` is set.
- `runTUI`: when `widthAuto`, `cfg.GlamourMaxWidth = math.MaxInt16`, so the pager wraps at
  the viewport width.
- `ui/pager.go` `glamourRender`: wrap width is
  `min(GlamourMaxWidth, viewport.Width())`; glamour is given `WithWordWrap(width)`.
- `ui/ui.go`: `tea.WindowSizeMsg` sets sizes on stash and pager; the pager answers
  `WindowSizeMsg` with `renderWithGlamour`, so a re-render does happen.
- `utils.CenterBlock` pads by `(viewport - widest line) / 2` after rendering.

## Hypotheses to check, with the debug bar ([04](04-debug-bar.md))

1. A config value (`width:` in the config file, or `GLOW_*` env) overrides the auto
   width. Unverified: the config file could not be read in the last session.
2. Glamour's own styles add margins (document margin, code block margin) so the text sits
   inside a narrower column than the wrap width.
3. The document's paragraphs are hard-wrapped at a smaller width in the source.
4. The viewer was started in a mode that doesn't set `widthAuto` (the `glow file.md`
   CLI path versus the `-t` TUI path versus `-p`).
5. Resize re-renders with a stale `viewport` copy: `renderWithGlamour(m, ...)` takes the
   model by value inside the handler; check that the width it sees is the new one.

## Test harness problem

Driving the TUI from a pseudo-terminal needs the harness to answer the queries the
program sends at start-up (`ESC ] 10 ; ?` for the foreground colour and `ESC [ 6 n`
for the cursor position), otherwise it waits and draws nothing. A first attempt in the
last session captured only those two queries. The harness must reply, for example with
`ESC ] 10 ; rgb:ffff/ffff/ffff ESC \` and `ESC [ 1 ; 1 R`, before sending keys.

## Plan

1. Build the debug bar first.
2. Reproduce in the real terminal; record the numbers for 80, 140 and 200 columns.
3. Fix the cause the numbers point to. If it is glamour margins, set the style's margin
   to 0 for the document and handle the padding ourselves.
4. Add a resize test: a unit test that sends `tea.WindowSizeMsg` to the model and checks
   that the wrap width and the line widths of the re-rendered content follow it.

## Acceptance

- At 200 columns with no `--width`, the widest rendered line is within 4 columns of the
  terminal width (or of the centered text column if a width limit is configured).
- Dragging the terminal narrower and wider re-wraps within one frame, with no leftover
  text from the previous layout.
- `--width N` keeps text at N columns, centered.
