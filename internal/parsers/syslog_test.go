package parsers

import (
	"fmt"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// derefStr dereferences a *string for comparison, returns "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefLevel dereferences a *LogLevel for comparison, returns "" if nil.
func derefLevel(l *types.LogLevel) types.LogLevel {
	if l == nil {
		return ""
	}
	return *l
}

// formatVal converts a value to string for test comparison.
func formatVal(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func TestSyslogRFC3164Parser_Name(t *testing.T) {
	p := NewSyslogRFC3164Parser()
	if got := p.Name(); got != "syslog-rfc3164" {
		t.Errorf("Name() = %q, want %q", got, "syslog-rfc3164")
	}
}

func TestSyslogRFC5424Parser_Name(t *testing.T) {
	p := NewSyslogRFC5424Parser()
	if got := p.Name(); got != "syslog-rfc5424" {
		t.Errorf("Name() = %q, want %q", got, "syslog-rfc5424")
	}
}

func TestSyslogRFC3164Parser_Parse(t *testing.T) {
	p := NewSyslogRFC3164Parser()

	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantTS    string
		wantSrc   string
		wantMsg   string
		wantLevel types.LogLevel
		wantExtra map[string]interface{}
	}{
		{
			name:      "with priority and PID",
			input:     "<34>Jan  5 14:32:01 myhost sshd[1234]: Accepted publickey for user",
			wantTS:    "Jan  5 14:32:01",
			wantSrc:   "sshd",
			wantMsg:   "Accepted publickey for user",
			wantLevel: types.LogLevelCritical,
			wantExtra: map[string]interface{}{
				"hostname": "myhost",
				"proc_id":  "1234",
				"priority": 34,
				"facility": "auth",
				"severity": "crit",
			},
		},
		{
			name:      "with priority no PID",
			input:     "<13>Oct 22 09:15:00 server01 kernel: segfault at 0000",
			wantTS:    "Oct 22 09:15:00",
			wantSrc:   "kernel",
			wantMsg:   "segfault at 0000",
			wantLevel: types.LogLevelInfo,
			wantExtra: map[string]interface{}{
				"hostname": "server01",
				"priority": 13,
				"facility": "user",
				"severity": "notice",
			},
		},
		{
			name:      "without priority with PID",
			input:     "Mar 12 06:30:45 webhost nginx[9876]: upstream timed out",
			wantTS:    "Mar 12 06:30:45",
			wantSrc:   "nginx",
			wantMsg:   "upstream timed out",
			wantLevel: "",
			wantExtra: map[string]interface{}{
				"hostname": "webhost",
				"proc_id":  "9876",
			},
		},
		{
			name:      "without priority without PID",
			input:     "Dec  1 23:59:59 loghost cron: job completed",
			wantTS:    "Dec  1 23:59:59",
			wantSrc:   "cron",
			wantMsg:   "job completed",
			wantLevel: "",
			wantExtra: map[string]interface{}{
				"hostname": "loghost",
			},
		},
		{
			name:    "non-matching line",
			input:   "this is not a syslog line",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("Parse(%q) = %+v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want non-nil", tt.input)
			}
			if derefStr(got.Timestamp) != tt.wantTS {
				t.Errorf("Timestamp = %q, want %q", derefStr(got.Timestamp), tt.wantTS)
			}
			if derefStr(got.Source) != tt.wantSrc {
				t.Errorf("Source = %q, want %q", derefStr(got.Source), tt.wantSrc)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if got.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", got.Raw, tt.input)
			}
			if derefLevel(got.Level) != tt.wantLevel {
				t.Errorf("Level = %q, want %q", derefLevel(got.Level), tt.wantLevel)
			}
			for k, wantV := range tt.wantExtra {
				gotV, ok := got.ExtraFields[k]
				if !ok {
					t.Errorf("ExtraFields missing key %q", k)
					continue
				}
				if gotStr, wantStr := formatVal(gotV), formatVal(wantV); gotStr != wantStr {
					t.Errorf("ExtraFields[%q] = %v, want %v", k, gotStr, wantStr)
				}
			}
		})
	}
}

