# 04 — debug bar

## Why

The width problem ([06](06-width-and-resize.md)) can't be diagnosed by guessing. A bar
showing the numbers the renderer actually uses makes it visible in the real terminal.

## Behaviour

- Toggle with `ctrl+d` (or the flag `--debug-bar`); off by default, one line high, drawn
  above the status bar.
- Fields, in this order:
  - terminal size `W×H` as the last `WindowSizeMsg` reported it
  - viewport width and height
  - wrap width handed to glamour, and `GlamourMaxWidth`
  - center padding applied (columns)
  - rendered line count, widest line (display columns), byte size
  - scroll offset / total lines, percent
  - last render duration in ms, and how many renders since start
  - mermaid blocks drawn / fell back
  - text-sizing ("zoom") support: on or off
  - when the sidebar exists: its width and the heading count
- Values that depend on resize update live.
- Truncate from the right with `…` when the bar is wider than the terminal.

## Implementation notes

- Collect the numbers in one struct (`ui/debugstats.go`) filled by `glamourRender` and the
  size handler; the bar only formats it.
- Counting render time needs a timestamp around `r.Render`; do it inside the command
  returned by `renderWithGlamour` and carry the value in `contentRenderedMsg`.
- No new dependencies.

## Acceptance

- Toggling shows and hides the bar without moving the text more than the bar's one row.
- Resize the terminal and `W×H` changes in the same frame the text re-wraps.
- Widest-line and wrap-width differing is visible at a glance (this is the bug to catch).
