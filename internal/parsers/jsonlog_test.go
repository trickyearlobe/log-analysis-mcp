package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func strPtr(s string) *string {
	return &s
}

func levelPtr(l types.LogLevel) *types.LogLevel {
	return &l
}

func TestJSONParserName(t *testing.T) {
	p := NewJSONParser()
	if got := p.Name(); got != "json" {
		t.Errorf("Name() = %q, want %q", got, "json")
	}
}

func TestJSONParserParse(t *testing.T) {
	p := NewJSONParser()

	tests := []struct {
		name        string
		line        string
		wantNil     bool
		wantTS      *string
		wantLevel   *types.LogLevel
		wantSource  *string
		wantMessage string
		wantRaw     string
		wantExtra   map[string]interface{}
	}{
		{
			name:        "basic JSON with all standard fields",
			line:        `{"timestamp":"2025-01-15T14:31:02Z","level":"INFO","source":"myapp","message":"Server started"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T14:31:02Z"),
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantSource:  strPtr("myapp"),
			wantMessage: "Server started",
			wantRaw:     `{"timestamp":"2025-01-15T14:31:02Z","level":"INFO","source":"myapp","message":"Server started"}`,
			wantExtra:   nil,
		},
		{
			name:        "non-JSON line returns nil",
			line:        "this is not JSON at all",
			wantNil:     true,
			wantMessage: "",
		},
		{
			name:        "empty string returns nil",
			line:        "",
			wantNil:     true,
			wantMessage: "",
		},
		{
			name:    "JSON array returns nil",
			line:    `[1, 2, 3]`,
			wantNil: true,
		},
		{
			name:        "empty JSON object",
			line:        `{}`,
			wantNil:     false,
			wantTS:      nil,
			wantLevel:   nil,
			wantSource:  nil,
			wantMessage: "",
			wantRaw:     `{}`,
			wantExtra:   nil,
		},

		// Field name variants: timestamp
		{
			name:        "timestamp variant: ts",
			line:        `{"ts":"2025-01-15T10:00:00Z","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantMessage: "hello",
		},
		{
			name:        "timestamp variant: time",
			line:        `{"time":"2025-01-15T10:00:00Z","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantMessage: "hello",
		},
		{
			name:        "timestamp variant: @timestamp",
			line:        `{"@timestamp":"2025-01-15T10:00:00Z","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantMessage: "hello",
		},
		{
			name:        "timestamp variant: date",
			line:        `{"date":"2025-01-15","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15"),
			wantMessage: "hello",
		},
		{
			name:        "timestamp variant: datetime",
			line:        `{"datetime":"2025-01-15T10:00:00Z","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantMessage: "hello",
		},
		{
			name:        "timestamp variant: t",
			line:        `{"t":"2025-01-15T10:00:00Z","msg":"hello"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantMessage: "hello",
		},

		// Field name variants: level
		{
			name:        "level variant: severity",
			line:        `{"severity":"ERROR","msg":"boom"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelError),
			wantMessage: "boom",
		},
		{
			name:        "level variant: log_level",
			line:        `{"log_level":"DEBUG","msg":"trace"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelDebug),
			wantMessage: "trace",
		},
		{
			name:        "level variant: loglevel",
			line:        `{"loglevel":"WARN","msg":"caution"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelWarn),
			wantMessage: "caution",
		},
		{
			name:        "level variant: lvl",
			line:        `{"lvl":"info","msg":"ok"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantMessage: "ok",
		},
		{
			name:        "level variant: priority",
			line:        `{"priority":"fatal","msg":"dead"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelFatal),
			wantMessage: "dead",
		},

		// Field name variants: message
		{
			name:        "message variant: msg",
			line:        `{"msg":"hello from msg"}`,
			wantNil:     false,
			wantMessage: "hello from msg",
		},
		{
			name:        "message variant: log",
			line:        `{"log":"hello from log"}`,
			wantNil:     false,
			wantMessage: "hello from log",
		},
		{
			name:        "message variant: text",
			line:        `{"text":"hello from text"}`,
			wantNil:     false,
			wantMessage: "hello from text",
		},
		{
			name:        "message variant: body",
			line:        `{"body":"hello from body"}`,
			wantNil:     false,
			wantMessage: "hello from body",
		},

		// Field name variants: source
		{
			name:        "source variant: logger",
			line:        `{"logger":"com.example.App","msg":"hi"}`,
			wantNil:     false,
			wantSource:  strPtr("com.example.App"),
			wantMessage: "hi",
		},
		{
			name:        "source variant: component",
			line:        `{"component":"auth","msg":"hi"}`,
			wantNil:     false,
			wantSource:  strPtr("auth"),
			wantMessage: "hi",
		},
		{
			name:        "source variant: module",
			line:        `{"module":"database","msg":"hi"}`,
			wantNil:     false,
			wantSource:  strPtr("database"),
			wantMessage: "hi",
		},
		{
			name:        "source variant: name",
			line:        `{"name":"worker","msg":"hi"}`,
			wantNil:     false,
			wantSource:  strPtr("worker"),
			wantMessage: "hi",
		},
		{
			name:        "source variant: service",
			line:        `{"service":"api-gateway","msg":"hi"}`,
			wantNil:     false,
			wantSource:  strPtr("api-gateway"),
			wantMessage: "hi",
		},

		// Level value normalization
		{
			name:        "level value: trace lowercase",
			line:        `{"level":"trace","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelTrace),
			wantMessage: "x",
		},
		{
			name:        "level value: TRACE uppercase",
			line:        `{"level":"TRACE","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelTrace),
			wantMessage: "x",
		},
		{
			name:        "level value: debug lowercase",
			line:        `{"level":"debug","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelDebug),
			wantMessage: "x",
		},
		{
			name:        "level value: information",
			line:        `{"level":"information","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantMessage: "x",
		},
		{
			name:        "level value: warning",
			line:        `{"level":"warning","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelWarn),
			wantMessage: "x",
		},
		{
			name:        "level value: WARNING uppercase",
			line:        `{"level":"WARNING","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelWarn),
			wantMessage: "x",
		},
		{
			name:        "level value: err lowercase",
			line:        `{"level":"err","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelError),
			wantMessage: "x",
		},
		{
			name:        "level value: ERR uppercase",
			line:        `{"level":"ERR","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelError),
			wantMessage: "x",
		},
		{
			name:        "level value: critical maps to FATAL",
			line:        `{"level":"critical","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelFatal),
			wantMessage: "x",
		},
		{
			name:        "level value: CRITICAL maps to FATAL",
			line:        `{"level":"CRITICAL","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelFatal),
			wantMessage: "x",
		},

		// Numeric level values
		{
			name:        "numeric level 10 = TRACE",
			line:        `{"level":10,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelTrace),
			wantMessage: "x",
		},
		{
			name:        "numeric level 20 = DEBUG",
			line:        `{"level":20,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelDebug),
			wantMessage: "x",
		},
		{
			name:        "numeric level 30 = INFO",
			line:        `{"level":30,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantMessage: "x",
		},
		{
			name:        "numeric level 40 = WARN",
			line:        `{"level":40,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelWarn),
			wantMessage: "x",
		},
		{
			name:        "numeric level 50 = ERROR",
			line:        `{"level":50,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelError),
			wantMessage: "x",
		},
		{
			name:        "numeric level 60 = FATAL",
			line:        `{"level":60,"msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelFatal),
			wantMessage: "x",
		},
		{
			name:        "numeric level as string 50 = ERROR",
			line:        `{"level":"50","msg":"x"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelError),
			wantMessage: "x",
		},

		// Numeric timestamp values
		{
			name:        "numeric timestamp epoch seconds",
			line:        `{"timestamp":1705312262,"msg":"epoch"}`,
			wantNil:     false,
			wantTS:      strPtr("1705312262"),
			wantMessage: "epoch",
		},
		{
			name:        "numeric timestamp with decimals",
			line:        `{"timestamp":1705312262.123,"msg":"epoch millis"}`,
			wantNil:     false,
			wantTS:      strPtr("1.705312262123e+09"),
			wantMessage: "epoch millis",
		},

		// Extra fields
		{
			name:        "extra fields preserved",
			line:        `{"message":"hello","level":"INFO","request_id":"abc-123","duration_ms":42}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantMessage: "hello",
			wantExtra: map[string]interface{}{
				"request_id":  "abc-123",
				"duration_ms": float64(42),
			},
		},
		{
			name:        "nested JSON objects in extra_fields",
			line:        `{"message":"request","http":{"method":"GET","path":"/api","status":200},"level":"INFO"}`,
			wantNil:     false,
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantMessage: "request",
			wantExtra: map[string]interface{}{
				"http": map[string]interface{}{
					"method": "GET",
					"path":   "/api",
					"status": float64(200),
				},
			},
		},
		{
			name:    "unrecognized level value goes to extra_fields",
			line:    `{"level":"CUSTOM_LEVEL","msg":"x"}`,
			wantNil: false,
			// Level pointer should be nil since CUSTOM_LEVEL is not recognized
			wantLevel:   nil,
			wantMessage: "x",
			wantExtra: map[string]interface{}{
				"level": "CUSTOM_LEVEL",
			},
		},

		// Mixed framework styles
		{
			name:        "Bunyan-style log",
			line:        `{"name":"myservice","hostname":"server1","pid":1234,"level":30,"msg":"listening","time":"2025-01-15T10:00:00Z","v":0}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantLevel:   levelPtr(types.LogLevelInfo),
			wantSource:  strPtr("myservice"),
			wantMessage: "listening",
		},
		{
			name:        "Logstash-style log",
			line:        `{"@timestamp":"2025-01-15T10:00:00Z","severity":"ERROR","message":"connection refused","service":"payment"}`,
			wantNil:     false,
			wantTS:      strPtr("2025-01-15T10:00:00Z"),
			wantLevel:   levelPtr(types.LogLevelError),
			wantSource:  strPtr("payment"),
			wantMessage: "connection refused",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Parse(tc.line)

			if tc.wantNil {
				if got != nil {
					t.Fatalf("Parse() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("Parse() = nil, want non-nil")
			}

			// Check Raw
			if tc.wantRaw != "" && got.Raw != tc.wantRaw {
				t.Errorf("Raw = %q, want %q", got.Raw, tc.wantRaw)
			}
			// Raw should always be set to the original line
			if got.Raw != tc.line {
				t.Errorf("Raw = %q, want original line %q", got.Raw, tc.line)
			}

			// Check Timestamp
			if tc.wantTS == nil {
				if got.Timestamp != nil {
					t.Errorf("Timestamp = %q, want nil", *got.Timestamp)
				}
			} else {
				if got.Timestamp == nil {
					t.Errorf("Timestamp = nil, want %q", *tc.wantTS)
				} else if *got.Timestamp != *tc.wantTS {
					t.Errorf("Timestamp = %q, want %q", *got.Timestamp, *tc.wantTS)
				}
			}

			// Check Level
			if tc.wantLevel == nil {
				if got.Level != nil {
					t.Errorf("Level = %q, want nil", *got.Level)
				}
			} else {
				if got.Level == nil {
					t.Errorf("Level = nil, want %q", *tc.wantLevel)
				} else if *got.Level != *tc.wantLevel {
					t.Errorf("Level = %q, want %q", *got.Level, *tc.wantLevel)
				}
			}

			// Check Source
			if tc.wantSource == nil {
				if got.Source != nil {
					t.Errorf("Source = %q, want nil", *got.Source)
				}
			} else {
				if got.Source == nil {
					t.Errorf("Source = nil, want %q", *tc.wantSource)
				} else if *got.Source != *tc.wantSource {
					t.Errorf("Source = %q, want %q", *got.Source, *tc.wantSource)
				}
			}

			// Check Message
			if got.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tc.wantMessage)
			}

			// Check ExtraFields
			if tc.wantExtra != nil {
				if got.ExtraFields == nil {
					t.Fatalf("ExtraFields = nil, want %v", tc.wantExtra)
				}
				for k, wantVal := range tc.wantExtra {
					gotVal, ok := got.ExtraFields[k]
					if !ok {
						t.Errorf("ExtraFields missing key %q", k)
						continue
					}
					if !extraFieldEqual(wantVal, gotVal) {
						t.Errorf("ExtraFields[%q] = %v (%T), want %v (%T)", k, gotVal, gotVal, wantVal, wantVal)
					}
				}
			}
		})
	}
}

func TestJSONParserDetect(t *testing.T) {
	p := NewJSONParser()

	tests := []struct {
		name      string
		lines     []string
		wantScore float64
	}{
		{
			name:      "empty input",
			lines:     []string{},
			wantScore: 0.0,
		},
		{
			name: "all JSON lines",
			lines: []string{
				`{"msg":"one","level":"INFO"}`,
				`{"msg":"two","level":"DEBUG"}`,
				`{"msg":"three","level":"ERROR"}`,
			},
			wantScore: 1.0,
		},
		{
			name: "no JSON lines",
			lines: []string{
				"Jan 15 10:00:00 host app: plain syslog",
				"this is not JSON",
				"neither is this",
			},
			wantScore: 0.0,
		},
		{
			name: "mixed JSON and non-JSON",
			lines: []string{
				`{"msg":"valid JSON"}`,
				"not JSON",
				`{"msg":"also valid"}`,
				"also not JSON",
			},
			wantScore: 0.5,
		},
		{
			name: "two thirds JSON",
			lines: []string{
				`{"msg":"one"}`,
				`{"msg":"two"}`,
				"not JSON",
			},
			wantScore: 2.0 / 3.0,
		},
		{
			name: "single valid JSON line",
			lines: []string{
				`{"level":"ERROR","message":"fail"}`,
			},
			wantScore: 1.0,
		},
		{
			name: "single non-JSON line",
			lines: []string{
				"just text",
			},
			wantScore: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Detect(tc.lines)
			// Use a small epsilon for float comparison
			if diff := got - tc.wantScore; diff > 0.001 || diff < -0.001 {
				t.Errorf("Detect() = %f, want %f", got, tc.wantScore)
			}
		})
	}
}

func TestJSONParserImplementsInterface(t *testing.T) {
	// Compile-time check that JSONParser satisfies Parser interface
	var _ Parser = (*JSONParser)(nil)
}

// extraFieldEqual does a recursive comparison of JSON-decoded values.
func extraFieldEqual(want, got interface{}) bool {
	switch w := want.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		if !ok || len(w) != len(g) {
			return false
		}
		for k, wv := range w {
			gv, exists := g[k]
			if !exists || !extraFieldEqual(wv, gv) {
				return false
			}
		}
		return true
	case float64:
		g, ok := got.(float64)
		return ok && w == g
	case string:
		g, ok := got.(string)
		return ok && w == g
	case bool:
		g, ok := got.(bool)
		return ok && w == g
	case nil:
		return got == nil
	default:
		return false
	}
}
