package analytics_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nacfson/nacfson_blog/internal/analytics"
	"github.com/nacfson/nacfson_blog/internal/content"
	"github.com/nacfson/nacfson_blog/internal/logx"
)

const (
	browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36"
	visitorIP = "198.51.100.23"
)

var (
	postA = analytics.PostPage(content.PostSummary{Slug: "cilium-zero-plaintext-k3s", Topic: content.TopicCloudNative})
	postB = analytics.PostPage(content.PostSummary{Slug: "the-first-footprint", Topic: content.TopicOpenSource})

	hex32  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type clock struct{ unixNano atomic.Int64 }

func newClock(t time.Time) *clock {
	c := &clock{}
	c.set(t)
	return c
}

func (c *clock) set(t time.Time)   { c.unixNano.Store(t.UnixNano()) }
func (c *clock) now() time.Time    { return time.Unix(0, c.unixNano.Load()) }
func noon() *clock                 { return newClock(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)) }
func keyBytes(fill byte) io.Reader { return bytes.NewReader(bytes.Repeat([]byte{fill}, 32)) }

func newRecorder(keyRand io.Reader, c *clock) (*analytics.Recorder, *bytes.Buffer) {
	var buf bytes.Buffer
	return analytics.NewWithSources(logx.New(&buf, "blog-web", "v1.2.3"), keyRand, c.now), &buf
}

func request(ua, realIP, remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("User-Agent", ua)
	if realIP != "" {
		req.Header.Set("X-Real-Ip", realIP)
	}
	req.RemoteAddr = remoteAddr
	return req
}

func browser() *http.Request { return request(browserUA, visitorIP, "10.42.0.7:51234") }

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("line is not one JSON object: %q (%v)", l, err)
		}
		out = append(out, m)
	}
	return out
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestRecordView_LineShape(t *testing.T) {
	envelope := []string{"time", "level", "msg", "service.name", "service.version",
		"event.name", "event.version", "event.id", "analytics.day", "visitor.id", "page.kind"}
	for _, tc := range []struct {
		page  analytics.Page
		extra map[string]any
	}{
		{analytics.Home, nil},
		{postA, map[string]any{"page.slug": "cilium-zero-plaintext-k3s", "page.topic": "cloud-native"}},
	} {
		t.Run(string(tc.page.Kind), func(t *testing.T) {
			r, buf := newRecorder(rand.Reader, noon())
			req := browser()
			req = req.WithContext(logx.WithTraceID(req.Context(), "4bf92f3577b34da6a3ce929d0e0e4736"))
			req.Header.Set("Referer", "https://news.example/item?id=1")
			r.RecordView(req, tc.page)

			got := lines(t, buf)
			if len(got) != 1 {
				t.Fatalf("want 1 line, got %d: %s", len(got), buf)
			}
			m := got[0]
			wantKeys := append([]string(nil), envelope...)
			for k := range tc.extra {
				wantKeys = append(wantKeys, k)
			}
			sort.Strings(wantKeys)
			if gotKeys := keys(m); strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
				t.Fatalf("keys = %v, want %v", gotKeys, wantKeys)
			}
			want := map[string]any{
				"level":           "info",
				"msg":             "page view",
				"service.name":    "blog-web",
				"service.version": "v1.2.3",
				"event.name":      "blog.analytics",
				"event.version":   float64(1),
				"analytics.day":   "2026-11-25",
				"page.kind":       string(tc.page.Kind),
			}
			for k, v := range tc.extra {
				want[k] = v
			}
			for k, v := range want {
				if m[k] != v {
					t.Errorf("%s = %v, want %v", k, m[k], v)
				}
			}
			if id, _ := m["visitor.id"].(string); !hex32.MatchString(id) {
				t.Errorf("visitor.id = %q, want 32 lowercase hex", id)
			}
			if id, _ := m["event.id"].(string); !uuidV4.MatchString(id) {
				t.Errorf("event.id = %q, want UUIDv4", id)
			}
			for _, leak := range []string{visitorIP, "10.42.0.7", "Mozilla", browserUA, "news.example"} {
				if strings.Contains(buf.String(), leak) {
					t.Errorf("line leaks %q: %s", leak, buf)
				}
			}
		})
	}
}

