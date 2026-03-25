package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// stubParser is a minimal parser for testing the multiline combiner.
// It recognizes lines starting with "LOG:" as new entries.
type stubParser struct{}

func (s *stubParser) Parse(line string) *types.ParsedLogEntry {
	if len(line) < 4 || line[:4] != "LOG:" {
		return nil
	}
	msg := line[4:]
	if len(msg) > 0 && msg[0] == ' ' {
		msg = msg[1:]
	}
	return &types.ParsedLogEntry{
		Message: msg,
		Raw:     line,
	}
}

func (s *stubParser) Detect(lines []string) float64 { return 0 }
func (s *stubParser) Name() string                   { return "stub" }

func TestMultilineCombine(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		startLine    int
		wantCount    int
		wantEntries  []struct {
			lineNumber int
			lineCount  int
			message    string
			stackTrace string
		}
	}{
		{
			name:      "empty input",
			lines:     []string{},
			startLine: 1,
			wantCount: 0,
		},
		{
			name: "single parsed line",
			lines: []string{
				"LOG: server started",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "server started", stackTrace: ""},
			},
		},
		{
			name: "multiple parsed lines no continuation",
			lines: []string{
				"LOG: first",
				"LOG: second",
				"LOG: third",
			},
			startLine: 1,
			wantCount: 3,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "first"},
				{lineNumber: 2, lineCount: 1, message: "second"},
				{lineNumber: 3, lineCount: 1, message: "third"},
			},
		},
		{
			name: "java stack trace",
			lines: []string{
				"LOG: NullPointerException",
				"\tat com.example.Handler.process(Handler.java:42)",
				"\tat com.example.Server.handle(Server.java:118)",
			},
			startLine: 10,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 10,
					lineCount:  3,
					message:    "NullPointerException",
					stackTrace: "\tat com.example.Handler.process(Handler.java:42)\n\tat com.example.Server.handle(Server.java:118)",
				},
			},
		},
		{
			name: "java stack trace with caused by",
			lines: []string{
				"LOG: Unhandled exception",
				"\tat com.example.Handler.process(Handler.java:42)",
				"Caused by: java.lang.IllegalStateException",
				"\tat com.example.Dao.find(Dao.java:67)",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  4,
					message:    "Unhandled exception",
					stackTrace: "\tat com.example.Handler.process(Handler.java:42)\nCaused by: java.lang.IllegalStateException\n\tat com.example.Dao.find(Dao.java:67)",
				},
			},
		},
		{
			name: "python traceback",
			lines: []string{
				"LOG: error in handler",
				"Traceback (most recent call last):",
				"  File \"app.py\", line 10, in main",
				"    result = process(data)",
				"  File \"app.py\", line 20, in process",
				"    return data['key']",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  6,
					message:    "error in handler",
					stackTrace: "Traceback (most recent call last):\n  File \"app.py\", line 10, in main\n    result = process(data)\n  File \"app.py\", line 20, in process\n    return data['key']",
				},
			},
		},
		{
			name: "dotnet stack trace",
			lines: []string{
				"LOG: System.NullReferenceException",
				"   at MyApp.Controllers.HomeController.Index()",
				"   at Microsoft.AspNetCore.Mvc.Internal.ActionMethodExecutor.Execute()",
			},
			startLine: 5,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 5,
					lineCount:  3,
					message:    "System.NullReferenceException",
					stackTrace: "   at MyApp.Controllers.HomeController.Index()\n   at Microsoft.AspNetCore.Mvc.Internal.ActionMethodExecutor.Execute()",
				},
			},
		},
		{
			name: "stack trace between two log entries",
			lines: []string{
				"LOG: first error",
				"\tat com.example.A.method(A.java:1)",
				"\tat com.example.B.method(B.java:2)",
				"LOG: second message",
			},
			startLine: 1,
			wantCount: 2,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  3,
					message:    "first error",
					stackTrace: "\tat com.example.A.method(A.java:1)\n\tat com.example.B.method(B.java:2)",
				},
				{
					lineNumber: 4,
					lineCount:  1,
					message:    "second message",
					stackTrace: "",
				},
			},
		},
		{
			name: "continuation lines with leading whitespace",
			lines: []string{
				"LOG: multiline message",
				"  continued on next line",
				"  and another continuation",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  3,
					message:    "multiline message",
					stackTrace: "  continued on next line\n  and another continuation",
				},
			},
		},
		{
			name: "unparseable lines without preceding entry become raw entries",
			lines: []string{
				"random garbage line",
				"another garbage line",
			},
			startLine: 1,
			wantCount: 2,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "random garbage line"},
				{lineNumber: 2, lineCount: 1, message: "another garbage line"},
			},
		},
		{
			name: "unparseable line before first entry is raw",
			lines: []string{
				"preamble text",
				"LOG: actual entry",
				"\tat stack.Frame(Frame.java:1)",
			},
			startLine: 1,
			wantCount: 2,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "preamble text"},
				{
					lineNumber: 2,
					lineCount:  2,
					message:    "actual entry",
					stackTrace: "\tat stack.Frame(Frame.java:1)",
				},
			},
		},
		{
			name: "startLine offset is respected",
			lines: []string{
				"LOG: entry at offset",
			},
			startLine: 500,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 500, lineCount: 1, message: "entry at offset"},
			},
		},
		{
			name: "mixed entries with and without continuations",
			lines: []string{
				"LOG: clean entry",
				"LOG: error entry",
				"\tat com.example.Foo.bar(Foo.java:10)",
				"Caused by: java.io.IOException",
				"\tat com.example.IO.read(IO.java:5)",
				"LOG: another clean entry",
				"LOG: final with continuation",
				"  wrapped text here",
			},
			startLine: 1,
			wantCount: 4,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "clean entry"},
				{
					lineNumber: 2,
					lineCount:  4,
					message:    "error entry",
					stackTrace: "\tat com.example.Foo.bar(Foo.java:10)\nCaused by: java.io.IOException\n\tat com.example.IO.read(IO.java:5)",
				},
				{lineNumber: 6, lineCount: 1, message: "another clean entry"},
				{
					lineNumber: 7,
					lineCount:  2,
					message:    "final with continuation",
					stackTrace: "  wrapped text here",
				},
			},
		},
		{
			name: "wrapped non-parseable non-continuation line after entry",
			lines: []string{
				"LOG: error occurred",
				"Details: something went wrong with the connection",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  2,
					message:    "error occurred",
					stackTrace: "Details: something went wrong with the connection",
				},
			},
		},
		{
			name: "no stack trace means empty string",
			lines: []string{
				"LOG: simple entry",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{lineNumber: 1, lineCount: 1, message: "simple entry", stackTrace: ""},
			},
		},
		{
			name: "many continuation lines",
			lines: []string{
				"LOG: deep stack",
				"\tat a.A.a(A.java:1)",
				"\tat b.B.b(B.java:2)",
				"\tat c.C.c(C.java:3)",
				"\tat d.D.d(D.java:4)",
				"\tat e.E.e(E.java:5)",
				"\tat f.F.f(F.java:6)",
				"\tat g.G.g(G.java:7)",
				"\tat h.H.h(H.java:8)",
				"\tat i.I.i(I.java:9)",
				"\tat j.J.j(J.java:10)",
			},
			startLine: 1,
			wantCount: 1,
			wantEntries: []struct {
				lineNumber int
				lineCount  int
				message    string
				stackTrace string
			}{
				{
					lineNumber: 1,
					lineCount:  11,
					message:    "deep stack",
					stackTrace: "\tat a.A.a(A.java:1)\n\tat b.B.b(B.java:2)\n\tat c.C.c(C.java:3)\n\tat d.D.d(D.java:4)\n\tat e.E.e(E.java:5)\n\tat f.F.f(F.java:6)\n\tat g.G.g(G.java:7)\n\tat h.H.h(H.java:8)\n\tat i.I.i(I.java:9)\n\tat j.J.j(J.java:10)",
				},
			},
		},
	}

	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combiner.Combine(tt.lines, tt.startLine)

			if len(got) != tt.wantCount {
				t.Fatalf("got %d entries, want %d", len(got), tt.wantCount)
			}

			for i, want := range tt.wantEntries {
				entry := got[i]
				if entry.LineNumber != want.lineNumber {
					t.Errorf("entry[%d].LineNumber = %d, want %d", i, entry.LineNumber, want.lineNumber)
				}
				if entry.LineCount != want.lineCount {
					t.Errorf("entry[%d].LineCount = %d, want %d", i, entry.LineCount, want.lineCount)
				}
				if entry.Message != want.message {
					t.Errorf("entry[%d].Message = %q, want %q", i, entry.Message, want.message)
				}
				if entry.StackTrace != want.stackTrace {
					t.Errorf("entry[%d].StackTrace = %q, want %q", i, entry.StackTrace, want.stackTrace)
				}
			}
		})
	}
}

