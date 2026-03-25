package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestAutoDetect(t *testing.T) {
	tests := []struct {
		name           string
		lines          []string
		wantFormat     types.LogFormat
		wantAbove50    bool
		wantSampleSize int
	}{
		{
			name:           "empty lines slice",
			lines:          []string{},
			wantFormat:     types.LogFormatUnknown,
			wantAbove50:    false,
			wantSampleSize: 0,
		},
		{
			name:           "all empty strings",
			lines:          []string{"", "", ""},
			wantFormat:     types.LogFormatUnknown,
			wantAbove50:    false,
			wantSampleSize: 0,
		},
		{
			name: "pure JSON logs",
			lines: []string{
				`{"timestamp":"2025-01-15T10:30:00Z","level":"INFO","message":"server started"}`,
				`{"timestamp":"2025-01-15T10:30:01Z","level":"DEBUG","message":"loading config"}`,
				`{"timestamp":"2025-01-15T10:30:02Z","level":"ERROR","message":"connection failed"}`,
			},
			wantFormat:     types.LogFormatJSON,
			wantAbove50:    true,
			wantSampleSize: 3,
		},
		{
			name: "pure syslog RFC 3164",
			lines: []string{
				`<134>Jan 15 10:30:00 myhost myapp[1234]: server started`,
				`<134>Jan 15 10:30:01 myhost myapp[1234]: loading config`,
				`<134>Jan 15 10:30:02 myhost myapp[1234]: connection failed`,
			},
			wantFormat:     types.LogFormatSyslogRFC3164,
			wantAbove50:    true,
			wantSampleSize: 3,
		},
		{
			name: "pure syslog RFC 5424",
			lines: []string{
				`<165>1 2025-01-15T10:30:00Z myhost myapp 1234 ID47 [exampleSDID@32473 iut="3"] server started`,
				`<165>1 2025-01-15T10:30:01Z myhost myapp 1234 ID48 [exampleSDID@32473 iut="3"] loading config`,
				`<165>1 2025-01-15T10:30:02Z myhost myapp 1234 ID49 [exampleSDID@32473 iut="3"] connection failed`,
			},
			wantFormat:     types.LogFormatSyslogRFC5424,
			wantAbove50:    true,
			wantSampleSize: 3,
		},
		{
			name: "pure apache combined",
			lines: []string{
				`192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"`,
				`10.0.0.1 - - [10/Jan/2025:13:55:37 -0700] "POST /api/data HTTP/1.1" 201 512 "-" "curl/7.68"`,
				`172.16.0.1 - admin [10/Jan/2025:13:55:38 -0700] "DELETE /api/item/5 HTTP/1.1" 404 128 "https://example.com" "Mozilla/5.0"`,
			},
			wantFormat:     types.LogFormatApacheCombined,
			wantAbove50:    true,
			wantSampleSize: 3,
		},
		{
			// Common-format lines also match the Combined regex (which has
			// optional referer/user-agent groups). Since Combined has higher
			// priority in the tiebreaker order, it wins. This is expected
			// behaviour — Common is only selected when the Combined parser
			// scores strictly lower.
			name: "apache common lines detected as combined due to priority",
			lines: []string{
				`192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api/users HTTP/1.1" 200 2326`,
				`10.0.0.1 - - [10/Jan/2025:13:55:37 -0700] "POST /api/data HTTP/1.1" 201 512`,
				`172.16.0.1 - admin [10/Jan/2025:13:55:38 -0700] "DELETE /api/item/5 HTTP/1.1" 404 128`,
			},
			wantFormat:     types.LogFormatApacheCombined,
			wantAbove50:    true,
			wantSampleSize: 3,
		},
		{
			name: "no format matches above 50 pct",
			lines: []string{
				"this is just plain text",
				"another unstructured line",
				"nothing parseable here",
				"more random text",
			},
			wantFormat:     types.LogFormatUnknown,
			wantAbove50:    false,
			wantSampleSize: 4,
		},
		{
			name: "majority JSON with some garbage",
			lines: []string{
				`{"level":"INFO","msg":"hello"}`,
				"not json at all",
				`{"level":"DEBUG","msg":"world"}`,
				`{"level":"WARN","msg":"careful"}`,
			},
			wantFormat:     types.LogFormatJSON,
			wantAbove50:    true,
			wantSampleSize: 4,
		},
		{
			name: "below 50 pct threshold",
			lines: []string{
				`{"level":"INFO","msg":"hello"}`,
				"garbage line 1",
				"garbage line 2",
				"garbage line 3",
				"garbage line 4",
			},
			wantFormat:     types.LogFormatUnknown,
			wantAbove50:    false,
			wantSampleSize: 5,
		},
		{
			name: "exactly 50 pct threshold",
			lines: []string{
				`{"level":"INFO","msg":"hello"}`,
				"not json",
				`{"level":"DEBUG","msg":"world"}`,
				"also not json",
			},
			wantFormat:     types.LogFormatJSON,
			wantAbove50:    true,
			wantSampleSize: 4,
		},
		{
			name: "empty lines filtered out",
			lines: []string{
				"",
				`{"level":"INFO","msg":"hello"}`,
				"",
				`{"level":"DEBUG","msg":"world"}`,
				"",
			},
			wantFormat:     types.LogFormatJSON,
			wantAbove50:    true,
			wantSampleSize: 2,
		},
		{
			name: "single line JSON",
			lines: []string{
				`{"level":"INFO","msg":"single"}`,
			},
			wantFormat:     types.LogFormatJSON,
			wantAbove50:    true,
			wantSampleSize: 1,
		},
		{
			name: "single line syslog",
			lines: []string{
				`<134>Jan 15 10:30:00 myhost myapp[1234]: single message`,
			},
			wantFormat:     types.LogFormatSyslogRFC3164,
			wantAbove50:    true,
			wantSampleSize: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AutoDetect(tt.lines)

			if got.Format != tt.wantFormat {
				t.Errorf("Format = %q, want %q", got.Format, tt.wantFormat)
			}
			if got.SampleSize != tt.wantSampleSize {
				t.Errorf("SampleSize = %d, want %d", got.SampleSize, tt.wantSampleSize)
			}
			if tt.wantAbove50 && got.Confidence < 0.5 {
				t.Errorf("Confidence = %f, want >= 0.5", got.Confidence)
			}
			if !tt.wantAbove50 && got.Confidence >= 0.5 && got.Format != types.LogFormatUnknown {
				t.Errorf("Confidence = %f, expected below 0.5 or format unknown", got.Confidence)
			}
		})
	}
}