func TestRecordView_EveryViewPrintsWithFreshEventID(t *testing.T) {
	r, buf := newRecorder(rand.Reader, noon())
	r.RecordView(browser(), postA)
	r.RecordView(browser(), postA)
	r.RecordView(browser(), postB)

	got := lines(t, buf)
	if len(got) != 3 {
		t.Fatalf("want 3 lines (no server-side dedup), got %d: %s", len(got), buf)
	}
	eventIDs := map[any]bool{}
	for _, m := range got {
		if m["visitor.id"] != got[0]["visitor.id"] {
			t.Errorf("same visitor got different ids: %v vs %v", m["visitor.id"], got[0]["visitor.id"])
		}
		eventIDs[m["event.id"]] = true
	}
	if len(eventIDs) != 3 {
		t.Errorf("want 3 distinct event.id, got %v", eventIDs)
	}
}

func TestRecordView_DistinctVisitors(t *testing.T) {
	r, buf := newRecorder(rand.Reader, noon())
	r.RecordView(browser(), analytics.Home)
	r.RecordView(request(browserUA+" Edg/131.0", visitorIP, "10.42.0.7:1"), analytics.Home)
	r.RecordView(request(browserUA, "198.51.100.24", "10.42.0.7:1"), analytics.Home)

	ids := map[any]bool{}
	for _, m := range lines(t, buf) {
		ids[m["visitor.id"]] = true
	}
	if len(ids) != 3 {
		t.Errorf("different IP or UA must give a different visitor.id, got %v", ids)
	}
}

func TestRecordView_PrefersXRealIP(t *testing.T) {
	r, buf := newRecorder(rand.Reader, noon())
	r.RecordView(request(browserUA, visitorIP, "10.42.0.7:1"), analytics.Home)
	r.RecordView(request(browserUA, visitorIP, "10.42.0.99:2"), postA)
	r.RecordView(request(browserUA, "not-an-ip", visitorIP+":3"), postB)
	r.RecordView(request(browserUA, "", "[::ffff:"+visitorIP+"]:4"), postA)

	got := lines(t, buf)
	if len(got) != 4 {
		t.Fatalf("want 4 lines, got %d: %s", len(got), buf)
	}
	for i, m := range got[1:] {
		if m["visitor.id"] != got[0]["visitor.id"] {
			t.Errorf("line %d: visitor.id %v, want %v (same client IP)", i+1, m["visitor.id"], got[0]["visitor.id"])
		}
	}
}

func TestRecordView_DrawsFreshKeyEachUTCDay(t *testing.T) {
	seoul := time.FixedZone("KST", 9*60*60)
	c := newClock(time.Date(2026, 11, 26, 8, 58, 0, 0, seoul))
	keyRand := bytes.NewReader(append(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)...))
	r, buf := newRecorder(keyRand, c)

	r.RecordView(browser(), analytics.Home)
	c.set(time.Date(2026, 11, 26, 8, 59, 59, 0, seoul))
	r.RecordView(browser(), analytics.Home)
	if keyRand.Len() != 32 {
		t.Fatalf("same UTC day must reuse the day key; unread key bytes = %d, want 32", keyRand.Len())
	}
	c.set(time.Date(2026, 11, 26, 9, 0, 0, 0, seoul))
	r.RecordView(browser(), analytics.Home)
	if keyRand.Len() != 0 {
		t.Fatalf("new UTC day must draw a fresh key; unread key bytes = %d, want 0", keyRand.Len())
	}

	got := lines(t, buf)
	if len(got) != 3 {
		t.Fatalf("want 3 lines, got %d: %s", len(got), buf)
	}
	if got[0]["analytics.day"] != "2026-11-25" || got[2]["analytics.day"] != "2026-11-26" {
		t.Errorf("days = %v, %v; want UTC 2026-11-25, 2026-11-26", got[0]["analytics.day"], got[2]["analytics.day"])
	}
	if got[0]["visitor.id"] != got[1]["visitor.id"] {
		t.Errorf("visitor.id must be stable within a UTC day: %v vs %v", got[0]["visitor.id"], got[1]["visitor.id"])
	}
	if got[1]["visitor.id"] == got[2]["visitor.id"] {
		t.Errorf("visitor.id must not link across days, both %v", got[1]["visitor.id"])
	}
}

