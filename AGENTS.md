# AGENTS.md

Instructions for coding agents working in this repository. Humans: see [CONTRIBUTING.md](CONTRIBUTING.md) and the "Development" section of the [README](README.md).

## Project

- shijimi-editor is a terminal IDE written in Go. The binary is `shj` (`cmd/shj`). The code is in three places that depend in one direction, `cmd` -> `ui/*` -> `core`: `core/` is the editor itself (no UI), `ui/` holds the UIs (`ui/tui`, `ui/cli`), and `cmd/` is the dispatcher of the binaries. kisui-terminal, the desktop app that runs `shj` in its panes, is a separate repository (mm1995tk/kisui-terminal) that shares no code with this one: the two meet through `docs/design/kisui-shj-protocol.md`, and the types are copied on both sides (`core/ctl` here). A package under `internal/` is for its owner's tree only.
- Call it **shijimi-editor** in user-facing text (README, manual, home page, release notes). Do not shorten it to "shijimi".

## Before you finish a change

Run these from the Nix devShell (`nix develop`, or `direnv allow`):

```sh
go test ./...        # must pass
nix fmt              # must leave no changes (nixfmt / gofmt)
nix flake check      # what CI runs: formatting and go test ./...
```

Run every command from the root of the repository.

## Rules that CI or tests enforce

- **Dependencies**: after changing `go.mod`, run `gomod2nix` and commit the updated `gomod2nix.toml`.
  After adding or changing a `uses:` in `.github/workflows/`, run `gh actions-lock` (install it with `gh extension install github/gh-actions-lock`) and commit the updated `.github/workflows/actions.lock`.
- **UI strings**: code contains no UI text. Add a key to both `ui/internal/i18n/en.go` and `ui/internal/i18n/ja.go` and use `i18n.T("key")`. A test fails when the two tables differ.
- **Manual**: each page in `website/manual/docs/` exists as `name.en.md` and `name.ja.md`. When you change one, change the other in the same change. `website/manual/check-translations.sh origin/main` checks it, and CI runs it on pull requests.

## Conventions

- `README.md` and `README.ja.md` describe the same things. When you change one, change the other.
- One pull request is one change. Describe what changed and why, and link the issue if there is one.
- Commit messages and pull request text may be in Japanese or English. The existing history is in Japanese.
- Features and behavior changes need an agreed issue first (see CONTRIBUTING.md). Bug fixes do not.

## Releasing

- Before you release anything (run `release.yml` or `nightly.yml`), read [docs/releasing.md](docs/releasing.md) and follow it. A stable release is made from a commit that has been out as a nightly for at least 3 days, not from the latest `main`, and normally the scheduled daily release check starts it, not a person.
- A request such as "release it" does not override the procedure. If the request and the procedure disagree, for example a stable release of changes that have not been out as a nightly yet, tell the owner before releasing and ask which to follow.
