# 03 — contents sidebar

## Idea

When the terminal is wide enough, show an index of the document's `#` and `##` headings
beside the text, as quick links. Depends on the heading list from
[02](02-scroll-and-sections.md).

## Behaviour

- Shown only when there is room: the terminal width is at least
  (`minTextWidth` + sidebar width + gap). Start with text 80, sidebar 28, gap 2 → 110
  columns. Otherwise it is hidden and the layout is exactly today's.
- "Full-screen" here means the terminal is as large as the screen, but a program can't
  tell the difference; the width threshold is the practical stand-in. State this in
  the help text so it is not a surprise.
- Entries: `#` headings flush left, `##` indented by two. Long titles are truncated with
  `…`. `###` and deeper are not listed.
- The entry for the section on screen is highlighted and follows the scroll position.
- `tab` toggles focus between text and sidebar; in the sidebar `j`/`k` move the selection
  and `enter` jumps to that heading. `t` toggles the sidebar on and off regardless of width
  (when there is room).
- The text stays at the left edge and wraps at the smaller of the column and the width
  left beside the sidebar (Spec 1, decisions 10 and 13).

## Layout notes

- Compose with `lipgloss.JoinHorizontal`, sidebar on the left, a one-column divider,
  then the viewport.
- The viewport width becomes `termWidth - sidebarWidth - gap`; glamour's wrap width must
  use that value, or text will be wider than its column. Re-render on toggle.
- The sidebar scrolls on its own if there are more headings than rows.

## Acceptance

- At 140 columns a document with ten `##` headings shows ten entries; at 100 columns
  the sidebar is absent and the output matches today's.
- Scrolling the text changes the highlighted entry; `enter` on an entry puts that heading
  at the top.
- Resizing across the threshold shows or hides the sidebar immediately, with no stale
  columns left on screen.
- A document with no headings never shows the sidebar.
