# Parser Specifications

### Auto-Detection Engine (`autodetect.go`)

The auto-detection engine determines the format of a log file by scoring each parser against a sample of lines.

**Algorithm:**

1. Read the first 10 non-empty lines of the file.
2. Run each parser against every sample line.
3. Score each parser: `score = successful_parses / total_sample_lines`.
4. Select the parser with the highest score, provided `score >= 0.5`.
5. If no parser scores ≥ 0.5, return `"unknown"` format with raw-line fallback.
6. If `format_hint` is provided and is not `"auto"`, skip auto-detection and use the specified parser directly.

**Parser Priority (tiebreaker):** JSON > Syslog RFC 5424 > Syslog RFC 3164 > Apache Combined > Apache Common

**Parser interface:**

```go
// Parser defines the interface every log format parser must implement.
type Parser interface {
    // Parse attempts to parse a single log line. Returns nil if the line
    // does not match this parser's format.
    Parse(line string) *types.ParsedLogEntry

    // Detect returns a confidence score (0.0–1.0) for a sample of lines.
    Detect(lines []string) float64

    // Name returns the format identifier (e.g., "syslog-rfc3164").
    Name() string
}
```

**AutoDetect signature:**

```go
func AutoDetect(lines []string) types.FormatDetectionResult
```

Scores every registered parser against the provided sample lines and returns the best match. Parsers are evaluated in priority order (JSON, Syslog RFC 5424, Syslog RFC 3164, Apache Combined, Apache Common) so that ties are broken deterministically.

### Syslog Parser (`syslog.go`)

Supports two syslog formats:

**RFC 3164 (BSD Syslog):**

```
<PRI>TIMESTAMP HOSTNAME APP-NAME[PROCID]: MESSAGE
```

RE2 regex pattern:
```
^(?:<(\d{1,3})>)?(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s+(.*)$
```

Extracted fields:
- `priority` (optional): Numeric priority value (facility × 8 + severity)
- `facility`: Derived from priority (kern, user, mail, daemon, auth, syslog, lpr, news, uucp, cron, local0-7)
- `severity`: Derived from priority (emerg, alert, crit, err, warning, notice, info, debug)
- `timestamp`: Parsed from `MMM DD HH:MM:SS` format (year inferred from current year)
- `hostname`: Source hostname
- `app_name`: Application name
- `pid` (optional): Process ID
- `message`: The log message content

**RFC 5424 (Modern Syslog):**

```
<PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MESSAGE
```

RE2 regex pattern:
```
^<(\d{1,3})>(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:\[([^\]]*)\]\s*)*(.*)$
```

Additional extracted fields:
- `version`: Syslog protocol version
- `msg_id`: Message identifier
- `structured_data`: Key-value pairs from structured data elements

**Level Mapping:** The parser normalizes syslog severity to standard log levels:

| Syslog Severity | Normalized Level |
| --------------- | ---------------- |
| emerg, alert    | FATAL            |
| crit            | CRITICAL         |
| err             | ERROR            |
| warning         | WARN             |
| notice, info    | INFO             |
| debug           | DEBUG            |

**RE2 Note:** All regex patterns in this specification are RE2-compatible (no lookaheads, lookbehinds, or backreferences). Patterns should be compiled once at init time for safety and performance.

### Apache/Nginx Parser (`apache.go`)

**Combined Log Format:**

```
%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"
```

Example:
```
192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"
```

RE2 regex pattern:
```
^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+(\S+)\s+(\S+)"\s+(\d{3})\s+(\S+)(?:\s+"([^"]*)")?(?:\s+"([^"]*)")?$
```

Extracted fields:
- `remote_host`: Client IP address
- `identity`: RFC 1413 identity (usually `-`)
- `user`: Authenticated user (or `-`)
- `timestamp`: Parsed from `DD/MMM/YYYY:HH:MM:SS ±HHMM`
- `method`: HTTP method (GET, POST, etc.)
- `path`: Request path
- `protocol`: HTTP protocol version
- `status`: HTTP status code (number)
- `bytes`: Response size in bytes (or 0 if `-`)
- `referer`: Referring URL (Combined format only)
- `user_agent`: Client user agent string (Combined format only)

**Common Log Format:** Same as Combined but without `referer` and `user_agent` fields.

**Level Mapping:** HTTP status codes are mapped to log levels:

| Status Range | Normalized Level |
| ------------ | ---------------- |
| 1xx, 2xx     | INFO             |
| 3xx          | INFO             |
| 4xx          | WARN             |
| 5xx          | ERROR            |

