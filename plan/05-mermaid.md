# 05 — mermaid

## Status: working, uncommitted

`utils.RenderMermaid(md, width)` replaces each ```` ```mermaid ```` fence with a
```` ```text ```` fence holding a box-drawing diagram, just before glamour renders. It is
called from `ui/pager.go` (`glamourRender`) and from `main.go`.

Supported: `flowchart`/`graph` top-down (`TB`, `TD`) with node shapes `[]`, `()`, `{}`,
edge kinds `-->`, `---`, `-.->`, `==>`, edge labels, `&` lists, one level of `subgraph`;
`stateDiagram-v2` with `[*]`, transitions with labels, `state "x" as y`, descriptions,
`<<choice>>`, self-loops (drawn as `↻ text` inside the node).

Anything else is left as the original source block. All 18 blocks of the test
document draw; golden files are in `utils/testdata/mermaid/`.

## Files

| File                       | Role                                          |
|----------------------------|-----------------------------------------------|
| `utils/mermaid.go`         | fence scan, width fit (label wrap 28 → 12)    |
| `utils/mermaid_parse.go`   | flowchart and state parsers → `mmGraph`       |
| `utils/mermaid_layout.go`  | layers, ordering, x placement, edge routing   |
| `utils/mermaid_draw.go`    | canvas of box-drawing cells                   |
| `utils/mermaid_test.go`    | goldens, parser, fallback and fence tests     |

Regenerate goldens after an intended change: `go test ./utils -run Fixtures -update`,
then read the diff.

## Known limits

- No `LR`/`RL`/`BT` direction.
- No nested subgraphs, no edges to a subgraph id.
- Subgraph boxes that would overlap another node make the whole block fall back.
- No composite states, notes or concurrency (`--`) in state diagrams.
- No sequence, class, ER, gantt or pie diagrams.
- Headings drawn with large-text escape codes are counted at normal width when centering.

## Backlog (only when a real document needs it)

1. `LR` flowcharts: layout runs with ranks left-to-right; the work is in the router.
2. Nested subgraphs.
3. Sequence diagrams: a separate, simpler renderer (lifelines and arrows).
4. Colour: tint subgraph borders and dotted edges from the active glamour style.
5. A `--no-mermaid` flag, if it is ever wanted.

## Risk

The layout code was written in one pass and is not yet read line by line by the owner.
The tests pin the current output; they don't prove it is right. Read
`layoutMermaid` in `utils/mermaid_layout.go` before changing it.
