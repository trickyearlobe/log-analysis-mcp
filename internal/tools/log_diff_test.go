package tools

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGzipTempLog creates a gzip-compressed temp log file and returns its path.
func writeGzipTempLog(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file %s: %v", path, err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("write gzip data %s: %v", path, err)
	}
	if err := gz.Close(); err != nil {
		f.Close()
		t.Fatalf("close gzip writer %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file %s: %v", path, err)
	}
	return path
}

func TestRunDiffLogs(t *testing.T) {
	// --- Test data: file vs file - different errors ---
	// Base has errors A ("Connection timeout") and B ("Disk full").
	// Target has errors B ("Disk full") and C ("Authentication failed").
	baseErrorsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"ERROR","source":"db","message":"Connection timeout to 192.168.1.100"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"ERROR","source":"db","message":"Connection timeout to 192.168.1.101"}`,
		`{"timestamp":"2025-01-15T10:00:04Z","level":"ERROR","source":"storage","message":"Disk full on /mnt/data"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"INFO","source":"web","message":"Request completed"}`,
	}, "\n") + "\n"
	baseDiffPath := writeTempLog(t, "base_diff.log", baseErrorsLog)

	targetErrorsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T11:00:01Z","level":"ERROR","source":"storage","message":"Disk full on /mnt/backup"}`,
		`{"timestamp":"2025-01-15T11:00:02Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T11:00:03Z","level":"ERROR","source":"auth","message":"Authentication failed for user admin"}`,
		`{"timestamp":"2025-01-15T11:00:04Z","level":"ERROR","source":"auth","message":"Authentication failed for user deploy"}`,
		`{"timestamp":"2025-01-15T11:00:05Z","level":"INFO","source":"web","message":"Request completed"}`,
	}, "\n") + "\n"
	targetDiffPath := writeTempLog(t, "target_diff.log", targetErrorsLog)

	// --- Test data: identical file ---
	identicalLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"ERROR","source":"db","message":"Connection timeout to 192.168.1.50"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"WARN","source":"web","message":"Slow response detected"}`,
	}, "\n") + "\n"
	identicalPath := writeTempLog(t, "identical.log", identicalLog)

	// --- Test data: single file with two time periods ---
	// Period 1 (10:00-10:59): mostly INFO with one ERROR.
	// Period 2 (11:00-11:59): many ERRORs.
	var singleFileLines []string
	// Period 1: 10 INFO + 1 ERROR
	for i := 0; i < 10; i++ {
		singleFileLines = append(singleFileLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T10:%02d:00Z","level":"INFO","source":"web","message":"Healthy request %d"}`, i, i))
	}
	singleFileLines = append(singleFileLines,
		`{"timestamp":"2025-01-15T10:10:00Z","level":"ERROR","source":"db","message":"Connection timeout to 10.0.0.1"}`)
	// Period 2: 3 INFO + 8 ERROR
	for i := 0; i < 3; i++ {
		singleFileLines = append(singleFileLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T11:%02d:00Z","level":"INFO","source":"web","message":"Healthy request %d"}`, i, i))
	}
	for i := 3; i < 11; i++ {
		singleFileLines = append(singleFileLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T11:%02d:00Z","level":"ERROR","source":"db","message":"Connection timeout to 10.0.0.%d"}`, i, i))
	}
	singleFilePath := writeTempLog(t, "single_file_ranges.log", strings.Join(singleFileLines, "\n")+"\n")

	// --- Test data: rate change detection ---
	// Base: 5 lines over 5 minutes (~1 line/min).
	var rateBaseLines []string
	for i := 0; i < 5; i++ {
		rateBaseLines = append(rateBaseLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T10:%02d:00Z","level":"INFO","source":"web","message":"Request %d"}`, i, i))
	}
	rateBasePath := writeTempLog(t, "rate_base.log", strings.Join(rateBaseLines, "\n")+"\n")

	// Target: 50 lines over 5 minutes (~10 lines/min).
	var rateTargetLines []string
	for i := 0; i < 50; i++ {
		min := i / 10
		sec := (i % 10) * 6
		rateTargetLines = append(rateTargetLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T11:%02d:%02dZ","level":"INFO","source":"web","message":"Request %d"}`, min, sec, i))
	}
	rateTargetPath := writeTempLog(t, "rate_target.log", strings.Join(rateTargetLines, "\n")+"\n")

	// --- Test data: source changes ---
	// Base has sources: web, db.
	sourceBaseLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"INFO","source":"web","message":"Request completed"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"INFO","source":"db","message":"Query executed"}`,
		`{"timestamp":"2025-01-15T10:00:04Z","level":"INFO","source":"web","message":"Another request"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"INFO","source":"db","message":"Connection pooled"}`,
	}, "\n") + "\n"
	sourceBasePath := writeTempLog(t, "source_base.log", sourceBaseLog)

	// Target has sources: web, auth.
	sourceTargetLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T11:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T11:00:02Z","level":"INFO","source":"auth","message":"Login attempt"}`,
		`{"timestamp":"2025-01-15T11:00:03Z","level":"INFO","source":"auth","message":"Token validated"}`,
		`{"timestamp":"2025-01-15T11:00:04Z","level":"INFO","source":"web","message":"Request completed"}`,
		`{"timestamp":"2025-01-15T11:00:05Z","level":"INFO","source":"auth","message":"Session created"}`,
	}, "\n") + "\n"
	sourceTargetPath := writeTempLog(t, "source_target.log", sourceTargetLog)

	// --- Test data: level distribution shift ---
	// Base: 90% INFO, 10% ERROR.
	var levelBaseLines []string
	for i := 0; i < 9; i++ {
		levelBaseLines = append(levelBaseLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T10:00:%02dZ","level":"INFO","source":"app","message":"Normal operation %d"}`, i, i))
	}
	levelBaseLines = append(levelBaseLines,
		`{"timestamp":"2025-01-15T10:00:09Z","level":"ERROR","source":"app","message":"Something failed once"}`)
	levelBasePath := writeTempLog(t, "level_base.log", strings.Join(levelBaseLines, "\n")+"\n")

	// Target: 20% INFO, 80% ERROR.
	var levelTargetLines []string
	for i := 0; i < 2; i++ {
		levelTargetLines = append(levelTargetLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T11:00:%02dZ","level":"INFO","source":"app","message":"Normal operation %d"}`, i, i))
	}
	for i := 2; i < 10; i++ {
		levelTargetLines = append(levelTargetLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T11:00:%02dZ","level":"ERROR","source":"app","message":"Critical failure %d"}`, i, i))
	}
	levelTargetPath := writeTempLog(t, "level_target.log", strings.Join(levelTargetLines, "\n")+"\n")

	// --- Test data: empty target (all resolved) ---
	baseWithErrorsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"ERROR","source":"db","message":"Connection timeout to 10.0.0.1"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"ERROR","source":"auth","message":"Authentication failed for user root"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"INFO","source":"web","message":"Request received"}`,
	}, "\n") + "\n"
	baseWithErrorsPath := writeTempLog(t, "base_with_errors.log", baseWithErrorsLog)

	targetNoErrorsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T11:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T11:00:02Z","level":"INFO","source":"web","message":"Request completed"}`,
		`{"timestamp":"2025-01-15T11:00:03Z","level":"INFO","source":"db","message":"Query executed"}`,
	}, "\n") + "\n"
	targetNoErrorsPath := writeTempLog(t, "target_no_errors.log", targetNoErrorsLog)

	// --- Test data: compressed file ---
	compressedContent := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"ERROR","source":"db","message":"Connection timeout to 10.0.0.5"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"INFO","source":"web","message":"Request completed"}`,
	}, "\n") + "\n"
	compressedBasePath := writeGzipTempLog(t, "compressed_base.log", compressedContent)

	compressedTargetContent := strings.Join([]string{
		`{"timestamp":"2025-01-15T11:00:01Z","level":"INFO","source":"web","message":"Request received"}`,
		`{"timestamp":"2025-01-15T11:00:02Z","level":"ERROR","source":"db","message":"Connection timeout to 10.0.0.6"}`,
		`{"timestamp":"2025-01-15T11:00:03Z","level":"ERROR","source":"auth","message":"Authentication failed for user admin"}`,
		`{"timestamp":"2025-01-15T11:00:04Z","level":"INFO","source":"web","message":"Request completed"}`,
	}, "\n") + "\n"
	compressedTargetPath := writeGzipTempLog(t, "compressed_target.log", compressedTargetContent)

	tests := []struct {
		name        string
		input       DiffLogsInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out DiffLogsOutput)
	}{
		{
			name: "file vs file - different errors",
			input: DiffLogsInput{
				BasePath:   baseDiffPath,
				TargetPath: targetDiffPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// Error A ("Connection timeout to <IP>") is in base only → resolved.
				foundResolvedTimeout := false
				for _, e := range out.ResolvedErrors {
					if strings.Contains(e.Pattern, "Connection timeout") {
						foundResolvedTimeout = true
						if e.BaseCount == 0 {
							t.Error("resolved Connection timeout should have BaseCount > 0")
						}
						if e.TargetCount != 0 {
							t.Errorf("resolved Connection timeout TargetCount = %d, want 0", e.TargetCount)
						}
						break
					}
				}
				if !foundResolvedTimeout {
					t.Error("expected 'Connection timeout' in ResolvedErrors")
				}

				// Error C ("Authentication failed") is in target only → new.
				foundNewAuth := false
				for _, e := range out.NewErrors {
					if strings.Contains(e.Pattern, "Authentication failed") {
						foundNewAuth = true
						if e.TargetCount == 0 {
							t.Error("new Authentication failed should have TargetCount > 0")
						}
						if e.BaseCount != 0 {
							t.Errorf("new Authentication failed BaseCount = %d, want 0", e.BaseCount)
						}
						break
					}
				}
				if !foundNewAuth {
					t.Error("expected 'Authentication failed' in NewErrors")
				}

				// Error B ("Disk full") is in both → changed.
				foundChangedDisk := false
				for _, e := range out.ChangedErrors {
					if strings.Contains(e.Pattern, "Disk full") {
						foundChangedDisk = true
						if e.BaseCount == 0 || e.TargetCount == 0 {
							t.Errorf("changed Disk full: BaseCount=%d, TargetCount=%d — both should be > 0",
								e.BaseCount, e.TargetCount)
						}
						break
					}
				}
				if !foundChangedDisk {
					t.Error("expected 'Disk full' in ChangedErrors")
				}
			},
		},
		{
			name: "file vs file - identical files",
			input: DiffLogsInput{
				BasePath:   identicalPath,
				TargetPath: identicalPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				if len(out.NewErrors) != 0 {
					t.Errorf("NewErrors len = %d, want 0 for identical files", len(out.NewErrors))
				}
				if len(out.ResolvedErrors) != 0 {
					t.Errorf("ResolvedErrors len = %d, want 0 for identical files", len(out.ResolvedErrors))
				}
				// All changed errors should have Change == 0.
				for i, e := range out.ChangedErrors {
					if e.Change != 0 {
						t.Errorf("ChangedErrors[%d].Change = %d, want 0 for identical files", i, e.Change)
					}
				}
				// No new or disappeared sources.
				if len(out.NewSources) != 0 {
					t.Errorf("NewSources len = %d, want 0 for identical files", len(out.NewSources))
				}
				if len(out.DisappearedSources) != 0 {
					t.Errorf("DisappearedSources len = %d, want 0 for identical files", len(out.DisappearedSources))
				}
			},
		},
		{
			name: "single file time ranges",
			input: DiffLogsInput{
				BasePath:     singleFilePath,
				BaseAfter:    "2025-01-15T10:00:00Z",
				BaseBefore:   "2025-01-15T10:59:59Z",
				TargetAfter:  "2025-01-15T11:00:00Z",
				TargetBefore: "2025-01-15T11:59:59Z",
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// Period 1 has 1 error; period 2 has 8 errors.
				if out.BaseSummary.ErrorCount >= out.TargetSummary.ErrorCount {
					t.Errorf("base ErrorCount (%d) should be less than target ErrorCount (%d)",
						out.BaseSummary.ErrorCount, out.TargetSummary.ErrorCount)
				}
				if out.TargetSummary.ErrorCount < 5 {
					t.Errorf("target ErrorCount = %d, expected at least 5", out.TargetSummary.ErrorCount)
				}
				if out.BaseSummary.ErrorCount > 3 {
					t.Errorf("base ErrorCount = %d, expected at most 3", out.BaseSummary.ErrorCount)
				}
			},
		},
		{
			name: "rate change detection",
			input: DiffLogsInput{
				BasePath:   rateBasePath,
				TargetPath: rateTargetPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// Base ~1 line/min, target ~10 lines/min.
				if out.BaseSummary.LinesPerMinute <= 0 {
					t.Errorf("base LinesPerMinute = %f, want > 0", out.BaseSummary.LinesPerMinute)
				}
				if out.TargetSummary.LinesPerMinute <= 0 {
					t.Errorf("target LinesPerMinute = %f, want > 0", out.TargetSummary.LinesPerMinute)
				}
				if out.TargetSummary.LinesPerMinute <= out.BaseSummary.LinesPerMinute {
					t.Errorf("target LinesPerMinute (%f) should be significantly greater than base (%f)",
						out.TargetSummary.LinesPerMinute, out.BaseSummary.LinesPerMinute)
				}
				// Expect at least 3x difference.
				ratio := out.TargetSummary.LinesPerMinute / out.BaseSummary.LinesPerMinute
				if ratio < 3.0 {
					t.Errorf("target/base lines per minute ratio = %f, expected >= 3.0", ratio)
				}
				// Summary line counts should reflect the data.
				if out.BaseSummary.TotalLines != 5 {
					t.Errorf("base TotalLines = %d, want 5", out.BaseSummary.TotalLines)
				}
				if out.TargetSummary.TotalLines != 50 {
					t.Errorf("target TotalLines = %d, want 50", out.TargetSummary.TotalLines)
				}
			},
		},
		{
			name: "source changes",
			input: DiffLogsInput{
				BasePath:   sourceBasePath,
				TargetPath: sourceTargetPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// "auth" is new (only in target).
				foundNewAuth := false
				for _, s := range out.NewSources {
					if s.Source == "auth" {
						foundNewAuth = true
						if s.BaseCount != 0 {
							t.Errorf("new source 'auth' BaseCount = %d, want 0", s.BaseCount)
						}
						if s.TargetCount == 0 {
							t.Error("new source 'auth' TargetCount should be > 0")
						}
						break
					}
				}
				if !foundNewAuth {
					t.Error("expected 'auth' in NewSources")
				}

				// "db" disappeared (only in base).
				foundDisappearedDB := false
				for _, s := range out.DisappearedSources {
					if s.Source == "db" {
						foundDisappearedDB = true
						if s.TargetCount != 0 {
							t.Errorf("disappeared source 'db' TargetCount = %d, want 0", s.TargetCount)
						}
						if s.BaseCount == 0 {
							t.Error("disappeared source 'db' BaseCount should be > 0")
						}
						break
					}
				}
				if !foundDisappearedDB {
					t.Error("expected 'db' in DisappearedSources")
				}

				// "web" is in both → changed.
				foundChangedWeb := false
				for _, s := range out.ChangedSources {
					if s.Source == "web" {
						foundChangedWeb = true
						if s.BaseCount == 0 || s.TargetCount == 0 {
							t.Errorf("changed source 'web': BaseCount=%d, TargetCount=%d — both should be > 0",
								s.BaseCount, s.TargetCount)
						}
						break
					}
				}
				if !foundChangedWeb {
					t.Error("expected 'web' in ChangedSources")
				}
			},
		},
		{
			name: "level distribution shift",
			input: DiffLogsInput{
				BasePath:   levelBasePath,
				TargetPath: levelTargetPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				var baseInfoPct, baseErrorPct float64
				var targetInfoPct, targetErrorPct float64
				for _, ld := range out.LevelChanges {
					switch strings.ToUpper(ld.Level) {
					case "INFO":
						baseInfoPct = ld.BasePercentage
						targetInfoPct = ld.TargetPercentage
					case "ERROR":
						baseErrorPct = ld.BasePercentage
						targetErrorPct = ld.TargetPercentage
					}
				}
				// Base: ~90% INFO.
				if baseInfoPct < 80.0 {
					t.Errorf("base INFO percentage = %f, want >= 80.0", baseInfoPct)
				}
				// Target: ~80% ERROR.
				if targetErrorPct < 70.0 {
					t.Errorf("target ERROR percentage = %f, want >= 70.0", targetErrorPct)
				}
				// The shift: INFO decreased, ERROR increased.
				if targetInfoPct >= baseInfoPct {
					t.Errorf("expected target INFO pct (%f) < base INFO pct (%f)", targetInfoPct, baseInfoPct)
				}
				if targetErrorPct <= baseErrorPct {
					t.Errorf("expected target ERROR pct (%f) > base ERROR pct (%f)", targetErrorPct, baseErrorPct)
				}
			},
		},
		{
			name: "empty target - all resolved",
			input: DiffLogsInput{
				BasePath:   baseWithErrorsPath,
				TargetPath: targetNoErrorsPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// All base errors should appear in ResolvedErrors.
				if len(out.ResolvedErrors) == 0 {
					t.Error("expected non-empty ResolvedErrors when target has no errors")
				}
				// Verify the base error patterns show up as resolved.
				foundTimeout := false
				foundAuthFailed := false
				for _, e := range out.ResolvedErrors {
					if strings.Contains(e.Pattern, "Connection timeout") {
						foundTimeout = true
					}
					if strings.Contains(e.Pattern, "Authentication failed") {
						foundAuthFailed = true
					}
				}
				if !foundTimeout {
					t.Error("expected 'Connection timeout' in ResolvedErrors")
				}
				if !foundAuthFailed {
					t.Error("expected 'Authentication failed' in ResolvedErrors")
				}
				// No new errors.
				if len(out.NewErrors) != 0 {
					t.Errorf("NewErrors len = %d, want 0 when target has no errors", len(out.NewErrors))
				}
				// Target summary should have zero error count.
				if out.TargetSummary.ErrorCount != 0 {
					t.Errorf("target ErrorCount = %d, want 0", out.TargetSummary.ErrorCount)
				}
			},
		},
		{
			name: "compressed file support",
			input: DiffLogsInput{
				BasePath:   compressedBasePath,
				TargetPath: compressedTargetPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// Compressed base has 3 lines, target has 4.
				if out.BaseSummary.TotalLines < 1 {
					t.Errorf("base TotalLines = %d, want > 0 (compressed file should be readable)", out.BaseSummary.TotalLines)
				}
				if out.TargetSummary.TotalLines < 1 {
					t.Errorf("target TotalLines = %d, want > 0 (compressed file should be readable)", out.TargetSummary.TotalLines)
				}
				// Target should have more errors (2 vs 1).
				if out.TargetSummary.ErrorCount <= out.BaseSummary.ErrorCount {
					t.Errorf("target ErrorCount (%d) should be > base ErrorCount (%d)",
						out.TargetSummary.ErrorCount, out.BaseSummary.ErrorCount)
				}
			},
		},
		{
			name: "error - missing file",
			input: DiffLogsInput{
				BasePath:   "/nonexistent/path/to/missing_base.log",
				TargetPath: targetDiffPath,
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "error - missing time ranges in single-file mode",
			input: DiffLogsInput{
				BasePath:  singleFilePath,
				BaseAfter: "2025-01-15T10:00:00Z",
				// Missing: BaseBefore, TargetAfter, TargetBefore.
			},
			wantErr:     true,
			errContains: "single-file mode requires",
		},
		{
			name: "error - overlapping time ranges",
			input: DiffLogsInput{
				BasePath:     singleFilePath,
				BaseAfter:    "2025-01-15T10:00:00Z",
				BaseBefore:   "2025-01-15T11:30:00Z",
				TargetAfter:  "2025-01-15T11:00:00Z",
				TargetBefore: "2025-01-15T12:00:00Z",
			},
			wantErr:     true,
			errContains: "must not overlap",
		},
		{
			name: "error - invalid timestamp",
			input: DiffLogsInput{
				BasePath:     singleFilePath,
				BaseAfter:    "not-a-timestamp",
				BaseBefore:   "2025-01-15T10:59:59Z",
				TargetAfter:  "2025-01-15T11:00:00Z",
				TargetBefore: "2025-01-15T11:59:59Z",
			},
			wantErr:     true,
			errContains: "invalid timestamp",
		},
		{
			name: "nil slices check",
			input: DiffLogsInput{
				BasePath:   identicalPath,
				TargetPath: identicalPath,
			},
			checkOutput: func(t *testing.T, out DiffLogsOutput) {
				// All slice fields must be non-nil empty slices, never nil.
				if out.NewErrors == nil {
					t.Error("NewErrors should be non-nil empty slice, got nil")
				}
				if out.ResolvedErrors == nil {
					t.Error("ResolvedErrors should be non-nil empty slice, got nil")
				}
				if out.ChangedErrors == nil {
					t.Error("ChangedErrors should be non-nil empty slice, got nil")
				}
				if out.LevelChanges == nil {
					t.Error("LevelChanges should be non-nil empty slice, got nil")
				}
				if out.NewSources == nil {
					t.Error("NewSources should be non-nil empty slice, got nil")
				}
				if out.DisappearedSources == nil {
					t.Error("DisappearedSources should be non-nil empty slice, got nil")
				}
				if out.ChangedSources == nil {
					t.Error("ChangedSources should be non-nil empty slice, got nil")
				}
				// Also verify zero-length for identical files where appropriate.
				if len(out.NewErrors) != 0 {
					t.Errorf("NewErrors len = %d, want 0", len(out.NewErrors))
				}
				if len(out.ResolvedErrors) != 0 {
					t.Errorf("ResolvedErrors len = %d, want 0", len(out.ResolvedErrors))
				}
				if len(out.NewSources) != 0 {
					t.Errorf("NewSources len = %d, want 0", len(out.NewSources))
				}
				if len(out.DisappearedSources) != 0 {
					t.Errorf("DisappearedSources len = %d, want 0", len(out.DisappearedSources))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunDiffLogs(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
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
