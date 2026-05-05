package metrics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer buffers events and writes them to daily JSONL files.
type Writer struct {
	dir     string
	mu      sync.Mutex
	buf     []Event
	done    chan struct{}
	stopped chan struct{}
	closed  bool
	flushCh chan struct{}
}

const (
	flushInterval = 1 * time.Second
	flushSize     = 100
)

// NewWriter creates a metrics writer that persists events to dir.
func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("metrics: create dir: %w", err)
	}
	w := &Writer{
		dir:     dir,
		buf:     make([]Event, 0, flushSize),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		flushCh: make(chan struct{}, 1),
	}
	go w.run()
	return w, nil
}

// Record adds an event to the buffer. Non-blocking.
func (w *Writer) Record(e Event) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.buf = append(w.buf, e)
	shouldFlush := len(w.buf) >= flushSize
	w.mu.Unlock()

	if shouldFlush {
		select {
		case w.flushCh <- struct{}{}:
		default:
		}
	}
}

// Close flushes remaining events and stops the background goroutine.
func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.done)
	<-w.stopped // wait for goroutine to exit
	return w.flush()
}

func (w *Writer) run() {
	defer close(w.stopped)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = w.flush()
		case <-w.flushCh:
			_ = w.flush()
		case <-w.done:
			return
		}
	}
}

func (w *Writer) flush() error {
	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return nil
	}
	events := w.buf
	w.buf = make([]Event, 0, flushSize)
	w.mu.Unlock()

	filename := fmt.Sprintf("events-%s.jsonl", time.Now().Format("2006-01-02"))
	path := filepath.Join(w.dir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("metrics: open %s: %w", path, err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	for _, e := range events {
		data, err := MarshalEvent(e)
		if err != nil {
			continue
		}
		bw.Write(data)
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// EventCount returns the number of buffered (unflushed) events. For testing.
func (w *Writer) EventCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buf)
}
