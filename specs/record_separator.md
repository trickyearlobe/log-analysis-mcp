# Record Separator — Behavioural Specification

## Purpose

Enable tools to treat multi-line log entries (Java stack traces, Python tracebacks, Go panics, PostgreSQL continuations) as single logical records. A user-supplied regex identifies the **start** of each new record; all lines between consecutive matches form one record.

## Scope

Affects tools: `log_extract_errors`, `log_summarize`, `log_filter`, `log_search`.

---

## Parameter

| Field              | Type   | Required | Description                                              |
|--------------------|--------|----------|----------------------------------------------------------|
| `record_separator` | string | no       | RE2 regex matching the **first line** of a new record.   |

When absent or empty, tools operate in line-by-line mode (existing behaviour unchanged).

Common examples:
- `^\d{4}-\d{2}-\d{2}` — ISO date at line start (most structured logs)
- `^<\d+>` — syslog priority prefix
- `^[A-Z], \[` — Ruby logger format
- `^\w+ \d{2}, \d{4}` — Java `util.logging` format

---

## Record Reader Contract

### Location

`internal/fileutil/record.go`

### Interface

```go
// Record holds one logical log record that may span multiple raw lines.
type Record struct {
    StartLine int      // 1-based line number of first line
    LineCount int      // number of raw lines in this record
    Text      string   // all lines joined by "\n"
}

// RecordScanner streams records from a file using a separator regex.
type RecordScanner struct { ... }

func NewRecordScanner(path string, separator *regexp.Regexp) (*RecordScanner, error)
func (rs *RecordScanner) Scan() bool
func (rs *RecordScanner) Record() Record
func (rs *RecordScanner) Err() error
func (rs *RecordScanner) Close() error
```

### Behaviour

1. Open file via `OpenReader` (supports .gz/.bz2/.zip transparently).
2. Wrap in `bufio.Reader` (no scanner — avoids fallback complexity with compressed streams).
3. Read lines sequentially via `ReadString('\n')`.
4. **Before first separator match:** each line is emitted as its own single-line record.
5. **After first separator match:** accumulate lines. When the next separator match is found, finalize the accumulated record and start a new one.
6. On EOF, finalize the last accumulated record.

### Safety Bounds

| Limit         | Value   | Behaviour when exceeded                                        |
|---------------|---------|----------------------------------------------------------------|
| Max lines     | 500     | Emit truncated record, then skip/discard until next separator. |
| Max bytes     | 65536   | Emit truncated record, then skip/discard until next separator. |

When a record is truncated, set `Record.Truncated = true`.

On overflow, lines are **discarded** (not accumulated) until the next separator match. This avoids creating fake record boundaries at non-separator lines.

### Edge Cases

- **Empty file:** `Scan()` returns false immediately, `Err()` is nil.
- **Separator never matches:** Every line is its own single-line record (no accumulation ever starts since the first match never occurs).
- **Separator matches every line:** Each line is its own record (equivalent to no separator).
- **Invalid regex:** Return error from `NewRecordScanner`, do not panic.
- **Single line exceeds 64KB:** Emit as one record with `Truncated=true`, `LineCount=1`.
- **Close before EOF:** Safe. Releases underlying reader. Subsequent `Scan()` returns false.

---

## Tool Integration

Each tool adds `record_separator` to its input struct. When present:

### log_filter, log_search

- Match predicate applies to the **entire record text** (all lines joined).
- A match returns the full record, not just the matching line.
- `line_number` in results refers to the record's `StartLine`.
- `context_lines` parameter is ignored when record_separator is active (the record IS the context).

### log_extract_errors

- Each record is parsed using the **first line only** (via the detected parser).
- Remaining lines become the `StackTrace` field (joined by "\n").
- Supersedes `include_stack_traces` when active — stack traces are inherently captured.
- Clustering operates on the normalized first-line message.

### log_summarize

- Counts **records** as entries (not raw lines). `total_lines` still reports raw line count.
- Level distribution uses the level parsed from each record's first line.

---

## Interaction with Existing MultilineCombiner

When `record_separator` is active, the existing `MultilineCombiner` is **not used**. The user-supplied separator takes precedence. This avoids conflicting boundary detection.

When `record_separator` is absent, `log_extract_errors` with `include_stack_traces=true` continues to use `MultilineCombiner` as before.