func TestMultilineCombineWithJSONParser(t *testing.T) {
	parser := NewJSONParser()
	combiner := NewMultilineCombiner(parser)

	lines := []string{
		`{"level":"ERROR","msg":"NullPointerException in handler"}`,
		"\tat com.example.Handler.process(Handler.java:42)",
		"\tat com.example.Server.handle(Server.java:118)",
		"Caused by: java.lang.IllegalStateException: bad state",
		"\tat com.example.Dao.find(Dao.java:67)",
		`{"level":"INFO","msg":"request completed"}`,
	}

	got := combiner.Combine(lines, 100)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}

	first := got[0]
	if first.LineNumber != 100 {
		t.Errorf("first.LineNumber = %d, want 100", first.LineNumber)
	}
	if first.LineCount != 5 {
		t.Errorf("first.LineCount = %d, want 5", first.LineCount)
	}
	if first.Message != "NullPointerException in handler" {
		t.Errorf("first.Message = %q, want %q", first.Message, "NullPointerException in handler")
	}
	wantStack := "\tat com.example.Handler.process(Handler.java:42)\n\tat com.example.Server.handle(Server.java:118)\nCaused by: java.lang.IllegalStateException: bad state\n\tat com.example.Dao.find(Dao.java:67)"
	if first.StackTrace != wantStack {
		t.Errorf("first.StackTrace = %q, want %q", first.StackTrace, wantStack)
	}
	if first.Level == nil {
		t.Error("first.Level should not be nil")
	} else if *first.Level != types.LogLevelError {
		t.Errorf("first.Level = %q, want %q", *first.Level, types.LogLevelError)
	}

	second := got[1]
	if second.LineNumber != 105 {
		t.Errorf("second.LineNumber = %d, want 105", second.LineNumber)
	}
	if second.LineCount != 1 {
		t.Errorf("second.LineCount = %d, want 1", second.LineCount)
	}
	if second.StackTrace != "" {
		t.Errorf("second.StackTrace = %q, want empty", second.StackTrace)
	}
}

