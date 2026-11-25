// Package analytics prints one structured "blog.analytics" log line per page
// view. Visitors are identified without cookies by an HMAC of client IP and
// User-Agent under a random key that lives only in memory for one UTC day.
// Deduplication is left to the log database; see docs/analytics-contract.md.
package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nacfson/nacfson_blog/internal/content"
)

// EventName is the registered Contract v3 event.name for page-view lines.
const EventName = "blog.analytics"

const (
	eventVersion = 1
	dayLayout    = "2006-01-02"
)

// PageKind names the kind of page a view landed on.
type PageKind string

const (
	PageHome PageKind = "home"
	PagePost PageKind = "post"
)

// Page identifies the viewed page. Home carries no Slug or Topic.
type Page struct {
	Kind  PageKind
	Slug  string
	Topic content.Topic
}

// Home is the post-list landing page.
var Home = Page{Kind: PageHome}

// PostPage returns the Page for a single post view.
func PostPage(s content.PostSummary) Page {
	return Page{Kind: PagePost, Slug: s.Slug, Topic: s.Topic}
}

func (p Page) attrs() []any {
	if p.Kind == PagePost {
		return []any{"page.kind", string(p.Kind), "page.slug", p.Slug, "page.topic", string(p.Topic)}
	}
	return []any{"page.kind", string(p.Kind)}
}

// Recorder prints every qualifying page view. It is safe for concurrent use.
type Recorder struct {
	logger  *slog.Logger
	keyRand io.Reader
	now     func() time.Time

	mu     sync.Mutex
	day    string
	dayKey [32]byte
}

// New returns a Recorder printing to logger.
func New(logger *slog.Logger) *Recorder {
	return &Recorder{logger: logger, keyRand: rand.Reader, now: time.Now}
}

// RecordView prints one page-view line unless the request comes from an
// excluded agent.
func (r *Recorder) RecordView(req *http.Request, p Page) {
	ua := req.UserAgent()
	if isExcludedAgent(ua) {
		return
	}
	day, visitor := r.identify(clientIP(req), ua)
	attrs := append([]any{
		"event.name", EventName,
		"event.version", eventVersion,
		"event.id", newEventID(),
		"analytics.day", day,
		"visitor.id", visitor,
	}, p.attrs()...)
	// A background context keeps logx from attaching the request trace_id,
	// which would link a visitor's views to access-log lines.
	r.logger.InfoContext(context.Background(), "page view", attrs...)
}

func (r *Recorder) identify(ip, ua string) (day, visitor string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	day = r.now().UTC().Format(dayLayout)
	if day != r.day {
		// Reading into the array overwrites the previous day's key in place.
		mustRead(r.keyRand, r.dayKey[:])
		r.day = day
	}
	m := hmac.New(sha256.New, r.dayKey[:])
	m.Write([]byte(ip + "\x00" + ua))
	return day, hex.EncodeToString(m.Sum(nil))[:32]
}

func newEventID() string {
	var b [16]byte
	mustRead(rand.Reader, b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// mustRead panics on failure: without randomness there is no safe ID to print,
// and crypto/rand itself treats a read failure as fatal.
func mustRead(src io.Reader, b []byte) {
	if _, err := io.ReadFull(src, b); err != nil {
		panic(fmt.Sprintf("analytics: read random bytes: %v", err))
	}
}

var botAgent = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|facebookexternalhit|preview|monitor|curl|wget|python-requests|go-http-client|headless|kube-probe`)

func isExcludedAgent(ua string) bool {
	return ua == "" || botAgent.MatchString(ua)
}

// clientIP trusts X-Real-Ip because Traefik overwrites it from the socket peer
// for untrusted clients; RemoteAddr is the fallback for direct connections.
func clientIP(req *http.Request) string {
	if a, err := netip.ParseAddr(strings.TrimSpace(req.Header.Get("X-Real-Ip"))); err == nil {
		return a.Unmap().String()
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Unmap().String()
	}
	return host
}
