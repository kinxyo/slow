# 00 — repo and rebrand

**Done** (Sprint 1, phase 0): the module is `github.com/kinxyo/slow`, the binary `slo`, config
`~/.config/slo/slo.yml`, cache `~/.cache/slo/slo.log`, env prefix `SLO_`. The repo
`kinxyo/slow` exists and `origin` points at it. What follows is the original plan.

## Decision

New repo `git@github.com:kinxyo/slow.git` (public). Full git history is kept so
Charm's commits and authorship stay visible.

| Thing        | Value                                                          |
|--------------|----------------------------------------------------------------|
| Project      | slow                                                           |
| Binary       | `slo` (like zigzag is `zz`)                                    |
| Module       | `github.com/kinxyo/slow`                                       |
| Config dir   | `~/.config/slo`, cache `~/.cache/slo` — must not share Glow's  |
| Upstream     | `upstream` remote → `charmbracelet/glow`, cherry-pick only     |

## Steps (separate commits, in this order)

1. Commit the current work as four commits: Makefile; `.gitignore` default and
   `--gitignore`; width and centering; mermaid. `testfile.md` is not committed as-is: move
   it to `testdata/` as a fixture or leave it out.
2. Rename commit: `go.mod` module path and every import (`charm.land/glow/v3`), the
   Makefile `NAME := slo`, config and cache paths, status-bar badge, help text, man
   page, Charm's tagline "Render markdown on the CLI, with pizzazz!".
3. Docs commit: new README (what it does, install, keys), credits section, remove the
   logo and `example.png`.
4. Point `origin` at the new repo, add `upstream`, push `main` and tags.

## License and credit

- Keep `LICENSE` (MIT) with Charm's copyright line. Add a second line for kinxyo.
- README section "Credits": based on Glow by Charm (github.com/charmbracelet/glow), MIT.
  Depends on Charm's glamour, bubbletea, lipgloss.
- Do not use Charm's name, logo or screenshots. MIT licenses the code, not the brand.

## Acceptance

- `grep -ri glow` finds only the credits, the `upstream` references, the license and
  places that name the glamour/Charm projects.
- `slo` and the pacman `glow` can be installed side by side without sharing config.
- `make test` passes; `git log` still shows Charm's history.
