package fileutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcessFiles(t *testing.T) {
	t.Run("basic parallel processing", func(t *testing.T) {
		dir := t.TempDir()
		paths := make([]string, 3)
		for i := 0; i < 3; i++ {
			p := filepath.Join(dir, fmt.Sprintf("file%d.log", i))
			os.WriteFile(p, []byte(fmt.Sprintf("content-%d", i)), 0644)
			paths[i] = p
		}

		results, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("got %d results, want 3", len(results))
		}
		for i, r := range results {
			want := fmt.Sprintf("content-%d", i)
			if r != want {
				t.Errorf("results[%d] = %q, want %q", i, r, want)
			}
		}
	})

	t.Run("preserves input order", func(t *testing.T) {
		paths := []string{"a", "b", "c", "d", "e"}

		results, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			// Reverse the timing so later paths complete first.
			delay := time.Duration(len(paths)-indexOf(paths, path)) * time.Millisecond
			time.Sleep(delay)
			return "result-" + path, nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		for i, p := range paths {
			want := "result-" + p
			if results[i] != want {
				t.Errorf("results[%d] = %q, want %q", i, results[i], want)
			}
		}
	})

	t.Run("empty paths list", func(t *testing.T) {
		results, err := ProcessFiles(context.Background(), []string{}, func(ctx context.Context, path string) (string, error) {
			t.Error("process should not be called for empty paths")
			return "", nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("single path", func(t *testing.T) {
		results, err := ProcessFiles(context.Background(), []string{"only"}, func(ctx context.Context, path string) (string, error) {
			return "processed-" + path, nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		if len(results) != 1 || results[0] != "processed-only" {
			t.Errorf("results = %v, want [processed-only]", results)
		}
	})

	t.Run("first error cancels others", func(t *testing.T) {
		errExpected := errors.New("deliberate failure")

		paths := []string{"ok1", "fail", "ok2", "ok3", "ok4"}

		_, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			if path == "fail" {
				return "", errExpected
			}
			// Other workers wait, checking for cancellation.
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return "done", nil
			}
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "deliberate failure") {
			t.Errorf("error = %q, want it to contain 'deliberate failure'", err.Error())
		}
	})

	t.Run("context already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately.

		_, err := ProcessFiles(ctx, []string{"a", "b"}, func(ctx context.Context, path string) (string, error) {
			t.Error("process should not be called when context is already cancelled")
			return "", nil
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cancel") {
			t.Errorf("error = %q, want context cancellation message", err.Error())
		}
	})

	t.Run("all files fail returns first error", func(t *testing.T) {
		paths := []string{"fail1", "fail2", "fail3"}

		_, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			return "", fmt.Errorf("error on %s", path)
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Confirm we got one error; which one is non-deterministic.
		if !strings.Contains(err.Error(), "error on") {
			t.Errorf("error = %q, want it to contain 'error on'", err.Error())
		}
	})

	t.Run("no goroutine leaks on error", func(t *testing.T) {
		var running atomic.Int32

		paths := make([]string, 20)
		for i := range paths {
			paths[i] = fmt.Sprintf("path-%d", i)
		}

		ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			running.Add(1)
			defer running.Add(-1)

			if path == "path-0" {
				return "", errors.New("fail early")
			}

			// Block until context is cancelled.
			<-ctx.Done()
			return "", ctx.Err()
		})

		// Give goroutines a moment to clean up.
		time.Sleep(50 * time.Millisecond)

		if r := running.Load(); r != 0 {
			t.Errorf("goroutines still running = %d, want 0", r)
		}
	})

	t.Run("concurrent execution actually happens", func(t *testing.T) {
		var concurrent atomic.Int32
		var maxConcurrent atomic.Int32

		paths := make([]string, 5)
		for i := range paths {
			paths[i] = fmt.Sprintf("path-%d", i)
		}

		results, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (int32, error) {
			c := concurrent.Add(1)
			// Track peak concurrency via CAS loop.
			for {
				old := maxConcurrent.Load()
				if c <= old || maxConcurrent.CompareAndSwap(old, c) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond) // Hold for overlap.
			concurrent.Add(-1)
			return c, nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		if len(results) != 5 {
			t.Fatalf("got %d results, want 5", len(results))
		}
		if maxConcurrent.Load() < 2 {
			t.Errorf("maxConcurrent = %d, expected at least 2 for parallel execution", maxConcurrent.Load())
		}
	})

	t.Run("struct result type", func(t *testing.T) {
		type FileResult struct {
			Path  string
			Lines int
		}

		dir := t.TempDir()
		paths := make([]string, 3)
		for i := 0; i < 3; i++ {
			p := filepath.Join(dir, fmt.Sprintf("file%d.log", i))
			content := strings.Repeat("line\n", i+1)
			os.WriteFile(p, []byte(content), 0644)
			paths[i] = p
		}

		results, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (FileResult, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return FileResult{}, err
			}
			lines := strings.Count(string(data), "\n")
			return FileResult{Path: path, Lines: lines}, nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		for i, r := range results {
			if r.Path != paths[i] {
				t.Errorf("results[%d].Path = %q, want %q", i, r.Path, paths[i])
			}
			wantLines := i + 1
			if r.Lines != wantLines {
				t.Errorf("results[%d].Lines = %d, want %d", i, r.Lines, wantLines)
			}
		}
	})

	t.Run("result order independent of completion order", func(t *testing.T) {
		delays := map[string]time.Duration{
			"slow":   50 * time.Millisecond,
			"medium": 25 * time.Millisecond,
			"fast":   1 * time.Millisecond,
		}
		paths := []string{"slow", "medium", "fast"}

		var mu sync.Mutex
		var completionOrder []string

		results, err := ProcessFiles(context.Background(), paths, func(ctx context.Context, path string) (string, error) {
			time.Sleep(delays[path])
			mu.Lock()
			completionOrder = append(completionOrder, path)
			mu.Unlock()
			return "result-" + path, nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}

		t.Logf("completion order: %v", completionOrder)

		// Result order must match input order regardless of completion order.
		for i, p := range paths {
			want := "result-" + p
			if results[i] != want {
				t.Errorf("results[%d] = %q, want %q (order not preserved!)", i, results[i], want)
			}
		}
	})

	t.Run("process receives correct context", func(t *testing.T) {
		type ctxKey string
		parentCtx := context.WithValue(context.Background(), ctxKey("test"), "value")

		results, err := ProcessFiles(parentCtx, []string{"a"}, func(ctx context.Context, path string) (string, error) {
			val, ok := ctx.Value(ctxKey("test")).(string)
			if !ok || val != "value" {
				return "", errors.New("context value not propagated")
			}
			return "ok", nil
		})
		if err != nil {
			t.Fatalf("ProcessFiles() error = %v", err)
		}
		if results[0] != "ok" {
			t.Errorf("results[0] = %q, want %q", results[0], "ok")
		}
	})
}

// indexOf returns the index of s in slice, or -1 if not found.
func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
