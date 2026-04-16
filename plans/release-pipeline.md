# Release Pipeline

## Goal

Add GitHub Actions workflow triggered by semver tags to build and publish binaries for linux/windows on arm64/x86_64. Add Makefile targets for semver tag bumping.

## Specs to Read

- `specs/build_and_run.md` — existing build setup, ldflags, Makefile
- `cmd/log-analysis-mcp/main.go` — version var injected via ldflags

## Design

### Version Source

- Git tags are the single source of truth for version (e.g. `v1.0.0`).
- `main.version` is injected at build time via `-X main.version=$(VERSION)`.
- Makefile `VERSION` derives from `git describe --tags` for local builds.

### GitHub Actions Workflow

- Trigger: push tags matching `v*.*.*`.
- Matrix: `GOOS=linux,windows` × `GOARCH=amd64,arm64` (4 binaries).
- Steps: checkout, setup-go, test, build matrix, create GitHub Release, upload assets.
- Binary naming: `log-analysis-mcp-{os}-{arch}` (`.exe` suffix for windows).
- Checksums: generate `checksums.txt` (SHA256) and attach to release.

### Makefile Targets

- `version` — print current version from latest git tag.
- `release-patch` — bump patch (v1.0.0 → v1.0.1), create annotated tag.
- `release-minor` — bump minor (v1.0.0 → v1.1.0), create annotated tag.
- `release-major` — bump major (v1.0.0 → v2.0.0), create annotated tag.
- Tags are local only (NEVER push per CLAUDE.md). User pushes manually.

## Files to Create/Modify

| File | Action |
|---|---|
| `.github/workflows/release.yml` | Create |
| `Makefile` | Edit — add version targets, update VERSION to use git tags |

## Steps

1. Create `.github/workflows/release.yml`.
2. Update `Makefile` with version targets and git-tag-based VERSION.
3. `go vet ./...` to confirm nothing broke.

## Acceptance

- `make version` prints current tag.
- `make release-patch` creates next patch tag locally.
- `.github/workflows/release.yml` is valid YAML with correct matrix.
- 4 binaries + checksums attached to GitHub Release on tag push.