func TestSyslogRFC5424Parser_Parse(t *testing.T) {
	p := NewSyslogRFC5424Parser()

	tests := []struct {
		name        string
		input       string
		wantNil     bool
		wantTS      string
		wantSrc     string
		wantMsg     string
		wantLevel   types.LogLevel
		wantExtra   map[string]interface{}
		absentExtra []string
	}{
		{
			name:      "basic RFC 5424",
			input:     "<165>1 2023-08-11T15:30:00.000Z server1 myapp 1234 ID47 - Application started",
			wantTS:    "2023-08-11T15:30:00.000Z",
			wantSrc:   "myapp",
			wantMsg:   "Application started",
			wantLevel: types.LogLevelInfo,
			wantExtra: map[string]interface{}{
				"hostname": "server1",
				"version":  "1",
				"proc_id":  "1234",
				"msg_id":   "ID47",
				"priority": 165,
				"facility": "local4",
				"severity": "notice",
			},
		},
		{
			name:      "with structured data",
			input:     "<34>1 2023-08-11T15:30:00Z host app 999 - [exampleSDID@32473 iut=\"3\" eventSource=\"Application\"] Login failed",
			wantTS:    "2023-08-11T15:30:00Z",
			wantSrc:   "app",
			wantMsg:   "Login failed",
			wantLevel: types.LogLevelCritical,
			wantExtra: map[string]interface{}{
				"hostname":        "host",
				"version":         "1",
				"proc_id":         "999",
				"priority":        34,
				"facility":        "auth",
				"severity":        "crit",
				"structured_data": "exampleSDID@32473 iut=\"3\" eventSource=\"Application\"",
			},
		},
		{
			name:      "with dash fields (nil values)",
			input:     "<14>1 2023-01-01T00:00:00Z myhost myapp - - - System ready",
			wantTS:    "2023-01-01T00:00:00Z",
			wantSrc:   "myapp",
			wantMsg:   "System ready",
			wantLevel: types.LogLevelInfo,
			wantExtra: map[string]interface{}{
				"hostname": "myhost",
				"version":  "1",
				"priority": 14,
				"facility": "user",
				"severity": "info",
			},
			absentExtra: []string{"proc_id", "msg_id", "structured_data"},
		},
		{
			name:    "non-matching line",
			input:   "just some random text",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("Parse(%q) = %+v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want non-nil", tt.input)
			}
			if derefStr(got.Timestamp) != tt.wantTS {
				t.Errorf("Timestamp = %q, want %q", derefStr(got.Timestamp), tt.wantTS)
			}
			if derefStr(got.Source) != tt.wantSrc {
				t.Errorf("Source = %q, want %q", derefStr(got.Source), tt.wantSrc)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if got.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", got.Raw, tt.input)
			}
			if derefLevel(got.Level) != tt.wantLevel {
				t.Errorf("Level = %q, want %q", derefLevel(got.Level), tt.wantLevel)
			}
			for k, wantV := range tt.wantExtra {
				gotV, ok := got.ExtraFields[k]
				if !ok {
					t.Errorf("ExtraFields missing key %q", k)
					continue
				}
				if gotStr, wantStr := formatVal(gotV), formatVal(wantV); gotStr != wantStr {
					t.Errorf("ExtraFields[%q] = %v, want %v", k, gotStr, wantStr)
				}
			}
			for _, key := range tt.absentExtra {
				if _, ok := got.ExtraFields[key]; ok {
					t.Errorf("ExtraFields should not contain %q for dash value", key)
				}
			}
		})
	}
}

