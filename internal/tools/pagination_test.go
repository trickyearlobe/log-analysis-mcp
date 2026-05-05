package tools

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestPagination_Search(t *testing.T) {
	path := writeTempLog(t, "pagination_search.log", buildSearchPaginationLog(10))
	recordPath := writeTempLog(t, "pagination_search_records.log", buildPaginationRecordLog(4))

	base := mustRunSearchLogs(t, SearchLogsInput{
		Path:       path,
		Pattern:    "match-",
		MaxResults: 10,
	})
	recordBase := mustRunSearchLogs(t, SearchLogsInput{
		Path:            recordPath,
		Pattern:         "trace-",
		MaxResults:      4,
		RecordSeparator: `^\d{4}-\d{2}-\d{2}`,
	})

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "offset=0 parity",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:       path,
					Pattern:    "match-",
					MaxResults: 10,
					Offset:     0,
				})
				if !reflect.DeepEqual(got, base) {
					t.Fatalf("offset=0 output mismatch\nwant: %#v\ngot:  %#v", base, got)
				}
			},
		},
		{
			name: "offset skips correct items",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:       path,
					Pattern:    "match-",
					MaxResults: 4,
					Offset:     3,
				})
				if got.TotalMatches != 10 {
					t.Fatalf("TotalMatches = %d, want 10", got.TotalMatches)
				}
				if len(got.Matches) != 4 {
					t.Fatalf("len(Matches) = %d, want 4", len(got.Matches))
				}
				if !reflect.DeepEqual(got.Matches[0], base.Matches[3]) {
					t.Fatalf("first match = %#v, want %#v", got.Matches[0], base.Matches[3])
				}
			},
		},
		{
			name: "offset + max_results = total",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:       path,
					Pattern:    "match-",
					MaxResults: 3,
					Offset:     7,
				})
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 10 {
					t.Fatalf("NextOffset = %d, want 10", got.NextOffset)
				}
				if !reflect.DeepEqual(got.Matches, base.Matches[7:10]) {
					t.Fatalf("tail page mismatch")
				}
			},
		},
		{
			name: "offset beyond total",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:       path,
					Pattern:    "match-",
					MaxResults: 5,
					Offset:     20,
				})
				if len(got.Matches) != 0 {
					t.Fatalf("len(Matches) = %d, want 0", len(got.Matches))
				}
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 20 {
					t.Fatalf("NextOffset = %d, want 20", got.NextOffset)
				}
			},
		},
		{
			name: "has_more and next_offset correctness",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:       path,
					Pattern:    "match-",
					MaxResults: 4,
					Offset:     3,
				})
				if !got.HasMore {
					t.Fatal("HasMore = false, want true")
				}
				if got.NextOffset != 7 {
					t.Fatalf("NextOffset = %d, want 7", got.NextOffset)
				}
			},
		},
		{
			name: "record_separator + offset",
			run: func(t *testing.T) {
				got := mustRunSearchLogs(t, SearchLogsInput{
					Path:            recordPath,
					Pattern:         "trace-",
					MaxResults:      2,
					Offset:          1,
					RecordSeparator: `^\d{4}-\d{2}-\d{2}`,
				})
				if got.TotalMatches != 4 {
					t.Fatalf("TotalMatches = %d, want 4", got.TotalMatches)
				}
				if !reflect.DeepEqual(got.Matches, recordBase.Matches[1:3]) {
					t.Fatalf("record page mismatch")
				}
				if !got.HasMore || got.NextOffset != 3 {
					t.Fatalf("HasMore/NextOffset = %v/%d, want true/3", got.HasMore, got.NextOffset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestPagination_Filter(t *testing.T) {
	path := writeTempLog(t, "pagination_filter.log", buildFilterPaginationLog(10))
	recordPath := writeTempLog(t, "pagination_filter_records.log", buildFilterRecordPaginationLog(4))

	base := mustRunFilterLogs(t, FilterLogsInput{
		Path:       path,
		Level:      []string{"ERROR"},
		MaxResults: 10,
	})
	recordBase := mustRunFilterLogs(t, FilterLogsInput{
		Path:            recordPath,
		Level:           []string{"ERROR"},
		MaxResults:      4,
		RecordSeparator: `^\{`,
	})

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "offset=0 parity",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:       path,
					Level:      []string{"ERROR"},
					MaxResults: 10,
					Offset:     0,
				})
				if !reflect.DeepEqual(got, base) {
					t.Fatalf("offset=0 output mismatch\nwant: %#v\ngot:  %#v", base, got)
				}
			},
		},
		{
			name: "offset skips correct items",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:       path,
					Level:      []string{"ERROR"},
					MaxResults: 4,
					Offset:     3,
				})
				if got.TotalMatched != 10 {
					t.Fatalf("TotalMatched = %d, want 10", got.TotalMatched)
				}
				if len(got.Entries) != 4 {
					t.Fatalf("len(Entries) = %d, want 4", len(got.Entries))
				}
				if !reflect.DeepEqual(got.Entries[0], base.Entries[3]) {
					t.Fatalf("first entry = %#v, want %#v", got.Entries[0], base.Entries[3])
				}
			},
		},
		{
			name: "offset + max_results = total",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:       path,
					Level:      []string{"ERROR"},
					MaxResults: 4,
					Offset:     6,
				})
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 10 {
					t.Fatalf("NextOffset = %d, want 10", got.NextOffset)
				}
				if !reflect.DeepEqual(got.Entries, base.Entries[6:10]) {
					t.Fatalf("tail page mismatch")
				}
			},
		},
		{
			name: "offset beyond total",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:       path,
					Level:      []string{"ERROR"},
					MaxResults: 5,
					Offset:     20,
				})
				if len(got.Entries) != 0 {
					t.Fatalf("len(Entries) = %d, want 0", len(got.Entries))
				}
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 20 {
					t.Fatalf("NextOffset = %d, want 20", got.NextOffset)
				}
			},
		},
		{
			name: "has_more and next_offset correctness",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:       path,
					Level:      []string{"ERROR"},
					MaxResults: 3,
					Offset:     2,
				})
				if !got.HasMore {
					t.Fatal("HasMore = false, want true")
				}
				if got.NextOffset != 5 {
					t.Fatalf("NextOffset = %d, want 5", got.NextOffset)
				}
			},
		},
		{
			name: "record_separator + offset",
			run: func(t *testing.T) {
				got := mustRunFilterLogs(t, FilterLogsInput{
					Path:            recordPath,
					Level:           []string{"ERROR"},
					MaxResults:      2,
					Offset:          1,
					RecordSeparator: `^\{`,
				})
				if got.TotalMatched != 4 {
					t.Fatalf("TotalMatched = %d, want 4", got.TotalMatched)
				}
				if !reflect.DeepEqual(got.Entries, recordBase.Entries[1:3]) {
					t.Fatalf("record page mismatch")
				}
				if !got.HasMore || got.NextOffset != 3 {
					t.Fatalf("HasMore/NextOffset = %v/%d, want true/3", got.HasMore, got.NextOffset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestPagination_ExtractErrors(t *testing.T) {
	path := writeTempLog(t, "pagination_extract_errors.log", buildExtractErrorsPaginationLog())
	base := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 10})
	if base.TotalClusters != 3 {
		t.Fatalf("setup TotalClusters = %d, want 3", base.TotalClusters)
	}
	if len(base.Clusters) != 3 {
		t.Fatalf("setup len(Clusters) = %d, want 3", len(base.Clusters))
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "offset=0 parity",
			run: func(t *testing.T) {
				got := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 10, Offset: 0})
				if !reflect.DeepEqual(got, base) {
					t.Fatalf("offset=0 output mismatch\nwant: %#v\ngot:  %#v", base, got)
				}
			},
		},
		{
			name: "offset slices sorted clusters",
			run: func(t *testing.T) {
				got := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 1, Offset: 1})
				if got.TotalClusters != 3 {
					t.Fatalf("TotalClusters = %d, want 3", got.TotalClusters)
				}
				if len(got.Clusters) != 1 {
					t.Fatalf("len(Clusters) = %d, want 1", len(got.Clusters))
				}
				if !reflect.DeepEqual(got.Clusters[0], base.Clusters[1]) {
					t.Fatalf("cluster = %#v, want %#v", got.Clusters[0], base.Clusters[1])
				}
			},
		},
		{
			name: "offset + max_results = total",
			run: func(t *testing.T) {
				got := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 2, Offset: 1})
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 3 {
					t.Fatalf("NextOffset = %d, want 3", got.NextOffset)
				}
				if !reflect.DeepEqual(got.Clusters, base.Clusters[1:3]) {
					t.Fatalf("tail page mismatch")
				}
			},
		},
		{
			name: "offset beyond total",
			run: func(t *testing.T) {
				got := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 2, Offset: 5})
				if len(got.Clusters) != 0 {
					t.Fatalf("len(Clusters) = %d, want 0", len(got.Clusters))
				}
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != 5 {
					t.Fatalf("NextOffset = %d, want 5", got.NextOffset)
				}
			},
		},
		{
			name: "has_more and next_offset correctness",
			run: func(t *testing.T) {
				got := mustRunExtractErrors(t, ExtractErrorsInput{Path: path, MaxClusters: 1, Offset: 1})
				if !got.HasMore {
					t.Fatal("HasMore = false, want true")
				}
				if got.NextOffset != 2 {
					t.Fatalf("NextOffset = %d, want 2", got.NextOffset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestPagination_DetectAnomalies(t *testing.T) {
	path := writeTempLog(t, "pagination_detect_anomalies.log", buildDetectAnomaliesPaginationLog())
	base := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 20})
	if base.TotalAnomalies != len(base.Anomalies) {
		t.Fatalf("setup TotalAnomalies = %d, want %d", base.TotalAnomalies, len(base.Anomalies))
	}
	if base.TotalAnomalies < 4 {
		t.Fatalf("setup TotalAnomalies = %d, want at least 4", base.TotalAnomalies)
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "offset=0 parity",
			run: func(t *testing.T) {
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 20, Offset: 0})
				if !reflect.DeepEqual(got, base) {
					t.Fatalf("offset=0 output mismatch\nwant: %#v\ngot:  %#v", base, got)
				}
			},
		},
		{
			name: "offset skips correct items",
			run: func(t *testing.T) {
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 2, Offset: 1})
				if len(got.Anomalies) != 2 {
					t.Fatalf("len(Anomalies) = %d, want 2", len(got.Anomalies))
				}
				if !reflect.DeepEqual(got.Anomalies, base.Anomalies[1:3]) {
					t.Fatalf("page mismatch")
				}
			},
		},
		{
			name: "total_anomalies correct and max_results caps output",
			run: func(t *testing.T) {
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 2})
				if got.TotalAnomalies != base.TotalAnomalies {
					t.Fatalf("TotalAnomalies = %d, want %d", got.TotalAnomalies, base.TotalAnomalies)
				}
				if len(got.Anomalies) != 2 {
					t.Fatalf("len(Anomalies) = %d, want 2", len(got.Anomalies))
				}
				if !got.HasMore || got.NextOffset != 2 {
					t.Fatalf("HasMore/NextOffset = %v/%d, want true/2", got.HasMore, got.NextOffset)
				}
			},
		},
		{
			name: "offset + max_results = total",
			run: func(t *testing.T) {
				offset := base.TotalAnomalies - 2
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 2, Offset: offset})
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != base.TotalAnomalies {
					t.Fatalf("NextOffset = %d, want %d", got.NextOffset, base.TotalAnomalies)
				}
				if !reflect.DeepEqual(got.Anomalies, base.Anomalies[offset:]) {
					t.Fatalf("tail page mismatch")
				}
			},
		},
		{
			name: "offset beyond total",
			run: func(t *testing.T) {
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 2, Offset: base.TotalAnomalies + 5})
				if len(got.Anomalies) != 0 {
					t.Fatalf("len(Anomalies) = %d, want 0", len(got.Anomalies))
				}
				if got.HasMore {
					t.Fatal("HasMore = true, want false")
				}
				if got.NextOffset != base.TotalAnomalies+5 {
					t.Fatalf("NextOffset = %d, want %d", got.NextOffset, base.TotalAnomalies+5)
				}
			},
		},
		{
			name: "has_more and next_offset correctness",
			run: func(t *testing.T) {
				got := mustRunDetectAnomalies(t, DetectAnomaliesInput{Path: path, WindowMinutes: 5, Sensitivity: "medium", MaxResults: 2, Offset: 1})
				if !got.HasMore {
					t.Fatal("HasMore = false, want true")
				}
				if got.NextOffset != 3 {
					t.Fatalf("NextOffset = %d, want 3", got.NextOffset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func mustRunSearchLogs(t *testing.T, input SearchLogsInput) SearchLogsOutput {
	t.Helper()
	out, err := RunSearchLogs(input)
	if err != nil {
		t.Fatalf("RunSearchLogs(%+v): %v", input, err)
	}
	return out
}

func mustRunFilterLogs(t *testing.T, input FilterLogsInput) FilterLogsOutput {
	t.Helper()
	out, err := RunFilterLogs(input)
	if err != nil {
		t.Fatalf("RunFilterLogs(%+v): %v", input, err)
	}
	return out
}

func mustRunExtractErrors(t *testing.T, input ExtractErrorsInput) ExtractErrorsOutput {
	t.Helper()
	out, err := RunExtractErrors(input)
	if err != nil {
		t.Fatalf("RunExtractErrors(%+v): %v", input, err)
	}
	return out
}

func mustRunDetectAnomalies(t *testing.T, input DetectAnomaliesInput) DetectAnomaliesOutput {
	t.Helper()
	out, err := RunDetectAnomalies(input)
	if err != nil {
		t.Fatalf("RunDetectAnomalies(%+v): %v", input, err)
	}
	return out
}

func buildSearchPaginationLog(total int) string {
	lines := make([]string, 0, total)
	for i := 0; i < total; i++ {
		lines = append(lines, fmt.Sprintf("2025-01-15T10:%02d:00Z INFO [app] match-%02d", i, i+1))
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildFilterPaginationLog(total int) string {
	lines := make([]string, 0, total)
	for i := 0; i < total; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T11:%02d:00Z","level":"ERROR","source":"app","message":"payment failure %02d"}`, i, i+1))
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildPaginationRecordLog(total int) string {
	lines := make([]string, 0, total*3)
	for i := 1; i <= total; i++ {
		lines = append(lines,
			fmt.Sprintf("2025-01-15T12:%02d:00Z ERROR [app] request %02d failed", i, i),
			fmt.Sprintf("\tat trace-%02d line one", i),
			fmt.Sprintf("\tat trace-%02d line two", i),
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildFilterRecordPaginationLog(total int) string {
	lines := make([]string, 0, total*2)
	for i := 1; i <= total; i++ {
		lines = append(lines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T12:%02d:00Z","level":"ERROR","source":"app","message":"request %02d failed"}`, i, i),
			fmt.Sprintf("\tat trace-%02d line one", i),
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildExtractErrorsPaginationLog() string {
	messages := []string{
		"database timeout while fetching profile",
		"database timeout while fetching profile",
		"database timeout while fetching profile",
		"database timeout while fetching profile",
		"database timeout while fetching profile",
		"cache warmup failed for shard east",
		"cache warmup failed for shard east",
		"cache warmup failed for shard east",
		"worker panic in email sender",
		"worker panic in email sender",
	}
	lines := make([]string, 0, len(messages))
	for i, msg := range messages {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T13:%02d:00Z","level":"ERROR","source":"app","message":%q}`, i, msg))
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildDetectAnomaliesPaginationLog() string {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"INFO","source":"app","msg":"baseline %02d"}`, i, i))
	}
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T02:10:%02dZ","level":"ERROR","source":"app","msg":"known database timeout"}`, i))
	}
	for i := 15; i < 25; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"INFO","source":"app","msg":"recovered %02d"}`, i, i))
	}
	lines = append(lines,
		`{"timestamp":"2025-01-15T04:00:00Z","level":"INFO","source":"app","msg":"after gap"}`,
		`{"timestamp":"2025-01-15T04:01:00Z","level":"INFO","source":"app","msg":"after gap steady"}`,
	)
	for i := 2; i < 10; i++ {
		level := "INFO"
		message := fmt.Sprintf("tail steady %02d", i)
		if i%2 == 0 {
			level = "ERROR"
			message = "ssl handshake failed on edge proxy"
		}
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2025-01-15T04:%02d:00Z","level":%q,"source":"app","msg":%q}`, i, level, message))
	}
	return strings.Join(lines, "\n") + "\n"
}