func TestAutoDetectPriorityTiebreaker(t *testing.T) {
	// JSON should win over syslog when both parse a line successfully.
	// JSON objects that also happen to match syslog patterns are unlikely,
	// but the priority system ensures deterministic results when scores tie.
	t.Run("JSON wins tie over other formats", func(t *testing.T) {
		lines := []string{
			`{"level":"INFO","msg":"hello"}`,
			`{"level":"DEBUG","msg":"world"}`,
		}
		got := AutoDetect(lines)
		if got.Format != types.LogFormatJSON {
			t.Errorf("Format = %q, want %q (JSON should win ties)", got.Format, types.LogFormatJSON)
		}
	})

	t.Run("RFC 5424 wins over RFC 3164 at equal score", func(t *testing.T) {
		// These lines are valid RFC 5424 — RFC 3164 should not match them
		// since the format is different, but if both somehow scored equally,
		// 5424 should win due to priority ordering.
		lines := []string{
			`<165>1 2025-01-15T10:30:00Z myhost myapp 1234 ID47 [exampleSDID@32473 iut="3"] test`,
			`<165>1 2025-01-15T10:30:01Z myhost myapp 1234 ID48 [exampleSDID@32473 iut="3"] test`,
		}
		got := AutoDetect(lines)
		if got.Format != types.LogFormatSyslogRFC5424 {
			t.Errorf("Format = %q, want %q", got.Format, types.LogFormatSyslogRFC5424)
		}
	})

	t.Run("combined wins over common at equal score", func(t *testing.T) {
		// Combined format lines also match the combined parser but not common.
		// Combined should win because it has higher priority.
		lines := []string{
			`192.168.1.1 - frank [10/Jan/2025:13:55:36 -0700] "GET /api HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"`,
			`10.0.0.1 - - [10/Jan/2025:13:55:37 -0700] "POST /data HTTP/1.1" 201 512 "-" "curl/7.68"`,
		}
		got := AutoDetect(lines)
		if got.Format != types.LogFormatApacheCombined {
			t.Errorf("Format = %q, want %q", got.Format, types.LogFormatApacheCombined)
		}
	})
}

func TestAutoDetectConfidenceValues(t *testing.T) {
	t.Run("all lines match gives 1.0", func(t *testing.T) {
		lines := []string{
			`{"level":"INFO","msg":"a"}`,
			`{"level":"DEBUG","msg":"b"}`,
			`{"level":"WARN","msg":"c"}`,
		}
		got := AutoDetect(lines)
		if got.Confidence != 1.0 {
			t.Errorf("Confidence = %f, want 1.0", got.Confidence)
		}
		if got.SuccessfulParses != 3 {
			t.Errorf("SuccessfulParses = %d, want 3", got.SuccessfulParses)
		}
	})

	t.Run("partial match gives correct ratio", func(t *testing.T) {
		lines := []string{
			`{"level":"INFO","msg":"a"}`,
			"garbage",
			`{"level":"WARN","msg":"c"}`,
			"more garbage",
		}
		got := AutoDetect(lines)
		if got.Confidence != 0.5 {
			t.Errorf("Confidence = %f, want 0.5", got.Confidence)
		}
		if got.SuccessfulParses != 2 {
			t.Errorf("SuccessfulParses = %d, want 2", got.SuccessfulParses)
		}
	})

	t.Run("no matches gives 0", func(t *testing.T) {
		lines := []string{
			"plain text",
			"more text",
		}
		got := AutoDetect(lines)
		if got.Confidence != 0 {
			t.Errorf("Confidence = %f, want 0", got.Confidence)
		}
	})
}

