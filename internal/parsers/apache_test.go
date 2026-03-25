package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestApacheCombinedParser_Name(t *testing.T) {
	p := NewApacheCombinedParser()
	if got := p.Name(); got != "apache-combined" {
		t.Errorf("Name() = %q, want %q", got, "apache-combined")
	}
}

func TestApacheCommonParser_Name(t *testing.T) {
	p := NewApacheCommonParser()
	if got := p.Name(); got != "apache-common" {
		t.Errorf("Name() = %q, want %q", got, "apache-common")
	}
}

func TestApacheCombinedParser_ImplementsInterface(t *testing.T) {
	var _ Parser = NewApacheCombinedParser()
}

func TestApacheCommonParser_ImplementsInterface(t *testing.T) {
	var _ Parser = NewApacheCommonParser()
}

func TestApacheCombinedParser_Parse(t *testing.T) {
	p := NewApacheCombinedParser()

	tests := []struct {
		name           string
		line           string
		wantNil        bool
		wantTimestamp  string
		wantLevel      types.LogLevel
		wantSource     string
		wantMessage    string
		wantExtraField map[string]interface{}
		wantAbsent     []string // keys that must NOT be in extra_fields
	}{
		{
			name:          "basic combined format",
			line:          `192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"`,
			wantNil:       false,
			wantTimestamp: "10/Jan/2025:13:55:36 -0700",
			wantLevel:     types.LogLevelInfo,
			wantSource:    "192.168.1.1",
			wantMessage:   "GET /api/users HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"remote_host": "192.168.1.1",
				"user":        "frank",
				"method":      "GET",
				"path":        "/api/users",
				"protocol":    "HTTP/1.1",
				"status":      "200",
				"bytes":       "2326",
				"referer":     "https://example.com",
				"user_agent":  "Mozilla/5.0",
			},
			wantAbsent: []string{"identity"},
		},
		{
			name:          "status 200 maps to INFO",
			line:          `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /index.html HTTP/1.1" 200 1024 "https://ref.com" "curl/7.68"`,
			wantNil:       false,
			wantTimestamp: "15/Feb/2025:08:00:00 +0000",
			wantLevel:     types.LogLevelInfo,
			wantSource:    "10.0.0.1",
			wantMessage:   "GET /index.html HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status": "200",
			},
			wantAbsent: []string{"user", "identity"},
		},
		{
			name:        "status 100 maps to INFO",
			line:        `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /check HTTP/1.1" 100 0 "-" "curl/7.68"`,
			wantNil:     false,
			wantLevel:   types.LogLevelInfo,
			wantMessage: "GET /check HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status": "100",
				"bytes":  "0",
			},
		},
		{
			name:        "status 301 maps to INFO",
			line:        `172.16.0.5 - alice [20/Mar/2025:14:30:00 +0100] "GET /old-page HTTP/1.1" 301 512 "https://example.com/home" "Mozilla/5.0 (Windows)"`,
			wantNil:     false,
			wantLevel:   types.LogLevelInfo,
			wantMessage: "GET /old-page HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status": "301",
				"user":   "alice",
			},
		},
		{
			name:        "status 404 maps to WARN",
			line:        `192.168.0.100 - - [05/Apr/2025:09:15:00 -0500] "GET /missing HTTP/1.1" 404 196 "-" "Mozilla/5.0"`,
			wantNil:     false,
			wantLevel:   types.LogLevelWarn,
			wantMessage: "GET /missing HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status": "404",
			},
		},
		{
			name:      "status 403 maps to WARN",
			line:      `192.168.0.100 - - [05/Apr/2025:09:15:00 -0500] "GET /secret HTTP/1.1" 403 0 "-" "Mozilla/5.0"`,
			wantNil:   false,
			wantLevel: types.LogLevelWarn,
			wantExtraField: map[string]interface{}{
				"status": "403",
			},
		},
		{
			name:        "status 500 maps to ERROR",
			line:        `10.10.10.10 - - [12/May/2025:22:45:00 +0000] "POST /api/submit HTTP/1.1" 500 0 "https://app.example.com/form" "Mozilla/5.0 (X11; Linux x86_64)"`,
			wantNil:     false,
			wantLevel:   types.LogLevelError,
			wantMessage: "POST /api/submit HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status":     "500",
				"method":     "POST",
				"bytes":      "0",
				"user_agent": "Mozilla/5.0 (X11; Linux x86_64)",
			},
		},
		{
			name:      "status 502 maps to ERROR",
			line:      `10.10.10.10 - - [12/May/2025:22:45:00 +0000] "GET /health HTTP/1.1" 502 42 "-" "kube-probe/1.25"`,
			wantNil:   false,
			wantLevel: types.LogLevelError,
			wantExtraField: map[string]interface{}{
				"status": "502",
			},
		},
		{
			name:        "missing bytes dash becomes 0",
			line:        `192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET /empty HTTP/1.0" 204 - "-" "test-agent"`,
			wantNil:     false,
			wantLevel:   types.LogLevelInfo,
			wantMessage: "GET /empty HTTP/1.0",
			wantExtraField: map[string]interface{}{
				"bytes": "0",
			},
		},
		{
			name:           "missing user dash omitted from extra_fields",
			line:           `192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET / HTTP/1.1" 200 100 "-" "bot"`,
			wantNil:        false,
			wantExtraField: map[string]interface{}{},
			wantAbsent:     []string{"user", "identity"},
		},
		{
			name:    "identity present when not dash",
			line:    `192.168.1.1 ident_user frank [10/Jan/2025:13:55:36 -0700] "GET / HTTP/1.1" 200 100 "-" "bot"`,
			wantNil: false,
			wantExtraField: map[string]interface{}{
				"identity": "ident_user",
				"user":     "frank",
			},
		},
		{
			name:           "referer dash omitted from extra_fields",
			line:           `1.2.3.4 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 50 "-" "agent"`,
			wantNil:        false,
			wantExtraField: map[string]interface{}{},
			wantAbsent:     []string{"referer"},
		},
		{
			name:    "referer present when not dash",
			line:    `1.2.3.4 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 50 "https://google.com" "agent"`,
			wantNil: false,
			wantExtraField: map[string]interface{}{
				"referer": "https://google.com",
			},
		},
		{
			name:    "empty string returns nil",
			line:    "",
			wantNil: true,
		},
		{
			name:    "non-matching line returns nil",
			line:    "this is just some random text that is not a log line",
			wantNil: true,
		},
		{
			name:    "json line returns nil",
			line:    `{"timestamp":"2025-01-10T13:55:36Z","level":"info","message":"hello"}`,
			wantNil: true,
		},
		{
			name:    "syslog line returns nil",
			line:    "Jan 10 13:55:36 myhost sshd[12345]: Accepted publickey for user from 192.168.1.1",
			wantNil: true,
		},
		{
			name:          "real-world nginx combined log",
			line:          `172.17.0.1 - - [10/Jan/2025:13:55:36 +0000] "GET /api/v2/health HTTP/2.0" 200 15 "https://dashboard.example.com/status" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"`,
			wantNil:       false,
			wantTimestamp: "10/Jan/2025:13:55:36 +0000",
			wantLevel:     types.LogLevelInfo,
			wantSource:    "172.17.0.1",
			wantMessage:   "GET /api/v2/health HTTP/2.0",
			wantExtraField: map[string]interface{}{
				"remote_host": "172.17.0.1",
				"method":      "GET",
				"path":        "/api/v2/health",
				"protocol":    "HTTP/2.0",
				"status":      "200",
				"bytes":       "15",
				"referer":     "https://dashboard.example.com/status",
				"user_agent":  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			},
		},
		{
			name:        "real-world nginx 499 client closed",
			line:        `10.244.0.1 - admin [25/Jun/2025:03:14:07 +0000] "POST /api/webhook HTTP/1.1" 499 0 "https://app.internal/hooks" "Python-urllib/3.11"`,
			wantNil:     false,
			wantLevel:   types.LogLevelWarn,
			wantSource:  "10.244.0.1",
			wantMessage: "POST /api/webhook HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"status":     "499",
				"user":       "admin",
				"bytes":      "0",
				"user_agent": "Python-urllib/3.11",
			},
		},
		{
			name:       "IPv6 remote host",
			line:       `::1 - - [10/Jan/2025:00:00:00 +0000] "GET /local HTTP/1.1" 200 42 "-" "curl/8.0"`,
			wantNil:    false,
			wantSource: "::1",
			wantExtraField: map[string]interface{}{
				"remote_host": "::1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := p.Parse(tt.line)

			if tt.wantNil {
				if entry != nil {
					t.Fatalf("Parse() returned non-nil, want nil")
				}
				return
			}

			if entry == nil {
				t.Fatalf("Parse() returned nil, want non-nil")
			}

			if entry.Raw != tt.line {
				t.Errorf("Raw = %q, want %q", entry.Raw, tt.line)
			}

			if tt.wantTimestamp != "" {
				if entry.Timestamp == nil {
					t.Fatalf("Timestamp is nil, want %q", tt.wantTimestamp)
				}
				if *entry.Timestamp != tt.wantTimestamp {
					t.Errorf("Timestamp = %q, want %q", *entry.Timestamp, tt.wantTimestamp)
				}
			}

			if tt.wantLevel != "" {
				if entry.Level == nil {
					t.Fatalf("Level is nil, want %q", tt.wantLevel)
				}
				if *entry.Level != tt.wantLevel {
					t.Errorf("Level = %q, want %q", *entry.Level, tt.wantLevel)
				}
			}

			if tt.wantSource != "" {
				if entry.Source == nil {
					t.Fatalf("Source is nil, want %q", tt.wantSource)
				}
				if *entry.Source != tt.wantSource {
					t.Errorf("Source = %q, want %q", *entry.Source, tt.wantSource)
				}
			}

			if tt.wantMessage != "" {
				if entry.Message != tt.wantMessage {
					t.Errorf("Message = %q, want %q", entry.Message, tt.wantMessage)
				}
			}

			for key, wantVal := range tt.wantExtraField {
				gotVal, ok := entry.ExtraFields[key]
				if !ok {
					t.Errorf("ExtraFields missing key %q", key)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("ExtraFields[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}

			for _, key := range tt.wantAbsent {
				if _, ok := entry.ExtraFields[key]; ok {
					t.Errorf("ExtraFields[%q] should be absent but is present", key)
				}
			}
		})
	}
}

func TestApacheCommonParser_Parse(t *testing.T) {
	p := NewApacheCommonParser()

	tests := []struct {
		name           string
		line           string
		wantNil        bool
		wantTimestamp  string
		wantLevel      types.LogLevel
		wantSource     string
		wantMessage    string
		wantExtraField map[string]interface{}
		wantAbsent     []string
	}{
		{
			name:          "basic common format",
			line:          `192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326`,
			wantNil:       false,
			wantTimestamp: "10/Jan/2025:13:55:36 -0700",
			wantLevel:     types.LogLevelInfo,
			wantSource:    "192.168.1.1",
			wantMessage:   "GET /api/users HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"remote_host": "192.168.1.1",
				"user":        "frank",
				"method":      "GET",
				"path":        "/api/users",
				"protocol":    "HTTP/1.1",
				"status":      "200",
				"bytes":       "2326",
			},
			wantAbsent: []string{"identity", "referer", "user_agent"},
		},
		{
			name:      "status 200 maps to INFO",
			line:      `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /index.html HTTP/1.1" 200 1024`,
			wantNil:   false,
			wantLevel: types.LogLevelInfo,
			wantExtraField: map[string]interface{}{
				"status": "200",
			},
		},
		{
			name:      "status 301 maps to INFO",
			line:      `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /old HTTP/1.1" 301 0`,
			wantNil:   false,
			wantLevel: types.LogLevelInfo,
			wantExtraField: map[string]interface{}{
				"status": "301",
			},
		},
		{
			name:      "status 404 maps to WARN",
			line:      `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /nope HTTP/1.1" 404 0`,
			wantNil:   false,
			wantLevel: types.LogLevelWarn,
			wantExtraField: map[string]interface{}{
				"status": "404",
			},
		},
		{
			name:      "status 500 maps to ERROR",
			line:      `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "POST /crash HTTP/1.1" 500 0`,
			wantNil:   false,
			wantLevel: types.LogLevelError,
			wantExtraField: map[string]interface{}{
				"status": "500",
				"method": "POST",
			},
		},
		{
			name:      "status 503 maps to ERROR",
			line:      `10.0.0.1 - - [15/Feb/2025:08:00:00 +0000] "GET /down HTTP/1.1" 503 0`,
			wantNil:   false,
			wantLevel: types.LogLevelError,
			wantExtraField: map[string]interface{}{
				"status": "503",
			},
		},
		{
			name:        "missing bytes dash becomes 0",
			line:        `192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "HEAD /empty HTTP/1.1" 204 -`,
			wantNil:     false,
			wantMessage: "HEAD /empty HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"bytes":  "0",
				"method": "HEAD",
			},
		},
		{
			name:       "missing user dash omitted",
			line:       `192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET / HTTP/1.1" 200 100`,
			wantNil:    false,
			wantAbsent: []string{"user", "identity"},
		},
		{
			name:    "identity and user both present",
			line:    `192.168.1.1 ident_val bob [10/Jan/2025:13:55:36 -0700] "GET / HTTP/1.1" 200 100`,
			wantNil: false,
			wantExtraField: map[string]interface{}{
				"identity": "ident_val",
				"user":     "bob",
			},
		},
		{
			name:    "empty string returns nil",
			line:    "",
			wantNil: true,
		},
		{
			name:    "non-matching line returns nil",
			line:    "ERROR 2025-01-10 something went wrong",
			wantNil: true,
		},
		{
			name:    "combined format line does not match common parser",
			line:    `192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"`,
			wantNil: true,
		},
		{
			name:        "no referer or user_agent fields in common format",
			line:        `1.2.3.4 - - [01/Jun/2025:12:00:00 +0000] "DELETE /api/item/42 HTTP/1.1" 200 0`,
			wantNil:     false,
			wantMessage: "DELETE /api/item/42 HTTP/1.1",
			wantExtraField: map[string]interface{}{
				"method": "DELETE",
				"path":   "/api/item/42",
			},
			wantAbsent: []string{"referer", "user_agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := p.Parse(tt.line)

			if tt.wantNil {
				if entry != nil {
					t.Fatalf("Parse() returned non-nil, want nil")
				}
				return
			}

			if entry == nil {
				t.Fatalf("Parse() returned nil, want non-nil")
			}

			if entry.Raw != tt.line {
				t.Errorf("Raw = %q, want %q", entry.Raw, tt.line)
			}

			if tt.wantTimestamp != "" {
				if entry.Timestamp == nil {
					t.Fatalf("Timestamp is nil, want %q", tt.wantTimestamp)
				}
				if *entry.Timestamp != tt.wantTimestamp {
					t.Errorf("Timestamp = %q, want %q", *entry.Timestamp, tt.wantTimestamp)
				}
			}

			if tt.wantLevel != "" {
				if entry.Level == nil {
					t.Fatalf("Level is nil, want %q", tt.wantLevel)
				}
				if *entry.Level != tt.wantLevel {
					t.Errorf("Level = %q, want %q", *entry.Level, tt.wantLevel)
				}
			}

			if tt.wantSource != "" {
				if entry.Source == nil {
					t.Fatalf("Source is nil, want %q", tt.wantSource)
				}
				if *entry.Source != tt.wantSource {
					t.Errorf("Source = %q, want %q", *entry.Source, tt.wantSource)
				}
			}

			if tt.wantMessage != "" {
				if entry.Message != tt.wantMessage {
					t.Errorf("Message = %q, want %q", entry.Message, tt.wantMessage)
				}
			}

			for key, wantVal := range tt.wantExtraField {
				gotVal, ok := entry.ExtraFields[key]
				if !ok {
					t.Errorf("ExtraFields missing key %q", key)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("ExtraFields[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}

			for _, key := range tt.wantAbsent {
				if _, ok := entry.ExtraFields[key]; ok {
					t.Errorf("ExtraFields[%q] should be absent but is present", key)
				}
			}
		})
	}
}

func TestStatusToLevel(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   types.LogLevel
	}{
		{"100 Continue", 100, types.LogLevelInfo},
		{"101 Switching Protocols", 101, types.LogLevelInfo},
		{"200 OK", 200, types.LogLevelInfo},
		{"201 Created", 201, types.LogLevelInfo},
		{"204 No Content", 204, types.LogLevelInfo},
		{"299 upper 2xx", 299, types.LogLevelInfo},
		{"301 Moved Permanently", 301, types.LogLevelInfo},
		{"302 Found", 302, types.LogLevelInfo},
		{"304 Not Modified", 304, types.LogLevelInfo},
		{"399 upper 3xx", 399, types.LogLevelInfo},
		{"400 Bad Request", 400, types.LogLevelWarn},
		{"401 Unauthorized", 401, types.LogLevelWarn},
		{"403 Forbidden", 403, types.LogLevelWarn},
		{"404 Not Found", 404, types.LogLevelWarn},
		{"429 Too Many Requests", 429, types.LogLevelWarn},
		{"499 upper 4xx", 499, types.LogLevelWarn},
		{"500 Internal Server Error", 500, types.LogLevelError},
		{"502 Bad Gateway", 502, types.LogLevelError},
		{"503 Service Unavailable", 503, types.LogLevelError},
		{"504 Gateway Timeout", 504, types.LogLevelError},
		{"599 upper 5xx", 599, types.LogLevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusToLevel(tt.status)
			if got != tt.want {
				t.Errorf("statusToLevel(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestApacheCombinedParser_Detect(t *testing.T) {
	p := NewApacheCombinedParser()

	tests := []struct {
		name      string
		lines     []string
		wantMin   float64
		wantMax   float64
		wantExact *float64
	}{
		{
			name:    "empty input returns 0",
			lines:   []string{},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name: "all matching lines",
			lines: []string{
				`192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET /a HTTP/1.1" 200 100 "-" "agent"`,
				`192.168.1.2 - - [10/Jan/2025:13:55:37 -0700] "POST /b HTTP/1.1" 201 200 "https://ref.com" "agent"`,
				`192.168.1.3 - - [10/Jan/2025:13:55:38 -0700] "DELETE /c HTTP/1.1" 404 0 "-" "curl/8.0"`,
			},
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name: "no matching lines",
			lines: []string{
				"this is not a log line",
				`{"json": true}`,
				"Jan 10 13:55:36 myhost sshd: login",
			},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name: "mixed matching and non-matching",
			lines: []string{
				`192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET /a HTTP/1.1" 200 100 "-" "agent"`,
				"not a log line",
				`192.168.1.3 - - [10/Jan/2025:13:55:38 -0700] "GET /c HTTP/1.1" 200 0 "-" "agent"`,
				"also not a log line",
			},
			wantMin: 0.49,
			wantMax: 0.51,
		},
		{
			name: "3 out of 4 matching",
			lines: []string{
				`10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET /1 HTTP/1.1" 200 0 "-" "a"`,
				`10.0.0.1 - - [01/Jan/2025:00:00:01 +0000] "GET /2 HTTP/1.1" 200 0 "-" "a"`,
				`10.0.0.1 - - [01/Jan/2025:00:00:02 +0000] "GET /3 HTTP/1.1" 200 0 "-" "a"`,
				"not a log",
			},
			wantMin: 0.74,
			wantMax: 0.76,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect(tt.lines)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Detect() = %f, want between %f and %f", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestApacheCommonParser_Detect(t *testing.T) {
	p := NewApacheCommonParser()

	tests := []struct {
		name    string
		lines   []string
		wantMin float64
		wantMax float64
	}{
		{
			name:    "empty input returns 0",
			lines:   []string{},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name: "all matching common lines",
			lines: []string{
				`192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET /a HTTP/1.1" 200 100`,
				`192.168.1.2 - bob [10/Jan/2025:13:55:37 -0700] "POST /b HTTP/1.1" 201 200`,
			},
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name: "combined format lines do not match common parser",
			lines: []string{
				`192.168.1.1 - - [10/Jan/2025:13:55:36 -0700] "GET /a HTTP/1.1" 200 100 "-" "agent"`,
				`192.168.1.2 - - [10/Jan/2025:13:55:37 -0700] "POST /b HTTP/1.1" 201 200 "https://ref.com" "agent"`,
			},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name: "mixed common and junk",
			lines: []string{
				`10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET /ok HTTP/1.1" 200 0`,
				"random junk",
			},
			wantMin: 0.49,
			wantMax: 0.51,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect(tt.lines)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Detect() = %f, want between %f and %f", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestApacheCombinedParser_RawPreserved(t *testing.T) {
	p := NewApacheCombinedParser()
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 42 "-" "test"`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if entry.Raw != line {
		t.Errorf("Raw not preserved: got %q", entry.Raw)
	}
}

func TestApacheCommonParser_RawPreserved(t *testing.T) {
	p := NewApacheCommonParser()
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 42`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if entry.Raw != line {
		t.Errorf("Raw not preserved: got %q", entry.Raw)
	}
}

func TestApacheCombinedParser_ExtraFieldsNotNil(t *testing.T) {
	p := NewApacheCombinedParser()
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 0 "-" "a"`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if entry.ExtraFields == nil {
		t.Error("ExtraFields is nil, want initialized map")
	}
}

func TestApacheCommonParser_ExtraFieldsNotNil(t *testing.T) {
	p := NewApacheCommonParser()
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 0`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if entry.ExtraFields == nil {
		t.Error("ExtraFields is nil, want initialized map")
	}
}

func TestApacheCombinedParser_RealWorldNginx(t *testing.T) {
	p := NewApacheCombinedParser()

	lines := []struct {
		name string
		line string
	}{
		{
			name: "nginx default combined with complex user agent",
			line: `66.249.66.1 - - [25/Jun/2025:10:00:00 +0000] "GET /robots.txt HTTP/1.1" 200 128 "-" "Googlebot/2.1 (+http://www.google.com/bot.html)"`,
		},
		{
			name: "nginx with authenticated user and referer",
			line: `203.0.113.50 - jdoe [25/Jun/2025:10:05:00 +0000] "POST /api/data HTTP/2.0" 201 4096 "https://myapp.example.com/dashboard" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`,
		},
		{
			name: "nginx upstream timeout",
			line: `10.0.0.1 - - [25/Jun/2025:10:10:00 +0000] "GET /slow-endpoint HTTP/1.1" 504 167 "-" "python-requests/2.31.0"`,
		},
	}

	for _, tt := range lines {
		t.Run(tt.name, func(t *testing.T) {
			entry := p.Parse(tt.line)
			if entry == nil {
				t.Fatalf("Parse() returned nil for real-world line: %s", tt.line)
			}
			if entry.Timestamp == nil {
				t.Error("Timestamp is nil")
			}
			if entry.Level == nil {
				t.Error("Level is nil")
			}
			if entry.Source == nil {
				t.Error("Source is nil")
			}
			if entry.Message == "" {
				t.Error("Message is empty")
			}
		})
	}
}

func TestApacheCombinedParser_UserAgentAlwaysPresent(t *testing.T) {
	p := NewApacheCombinedParser()
	// user_agent should always be in extra_fields for combined, even when empty string
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 0 "-" ""`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if _, ok := entry.ExtraFields["user_agent"]; !ok {
		t.Error("user_agent should be present in extra_fields even when empty")
	}
}

func TestApacheCommonParser_NoUserAgentOrReferer(t *testing.T) {
	p := NewApacheCommonParser()
	line := `10.0.0.1 - - [01/Jan/2025:00:00:00 +0000] "GET / HTTP/1.1" 200 0`
	entry := p.Parse(line)
	if entry == nil {
		t.Fatal("Parse() returned nil")
	}
	if _, ok := entry.ExtraFields["user_agent"]; ok {
		t.Error("common format should never have user_agent")
	}
	if _, ok := entry.ExtraFields["referer"]; ok {
		t.Error("common format should never have referer")
	}
}
