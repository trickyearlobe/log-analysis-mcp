package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestErlangSASLParser_Parse(t *testing.T) {
	p := NewErlangSASLParser()

	tests := []struct {
		name      string
		line      string
		wantNil   bool
		wantLevel types.LogLevel
		wantMsg   string
		wantTS    string
	}{
		{
			name:      "error report",
			line:      "=ERROR REPORT==== 15-Jan-2025::10:00:01 ===",
			wantLevel: types.LogLevelError,
			wantMsg:   "ERROR REPORT",
			wantTS:    "2025-01-15T10:00:01Z",
		},
		{
			name:      "crash report",
			line:      "=CRASH REPORT==== 27-May-1996::13:38:56 ===",
			wantLevel: types.LogLevelFatal,
			wantMsg:   "CRASH REPORT",
			wantTS:    "1996-05-27T13:38:56Z",
		},
		{
			name:      "progress report",
			line:      "=PROGRESS REPORT==== 1-Feb-2025::08:30:00 ===",
			wantLevel: types.LogLevelInfo,
			wantMsg:   "PROGRESS REPORT",
		},
		{
			name:      "supervisor report",
			line:      "=SUPERVISOR REPORT==== 15-Jan-2025::10:00:01 ===",
			wantLevel: types.LogLevelWarn,
			wantMsg:   "SUPERVISOR REPORT",
		},
		{
			name:      "warning report",
			line:      "=WARNING REPORT==== 15-Jan-2025::10:00:01 ===",
			wantLevel: types.LogLevelWarn,
			wantMsg:   "WARNING REPORT",
		},
		{
			name:    "continuation line - not a header",
			line:    "          supervisor: {local,sasl_safe_sup}",
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantNil: true,
		},
		{
			name:    "non-matching format",
			line:    "2025-01-15 INFO something happened",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := p.Parse(tt.line)
			if tt.wantNil {
				if entry != nil {
					t.Errorf("expected nil, got %+v", entry)
				}
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			if entry.Level == nil || *entry.Level != tt.wantLevel {
				t.Errorf("level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", entry.Message, tt.wantMsg)
			}
			if tt.wantTS != "" && (entry.Timestamp == nil || *entry.Timestamp != tt.wantTS) {
				ts := ""
				if entry.Timestamp != nil {
					ts = *entry.Timestamp
				}
				t.Errorf("timestamp = %q, want %q", ts, tt.wantTS)
			}
		})
	}
}

func TestErlangSASLParser_Detect(t *testing.T) {
	p := NewErlangSASLParser()

	tests := []struct {
		name    string
		lines   []string
		wantMin float64
	}{
		{
			name: "pure SASL headers",
			lines: []string{
				"=ERROR REPORT==== 15-Jan-2025::10:00:01 ===",
				"=PROGRESS REPORT==== 15-Jan-2025::10:00:02 ===",
			},
			wantMin: 0.9,
		},
		{
			name: "mixed SASL with continuations",
			lines: []string{
				"=ERROR REPORT==== 15-Jan-2025::10:00:01 ===",
				"          supervisor: {local,sasl_safe_sup}",
				"             started: [{pid,<0.43.0>}]",
				"=PROGRESS REPORT==== 15-Jan-2025::10:00:02 ===",
				"          application: kernel",
			},
			wantMin: 0.6,
		},
		{
			name: "no SASL lines",
			lines: []string{
				"INFO Starting application",
				"DEBUG Connected to database",
			},
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := p.Detect(tt.lines)
			if score < tt.wantMin {
				t.Errorf("score = %f, want >= %f", score, tt.wantMin)
			}
		})
	}
}

func TestErlangSASLParser_Name(t *testing.T) {
	p := NewErlangSASLParser()
	if p.Name() != "erlang-sasl" {
		t.Errorf("Name() = %q, want %q", p.Name(), "erlang-sasl")
	}
}
