## log_file_info

Lightweight metadata tool for pre-analysis decisions.

### Input

| Field | Type   | Required | Description             |
|-------|--------|----------|-------------------------|
| path  | string | yes      | Path to the log file    |

### Output

| Field            | Type   | Description                                         |
|------------------|--------|-----------------------------------------------------|
| path             | string | Absolute path to the file                           |
| size_bytes       | int    | File size in bytes                                  |
| line_count       | int    | Total number of lines                               |
| first_timestamp  | string | First detected timestamp (empty if none found)      |
| last_timestamp   | string | Last detected timestamp (empty if none found)       |
| compression_type | string | "gzip", "bzip2", or "none"                          |
| is_binary        | bool   | True if file appears to contain binary content       |

### Behaviour

- Single streaming pass for line count + timestamps.
- Binary detection: file is binary if first 512 bytes contain a NUL byte.
- Compression detection: based on file extension (.gz → gzip, .bz2 → bzip2).
- Timestamps: parse first/last lines that match any known parser format.
- For compressed files, size_bytes is the compressed size on disk.
- Uses `fileutil.OpenReader` so compressed files are decompressed for line counting.