func TestPriorityDerivation(t *testing.T) {
	p := NewSyslogRFC3164Parser()

	tests := []struct {
		name         string
		input        string
		wantPriority int
		wantFacility string
		wantSeverity string
	}{
		{
			name:         "priority 0 (kern.emerg)",
			input:        "<0>Jan  1 00:00:00 host app: msg",
			wantPriority: 0,
			wantFacility: "kern",
			wantSeverity: "emerg",
		},
		{
			name:         "priority 34 (auth.crit)",
			input:        "<34>Jan  1 00:00:00 host app: msg",
			wantPriority: 34,
			wantFacility: "auth",
			wantSeverity: "crit",
		},
		{
			name:         "priority 38 (auth.info)",
			input:        "<38>Jan  1 00:00:00 host app: msg",
			wantPriority: 38,
			wantFacility: "auth",
			wantSeverity: "info",
		},
		{
			name:         "priority 165 (local4.notice)",
			input:        "<165>Jan  1 00:00:00 host app: msg",
			wantPriority: 165,
			wantFacility: "local4",
			wantSeverity: "notice",
		},
		{
			name:         "priority 191 (local7.debug)",
			input:        "<191>Jan  1 00:00:00 host app: msg",
			wantPriority: 191,
			wantFacility: "local7",
			wantSeverity: "debug",
		},
		{
			name:         "priority 11 (user.err)",
			input:        "<11>Jan  1 00:00:00 host app: msg",
			wantPriority: 11,
			wantFacility: "user",
			wantSeverity: "err",
		},
		{
			name:         "priority 12 (user.warning)",
			input:        "<12>Jan  1 00:00:00 host app: msg",
			wantPriority: 12,
			wantFacility: "user",
			wantSeverity: "warning",
		},
		{
			name:         "priority 9 (user.alert)",
			input:        "<9>Jan  1 00:00:00 host app: msg",
			wantPriority: 9,
			wantFacility: "user",
			wantSeverity: "alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(tt.input)
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want non-nil", tt.input)
			}
			if pri, ok := got.ExtraFields["priority"].(int); !ok || pri != tt.wantPriority {
				t.Errorf("priority = %v, want %d", got.ExtraFields["priority"], tt.wantPriority)
			}
			if fac, ok := got.ExtraFields["facility"].(string); !ok || fac != tt.wantFacility {
				t.Errorf("facility = %v, want %q", got.ExtraFields["facility"], tt.wantFacility)
			}
			if sev, ok := got.ExtraFields["severity"].(string); !ok || sev != tt.wantSeverity {
				t.Errorf("severity = %v, want %q", got.ExtraFields["severity"], tt.wantSeverity)
			}
		})
	}
}

func TestSeverityToLevelMapping(t *testing.T) {
	p := NewSyslogRFC3164Parser()

	// facility=0 (kern) so priority equals severity code directly
	tests := []struct {
		name      string
		priority  int
		wantLevel types.LogLevel
	}{
		{"emerg (0)", 0, types.LogLevelFatal},
		{"alert (1)", 1, types.LogLevelFatal},
		{"crit (2)", 2, types.LogLevelCritical},
		{"err (3)", 3, types.LogLevelError},
		{"warning (4)", 4, types.LogLevelWarn},
		{"notice (5)", 5, types.LogLevelInfo},
		{"info (6)", 6, types.LogLevelInfo},
		{"debug (7)", 7, types.LogLevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := fmt.Sprintf("<%d>Jan  1 00:00:00 host app: msg", tt.priority)
			got := p.Parse(line)
			if got == nil {
				t.Fatalf("Parse(%q) = nil", line)
			}
			if derefLevel(got.Level) != tt.wantLevel {
				t.Errorf("Level = %q, want %q", derefLevel(got.Level), tt.wantLevel)
			}
		})
	}
}

