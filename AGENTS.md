# AGENTS.md

Instructions for coding agents working in this repository. Humans: see [CONTRIBUTING.md](CONTRIBUTING.md) and the "Development" section of the [README](README.md).

## Project

- shijimi-editor is a terminal IDE written in Go. The binary is `shj` (`cmd/shj`). The code is under `internal/`.
- Call it **shijimi-editor** in user-facing text (README, manual, home page, release notes). Do not shorten it to "shijimi".

## Before you finish a change

Run these from the Nix devShell (`nix develop`, or `direnv allow`):

```sh
go test ./...        # must pass
nix fmt              # must leave no changes (nixfmt / gofmt / taplo)
```

`nix build` builds the package and also runs the tests.

## Rules that CI or tests enforce

- **Dependencies**: after changing `go.mod`, run `gomod2nix` and commit the updated `gomod2nix.toml`.
- **UI strings**: code contains no UI text. Add a key to both `internal/i18n/en.go` and `internal/i18n/ja.go` and use `i18n.T("key")`. A test fails when the two tables differ.
- **Manual**: each page in `website/manual/docs/` exists as `name.en.md` and `name.ja.md`. When you change one, change the other in the same change. `website/manual/check-translations.sh origin/main` checks it, and CI runs it on pull requests.

## Conventions

- `README.md` and `README.ja.md` describe the same things. When you change one, change the other.
- One pull request is one change. Describe what changed and why, and link the issue if there is one.
- Commit messages and pull request text may be in Japanese or English. The existing history is in Japanese.
- Features and behavior changes need an agreed issue first (see CONTRIBUTING.md). Bug fixes do not.
