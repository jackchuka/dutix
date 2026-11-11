# Agent guide

## Architecture

```
cmd/dutix → internal/cli → internal/domain → internal/macos (Bridge interface) → macos-apphandlers-bridge
```

- **All macOS access goes through [macos-apphandlers-bridge](https://github.com/jackchuka/macos-apphandlers-bridge).** That covers LaunchServices, NSWorkspace, CFPreferences (including `LSHandlers`), `lsregister`, and Info.plist reads. Don't call system tools (`defaults`, `plutil`, `lsregister`, `duti`, ...) with `os/exec` or add cgo in dutix.
- **`internal/macos` only wraps bridge calls**: it converts the bridge's types into its own and wraps errors such as `ErrUserDeclined`. If the bridge lacks something, add it to the bridge first (see below), then call it here.
- **`internal/domain` has the logic and talks only to the `macos.Bridge` interface**, so it's tested with the generated mock (`go generate ./...` after changing the interface).
- **`internal/cli` handles flags, prompts, and output.** With `--output json|yaml`, commands hand their data to `ctx.SetData` (or `ctx.Formatter()`), and `cli.Run` writes one envelope. Don't write JSON or YAML to stdout directly.
- Errors map to exit codes in `internal/cli/shared/exit.go`. Return `shared.Usagef` for invalid input, and wrap `domain.ErrAppNotFound` / `domain.ErrAmbiguousApp` instead of formatting new "not found" messages.

## Changing the bridge

1. Work in a local clone of the bridge and point dutix at it: `go mod edit -replace github.com/jackchuka/macos-apphandlers-bridge=../macos-apphandlers-bridge`.
2. Release the bridge, then `go mod edit -dropreplace github.com/jackchuka/macos-apphandlers-bridge && go get github.com/jackchuka/macos-apphandlers-bridge@<version>`. A `replace` must never be merged.

## Checks

```bash
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
golangci-lint run ./...
```

Changing defaults on macOS 26.4 and later shows a confirmation dialog for each UTI or URL scheme, so prefer unit tests with the mock, and use extensions that no app declares for manual checks.
