package logx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/logx"
)

func TestNew_RequiredV3FieldsAndTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v1.2.3")
	logger.Info("server starting", "port", "8080")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSON line, got %d: %q", len(lines), buf.String())
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", lines[0], err)
	}

	for _, req := range []string{"time", "level", "msg", "service.name", "service.version"} {
		if _, ok := m[req]; !ok {
			t.Fatalf("missing required key %q in %s", req, lines[0])
		}
	}

	timeRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
	if timeStr, _ := m["time"].(string); !timeRegex.MatchString(timeStr) {
		t.Fatalf("time %q does not match RFC3339 UTC millisecond format", m["time"])
	}
	if m["level"] != "info" {
		t.Fatalf("expected level=info, got %v", m["level"])
	}
	if m["msg"] != "server starting" {
		t.Fatalf("expected msg='server starting', got %v", m["msg"])
	}
	if m["service.name"] != "blog-web" {
		t.Fatalf("expected service.name='blog-web', got %v", m["service.name"])
	}
	if m["service.version"] != "v1.2.3" {
		t.Fatalf("expected service.version='v1.2.3', got %v", m["service.version"])
	}
	if m["port"] != "8080" {
		t.Fatalf("expected port='8080', got %v", m["port"])
	}
}

func TestNew_LevelsAndFatalStacktrace(t *testing.T) {
	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v0.0.0-abc1234")

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg", "error", context.DeadlineExceeded)
	logger.Log(context.Background(), logx.LevelFatal, "fatal msg", "error", context.Canceled)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}

	wantLevels := []string{"debug", "info", "warn", "error", "fatal"}
	for i, want := range wantLevels {
		var m map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &m); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if m["level"] != want {
			t.Fatalf("line %d: expected level %q, got %v", i, want, m["level"])
		}
		if want == "fatal" {
			st, _ := m["exception.stacktrace"].(string)
			if st == "" {
				t.Fatalf("fatal line missing exception.stacktrace: %s", lines[i])
			}
			if m["exception.type"] == nil || m["exception.message"] == nil {
				t.Fatalf("fatal line missing exception fields: %s", lines[i])
			}
		}
	}
}

func TestException_UnwrapsInnermostError(t *testing.T) {
	rootErr := &os.PathError{Op: "open", Path: "/nonexistent", Err: os.ErrNotExist}
	urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: rootErr}
	wrapped := fmt.Errorf("sync failed: %w", fmt.Errorf("fetch archive: %w", urlErr))

	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v1.0.0")
	logger.Error("operation failed", "error", wrapped)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, hasRawError := m["error"]; hasRawError {
		t.Fatalf("expected 'error' attr to be rewritten, got %v", m)
	}
	if m["exception.message"] != wrapped.Error() {
		t.Fatalf("expected exception.message %q, got %v", wrapped.Error(), m["exception.message"])
	}
	wantType := fmt.Sprintf("%T", rootErr)
	if m["exception.type"] != wantType {
		t.Fatalf("expected innermost exception.type %q, got %v", wantType, m["exception.type"])
	}

	// Verify joined errors also unwrap to concrete error
	joined := errors.Join(errors.New("wrapper note"), &url.Error{Op: "Get", URL: "http://example.com", Err: context.DeadlineExceeded})
	attrs := logx.Exception(joined)
	attrMap := make(map[string]any)
	for i := 0; i+1 < len(attrs); i += 2 {
		attrMap[attrs[i].(string)] = attrs[i+1]
	}
	if attrMap["exception.type"] != fmt.Sprintf("%T", context.DeadlineExceeded) {
		t.Fatalf("expected joined innermost type %T, got %v", context.DeadlineExceeded, attrMap["exception.type"])
	}

	// Verify plain errors.New does not emit forbidden *errors.errorString
	plainAttrs := logx.Exception(errors.New("disk full"))
	plainMap := make(map[string]any)
	for i := 0; i+1 < len(plainAttrs); i += 2 {
		plainMap[plainAttrs[i].(string)] = plainAttrs[i+1]
	}
	for _, forbidden := range []string{"*fmt.wrapError", "*errors.errorString", "*errors.joinError", "error", ""} {
		if plainMap["exception.type"] == forbidden {
			t.Fatalf("exception.type must not be generic %q", forbidden)
		}
	}
}