### JSON Log Parser (`jsonlog.go`)

Parses each line as a standalone JSON object using `encoding/json`. Handles various common JSON logging frameworks (Winston, Bunyan, Pino, Logstash, structlog, zerolog, zap, etc.).

**Field Normalization:**

The parser normalizes field names from various logging frameworks to a standard set:

| Standard Field | Accepted Variants                                               |
| -------------- | --------------------------------------------------------------- |
| `timestamp`    | `ts`, `time`, `timestamp`, `@timestamp`, `date`, `datetime`, `t` |
| `level`        | `level`, `severity`, `log_level`, `loglevel`, `lvl`, `priority` |
| `message`      | `msg`, `message`, `log`, `text`, `body`                         |
| `source`       | `source`, `logger`, `component`, `module`, `name`, `service`    |

**Level Normalization:**

The parser normalizes level values to uppercase standard forms:

| Normalized | Accepted Variants                            |
| ---------- | -------------------------------------------- |
| `TRACE`    | `trace`, `TRACE`, `10`                       |
| `DEBUG`    | `debug`, `DEBUG`, `20`                       |
| `INFO`     | `info`, `INFO`, `information`, `30`          |
| `WARN`     | `warn`, `WARN`, `warning`, `WARNING`, `40`   |
| `ERROR`    | `error`, `ERROR`, `err`, `ERR`, `50`         |
| `FATAL`    | `fatal`, `FATAL`, `critical`, `CRITICAL`, `60` |

**Extra Fields:** Any JSON fields not matching the standard set are placed in `extra_fields` as `map[string]interface{}`.

**Error Handling:** Lines that are not valid JSON are returned as parse errors with the raw content preserved.

**Parse Method Behavior:**

The `Parse` method must:

1. Attempt to unmarshal the input line as a JSON object (`map[string]interface{}`).
2. If the line is not valid JSON, return a parse error with the raw content preserved.
3. Iterate over the unmarshalled map and normalize field names using the field mapping table above (e.g., `msg` → `message`, `ts` → `timestamp`).
4. Normalize the level value to its uppercase standard form using the level normalization table above (e.g., `warning` → `WARN`, `50` → `ERROR`).
5. Place any JSON fields that do not match a standard field name into `extra_fields` as `map[string]interface{}`.
6. Return the populated `ParsedLogEntry`.

### Multiline Combiner (`multiline.go`)

The multiline combiner aggregates continuation lines (such as stack traces) with their originating log entry.

**Detection Patterns:**

| Pattern Type         | Detection Rule                                                       |
| -------------------- | -------------------------------------------------------------------- |
| Java stack trace     | Lines starting with `\tat ` or `Caused by:`                         |
| Python traceback     | Block starting with `Traceback (most recent call last):`             |
| .NET stack trace     | Lines starting with `   at ` (three spaces + "at")                   |
| Generic continuation | Lines starting with whitespace that follow a parseable log entry      |
| Wrapped lines        | Lines that don't match any known log format after a parseable entry   |

**Algorithm:**

1. Process lines sequentially.
2. When a line matches a known log entry format, start a new entry.
3. When a subsequent line matches a continuation pattern, append it to the current entry's `stack_trace` or `continuation` field.
4. When the next parseable log entry is found, finalize the previous entry.
5. The combined entry preserves the original `line_number` of the first line, and includes a `line_count` field indicating how many raw lines it spans.

**Output Enhancement:**

When multiline combining is active, parsed records gain additional fields:

```json
{
  "line_number": 1523,
  "line_count": 12,
  "timestamp": "2025-01-15T14:31:02.000Z",
  "level": "ERROR",
  "source": "app",
  "message": "Unhandled exception in request handler",
  "stack_trace": "java.lang.NullPointerException\n\tat com.example.Handler.process(Handler.java:42)\n\tat com.example.Server.handle(Server.java:118)\nCaused by: java.lang.IllegalStateException\n\tat com.example.Dao.find(Dao.java:67)",
  "raw": "(first line only)",
  "extra_fields": {}
}
```

**Multiline detection regex patterns:**

| Name             | Pattern                                    |
| ---------------- | ------------------------------------------ |
| `javaStackRe`    | `^\tat `                                   |
| `causedByRe`     | `^Caused by:`                              |
| `pythonTBRe`     | `^Traceback \(most recent call last\):`    |
| `dotnetStackRe`  | `^   at `                                  |
| `continuationRe` | `^\s+`                                     |