# Contributing to shijimi

Thank you for your interest in shijimi. It is developed by one person, so contributions are accepted as follows. You can write issues and pull requests in English or Japanese.

## What to send

- **Bug reports**: open an issue. Include the shijimi version (`shj --version`), the OS, the terminal, and the steps to reproduce.
- **Bug fixes**: send a pull request directly. If the fix is large, open an issue first.
- **New features and changes in behavior**: open an issue first and wait for a reply before you write code. A pull request for a feature that was not agreed on in an issue may be closed.
- **Security issues**: do not open a public issue. Follow [SECURITY.md](SECURITY.md) if it exists; otherwise use "Report a vulnerability" in the Security tab.

## Development

Use the Nix devShell. With direnv, `.envrc` enters it automatically.

```sh
nix develop            # or: direnv allow
go test ./...
nix fmt                # format with nixfmt / gofmt / taplo
nix build              # build the package (also runs the tests)
```

- After changing `go.mod`, run `gomod2nix` to update `gomod2nix.toml`.
- User-visible strings go in both [`internal/i18n/en.go`](internal/i18n/en.go) and [`internal/i18n/ja.go`](internal/i18n/ja.go). A test checks that the keys match.
- The manual has an English page (`name.en.md`) and a Japanese page (`name.ja.md`). When you change one, change the other too. If you cannot write one of the languages, say so in the pull request.

See "Development" and "Editing the manual" in the [README](README.md) for details.

## Pull requests

- Keep one pull request to one change.
- Make sure `go test ./...` passes and `nix fmt` leaves no changes.
- Describe what changed and why, and link the issue if there is one.

By sending a pull request, you agree that your contribution is licensed under the [MIT License](LICENSE).