func TestMultilineCombineWithSyslogParser(t *testing.T) {
	parser := NewSyslogRFC3164Parser()
	combiner := NewMultilineCombiner(parser)

	lines := []string{
		"<131>Jan 15 10:30:00 myhost myapp[1234]: Application error occurred",
		"\tat com.example.Service.run(Service.java:55)",
		"\tat com.example.Main.main(Main.java:10)",
		"<134>Jan 15 10:30:01 myhost myapp[1234]: Recovery successful",
	}

	got := combiner.Combine(lines, 1)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}

	if got[0].LineCount != 3 {
		t.Errorf("first entry LineCount = %d, want 3", got[0].LineCount)
	}
	if got[0].StackTrace == "" {
		t.Error("first entry should have a stack trace")
	}

	if got[1].LineCount != 1 {
		t.Errorf("second entry LineCount = %d, want 1", got[1].LineCount)
	}
	if got[1].StackTrace != "" {
		t.Errorf("second entry StackTrace = %q, want empty", got[1].StackTrace)
	}
}

func TestMultilineCombinerNilReturn(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	got := combiner.Combine(nil, 1)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}

	got = combiner.Combine([]string{}, 1)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestIsContinuation(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "java stack frame",
			line: "\tat com.example.Foo.bar(Foo.java:42)",
			want: true,
		},
		{
			name: "caused by",
			line: "Caused by: java.lang.NullPointerException",
			want: true,
		},
		{
			name: "python traceback header",
			line: "Traceback (most recent call last):",
			want: true,
		},
		{
			name: "dotnet stack frame",
			line: "   at MyApp.Program.Main(String[] args)",
			want: true,
		},
		{
			name: "leading whitespace",
			line: "  indented continuation",
			want: true,
		},
		{
			name: "tab indented",
			line: "\tindented with tab",
			want: true,
		},
		{
			name: "normal log line",
			line: "2025-01-15 INFO normal log line",
			want: false,
		},
		{
			name: "empty string",
			line: "",
			want: false,
		},
		{
			name: "line starting with letter",
			line: "No indentation here",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContinuation(tt.line)
			if got != tt.want {
				t.Errorf("isContinuation(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestNewMultilineCombiner(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)
	if combiner == nil {
		t.Fatal("NewMultilineCombiner returned nil")
	}
	if combiner.parser != parser {
		t.Error("combiner.parser is not the parser that was passed in")
	}
}

func TestMultilineCombinePreservesRaw(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	lines := []string{
		"LOG: error occurred",
		"\tat com.example.Foo.bar(Foo.java:1)",
	}

	got := combiner.Combine(lines, 1)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].Raw != "LOG: error occurred" {
		t.Errorf("Raw = %q, want %q", got[0].Raw, "LOG: error occurred")
	}
}

func TestMultilineCombineRawEntriesPreserveRaw(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	lines := []string{
		"unparseable line",
	}

	got := combiner.Combine(lines, 1)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].Raw != "unparseable line" {
		t.Errorf("Raw = %q, want %q", got[0].Raw, "unparseable line")
	}
	if got[0].Message != "unparseable line" {
		t.Errorf("Message = %q, want %q", got[0].Message, "unparseable line")
	}
}

