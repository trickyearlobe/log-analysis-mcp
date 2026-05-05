# Tool: `log_list_archive`

**Description (shown to LLM):**
> List the contents of an archive file (zip, tar.gz, tar.bz2). Returns entry names, sizes, and modification times. Use to discover which log files exist inside an archive before extracting them.

---

## Input Schema

| Parameter     | Type     | Required | Default | Description                                    |
| ------------- | -------- | -------- | ------- | ---------------------------------------------- |
| `path`        | `string` | Yes      | —       | Path to the archive file                       |
| `max_entries` | `int`    | No       | `200`   | Maximum entries to return (max 1000)           |
| `pattern`     | `string` | No       | —       | Glob pattern to filter entries (e.g. `*.log`)  |

### Go Input Struct

```go
type ListArchiveInput struct {
    Path       string `json:"path"                    jsonschema:"Path to the archive file (.zip, .tar.gz, .tar.bz2)"`
    MaxEntries int    `json:"max_entries,omitempty"    jsonschema:"Maximum entries to return (max 1000)"`
    Pattern    string `json:"pattern,omitempty"        jsonschema:"Glob pattern to filter entry names (e.g. *.log)"`
}
```

### Default Values (applied in handler)

| Field        | Default |
| ------------ | ------- |
| `MaxEntries` | `200`   |

---

## Output Format

### Go Output Structs

```go
type ArchiveEntry struct {
    Name    string `json:"name"`
    Size    int64  `json:"size"`
    ModTime string `json:"mod_time"`
    IsDir   bool   `json:"is_dir"`
}

type ListArchiveOutput struct {
    Entries      []ArchiveEntry `json:"entries"`
    TotalEntries int            `json:"total_entries"`
    ArchiveType  string         `json:"archive_type"`
    HasMore      bool           `json:"has_more"`
}
```

### JSON Example

```json
{
  "entries": [
    {"name": "logs/app.log", "size": 1048576, "mod_time": "2025-01-15T10:00:00Z", "is_dir": false},
    {"name": "logs/error.log", "size": 524288, "mod_time": "2025-01-15T10:00:00Z", "is_dir": false}
  ],
  "total_entries": 2,
  "archive_type": "tar.gz",
  "has_more": false
}
```

---

## Supported Archive Types

| Extension        | Detection      | Implementation                                       |
|------------------|---------------|------------------------------------------------------|
| `.zip`           | Extension     | `archive/zip.OpenReader` → iterate `File` entries    |
| `.tar.gz`, `.tgz`| Extension    | `os.Open` → `gzip.NewReader` → `tar.NewReader`      |
| `.tar.bz2`      | Extension     | `os.Open` → `bzip2.NewReader` → `tar.NewReader`     |
| `.tar`           | Extension     | `os.Open` → `tar.NewReader`                          |

Extension matching is case-insensitive.

---

## Handler Signature

```go
func handleListArchive(ctx context.Context, req *mcp.CallToolRequest, input ListArchiveInput) (*mcp.CallToolResult, error)
```

---

## Edge Cases

- **Not an archive:** Return error `INVALID_INPUT: file does not have a recognised archive extension`.
- **Empty archive:** Return empty entries, `total_entries: 0`.
- **Corrupt archive:** Return error propagated from stdlib reader.
- **Pattern match:** Uses `filepath.Match` (Go glob). Invalid pattern returns validation error.
- **Entries exceed max:** Set `has_more: true`, return only first `max_entries`.
