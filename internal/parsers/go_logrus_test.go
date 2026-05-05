package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestGoLogrusBracketParser(t *testing.T) {
	p := NewGoLogrusBracketParser()

	tests := []struct {
		name    string
		input   string
		wantNil bool
		level   types.LogLevel
		source  string
		msgPfx  string
	}{
		{
			name:   "error with message",
			input:  `2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: Final result of CNI ADD was an error. error=stat /var/lib/calico/nodename: no such file or directory`,
			level:  types.LogLevelError,
			source: "plugin.go",
			msgPfx: "Final result of CNI ADD",
		},
		{
			name:   "info with key-value pairs",
			input:  `2026-05-05 13:54:33.390 [INFO][39192] utils.go 188: Calico CNI releasing IP address ContainerID="20586f653056abd58e2ca1e7101fccad18a5b9e9d493864ee2bdc26c2ed18432"`,
			level:  types.LogLevelInfo,
			source: "utils.go",
			msgPfx: "Calico CNI releasing",
		},
		{
			name:   "warning level",
			input:  `2026-05-05 13:54:33.407 [WARNING][39212] ipam_plugin.go 434: Asked to release address but it doesn't exist. Ignoring ContainerID="abc123"`,
			level:  types.LogLevelWarn,
			source: "ipam_plugin.go",
			msgPfx: "Asked to release address",
		},
		{
			name:   "debug level",
			input:  `2026-01-15 08:00:00.001 [DEBUG][100] felix.go 42: Processing update key="/calico/resources"`,
			level:  types.LogLevelDebug,
			source: "felix.go",
			msgPfx: "Processing update",
		},
		{
			name:    "not matching - syslog",
			input:   `May  5 00:05:05 myhost syslogd[403]: ASL Sender Statistics`,
			wantNil: true,
		},
		{
			name:    "not matching - JSON",
			input:   `{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","msg":"hello"}`,
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
			result := p.Parse(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Level == nil || *result.Level != tt.level {
				t.Errorf("level = %v, want %v", result.Level, tt.level)
			}
			if result.Source == nil || *result.Source != tt.source {
				t.Errorf("source = %v, want %q", result.Source, tt.source)
			}
			if result.Timestamp == nil {
				t.Error("timestamp is nil")
			}
			if len(result.Message) < len(tt.msgPfx) || result.Message[:len(tt.msgPfx)] != tt.msgPfx {
				t.Errorf("message = %q, want prefix %q", result.Message, tt.msgPfx)
			}
		})
	}
}

func TestGoLogrusBracketKeyValueExtraction(t *testing.T) {
	p := NewGoLogrusBracketParser()

	input := `2026-05-05 13:54:33.443 [INFO][39254] ipam_plugin.go 417: Releasing address using handleID ContainerID="abc123" HandleID="k8s-pod-network.abc123" Workload="my-pod-eth0"`
	result := p.Parse(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check extracted key-value pairs
	if v, ok := result.ExtraFields["ContainerID"]; !ok || v != "abc123" {
		t.Errorf("ContainerID = %v, want abc123", v)
	}
	if v, ok := result.ExtraFields["HandleID"]; !ok || v != "k8s-pod-network.abc123" {
		t.Errorf("HandleID = %v, want k8s-pod-network.abc123", v)
	}
	if v, ok := result.ExtraFields["Workload"]; !ok || v != "my-pod-eth0" {
		t.Errorf("Workload = %v, want my-pod-eth0", v)
	}
	// PID should always be extracted
	if v, ok := result.ExtraFields["pid"]; !ok || v != "39254" {
		t.Errorf("pid = %v, want 39254", v)
	}
}

func TestGoLogrusBracketDetect(t *testing.T) {
	p := NewGoLogrusBracketParser()

	lines := []string{
		`2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: Final result of CNI ADD was an error.`,
		`2025-12-09 11:49:52.950 [INFO][7077] plugin.go 200: Processing request`,
		`2025-12-09 11:49:52.953 [WARNING][7123] k8s.go 579: Something happened`,
	}

	score := p.Detect(lines)
	if score < 0.9 {
		t.Errorf("Detect score = %f, want >= 0.9", score)
	}
}

func TestGoLogrusBracketAutoDetect(t *testing.T) {
	lines := []string{
		`2026-05-05 13:54:33.390 [INFO][39192] utils.go 188: Calico CNI releasing IP address ContainerID="abc"`,
		`2026-05-05 13:54:33.403 [INFO][39212] ipam_plugin.go 417: Releasing address`,
		`2026-05-05 13:54:33.407 [WARNING][39212] ipam_plugin.go 434: Asked to release address`,
		`2026-05-05 13:54:33.431 [ERROR][39242] k8s.go 572: CNI_CONTAINERID mismatch`,
	}

	result := AutoDetect(lines)
	if result.Format != types.LogFormatGoLogrusBracket {
		t.Errorf("AutoDetect = %q, want %q", result.Format, types.LogFormatGoLogrusBracket)
	}
	if result.Confidence < 0.9 {
		t.Errorf("confidence = %f, want >= 0.9", result.Confidence)
	}
}