func TestMultilineCombineConsecutiveStackTraces(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	lines := []string{
		"LOG: first error",
		"\tat a.A.a(A.java:1)",
		"\tat b.B.b(B.java:2)",
		"LOG: second error",
		"\tat c.C.c(C.java:3)",
		"\tat d.D.d(D.java:4)",
	}

	got := combiner.Combine(lines, 1)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}

	if got[0].LineCount != 3 {
		t.Errorf("first.LineCount = %d, want 3", got[0].LineCount)
	}
	if got[0].StackTrace != "\tat a.A.a(A.java:1)\n\tat b.B.b(B.java:2)" {
		t.Errorf("first.StackTrace = %q", got[0].StackTrace)
	}

	if got[1].LineCount != 3 {
		t.Errorf("second.LineCount = %d, want 3", got[1].LineCount)
	}
	if got[1].StackTrace != "\tat c.C.c(C.java:3)\n\tat d.D.d(D.java:4)" {
		t.Errorf("second.StackTrace = %q", got[1].StackTrace)
	}
}

func TestMultilineCombineOnlyContinuationLines(t *testing.T) {
	parser := &stubParser{}
	combiner := NewMultilineCombiner(parser)

	// All lines are continuation/indented but there is no preceding log entry.
	// They should each become a raw entry since there's nothing to attach to.
	lines := []string{
		"\tat orphaned.Frame(Frame.java:1)",
		"\tat orphaned.Frame2(Frame.java:2)",
	}

	got := combiner.Combine(lines, 1)
	// These are unparseable lines without a preceding entry — they become raw entries.
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].LineNumber != 1 {
		t.Errorf("entry[0].LineNumber = %d, want 1", got[0].LineNumber)
	}
	if got[1].LineNumber != 2 {
		t.Errorf("entry[1].LineNumber = %d, want 2", got[1].LineNumber)
	}
}
