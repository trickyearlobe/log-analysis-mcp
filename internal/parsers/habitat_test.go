package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestHabitatSupParser_Parse(t *testing.T) {
	p := NewHabitatSupParser()

	tests := []struct {
		name       string
		line       string
		wantNil    bool
		wantLevel  types.LogLevel
		wantSource string
		wantMsg    string
	}{
		{
			name:       "info level",
			line:       "hab-sup(MN): 2025-01-15T10:00:01.001234Z INFO hab_sup::manager: Loading service core/redis/1.0.0",
			wantLevel:  types.LogLevelInfo,
			wantSource: "hab_sup::manager",
			wantMsg:    "Loading service core/redis/1.0.0",
		},
		{
			name:       "error level",
			line:       "hab-sup(MN): 2025-01-15T10:00:01.004567Z ERROR hab_sup::manager: Failed to bind port 6379",
			wantLevel:  types.LogLevelError,
			wantSource: "hab_sup::manager",
			wantMsg:    "Failed to bind port 6379",
		},
		{
			name:       "debug level no subseconds",
			line:       "hab-sup(MN): 2025-01-15T10:00:01Z DEBUG hab_sup::census: Census updated",
			wantLevel:  types.LogLevelDebug,
			wantSource: "hab_sup::census",
			wantMsg:    "Census updated",
		},
		{
			name:       "trace level",
			line:       "hab-sup(MN): 2025-01-15T10:00:01.005678Z TRACE hab_sup::util: Entering butterfly gossip loop",
			wantLevel:  types.LogLevelTrace,
			wantSource: "hab_sup::util",
			wantMsg:    "Entering butterfly gossip loop",
		},
		{
			name:       "warn level with ctl_gateway source",
			line:       "hab-sup(MN): 2025-01-15T10:00:01.003456Z WARN ctl_gateway: Connection timeout",
			wantLevel:  types.LogLevelWarn,
			wantSource: "ctl_gateway",
			wantMsg:    "Connection timeout",
		},
		{
			name:    "non-matching format",
			line:    "2025-01-15 INFO something happened",
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    "",
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
			if entry.Source == nil || *entry.Source != tt.wantSource {
				src := ""
				if entry.Source != nil {
					src = *entry.Source
				}
				t.Errorf("source = %q, want %q", src, tt.wantSource)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", entry.Message, tt.wantMsg)
			}
			if entry.Timestamp == nil || *entry.Timestamp == "" {
				t.Error("expected non-empty timestamp")
			}
			if entry.ExtraFields == nil || entry.ExtraFields["worker"] == nil {
				t.Error("expected worker in extra_fields")
			}
		})
	}
}

func TestHabitatSupParser_Detect(t *testing.T) {
	p := NewHabitatSupParser()

	tests := []struct {
		name    string
		lines   []string
		wantMin float64
	}{
		{
			name: "all habitat lines",
			lines: []string{
				"hab-sup(MN): 2025-01-15T10:00:01.001234Z INFO hab_sup::manager: Starting",
				"hab-sup(MN): 2025-01-15T10:00:02.001234Z WARN ctl_gateway: Timeout",
			},
			wantMin: 0.9,
		},
		{
			name: "no habitat lines",
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

func TestHabitatSupParser_Name(t *testing.T) {
	p := NewHabitatSupParser()
	if p.Name() != "habitat-sup" {
		t.Errorf("Name() = %q, want %q", p.Name(), "habitat-sup")
	}
}
