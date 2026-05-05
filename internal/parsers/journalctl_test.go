package parsers

import (
	"testing"
)

func TestJournalISOParser_Parse(t *testing.T) {
	p := NewJournalISOParser()

	tests := []struct {
		name       string
		line       string
		wantNil    bool
		wantSource string
		wantMsg    string
		wantTS     string
		wantPID    string
	}{
		{
			name:       "standard with PID",
			line:       "2025-01-15T10:00:01+0000 myhost sshd[1234]: Accepted publickey for user from 10.0.0.1",
			wantSource: "sshd",
			wantMsg:    "Accepted publickey for user from 10.0.0.1",
			wantTS:     "2025-01-15T10:00:01+0000",
			wantPID:    "1234",
		},
		{
			name:       "kernel message without PID",
			line:       "2025-01-15T10:00:01+0000 myhost kernel: eth0: renamed from veth3abc1234",
			wantSource: "kernel",
			wantMsg:    "eth0: renamed from veth3abc1234",
			wantTS:     "2025-01-15T10:00:01+0000",
		},
		{
			name:       "negative timezone offset",
			line:       "2025-01-15T10:00:01-0500 myhost audit[999]: type=USER_AUTH msg=audit(1736935201.001:42)",
			wantSource: "audit",
			wantMsg:    "type=USER_AUTH msg=audit(1736935201.001:42)",
			wantTS:     "2025-01-15T10:00:01-0500",
			wantPID:    "999",
		},
		{
			name:       "sub-second precision",
			line:       "2025-01-15T10:00:01.123456+0000 myhost dockerd[888]: container started",
			wantSource: "dockerd",
			wantMsg:    "container started",
			wantTS:     "2025-01-15T10:00:01.123456+0000",
			wantPID:    "888",
		},
		{
			name:       "systemd with PID 1",
			line:       "2025-01-15T10:00:01+0000 myhost systemd[1]: Started OpenSSH server daemon.",
			wantSource: "systemd",
			wantMsg:    "Started OpenSSH server daemon.",
			wantTS:     "2025-01-15T10:00:01+0000",
			wantPID:    "1",
		},
		{
			name:    "non-matching format",
			line:    "Jan 15 10:00:01 myhost sshd[1234]: something",
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
			if entry.Timestamp == nil || *entry.Timestamp != tt.wantTS {
				ts := ""
				if entry.Timestamp != nil {
					ts = *entry.Timestamp
				}
				t.Errorf("timestamp = %q, want %q", ts, tt.wantTS)
			}
			// Level should be nil (not present in journal short-iso format)
			if entry.Level != nil {
				t.Errorf("level should be nil for journal format, got %v", *entry.Level)
			}
			// Check PID in extras
			if tt.wantPID != "" {
				if entry.ExtraFields == nil || entry.ExtraFields["pid"] != tt.wantPID {
					t.Errorf("pid = %v, want %q", entry.ExtraFields["pid"], tt.wantPID)
				}
			}
			// Hostname should always be present
			if entry.ExtraFields == nil || entry.ExtraFields["hostname"] == nil {
				t.Error("expected hostname in extra_fields")
			}
		})
	}
}

func TestJournalISOParser_Detect(t *testing.T) {
	p := NewJournalISOParser()

	tests := []struct {
		name    string
		lines   []string
		wantMin float64
	}{
		{
			name: "all journal lines",
			lines: []string{
				"2025-01-15T10:00:01+0000 myhost sshd[1234]: Accepted publickey",
				"2025-01-15T10:00:02+0000 myhost systemd[1]: Started service",
			},
			wantMin: 0.9,
		},
		{
			name: "no journal lines",
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

func TestJournalISOParser_Name(t *testing.T) {
	p := NewJournalISOParser()
	if p.Name() != "journalctl-short-iso" {
		t.Errorf("Name() = %q, want %q", p.Name(), "journalctl-short-iso")
	}
}
