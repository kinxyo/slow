# slow — plan

`slow` is a markdown viewer for long documents, derived from Charm's Glow (MIT).
Repo `github.com/kinxyo/slow`, binary `slo`, module `github.com/kinxyo/slow`.

Why it exists: Glow's pager is a plain viewport. The documents I review are long,
have mermaid diagrams, live in gitignored folders, and I want to jump between
sections fast. Specs live next to this file, one per item. Each has acceptance
criteria so "done" is checkable.

## Bucket list

### Done in the working tree, not committed
- [x] Mermaid rendering: flowchart (top-down) and stateDiagram-v2 as text drawings,
      source block as fallback (`utils/mermaid*.go`) — see [05-mermaid.md](05-mermaid.md)
- [x] `.gitignore` ignored by default, `--gitignore` to obey it
- [x] Width cap of 120 removed, pager follows the terminal
- [x] Rendered markdown centered in the terminal (`utils/center.go`)
- [x] Makefile (`make prod` installs to `~/bin`)

### Ground work
- [x] 00 — commit the above as separate commits, then rebrand to slow/slo
      — [00-repo-and-rebrand.md](00-repo-and-rebrand.md)
- [x] 01 — strip inherited CI/release, keep what runs on a free account
      — [01-ci-and-release.md](01-ci-and-release.md)

### Features, in the order I'd build them
- [ ] 02 — scroll acceleration and section jumping (`]`/`[`) — [02-scroll-and-sections.md](02-scroll-and-sections.md)
- [ ] 03 — contents sidebar from `#`/`##` headings, shown at full-screen size
      — [03-contents-sidebar.md](03-contents-sidebar.md)
- [x] 04 — debug bar (render stats, width, zoom) — [04-debug-bar.md](04-debug-bar.md)
- [x] 06 — width problem and live resize — [06-width-and-resize.md](06-width-and-resize.md)
- [ ] 05 — mermaid extras: `LR`, nested subgraphs, more diagram types (only when a real
      document needs it) — [05-mermaid.md](05-mermaid.md)

### Maybe
- [ ] Upstream PRs for the `.gitignore` default and width fixes (not mermaid)
- [ ] Pull fixes from `upstream` (Charm) by cherry-pick, never a merge

## Order of work

Debug bar (04) is cheap and makes the width investigation (06) possible, so it goes
before it. Sections (02) come before the sidebar (03) because both need the
heading list.

## Conventions

- Go, `make test` = vet + `-race` tests. Lint with `golangci-lint run`.
- Behaviour that can't be checked in a unit test gets a pty-driven check; see
  [06-width-and-resize.md](06-width-and-resize.md) for the harness problem.
- Learning notes go to `~/Learn/notes/<domain>.md`.
