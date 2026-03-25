package fileutil

import (
	"context"
	"fmt"
	"sync"
)

// ProcessFiles runs a processing function concurrently across multiple file paths.
// Results are returned in the same order as the input paths list, regardless of
// completion order. If any processor returns an error, the context is cancelled
// and the first error is returned. No goroutines leak — all spawned tasks
// terminate before this function returns.
func ProcessFiles[T any](ctx context.Context, paths []string, process func(ctx context.Context, path string) (T, error)) ([]T, error) {
	if len(paths) == 0 {
		return []T{}, nil
	}

	// Check if context is already cancelled before spawning work.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before processing: %w", err)
	}

	// Cancellable child context so we can stop all workers on first error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]T, len(paths))
	errs := make([]error, len(paths))

	var wg sync.WaitGroup
	wg.Add(len(paths))

	// errOnce captures only the first error encountered.
	var (
		firstErr  error
		errOnce   sync.Once
	)

	for i, p := range paths {
		go func(idx int, path string) {
			defer wg.Done()

			// Bail early if context is already done.
			if ctx.Err() != nil {
				errs[idx] = ctx.Err()
				return
			}

			result, err := process(ctx, path)
			if err != nil {
				errs[idx] = err
				errOnce.Do(func() {
					firstErr = fmt.Errorf("processing %s: %w", path, err)
					cancel() // signal other workers to stop
				})
				return
			}
			results[idx] = result
		}(i, p)
	}

	wg.Wait()

	if firstErr != nil {
		return results, firstErr
	}
	return results, nil
}