func TestTruncateStacktrace_CappedAt8KiB(t *testing.T) {
	exact := strings.Repeat("x", 8191) + "\n"
	if got := logx.TruncateStacktrace(exact); got != exact {
		t.Fatalf("expected exact 8192-byte trace to be unchanged (len=%d)", len(got))
	}

	var b strings.Builder
	b.WriteString("goroutine 1 [running]:\nmain.fail()\n")
	for b.Len() < 20*1024 {
		b.WriteString("\t/app/cmd/blog-server/main.go:42 +0x123\n")
	}
	big := b.String()

	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v1.0.0")
	logger.Error("panic recovered",
		"error", context.DeadlineExceeded,
		"exception.stacktrace", big,
	)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	got, _ := m["exception.stacktrace"].(string)
	if len(got) > 8192 {
		t.Fatalf("expected truncated stacktrace <= 8192 bytes, got %d", len(got))
	}
	if !strings.HasPrefix(got, "goroutine 1 [running]:\n") {
		t.Fatalf("expected top of stacktrace preserved, got prefix %q", got[:40])
	}

	markerRe := regexp.MustCompile(`\n…\[truncated (\d+) bytes\]$`)
	sub := markerRe.FindStringSubmatch(got)
	if sub == nil {
		t.Fatalf("expected truncation suffix matching \\n…[truncated N bytes], got tail %q", got[len(got)-40:])
	}
	truncatedBytes, err := strconv.Atoi(sub[1])
	if err != nil {
		t.Fatalf("invalid byte count: %v", err)
	}
	keptLen := len(got) - len(sub[0])
	if keptLen+truncatedBytes != len(big) {
		t.Fatalf("expected keptLen (%d) + truncatedBytes (%d) == original (%d)", keptLen, truncatedBytes, len(big))
	}
}

func TestTraceID_ParseAndContextPropagation(t *testing.T) {
	tid, ok := logx.ParseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if !ok || tid != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected valid trace_id from traceparent, got %q (ok=%v)", tid, ok)
	}

	for _, invalid := range []string{
		"",
		"invalid",
		"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"00-4bf92f35-77b3-4da6-a3ce-929d0e0e4736-00f067aa0ba902b7-01",
	} {
		if _, ok := logx.ParseTraceparent(invalid); ok {
			t.Fatalf("expected invalid traceparent %q to be rejected", invalid)
		}
	}

	gen := logx.ResolveOrGenerateTraceID("")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(gen) || gen == strings.Repeat("0", 32) {
		t.Fatalf("generated trace_id %q is invalid", gen)
	}

	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v1.0.0")
	ctx := logx.WithTraceID(context.Background(), tid)
	logger.InfoContext(ctx, "request handled")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["trace_id"] != tid {
		t.Fatalf("expected trace_id %q from context, got %v", tid, m["trace_id"])
	}
}

func TestStripsPlatformReservedKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := logx.New(&buf, "blog-web", "v1.0.0")
	logger.Info("attempt reserved keys",
		"k8s.namespace.name", "proj-fake",
		"k8s.pod.name", "fake-pod",
		"log.iostream", "stdout",
		"observed_time", "2026-01-01T00:00:00Z",
		"_time", "2026-01-01T00:00:00Z",
		"safe_key", "allowed",
	)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, reserved := range []string{"k8s.namespace.name", "k8s.pod.name", "log.iostream", "observed_time", "_time"} {
		if _, exists := m[reserved]; exists {
			t.Fatalf("reserved key %q must be stripped, got %v", reserved, m)
		}
	}
	if m["safe_key"] != "allowed" {
		t.Fatalf("expected safe_key='allowed', got %v", m["safe_key"])
	}
}

func TestNoPlainLogInBlogWebPackages(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{
		filepath.Join(root, "cmd", "blog-server"),
		filepath.Join(root, "internal", "server"),
		filepath.Join(root, "internal", "syncer"),
		filepath.Join(root, "internal", "content"),
	}
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(b)
			if strings.Contains(src, "log.Printf") || strings.Contains(src, "log.Println") || strings.Contains(src, "log.Fatalf") {
				t.Errorf("%s still contains plain log.Print*/Fatal* calls", path)
			}
			if strings.Contains(src, "NewTextHandler") {
				t.Errorf("%s contains slog.NewTextHandler", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
