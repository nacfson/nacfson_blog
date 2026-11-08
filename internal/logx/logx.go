package logx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// LevelFatal represents a fatal log level (OpenTelemetry SeverityNumber 21).
	LevelFatal = slog.LevelError + 4

	// DefaultServiceName is the canonical service.name for nacfson_blog.
	DefaultServiceName = "blog-web"

	// DefaultServiceVersion is the fallback version for local builds outside Git.
	DefaultServiceVersion = "v0.0.0"

	// MaxStacktraceBytes is the Contract v3 8 KiB stacktrace cap (FR-004, FR-018c).
	MaxStacktraceBytes = 8192

	// TimeFormat is the Contract v3 UTC millisecond RFC 3339 timestamp layout.
	TimeFormat = "2006-01-02T15:04:05.000Z07:00"
)

type traceIDContextKey struct{}

var genericExceptionTypes = map[string]bool{
	"*fmt.wrapError":      true,
	"*errors.errorString": true,
	"*errors.joinError":   true,
	"error":               true,
	"Exception":           true,
	"Error":               true,
	"java.lang.Exception": true,
}

func init() {
	slog.SetDefault(New(os.Stderr, DefaultServiceName, DefaultServiceVersion))
}

// New constructs a structured JSON slog.Logger conforming to Project Log Contract v3.
func New(w io.Writer, service, version string) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	if service == "" {
		service = DefaultServiceName
	}
	if version == "" {
		version = DefaultServiceVersion
	}

	base := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttr,
	})
	withService := base.WithAttrs([]slog.Attr{
		slog.String("service.name", service),
		slog.String("service.version", version),
	})
	return slog.New(&contractHandler{inner: withService})
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if isReservedKey(a.Key) {
		return slog.Attr{}
	}
	if len(groups) == 0 {
		switch a.Key {
		case slog.TimeKey:
			t := a.Value.Time()
			if t.IsZero() {
				t = time.Now()
			}
			return slog.String(slog.TimeKey, t.UTC().Format(TimeFormat))
		case slog.LevelKey:
			lvl, ok := a.Value.Any().(slog.Level)
			if !ok {
				return slog.String(slog.LevelKey, strings.ToLower(a.Value.String()))
			}
			return slog.String(slog.LevelKey, levelString(lvl))
		}
	}
	if a.Key == "exception.stacktrace" {
		return slog.String("exception.stacktrace", TruncateStacktrace(a.Value.String()))
	}
	return a
}

