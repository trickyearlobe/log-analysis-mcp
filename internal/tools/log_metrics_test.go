package tools

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/metrics"
)

func TestRunLogMetrics(t *testing.T) {
	dir := t.TempDir()

	// Write some test events
	events := []metrics.Event{
		{Timestamp: time.Now(), Tool: "log_search", Status: metrics.StatusOK, DurationMs: 100, ResponseBytes: 2000},
		{Timestamp: time.Now(), Tool: "log_search", Status: metrics.StatusOK, DurationMs: 200, ResponseBytes: 3000},
		{Timestamp: time.Now(), Tool: "log_search", Status: metrics.StatusError, DurationMs: 50, ErrorCode: "FILE_NOT_FOUND"},
		{Timestamp: time.Now(), Tool: "log_filter", Status: metrics.StatusOK, DurationMs: 300, ResponseBytes: 5000},
	}
	writeTestMetrics(t, dir, events)

	tests := []struct {
		name  string
		input LogMetricsInput
		check func(t *testing.T, out *metrics.QueryOutput)
	}{
		{
			name:  "default group by tool",
			input: LogMetricsInput{Since: "1h", GroupBy: "tool", TopK: 10},
			check: func(t *testing.T, out *metrics.QueryOutput) {
				if out.TotalEvents != 4 {
					t.Errorf("total = %d, want 4", out.TotalEvents)
				}
				if len(out.Groups) != 2 {
					t.Errorf("groups = %d, want 2", len(out.Groups))
				}
			},
		},
		{
			name:  "filter by tool",
			input: LogMetricsInput{Since: "1h", GroupBy: "tool", Tool: "log_filter", TopK: 10},
			check: func(t *testing.T, out *metrics.QueryOutput) {
				if out.TotalEvents != 1 {
					t.Errorf("total = %d, want 1", out.TotalEvents)
				}
			},
		},
		{
			name:  "empty dir returns zero",
			input: LogMetricsInput{Since: "1h", GroupBy: "tool", TopK: 10},
			check: func(t *testing.T, out *metrics.QueryOutput) {
				if out.TotalEvents != 0 {
					t.Errorf("total = %d, want 0", out.TotalEvents)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := dir
			if tt.name == "empty dir returns zero" {
				testDir = t.TempDir()
			}
			out, err := RunLogMetrics(testDir, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, out)
		})
	}
}

func writeTestMetrics(t *testing.T, dir string, events []metrics.Event) {
	t.Helper()
	filename := filepath.Join(dir, "events-"+time.Now().Format("2006-01-02")+".jsonl")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, e := range events {
		data, _ := metrics.MarshalEvent(e)
		f.Write(data)
		f.Write([]byte("\n"))
	}
}