func TestAutoDetectSuccessfulParsesCount(t *testing.T) {
	lines := []string{
		`<134>Jan 15 10:30:00 myhost myapp[1234]: line one`,
		"not syslog",
		`<134>Jan 15 10:30:01 myhost myapp[1234]: line two`,
		`<134>Jan 15 10:30:02 myhost myapp[1234]: line three`,
	}
	got := AutoDetect(lines)
	if got.SuccessfulParses != 3 {
		t.Errorf("SuccessfulParses = %d, want 3", got.SuccessfulParses)
	}
	if got.SampleSize != 4 {
		t.Errorf("SampleSize = %d, want 4", got.SampleSize)
	}
}

func TestAutoDetectWithHint(t *testing.T) {
	sampleLines := []string{
		"some ambiguous line",
		"another ambiguous line",
	}

	t.Run("empty hint delegates to AutoDetect", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "")
		if result.Format != types.LogFormatUnknown {
			t.Errorf("Format = %q, want unknown for ambiguous lines", result.Format)
		}
		if parser != nil {
			t.Error("parser should be nil for unknown format")
		}
	})

	t.Run("auto hint delegates to AutoDetect", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "auto")
		if result.Format != types.LogFormatUnknown {
			t.Errorf("Format = %q, want unknown for ambiguous lines", result.Format)
		}
		if parser != nil {
			t.Error("parser should be nil for unknown format")
		}
	})

	t.Run("json hint returns JSON parser", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "json")
		if result.Format != types.LogFormatJSON {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatJSON)
		}
		if result.Confidence != 1.0 {
			t.Errorf("Confidence = %f, want 1.0 for explicit hint", result.Confidence)
		}
		if parser == nil {
			t.Fatal("parser should not be nil for known hint")
		}
		if parser.Name() != "json" {
			t.Errorf("parser.Name() = %q, want %q", parser.Name(), "json")
		}
	})

	t.Run("syslog-rfc3164 hint returns correct parser", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "syslog-rfc3164")
		if result.Format != types.LogFormatSyslogRFC3164 {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatSyslogRFC3164)
		}
		if parser == nil || parser.Name() != "syslog-rfc3164" {
			t.Error("expected syslog-rfc3164 parser")
		}
	})

	t.Run("syslog-rfc5424 hint returns correct parser", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "syslog-rfc5424")
		if result.Format != types.LogFormatSyslogRFC5424 {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatSyslogRFC5424)
		}
		if parser == nil || parser.Name() != "syslog-rfc5424" {
			t.Error("expected syslog-rfc5424 parser")
		}
	})

	t.Run("apache-combined hint returns correct parser", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "apache-combined")
		if result.Format != types.LogFormatApacheCombined {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatApacheCombined)
		}
		if parser == nil || parser.Name() != "apache-combined" {
			t.Error("expected apache-combined parser")
		}
	})

	t.Run("apache-common hint returns correct parser", func(t *testing.T) {
		result, parser := AutoDetectWithHint(sampleLines, "apache-common")
		if result.Format != types.LogFormatApacheCommon {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatApacheCommon)
		}
		if parser == nil || parser.Name() != "apache-common" {
			t.Error("expected apache-common parser")
		}
	})

	t.Run("unknown hint falls through to AutoDetect", func(t *testing.T) {
		jsonLines := []string{
			`{"level":"INFO","msg":"a"}`,
			`{"level":"DEBUG","msg":"b"}`,
		}
		result, parser := AutoDetectWithHint(jsonLines, "nonexistent-format")
		if result.Format != types.LogFormatJSON {
			t.Errorf("Format = %q, want %q (should fall through to autodetect)", result.Format, types.LogFormatJSON)
		}
		if parser == nil {
			t.Error("parser should not be nil when autodetect finds JSON")
		}
	})

	t.Run("hint with valid autodetect returns parser", func(t *testing.T) {
		jsonLines := []string{
			`{"level":"INFO","msg":"a"}`,
			`{"level":"DEBUG","msg":"b"}`,
		}
		result, parser := AutoDetectWithHint(jsonLines, "auto")
		if result.Format != types.LogFormatJSON {
			t.Errorf("Format = %q, want %q", result.Format, types.LogFormatJSON)
		}
		if parser == nil {
			t.Error("parser should not be nil")
		}
	})
}

func TestAutoDetectWithHintSampleSize(t *testing.T) {
	lines := []string{"a", "b", "c"}
	result, _ := AutoDetectWithHint(lines, "json")
	if result.SampleSize != 3 {
		t.Errorf("SampleSize = %d, want 3", result.SampleSize)
	}
}
