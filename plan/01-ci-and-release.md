# 01 — CI and release

**Done.** Inherited workflows, `.goreleaser.yml`, `Dockerfile` and the issue templates were removed;
`ci.yml` and `release.yaml` (borrowed from zigzag: every push to `main` tests, builds five
targets and publishes `v0.1.<run>`) were added; `lint.yml` and `dependabot.yml` stay.
The table below is the analysis it was based on.

Account: free GitHub, `kinxyo`, repo public. Public repos get unlimited Actions minutes
on GitHub-hosted runners, so cost is not the constraint. The constraint is that most
inherited workflows assume Charm's org, secrets and accounts.

## What was inherited and what happens on `kinxyo/slow`

| File                     | What it does                                            | Verdict |
|--------------------------|---------------------------------------------------------|---------|
| `build.yml` build        | calls `charmbracelet/meta` reusable build               | **replace** with a local job: `go build ./...`, `go test -race ./...` |
| `build.yml` snapshot     | goreleaser snapshot, needs `GORELEASER_KEY` (Pro)       | **remove** — fails without the key |
| `build.yml` govulncheck  | reusable from `charmbracelet/meta`                      | **keep as a local job** (`govulncheck ./...`), free |
| `build.yml` semgrep      | reusable, may need a token                              | **remove** |
| `build.yml` ruleguard    | reusable, Charm's rule set                              | **remove** |
| `lint.yml`               | golangci-lint-action, `--issues-exit-code=0`            | **keep**; make blocking once the pre-existing findings are fixed |
| `coverage.yml`           | goveralls; needs the repo enabled on coveralls.io       | **remove** (or `go test -cover` in the test job) |
| `dependabot-sync.yml`    | syncs dependabot config from Charm via a PAT            | **remove** |
| `lint-sync.yml`          | syncs lint config from Charm                            | **remove** |
| `goreleaser.yml`         | needs Docker Hub, Snapcraft, Fury, nfpm GPG, a PAT      | **replace**, see below |
| `.github/dependabot.yml` | monthly gomod and actions updates                       | **keep** — it is native GitHub, no secrets |
| `.goreleaser.yml`        | includes Charm's remote config; publishes to Charm taps | **rewrite** |
| `Dockerfile`             | Charm's image                                           | **remove** unless wanted |
| `ISSUE_TEMPLATE/`        | Glow's templates                                        | **rewrite or remove** |

Note: the reusable workflows from `charmbracelet/meta` would run on a public repo, but
they would show red for the missing secrets, and a repo depending on someone else's
`@main` workflow can change under you. Do not depend on them.

## Target

Workflows kept, all secret-free and on `ubuntu-latest`:

1. `ci.yml` on push and PR: `go vet`, `go test -race ./...`, `go build`.
2. `lint.yml`: as it is today.
3. `release.yml` on tag `v*.*.*`: goreleaser building Linux (amd64, arm64) and macOS
   archives plus checksums, uploaded to the GitHub release with the built-in
   `GITHUB_TOKEN`. No homebrew, docker, snap, AUR, nfpm signing.

`.goreleaser.yml` rewritten: no `includes:` from Charm, binary `slo`, `main: .`, ldflags
for version and commit. Windows only if wanted (`console_windows.go` exists).

## Acceptance

- A push to `main` runs `ci` and `lint` and both are green on a clean checkout.
- Pushing a tag `v0.1.0` produces a GitHub release with archives for the targets.
- No workflow references `charmbracelet/meta` or a secret other than `GITHUB_TOKEN`.
- Running `gh workflow list` shows only workflows that pass.

## Not decided

- Whether to publish to AUR (the user runs Arch/CachyOS). That needs an AUR account and an
  SSH key, so it stays out of CI for now.