func TestSyslogRFC3164Parser_Detect(t *testing.T) {
	p := NewSyslogRFC3164Parser()

	tests := []struct {
		name      string
		lines     []string
		wantScore float64
	}{
		{
			name:      "all matching",
			lines:     []string{"<34>Jan  5 14:32:01 host sshd[1234]: msg1", "<13>Oct 22 09:15:00 host kernel: msg2"},
			wantScore: 1.0,
		},
		{
			name:      "none matching",
			lines:     []string{"random text", "another random line"},
			wantScore: 0.0,
		},
		{
			name:      "half matching",
			lines:     []string{"<34>Jan  5 14:32:01 host sshd[1234]: msg", "not a syslog line"},
			wantScore: 0.5,
		},
		{
			name:      "empty slice",
			lines:     []string{},
			wantScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect(tt.lines)
			if got != tt.wantScore {
				t.Errorf("Detect() = %f, want %f", got, tt.wantScore)
			}
		})
	}
}

func TestSyslogRFC5424Parser_Detect(t *testing.T) {
	p := NewSyslogRFC5424Parser()

	tests := []struct {
		name      string
		lines     []string
		wantScore float64
	}{
		{
			name: "all matching",
			lines: []string{
				"<165>1 2023-08-11T15:30:00.000Z server1 myapp 1234 ID47 - Application started",
				"<14>1 2023-01-01T00:00:00Z myhost myapp - - - System ready",
			},
			wantScore: 1.0,
		},
		{
			name:      "none matching",
			lines:     []string{"random text", "another line"},
			wantScore: 0.0,
		},
		{
			name: "half matching",
			lines: []string{
				"<165>1 2023-08-11T15:30:00.000Z server1 myapp 1234 ID47 - Application started",
				"not a syslog line",
			},
			wantScore: 0.5,
		},
		{
			name:      "empty slice",
			lines:     []string{},
			wantScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect(tt.lines)
			if got != tt.wantScore {
				t.Errorf("Detect() = %f, want %f", got, tt.wantScore)
			}
		})
	}
}

// TestParserInterface verifies both parsers satisfy the Parser interface.
func TestParserInterface(t *testing.T) {
	var _ Parser = NewSyslogRFC3164Parser()
	var _ Parser = NewSyslogRFC5424Parser()
}

func TestSyslogRFC3164Parser_NoPriorityLevelIsNil(t *testing.T) {
	p := NewSyslogRFC3164Parser()
	got := p.Parse("Dec  1 23:59:59 loghost cron: job completed")
	if got == nil {
		t.Fatal("Parse returned nil, want non-nil")
	}
	if got.Level != nil {
		t.Errorf("Level = %q, want nil (no priority present)", *got.Level)
	}
}

func TestSyslogRFC5424Parser_PriorityDerivation(t *testing.T) {
	p := NewSyslogRFC5424Parser()

	tests := []struct {
		name         string
		input        string
		wantPriority int
		wantFacility string
		wantSeverity string
	}{
		{
			name:         "priority 0 (kern.emerg)",
			input:        "<0>1 2023-01-01T00:00:00Z host app - - - msg",
			wantPriority: 0,
			wantFacility: "kern",
			wantSeverity: "emerg",
		},
		{
			name:         "priority 165 (local4.notice)",
			input:        "<165>1 2023-01-01T00:00:00Z host app - - - msg",
			wantPriority: 165,
			wantFacility: "local4",
			wantSeverity: "notice",
		},
		{
			name:         "priority 110 (audit.info)",
			input:        "<110>1 2023-01-01T00:00:00Z host app - - - msg",
			wantPriority: 110,
			wantFacility: "audit",
			wantSeverity: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(tt.input)
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want non-nil", tt.input)
			}
			if pri, ok := got.ExtraFields["priority"].(int); !ok || pri != tt.wantPriority {
				t.Errorf("priority = %v, want %d", got.ExtraFields["priority"], tt.wantPriority)
			}
			if fac, ok := got.ExtraFields["facility"].(string); !ok || fac != tt.wantFacility {
				t.Errorf("facility = %v, want %q", got.ExtraFields["facility"], tt.wantFacility)
			}
			if sev, ok := got.ExtraFields["severity"].(string); !ok || sev != tt.wantSeverity {
				t.Errorf("severity = %v, want %q", got.ExtraFields["severity"], tt.wantSeverity)
			}
		})
	}
}
