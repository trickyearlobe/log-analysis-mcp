# Add Copilot CLI to --install

## Goal

Add GitHub Copilot CLI support to `--install`/`--uninstall` so the MCP server registers correctly.

## Specs to Read

- Nuclia resource `207a2b545f4840f39f38c11b9b2e0f36` (IDE Installation Reference) — already read.
- `internal/install/ide.go`, `config.go`, `install_test.go` — already read.

## Steps

1. Add Copilot CLI to `SupportedIDEs()` in `ide.go`.
2. Add Copilot detection in `UpsertServer()` in `config.go` — emit `type`, `args`, `env`, `tools` fields.
3. Add tests: install, update, shape check, idempotent, uninstall for Copilot CLI.
4. `go test -race ./internal/install/` after each change.
5. `go vet ./...` before commit.

## Acceptance

- `--install` writes correct Copilot CLI entry with `type: "local"`, `args: []`, `env: {}`, `tools: ["*"]`.
- Re-install updates old entries missing Copilot-required fields.
- All existing tests still pass.