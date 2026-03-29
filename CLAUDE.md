# Rules

## CLAUDE.md

- CLAUDE.md is operating rules for the AI, not project documentation.
- Keep it concise. Every line costs context window budget.
- If we change our working practices, CLAUDE.md must be updated.
- Generic rules are uploaded to Nuclia with `setup/best-practices` labels.
- Project-type rules are uploaded with `setup/best-practices` + `setup/<project-type>` labels.
- When starting a new project, pull generic + matching project-type resources from Nuclia and compose a local CLAUDE.md.
- Rules are specific and actionable. "NEVER write to stdout" not "be careful with stdout".
- Hard constraints use NEVER in caps. No ambiguity.
- Explicit permission boundaries — say what needs human approval.
- No implementation code in CLAUDE.md or specs. That's what TDD is for.

## Token Efficiency

- Always be concise and NEVER include preamble or narrative in generated files.
- Only read specs, todos, or plans relevant to the current task.
- Be concise when creating or updating specs and todos so tokens are not wasted retrieving context.

## Knowledge

- Component specs and todos live in `specs/`. Each component spec is self-contained. Read only what you need for the current task.
- Tool specs are in `specs/tools/<tool_name>.md`. Each is self-contained.
- Cross-cutting specs: `specs/types.md`, `specs/parsers.md`, `specs/fileutil.md`, `specs/error_handling.md`, `specs/performance.md`.
- Infrastructure specs: `specs/build_and_run.md`, `specs/server_entry.md`, `specs/resources_and_prompts.md`.
- Background research (MCP protocol, Go SDK, log formats, analysis tasks) is available via Nuclia RAG through MCP. Query it when specs are insufficient.
- Work plans live in `plans/`. One file per task or feature.

## Cross-Project Knowledge Base

- The Nuclia KB is shared across all projects. Each project gets its own labelset.
- One resource can carry labels from multiple project labelsets.
- Standard labels for dev project labelsets: `bug`, `enhancement`, `architecture`, `debugging`, `dependency`, `ops`, `api`.
- What to upload: Hard-won debugging knowledge, root cause analyses, architecture decisions (the why, not the what), environment gotchas, integration quirks, performance findings. Prioritize knowledge that gets lost between sessions.
- At session start: Query `nuclia_find` filtered to the project's labelset to check for known issues before investigating. Search without filters when the problem might span projects.
- After fixing a hard bug: Upload the finding with the project's labels. Include: symptoms, root cause, fix, and how long it went undetected.
- Before uploading external markdown/HTML to Nuclia, clean it with `nuclia_clean_text` to strip noise (images, relative links, admonitions, certs, base64) that degrades retrieval quality.

## Nuclia Labelsets

- Each Go MCP server project gets its own labelset (e.g. `log-analysis-mcp`) to track design, tasks, bugs, architecture decisions.
- Reference labelsets may exist for domain research (e.g. `rag` for RAG research). These are read-only. NEVER create, modify, or delete resources with research labelset labels from the implementation project.
- The `rag/mcp` label within the `rag` labelset covers MCP design and effectiveness research. Filter: `/classification.labels/rag/mcp`.

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

## Quality Maintenance

- Session start checklist: (a) read CLAUDE.md, (b) read the plan, (c) check for draft files pending review, (d) check git status.
- TODO hygiene: a session should not end with a net increase in TODOs unless they are genuinely open questions.
- Always update todos when items are completed or blocked to avoid losing context.

## Workflow

- Read the relevant spec before writing any code.
- Write the test first, then implement to pass it.
- Run `go test -race ./internal/<package>/` after every change.
- Run `go vet ./...` before considering a task done.
- One tool per file. One test file beside each source file. Tests in the same package.

## Permissions

- Ask before deleting or renaming existing files.
- Ask before restructuring directory layout.
- Ask before adding any dependency beyond `github.com/modelcontextprotocol/go-sdk`.
- Ask before changing public interfaces in `internal/types/`, `client/`, or `tools/`.
- Do not start implementation without a plan in `plans/`.

## Spawned Agents

- Scope spawned agents tightly. One file or one narrow topic per agent.
- If a task requires many changes, split across multiple agents rather than risking context exhaustion.
- Spawned agents NEVER run git commands (add, commit, push, status, etc.). Only the main Claude commits.
- Every spawn message MUST include: Do NOT run any git commands (add, commit, push, etc.). Write files only — the caller handles git.
- ALWAYS make sure the main thread and all agents are using `file-edit-mcp` tools (`fem-*`) for file operations instead of console.

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

## File Format

- No headings deeper than H3. Keep files under ~500 lines. Split if longer.

## Git

- All work is local. NEVER push, create PRs, or interact with remotes.
- Commit early and often. One logical change per commit.
- Commit messages: imperative mood, `<scope>: <what>` (e.g. `parsers: add syslog RFC5424 support`).
- Linear history only. Rebase, never merge.
- Do not commit generated files, binaries, or `bin/`.
- Do not commit secrets, credentials, or API keys. Use environment variables.
- NEVER include personal hostnames, IPs, usernames, or internal domain names in code, specs, docs, plans, or commit messages. Use generic examples (`example.com`, `10.0.0.1`, `user@host`).
- Run `go test -race ./...` and `go vet ./...` before every commit.

## Security

- Check dependencies for known vulnerabilities: `govulncheck ./...`.
- Vet new dependencies before adding. Check for maintenance, reputation, and known issues.

## Licensing

- All code must be licensed as Apache 2.0.
- Licenses for dependencies must be compatible with Apache 2.0.
- Maintain `DEPENDENCIES.md` for supply chain analysis.

## Build

```
go build -o bin/log-analysis-mcp ./cmd/log-analysis-mcp
go test -race ./...
go vet ./...
```
