# 02 — scroll acceleration and section jumping

## Problem

The pager scrolls one line per keypress or wheel notch, always. In a long document,
reaching a section takes hundreds of keypresses. There's no way to jump by section.

## Part A — acceleration

Hold `j`/`k` (or the arrow keys) or spin the mouse wheel, and the step grows.

- Track the time of the previous scroll event and a streak count in the pager model.
- If the gap to the previous event is under a threshold (start with 120 ms), the streak
  grows; otherwise it resets to 1.
- Step size follows the streak: 1 line for the first events, then ramping to a cap
  (start: ramp 1 → 12 lines; tune by feel). Wheel gets its own, gentler ramp.
- Reset on any other key, on direction change, and on reaching the top or bottom.
- Bubble Tea v2 can report key repeat only on terminals with the kitty keyboard
  protocol, so timing between events is the portable signal. **Open question:** check
  what `tea.KeyPressMsg` offers in the vendored version before settling on timing.
- Constants (`threshold`, `maxStep`) live in one place so they can be tuned.

## Part B — jumping between sections

- `]` / `[`: next / previous heading (any level). `}` / `{`: next / previous top-level
  heading (`#` and `##` only, same set the sidebar uses).
- `g`/`G` already go to top and bottom; unchanged.
- The target heading lands at the top of the viewport, with a small offset so context
  above stays visible (start: 1 line).

### Heading list

- Build it from the markdown source, not from the rendered text: walk lines, track fenced
  code blocks (``` and ~~~) so `# comment` inside code is not a heading, accept ATX
  headings `#`..`######` and ignore setext headings at first.
- Map each heading to a rendered line number. Glamour output keeps headings in order,
  so a sequential search of the rendered lines for each heading's text works. **Open
  question:** decide how to handle headings that contain inline markup or a repeated text
  (search forward from the previous match, never backwards).
- Rebuild the map after every re-render (resize, reload, mermaid expansion changes the
  line count).
- The same list feeds the sidebar in [03](03-contents-sidebar.md).

## Acceptance

- Holding `j` for one second moves visibly more than 60 lines of a long document; a
  single tap still moves exactly one line.
- `]` from the top lands on the first heading; repeated `]` visits every heading in order
  and does nothing past the last; `[` mirrors it.
- Headings inside code fences are skipped.
- After resizing the terminal, `]` still lands on the right heading.
- Unit tests: heading extraction (fences, levels, empty), step-ramp function (pure
  function from gap and streak to step), heading-to-line mapping on a fixture.
