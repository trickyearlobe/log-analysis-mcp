# Rules

## Claude.md

- CLAUDE.md is operating rules for the AI, not project documentation.
- Keep it concise. Every line costs context window budget.
- If we change our working practices, CLAUDE.md must be updated.
- If we update CLAUDE.md, upload it to Nuclia RAG with `setup/best-practices` tags.
- Rules are specific and actionable. "NEVER write to stdout" not "be careful with stdout".
- Hard constraints use NEVER in caps. No ambiguity.
- Explicit permission boundaries — say what needs human approval.
- No implementation code in CLAUDE.md or specs. That's what TDD is for.
- When starting a new project, review the CLAUDE.md in Nuclia to check if best practices need to evolve.

## Knowledge

- Specs live in `specs/`. One file per concern. Read only what you need for the current task.
- Tool specs are in `specs/tools/<tool_name>.md`. Each is self-contained.
- Cross-cutting specs: `specs/types.md`, `specs/parsers.md`, `specs/fileutil.md`, `specs/error_handling.md`, `specs/performance.md`.
- Infrastructure specs: `specs/build_and_run.md`, `specs/server_entry.md`, `specs/resources_and_prompts.md`.
- When researching, find knowledge, put it into Nuclia RAG MCP so we have it tagged and cached for future use.
- Background research (MCP protocol, Go SDK, log formats, analysis tasks) is available via Nuclia RAG through MCP. Query it when specs are insufficient.
- Work plans live in `plans/`. One file per task or feature.

## Specs

- Specs are the source of truth. Code follows specs, not the other way around.
- If a spec is wrong or incomplete, update the spec first, then update the code.
- When implementation reveals a spec gap, add a `TODO:` comment in code and note it in the plan.
- Never silently diverge from a spec.
- Do not modify specs without asking.
- Specs define *what*, not *how*. They contain contracts (structs, interfaces, signatures), expected outputs (JSON examples), reference data (regex patterns, threshold tables), and behaviour descriptions. No function bodies or algorithm implementations — that's what TDD is for.

## Planning

- Before starting work, create a plan in `plans/<task>.md`.
- Plans are short: goal, which specs to read, ordered steps, and acceptance criteria.
- Delete the plan when the work is done. Git is the history.

## Workflow

- Read the relevant spec before writing any code.
- Write the test first, then implement to pass it.
- Run `go test -race ./internal/<package>/` after every change.
- Run `go vet ./...` before considering a task done.
- One tool per file. One test file beside each source file. Tests in the same package.

## Permissions

- Ask before deleting or renaming existing files.
- Ask before adding any dependency beyond `github.com/modelcontextprotocol/go-sdk`.
- Ask before changing the public interface of `internal/types/`.
- Do not start implementation without a plan in `plans/`.
- Spawned agents NEVER run git commands (add, commit, push, status, etc.). Only the main Claude commits.

## Hard Rules

- NEVER write to stdout. MCP owns it. Use `slog` or `fmt.Fprintf(os.Stderr, ...)`.
- NEVER load a whole file into memory. Stream with `bufio.Scanner` or `bufio.NewReader`.
- NEVER panic. Return errors. The SDK packs tool errors into `isError: true`.
- Go regex is RE2. No lookaheads, lookbehinds, or backreferences.
- No external deps beyond the MCP SDK. Stdlib only.
- All tool outputs are Go structs marshaled to JSON. No freeform text.

## Conventions

- `gofmt` enforced. `snake_case` tool names. `CamelCase` Go types.
- Struct tags: `json:"field"` + `jsonschema:"description=...,required,minimum=N"`.
- Errors: `fmt.Errorf("context: %w", err)`.
- Tests: table-driven subtests with `t.Run`.
- Logging: `slog.Info`/`slog.Error` with structured key-value pairs.
- Code comments explain *why*, not *what*. No obvious comments.
- Every exported type and function gets a one-line GoDoc comment.

## Git

- All work is local. NEVER push, create PRs, or interact with remotes.
- Commit early and often. One logical change per commit.
- Commit messages: imperative mood, `<scope>: <what>` (e.g. `parsers: add syslog RFC5424 support`).
- Linear history only. Rebase, never merge.
- Do not commit generated files, binaries, or `bin/`.
- Do not commit secrets, credentials, or API keys. Use environment variables.
- Run `go test -race ./...` and `go vet ./...` before every commit.

## Security

- Check dependencies for known vulnerabilities: `govulncheck ./...`.
- Vet new dependencies before adding. Check for maintenance, reputation, and known issues.

## Build

```
go build -o bin/log-analysis-mcp ./cmd/log-analysis-mcp
go test -race ./...
go vet ./...
```
