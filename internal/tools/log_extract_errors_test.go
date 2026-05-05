package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestRunExtractErrors(t *testing.T) {
	jsonLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Connection timeout to host 10.0.1.5"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"info","msg":"Heartbeat ok"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"Connection timeout to host 10.0.1.6"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"error","msg":"Connection timeout to host 10.0.1.7"}`,
		`{"timestamp":"2025-01-15T02:04:00Z","level":"info","msg":"Processing request"}`,
		`{"timestamp":"2025-01-15T02:05:00Z","level":"error","msg":"NullPointerException in UserService.getProfile"}`,
		`{"timestamp":"2025-01-15T02:06:00Z","level":"info","msg":"Request completed"}`,
		`{"timestamp":"2025-01-15T04:00:00Z","level":"error","msg":"Connection timeout to host 10.0.1.8"}`,
	}, "\n") + "\n"
	jsonPath := writeTempLog(t, "json.log", jsonLog)

	noErrorsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"info","msg":"Starting up"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"debug","msg":"Loading config"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"info","msg":"Ready"}`,
	}, "\n") + "\n"
	noErrorsPath := writeTempLog(t, "noerrors.log", noErrorsLog)

	emptyPath := writeTempLog(t, "empty.log", "")

	distinctLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Connection timeout to host 10.0.1.5"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"error","msg":"Disk full on volume data"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"Authentication failed for user admin"}`,
	}, "\n") + "\n"
	distinctPath := writeTempLog(t, "distinct.log", distinctLog)

	clusterNames := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima", "mike", "november", "oscar",
		"papa", "quebec", "romeo", "sierra", "tango",
		"uniform", "victor", "whiskey", "xray", "yankee",
		"zulu", "able", "baker", "candy", "dreamer",
	}
	var manyClusterLines []string
	for i, name := range clusterNames {
		manyClusterLines = append(manyClusterLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"error","msg":"Service %s crashed unexpectedly"}`, i%60, name))
	}
	manyClustersLog := strings.Join(manyClusterLines, "\n") + "\n"
	manyClustersPath := writeTempLog(t, "manyclusters.log", manyClustersLog)

	sortLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Rare error alpha"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"error","msg":"Common error beta"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"Common error beta"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"error","msg":"Common error beta"}`,
		`{"timestamp":"2025-01-15T02:04:00Z","level":"error","msg":"Medium error gamma"}`,
		`{"timestamp":"2025-01-15T02:05:00Z","level":"error","msg":"Medium error gamma"}`,
	}, "\n") + "\n"
	sortPath := writeTempLog(t, "sort.log", sortLog)

	stackLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"info","msg":"Starting service"}`,
		`{"timestamp":"2025-01-15T02:00:01Z","level":"info","msg":"Loading configuration"}`,
		`{"timestamp":"2025-01-15T02:00:02Z","level":"info","msg":"Connecting to database"}`,
		`{"timestamp":"2025-01-15T02:00:03Z","level":"info","msg":"Database connected"}`,
		`{"timestamp":"2025-01-15T02:00:04Z","level":"info","msg":"Service ready"}`,
		`{"timestamp":"2025-01-15T02:00:10Z","level":"error","msg":"NullPointerException in UserService"}`,
		"\tat com.example.UserService.getProfile(UserService.java:142)",
		"\tat com.example.ApiController.handleRequest(ApiController.java:89)",
		`{"timestamp":"2025-01-15T02:01:00Z","level":"info","msg":"Heartbeat ok"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"NullPointerException in UserService"}`,
		"\tat com.example.UserService.getProfile(UserService.java:142)",
		"\tat com.example.ApiController.handleRequest(ApiController.java:89)",
		`{"timestamp":"2025-01-15T02:03:00Z","level":"info","msg":"Processing complete"}`,
	}, "\n") + "\n"
	stackPath := writeTempLog(t, "stack.log", stackLog)

	multiLevelLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Standard error happened"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"fatal","msg":"Fatal crash occurred"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"critical","msg":"Critical failure detected"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"info","msg":"This is info"}`,
	}, "\n") + "\n"
	multiLevelPath := writeTempLog(t, "multilevel.log", multiLevelLog)

	percentLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Error A"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"info","msg":"Info one"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"Error A"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"info","msg":"Info two"}`,
		`{"timestamp":"2025-01-15T02:04:00Z","level":"error","msg":"Error A"}`,
		`{"timestamp":"2025-01-15T02:05:00Z","level":"info","msg":"Info three"}`,
		`{"timestamp":"2025-01-15T02:06:00Z","level":"error","msg":"Error A"}`,
		`{"timestamp":"2025-01-15T02:07:00Z","level":"info","msg":"Info four"}`,
		`{"timestamp":"2025-01-15T02:08:00Z","level":"error","msg":"Error A"}`,
		`{"timestamp":"2025-01-15T02:09:00Z","level":"info","msg":"Info five"}`,
	}, "\n") + "\n"
	percentPath := writeTempLog(t, "percent.log", percentLog)

	var sampleCapLines []string
	for i := 0; i < 5; i++ {
		sampleCapLines = append(sampleCapLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"error","msg":"Repeated error message number %d"}`, i, i))
	}
	sampleCapLog := strings.Join(sampleCapLines, "\n") + "\n"
	sampleCapPath := writeTempLog(t, "samplecap.log", sampleCapLog)

	tests := []struct {
		name        string
		input       ExtractErrorsInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out ExtractErrorsOutput)
	}{
		{name: "json log with error entries produces clusters", input: ExtractErrorsInput{Path: jsonPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.TotalErrors != 5 {
				t.Errorf("TotalErrors = %d, want 5", out.TotalErrors)
			}
			if len(out.Clusters) != 2 {
				t.Errorf("len(Clusters) = %d, want 2", len(out.Clusters))
			}
		}},
		{name: "multiple similar errors grouped into one cluster", input: ExtractErrorsInput{Path: jsonPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) < 1 {
				t.Fatal("expected at least one cluster")
			}
			if out.Clusters[0].Count != 4 {
				t.Errorf("top cluster count = %d, want 4", out.Clusters[0].Count)
			}
			if !strings.Contains(out.Clusters[0].Pattern, "<IP>") {
				t.Errorf("pattern %q should contain <IP>", out.Clusters[0].Pattern)
			}
		}},
		{name: "different errors produce separate clusters", input: ExtractErrorsInput{Path: distinctPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 3 {
				t.Errorf("len(Clusters) = %d, want 3", len(out.Clusters))
			}
			if out.TotalErrors != 3 {
				t.Errorf("TotalErrors = %d, want 3", out.TotalErrors)
			}
		}},
		{name: "stack traces captured when true", input: ExtractErrorsInput{Path: stackPath, IncludeStackTraces: true}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.TotalErrors == 0 {
				t.Fatal("expected at least one error")
			}
			if len(out.Clusters) < 1 {
				t.Fatal("expected at least one cluster")
			}
			if out.Clusters[0].StackTrace == nil {
				t.Fatal("expected stack trace")
			}
			if !strings.Contains(*out.Clusters[0].StackTrace, "com.example.UserService") {
				t.Error("stack trace missing expected content")
			}
		}},
		{name: "stack traces not captured when false", input: ExtractErrorsInput{Path: stackPath, IncludeStackTraces: false}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.TotalErrors == 0 {
				t.Fatal("expected at least one error")
			}
			if len(out.Clusters) < 1 {
				t.Fatal("expected at least one cluster")
			}
			if out.Clusters[0].StackTrace != nil {
				t.Errorf("expected no stack trace, got %q", *out.Clusters[0].StackTrace)
			}
		}},
		{name: "percentage of all lines calculated correctly", input: ExtractErrorsInput{Path: percentPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.TotalErrors != 5 {
				t.Errorf("TotalErrors = %d, want 5", out.TotalErrors)
			}
			if out.ErrorRate.PercentageOfAllLines < 49.9 || out.ErrorRate.PercentageOfAllLines > 50.1 {
				t.Errorf("PercentageOfAllLines = %.2f, want ~50", out.ErrorRate.PercentageOfAllLines)
			}
		}},
		{name: "cluster percentage sums to approximately 100", input: ExtractErrorsInput{Path: distinctPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			sum := 0.0
			for _, c := range out.Clusters {
				sum += c.Percentage
			}
			if sum < 99.9 || sum > 100.1 {
				t.Errorf("cluster percentages sum to %.2f", sum)
			}
		}},
		{name: "max clusters limits output", input: ExtractErrorsInput{Path: manyClustersPath, MaxClusters: 5}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 5 {
				t.Errorf("len(Clusters) = %d, want 5", len(out.Clusters))
			}
			if out.TotalErrors != 30 {
				t.Errorf("TotalErrors = %d, want 30", out.TotalErrors)
			}
		}},
		{name: "default max clusters is 20", input: ExtractErrorsInput{Path: manyClustersPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 20 {
				t.Errorf("len(Clusters) = %d, want 20", len(out.Clusters))
			}
		}},
		{name: "sorted by count descending", input: ExtractErrorsInput{Path: sortPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) < 3 {
				t.Fatalf("len(Clusters) = %d, want 3", len(out.Clusters))
			}
			if out.Clusters[0].Count != 3 {
				t.Errorf("[0].Count = %d, want 3", out.Clusters[0].Count)
			}
			if out.Clusters[1].Count != 2 {
				t.Errorf("[1].Count = %d, want 2", out.Clusters[1].Count)
			}
			if out.Clusters[2].Count != 1 {
				t.Errorf("[2].Count = %d, want 1", out.Clusters[2].Count)
			}
			for i := 1; i < len(out.Clusters); i++ {
				if out.Clusters[i].Count > out.Clusters[i-1].Count {
					t.Errorf("not sorted at %d", i)
				}
			}
		}},
		{name: "empty file returns no clusters", input: ExtractErrorsInput{Path: emptyPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.Clusters == nil {
				t.Error("Clusters should be non-nil")
			}
			if len(out.Clusters) != 0 {
				t.Errorf("len = %d, want 0", len(out.Clusters))
			}
			if out.TotalErrors != 0 {
				t.Errorf("TotalErrors = %d, want 0", out.TotalErrors)
			}
		}},
		{name: "file with no errors returns empty clusters", input: ExtractErrorsInput{Path: noErrorsPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.Clusters == nil {
				t.Error("Clusters should be non-nil")
			}
			if len(out.Clusters) != 0 {
				t.Errorf("len = %d, want 0", len(out.Clusters))
			}
			if out.TotalErrors != 0 {
				t.Errorf("TotalErrors = %d, want 0", out.TotalErrors)
			}
		}},
		{name: "file not found returns error", input: ExtractErrorsInput{Path: "/nonexistent/file.log"}, wantErr: true, errContains: "FILE_NOT_FOUND"},
		{name: "levels included always lists error fatal critical", input: ExtractErrorsInput{Path: jsonPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			want := []string{"ERROR", "FATAL", "CRITICAL"}
			if len(out.LevelsIncluded) != 3 {
				t.Fatalf("LevelsIncluded = %v", out.LevelsIncluded)
			}
			for i, w := range want {
				if out.LevelsIncluded[i] != w {
					t.Errorf("[%d] = %q, want %q", i, out.LevelsIncluded[i], w)
				}
			}
		}},
		{name: "fatal and critical levels are captured", input: ExtractErrorsInput{Path: multiLevelPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.TotalErrors < 2 {
				t.Errorf("TotalErrors = %d, want >= 2", out.TotalErrors)
			}
		}},
		{name: "first seen and last seen track correctly", input: ExtractErrorsInput{Path: jsonPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) < 1 {
				t.Fatal("no clusters")
			}
			top := out.Clusters[0]
			if top.FirstSeen.Timestamp == nil {
				t.Fatal("FirstSeen.Timestamp nil")
			}
			if top.LastSeen.Timestamp == nil {
				t.Fatal("LastSeen.Timestamp nil")
			}
			if *top.FirstSeen.Timestamp == *top.LastSeen.Timestamp {
				t.Error("FirstSeen == LastSeen")
			}
			if top.FirstSeen.LineNumber >= top.LastSeen.LineNumber {
				t.Errorf("FirstSeen.Line %d >= LastSeen.Line %d", top.FirstSeen.LineNumber, top.LastSeen.LineNumber)
			}
		}},
		{name: "sample messages capped at 3", input: ExtractErrorsInput{Path: sampleCapPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) < 1 {
				t.Fatal("no clusters")
			}
			if len(out.Clusters[0].SampleMessages) != 3 {
				t.Errorf("len(SampleMessages) = %d, want 3", len(out.Clusters[0].SampleMessages))
			}
		}},
		{name: "errors per hour computed from timestamps", input: ExtractErrorsInput{Path: jsonPath}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if out.ErrorRate.ErrorsPerHour < 2.0 || out.ErrorRate.ErrorsPerHour > 3.0 {
				t.Errorf("ErrorsPerHour = %.2f, want ~2.5", out.ErrorRate.ErrorsPerHour)
			}
		}},
		{name: "max clusters clamped to minimum 1", input: ExtractErrorsInput{Path: sortPath, MaxClusters: -5}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 1 {
				t.Errorf("len = %d, want 1", len(out.Clusters))
			}
		}},
		{name: "normalization replaces UUIDs", input: ExtractErrorsInput{Path: writeTempLog(t, "uuid.log", strings.Join([]string{`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Failed for request 550e8400-e29b-41d4-a716-446655440000"}`, `{"timestamp":"2025-01-15T02:01:00Z","level":"error","msg":"Failed for request a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`}, "\n")+"\n")}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 1 {
				t.Errorf("len = %d, want 1", len(out.Clusters))
			}
			if len(out.Clusters) > 0 && !strings.Contains(out.Clusters[0].Pattern, "<UUID>") {
				t.Errorf("pattern %q missing <UUID>", out.Clusters[0].Pattern)
			}
		}},
		{name: "normalization replaces hex values", input: ExtractErrorsInput{Path: writeTempLog(t, "hex.log", strings.Join([]string{`{"timestamp":"2025-01-15T02:00:00Z","level":"error","msg":"Segfault at 0xDEADBEEF"}`, `{"timestamp":"2025-01-15T02:01:00Z","level":"error","msg":"Segfault at 0xCAFEBABE"}`}, "\n")+"\n")}, checkOutput: func(t *testing.T, out ExtractErrorsOutput) {
			if len(out.Clusters) != 1 {
				t.Errorf("len = %d, want 1", len(out.Clusters))
			}
			if len(out.Clusters) > 0 && !strings.Contains(out.Clusters[0].Pattern, "<HEX>") {
				t.Errorf("pattern %q missing <HEX>", out.Clusters[0].Pattern)
			}
		}},
		{name: "binary file returns error", input: ExtractErrorsInput{Path: writeTempBinary(t)}, wantErr: true, errContains: "BINARY_FILE"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunExtractErrors(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q missing %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkOutput != nil {
				tc.checkOutput(t, out)
			}
		})
	}
}

func TestNormalizeMessage(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"UUID replaced", "Request 550e8400-e29b-41d4-a716-446655440000 failed", "Request <UUID> failed"},
		{"IP address replaced", "Connection to 192.168.1.100 refused", "Connection to <IP> refused"},
		{"hex value replaced", "Segfault at address 0xDEADBEEF", "Segfault at address <HEX>"},
		{"file path replaced", "Cannot read /var/log/app.log", "Cannot read <PATH>"},
		{"quoted string replaced", `Key "session_token" not found`, "Key <STR> not found"},
		{"standalone numbers replaced", "Retry attempt 42 of 100", "Retry attempt <N> of <N>"},
		{"multiple replacements", "Error on 10.0.1.5 request 550e8400-e29b-41d4-a716-446655440000 code 500", "Error on <IP> request <UUID> code <N>"},
		{"empty message unchanged", "", ""},
		{"no variables unchanged", "Simple error message", "Simple error message"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMessage(tc.input)
			if got != tc.want {
				t.Errorf("normalizeMessage(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRunExtractErrorsSortByImpact(t *testing.T) {
	// 2 FATAL errors vs 3 ERROR errors.
	// By count: ERROR cluster wins (3 > 2).
	// By impact: FATAL wins (2×10=20 > 3×5=15).
	// Use IPs so the ERROR messages normalize to the same pattern.
	impactLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"fatal","msg":"Database connection lost"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"fatal","msg":"Database connection lost"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"error","msg":"Request timeout for host 10.0.0.1"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"error","msg":"Request timeout for host 10.0.0.2"}`,
		`{"timestamp":"2025-01-15T02:04:00Z","level":"error","msg":"Request timeout for host 10.0.0.3"}`,
	}, "\n") + "\n"
	impactPath := writeTempLog(t, "impact.log", impactLog)

	t.Run("sort_by count puts higher-count cluster first", func(t *testing.T) {
		out, err := RunExtractErrors(ExtractErrorsInput{Path: impactPath, SortBy: "count"})
		if err != nil {
			t.Fatalf("RunExtractErrors: %v", err)
		}
		if len(out.Clusters) < 2 {
			t.Fatalf("expected at least 2 clusters, got %d", len(out.Clusters))
		}
		if out.Clusters[0].Count != 3 {
			t.Errorf("first cluster count = %d, want 3 (ERROR cluster)", out.Clusters[0].Count)
		}
	})

	t.Run("sort_by impact puts higher-impact cluster first", func(t *testing.T) {
		out, err := RunExtractErrors(ExtractErrorsInput{Path: impactPath, SortBy: "impact"})
		if err != nil {
			t.Fatalf("RunExtractErrors: %v", err)
		}
		if len(out.Clusters) < 2 {
			t.Fatalf("expected at least 2 clusters, got %d", len(out.Clusters))
		}
		// FATAL cluster: 2×10=20, ERROR cluster: 3×5=15
		if out.Clusters[0].ImpactScore != 20 {
			t.Errorf("first cluster impact = %v, want 20 (FATAL cluster)", out.Clusters[0].ImpactScore)
		}
		if out.Clusters[1].ImpactScore != 15 {
			t.Errorf("second cluster impact = %v, want 15 (ERROR cluster)", out.Clusters[1].ImpactScore)
		}
	})

	t.Run("impact_score always populated regardless of sort_by", func(t *testing.T) {
		out, err := RunExtractErrors(ExtractErrorsInput{Path: impactPath, SortBy: "count"})
		if err != nil {
			t.Fatalf("RunExtractErrors: %v", err)
		}
		for _, c := range out.Clusters {
			if c.ImpactScore == 0 {
				t.Errorf("cluster %q has zero ImpactScore", c.Pattern)
			}
		}
	})

	t.Run("invalid sort_by rejected", func(t *testing.T) {
		_, err := RunExtractErrors(ExtractErrorsInput{Path: impactPath, SortBy: "invalid"})
		if err == nil {
			t.Fatal("expected error for invalid sort_by, got nil")
		}
		if !strings.Contains(err.Error(), "VALIDATION_ERROR") {
			t.Errorf("error = %v, want VALIDATION_ERROR", err)
		}
	})
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		level types.LogLevel
		want  float64
	}{
		{types.LogLevelFatal, 10},
		{types.LogLevelCritical, 8},
		{types.LogLevelError, 5},
		{types.LogLevelWarn, 2},
		{types.LogLevelInfo, 1},
	}
	for _, tc := range tests {
		t.Run(string(tc.level), func(t *testing.T) {
			got := severityWeight(tc.level)
			if got != tc.want {
				t.Errorf("severityWeight(%q) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}