func levelString(lvl slog.Level) string {
	switch {
	case lvl >= LevelFatal:
		return "fatal"
	case lvl >= slog.LevelError:
		return "error"
	case lvl >= slog.LevelWarn:
		return "warn"
	case lvl >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

func isReservedKey(k string) bool {
	if k == "" {
		return false
	}
	return strings.HasPrefix(k, "_") ||
		strings.HasPrefix(k, "k8s.") ||
		k == "k8s" ||
		k == "log.iostream" ||
		k == "observed_time"
}

type contractHandler struct {
	inner      slog.Handler
	hasTraceID bool
}

func (h *contractHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *contractHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cleaned := make([]slog.Attr, 0, len(attrs))
	hasTrace := h.hasTraceID
	for _, a := range attrs {
		if isReservedKey(a.Key) {
			continue
		}
		if a.Key == "trace_id" && a.Value.String() != "" {
			hasTrace = true
		}
		if a.Key == "exception.stacktrace" {
			a = slog.String("exception.stacktrace", TruncateStacktrace(a.Value.String()))
		}
		cleaned = append(cleaned, a)
	}
	return &contractHandler{
		inner:      h.inner.WithAttrs(cleaned),
		hasTraceID: hasTrace,
	}
}

func (h *contractHandler) WithGroup(name string) slog.Handler {
	if isReservedKey(name) {
		return h
	}
	return &contractHandler{
		inner:      h.inner.WithGroup(name),
		hasTraceID: h.hasTraceID,
	}
}

func (h *contractHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	var (
		hasTraceAttr bool
		hasExcMsg    bool
		hasExcType   bool
		hasExcStack  bool
		extractedErr error
		extractedStr string
	)

	r.Attrs(func(a slog.Attr) bool {
		if isReservedKey(a.Key) {
			return true
		}
		switch a.Key {
		case "trace_id":
			if a.Value.String() != "" {
				hasTraceAttr = true
			}
			out.AddAttrs(a)
		case "error", "err":
			switch v := a.Value.Any().(type) {
			case error:
				if v != nil {
					extractedErr = v
				}
			case string:
				if v != "" {
					extractedStr = v
				}
			default:
				if a.Value.Any() != nil {
					s := fmt.Sprint(a.Value.Any())
					if s != "" {
						extractedStr = s
					}
				}
			}
		case "exception.message":
			if a.Value.String() != "" {
				hasExcMsg = true
			}
			out.AddAttrs(a)
		case "exception.type":
			tStr := a.Value.String()
			if tStr != "" {
				if genericExceptionTypes[tStr] {
					tStr = "errors.Error"
				}
				hasExcType = true
				out.AddAttrs(slog.String("exception.type", tStr))
			}
		case "exception.stacktrace":
			st := TruncateStacktrace(a.Value.String())
			if st != "" {
				hasExcStack = true
				out.AddAttrs(slog.String("exception.stacktrace", st))
			}
		default:
			out.AddAttrs(a)
		}
		return true
	})

	if !h.hasTraceID && !hasTraceAttr {
		if tid := TraceIDFromContext(ctx); tid != "" {
			out.AddAttrs(slog.String("trace_id", tid))
		}
	}

	if extractedErr != nil {
		if !hasExcType {
			out.AddAttrs(slog.String("exception.type", ExceptionType(extractedErr)))
			hasExcType = true
		}
		if !hasExcMsg {
			out.AddAttrs(slog.String("exception.message", extractedErr.Error()))
			hasExcMsg = true
		}
	} else if extractedStr != "" {
		if !hasExcType {
			out.AddAttrs(slog.String("exception.type", "errors.Error"))
			hasExcType = true
		}
		if !hasExcMsg {
			out.AddAttrs(slog.String("exception.message", extractedStr))
			hasExcMsg = true
		}
	}

	if r.Level >= slog.LevelError {
		if !hasExcType {
			out.AddAttrs(slog.String("exception.type", "errors.Error"))
		}
		if !hasExcMsg {
			msg := r.Message
			if msg == "" {
				msg = "unexpected error"
			}
			out.AddAttrs(slog.String("exception.message", msg))
		}
	}

	if r.Level >= LevelFatal && !hasExcStack {
		out.AddAttrs(slog.String("exception.stacktrace", TruncateStacktrace(string(debug.Stack()))))
	}

	return h.inner.Handle(ctx, out)
}

// Exception returns the Contract v3 exception.type (innermost unwrapped error type)
// and exception.message attributes for an error.
func Exception(err error) []any {
	if err == nil {
		return nil
	}
	return []any{
		"exception.type", ExceptionType(err),
		"exception.message", err.Error(),
	}
}

// ExceptionType returns the non-generic Go type name of the innermost unwrapped error.
func ExceptionType(err error) string {
	if err == nil {
		return "errors.Error"
	}
	inner := innermostError(err)
	t := fmt.Sprintf("%T", inner)
	if t == "" || genericExceptionTypes[t] {
		return "errors.Error"
	}
	return t
}

func innermostError(err error) error {
	var lastConcrete error
	for err != nil {
		if !genericExceptionTypes[fmt.Sprintf("%T", err)] {
			lastConcrete = err
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			if next := u.Unwrap(); next != nil {
				err = next
				continue
			}
		}
		if u, ok := err.(interface{ Unwrap() []error }); ok {
			list := u.Unwrap()
			var firstNonNil error
			for _, item := range list {
				if item == nil {
					continue
				}
				if firstNonNil == nil {
					firstNonNil = item
				}
				deep := innermostError(item)
				if !genericExceptionTypes[fmt.Sprintf("%T", deep)] {
					return deep
				}
			}
			if firstNonNil != nil {
				err = firstNonNil
				continue
			}
		}
		break
	}
	if lastConcrete != nil {
		return lastConcrete
	}
	return err
}

// TruncateStacktrace caps a stack trace at 8 KiB (8192 bytes), preserving the top
// lines and appending "\n…[truncated N bytes]" when truncated.
func TruncateStacktrace(s string) string {
	if len(s) <= MaxStacktraceBytes {
		return s
	}
	maxSuffixLen := len(fmt.Sprintf("\n…[truncated %d bytes]", len(s)))
	budget := MaxStacktraceBytes - maxSuffixLen
	if budget < 1 {
		budget = 1
	}
	cutPos := budget
	if nl := strings.LastIndexByte(s[:budget], '\n'); nl > 0 {
		cutPos = nl
	} else {
		for cutPos > 0 && !utf8.RuneStart(s[cutPos]) {
			cutPos--
		}
	}
	truncated := len(s) - cutPos
	return fmt.Sprintf("%s\n…[truncated %d bytes]", s[:cutPos], truncated)
}

// Fatal logs a fatal message with exception details and stacktrace, then exits with code 1.
func Fatal(logger *slog.Logger, msg string, err error, attrs ...any) {
	if logger == nil {
		logger = slog.Default()
	}
	all := make([]any, 0, len(attrs)+2)
	if err != nil {
		all = append(all, "error", err)
	}
	all = append(all, attrs...)
	logger.Log(context.Background(), LevelFatal, msg, all...)
	os.Exit(1)
}

// WithTraceID stores a 32-hex trace_id in ctx.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFromContext retrieves the trace_id from ctx, if present.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	tid, _ := ctx.Value(traceIDContextKey{}).(string)
	return tid
}

// ParseTraceparent parses a W3C traceparent header ("00-<32-hex-trace-id>-<16-hex-parent-id>-<2-hex-flags>")
// and returns the 32 lowercase hex trace_id if valid.
func ParseTraceparent(header string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) != 4 {
		return "", false
	}
	ver := strings.ToLower(parts[0])
	traceID := strings.ToLower(parts[1])
	parentID := strings.ToLower(parts[2])
	flags := strings.ToLower(parts[3])

	if len(ver) != 2 || !isLowerHex(ver) || ver == "ff" {
		return "", false
	}
	if len(traceID) != 32 || !isLowerHex(traceID) || isAllZero(traceID) {
		return "", false
	}
	if len(parentID) != 16 || !isLowerHex(parentID) || isAllZero(parentID) {
		return "", false
	}
	if len(flags) != 2 || !isLowerHex(flags) {
		return "", false
	}
	return traceID, true
}

// ResolveOrGenerateTraceID extracts a valid 32-lowercase-hex trace_id from a W3C traceparent
// header, or generates a fresh random 16-byte (32-hex) trace_id if absent or invalid.
func ResolveOrGenerateTraceID(traceparent string) string {
	if tid, ok := ParseTraceparent(traceparent); ok {
		return tid
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		s := hex.EncodeToString(b[:])
		if !isAllZero(s) {
			return s
		}
	}
	return fmt.Sprintf("%032x", uint64(time.Now().UnixNano()))
}

// FormatTraceparent formats a W3C traceparent header for outbound HTTP calls given a 32-hex trace_id.
func FormatTraceparent(traceID string) string {
	if len(traceID) != 32 || !isLowerHex(traceID) || isAllZero(traceID) {
		traceID = ResolveOrGenerateTraceID("")
	}
	var span [8]byte
	_, _ = rand.Read(span[:])
	parentID := hex.EncodeToString(span[:])
	if isAllZero(parentID) {
		parentID = "0000000000000001"
	}
	return "00-" + traceID + "-" + parentID + "-01"
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return len(s) > 0
}

func isAllZero(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}
