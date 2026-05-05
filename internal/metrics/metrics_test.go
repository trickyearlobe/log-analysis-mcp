package metrics

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWriterConcurrency(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				w.Record(Event{
					Timestamp:  time.Now(),
					Tool:       "test_tool",
					Status:     StatusOK,
					DurationMs: int64(n*50 + j),
				})
			}
		}(i)
	}
	wg.Wait()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify all 500 events were written
	files, _ := filepath.Glob(filepath.Join(dir, "events-*.jsonl"))
	if len(files) == 0 {
		t.Fatal("no event files created")
	}

	events, err := readEvents(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 500 {
		t.Errorf("expected 500 events, got %d", len(events))
	}
}

func TestWriterFlushOnClose(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	w.Record(Event{
		Timestamp:  time.Now(),
		Tool:       "log_search",
		Status:     StatusOK,
		DurationMs: 42,
	})

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	events, _ := readEvents(dir, time.Time{})
	if len(events) != 1 {
		t.Errorf("expected 1 event after close, got %d", len(events))
	}
}

func TestReaderQuery(t *testing.T) {
	dir := t.TempDir()

	// Write test events directly to a file
	events := []Event{
		{Timestamp: time.Now(), Tool: "log_search", Status: StatusOK, DurationMs: 100, ResponseBytes: 2000},
		{Timestamp: time.Now(), Tool: "log_search", Status: StatusOK, DurationMs: 200, ResponseBytes: 3000},
		{Timestamp: time.Now(), Tool: "log_search", Status: StatusError, DurationMs: 50, ErrorCode: "FILE_NOT_FOUND"},
		{Timestamp: time.Now(), Tool: "log_filter", Status: StatusOK, DurationMs: 300, ResponseBytes: 5000},
		{Timestamp: time.Now(), Tool: "log_filter", Status: StatusOK, DurationMs: 400, ResponseBytes: 6000, Warning: WarnSlowCall},
	}
	writeTestEvents(t, dir, events)

	tests := []struct {
		name  string
		input QueryInput
		check func(t *testing.T, out QueryOutput)
	}{
		{
			name:  "group by tool",
			input: QueryInput{Since: "1h", GroupBy: "tool", TopK: 10},
			check: func(t *testing.T, out QueryOutput) {
				if out.TotalEvents != 5 {
					t.Errorf("total = %d, want 5", out.TotalEvents)
				}
				if len(out.Groups) != 2 {
					t.Errorf("groups = %d, want 2", len(out.Groups))
				}
				// log_search has 3 calls, should be first
				if out.Groups[0].Key != "log_search" {
					t.Errorf("first group = %q, want log_search", out.Groups[0].Key)
				}
				if out.Groups[0].Calls != 3 {
					t.Errorf("log_search calls = %d, want 3", out.Groups[0].Calls)
				}
				if out.Groups[0].Errors != 1 {
					t.Errorf("log_search errors = %d, want 1", out.Groups[0].Errors)
				}
			},
		},
		{
			name:  "group by status",
			input: QueryInput{Since: "1h", GroupBy: "status", TopK: 10},
			check: func(t *testing.T, out QueryOutput) {
				if len(out.Groups) != 2 {
					t.Errorf("groups = %d, want 2", len(out.Groups))
				}
			},
		},
		{
			name:  "group by warning",
			input: QueryInput{Since: "1h", GroupBy: "warning", TopK: 10},
			check: func(t *testing.T, out QueryOutput) {
				// 2 groups: "<none>" and "SLOW_CALL"
				if len(out.Groups) != 2 {
					t.Errorf("groups = %d, want 2", len(out.Groups))
				}
			},
		},
		{
			name:  "filter by tool",
			input: QueryInput{Since: "1h", GroupBy: "tool", Tool: "log_filter", TopK: 10},
			check: func(t *testing.T, out QueryOutput) {
				if out.TotalEvents != 2 {
					t.Errorf("total = %d, want 2", out.TotalEvents)
				}
			},
		},
		{
			name:  "top_k limit",
			input: QueryInput{Since: "1h", GroupBy: "tool", TopK: 1},
			check: func(t *testing.T, out QueryOutput) {
				if len(out.Groups) != 1 {
					t.Errorf("groups = %d, want 1", len(out.Groups))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Query(dir, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, out)
		})
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		values []int64
		pct    int
		want   int64
	}{
		{"p50 of 1 value", []int64{100}, 50, 100},
		{"p50 of 2 values", []int64{100, 200}, 50, 100},
		{"p95 of 10 values", []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 95, 10},
		{"p50 of 10 values", []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 50, 5},
		{"empty", []int64{}, 50, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.values, tt.pct)
			if got != tt.want {
				t.Errorf("percentile(%v, %d) = %d, want %d", tt.values, tt.pct, got, tt.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"1h", time.Hour},
		{"", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func writeTestEvents(t *testing.T, dir string, events []Event) {
	t.Helper()
	filename := filepath.Join(dir, "events-"+time.Now().Format("2006-01-02")+".jsonl")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, e := range events {
		data, _ := MarshalEvent(e)
		f.Write(data)
		f.Write([]byte("\n"))
	}
}