func TestRecordView_RestartWithNewKeyGivesNewVisitorID(t *testing.T) {
	before, buf1 := newRecorder(keyBytes(1), noon())
	afterRestart, buf2 := newRecorder(keyBytes(2), noon())
	sameKey, buf3 := newRecorder(keyBytes(1), noon())
	before.RecordView(browser(), analytics.Home)
	afterRestart.RecordView(browser(), analytics.Home)
	sameKey.RecordView(browser(), analytics.Home)

	a, b, c := lines(t, buf1)[0]["visitor.id"], lines(t, buf2)[0]["visitor.id"], lines(t, buf3)[0]["visitor.id"]
	if a == b {
		t.Errorf("a restart draws a new key, so the same visitor counts twice that day; both ids %v", a)
	}
	if a != c {
		t.Errorf("visitor.id must depend only on the day key, IP, and UA: %v vs %v", a, c)
	}
}

func TestRecordView_ExcludesBotsAndEmptyAgent(t *testing.T) {
	r, buf := newRecorder(rand.Reader, noon())
	for _, ua := range []string{
		"",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"facebookexternalhit/1.1",
		"curl/8.5.0",
		"Wget/1.21",
		"python-requests/2.32",
		"Go-http-client/1.1",
		"kube-probe/1.31",
		"Mozilla/5.0 HeadlessChrome/131.0",
		"Slackbot-LinkExpanding 1.0",
		"UptimeRobot/2.0; http://www.uptimerobot.com/ Monitor",
	} {
		r.RecordView(request(ua, visitorIP, "10.42.0.7:1"), analytics.Home)
	}
	if buf.Len() != 0 {
		t.Fatalf("excluded agents must print nothing, got: %s", buf)
	}
	r.RecordView(browser(), analytics.Home)
	if len(lines(t, buf)) != 1 {
		t.Fatalf("browser agent must print, got: %s", buf)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func TestRecordView_ConcurrentAcrossDayBoundary(t *testing.T) {
	c := newClock(time.Date(2026, 11, 25, 23, 59, 59, 0, time.UTC))
	var out lockedBuffer
	r := analytics.NewWithSources(logx.New(&out, "blog-web", "v1.2.3"), rand.Reader, c.now)

	const n = 200
	r.RecordView(browser(), postA)
	var wg sync.WaitGroup
	for i := 1; i < n; i++ {
		if i == n/2 {
			c.set(time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC))
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.RecordView(browser(), postA)
		}()
	}
	wg.Wait()

	got := lines(t, &out.buf)
	if len(got) != n {
		t.Fatalf("want %d lines, got %d", n, len(got))
	}
	idByDay := map[any]any{}
	eventIDs := map[any]bool{}
	for _, m := range got {
		day := m["analytics.day"]
		if id, ok := idByDay[day]; ok && id != m["visitor.id"] {
			t.Fatalf("day %v has two visitor.id values: %v and %v", day, id, m["visitor.id"])
		}
		idByDay[day] = m["visitor.id"]
		eventIDs[m["event.id"]] = true
	}
	if len(idByDay) != 2 {
		t.Errorf("want lines on 2 UTC days, got %v", idByDay)
	}
	if len(eventIDs) != n {
		t.Errorf("want %d distinct event.id, got %d", n, len(eventIDs))
	}
}
