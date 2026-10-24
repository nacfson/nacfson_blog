package syncer_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nacfson/nacfson_blog/internal/syncer"
)

type mockReloader struct {
	count atomic.Int32
	err   error
}

func (m *mockReloader) Reload() error {
	m.count.Add(1)
	return m.err
}

func buildTarGz(t *testing.T, entries []tar.Header, bodies [][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for i, hdr := range entries {
		if err := tw.WriteHeader(&hdr); err != nil {
			t.Fatal(err)
		}
		if len(bodies[i]) > 0 {
			if _, err := tw.Write(bodies[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSyncOnce_ExtractsContentAndHonorsETag304(t *testing.T) {
	payload := buildTarGz(t, []tar.Header{
		{Name: "nacfson-blog-main/content/posts/hello.md", Mode: 0o644, Size: 5, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("hello")})

	etag := `"rev-111"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	workDir := t.TempDir()
	rel := &mockReloader{}
	s := syncer.NewFluxTarballSyncer(srv.URL, workDir, rel)

	updated, err := s.SyncOnce(context.Background())
	if err != nil || !updated {
		t.Fatalf("first sync: expected updated=true, err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(s.ActiveContentDir(), "posts", "hello.md"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("unexpected extracted file: %q (err=%v)", string(got), err)
	}
	if rel.count.Load() != 1 {
		t.Fatalf("expected 1 reload call, got %d", rel.count.Load())
	}

	updated, err = s.SyncOnce(context.Background())
	if err != nil || updated {
		t.Fatalf("second sync with matching ETag: expected updated=false, err=%v", err)
	}
}

func TestSyncOnce_RejectsZipSlipAndSymlinks(t *testing.T) {
	zipSlip := buildTarGz(t, []tar.Header{
		{Name: "repo/content/../../evil.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("evil")})

	symlinkTar := buildTarGz(t, []tar.Header{
		{Name: "repo/content/posts/link.md", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
	}, [][]byte{nil})

	for name, archive := range map[string][]byte{"zip-slip": zipSlip, "symlink": symlinkTar} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"bad"`)
				_, _ = w.Write(archive)
			}))
			defer srv.Close()

			s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
			if _, err := s.SyncOnce(context.Background()); err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
		})
	}
}

func TestSyncOnce_PreservesActiveContentOnCorruptTarball(t *testing.T) {
	seedDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(seedDir, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "posts", "seed.md"), []byte("seed-ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"corrupt"`)
		_, _ = w.Write([]byte("not-a-valid-gzip-stream"))
	}))
	defer srv.Close()

	s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
	if err := s.SeedIfEmpty(seedDir); err != nil {
		t.Fatalf("SeedIfEmpty: %v", err)
	}
	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatalf("expected error on corrupt gzip")
	}
	got, err := os.ReadFile(filepath.Join(s.ActiveContentDir(), "posts", "seed.md"))
	if err != nil || string(got) != "seed-ok" {
		t.Fatalf("expected seed content preserved after failed sync, got %q (err=%v)", string(got), err)
	}
}

func TestFluxTarballSyncer_Start(t *testing.T) {
	payload := buildTarGz(t, []tar.Header{
		{Name: "content/posts/test.md", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("test")})

	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		w.Header().Set("ETag", fmt.Sprintf(`"rev-%d"`, count))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	rel := &mockReloader{}
	s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), rel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx, 20*time.Millisecond)

	// Wait until at least 2 syncs have executed
	deadline := time.Now().Add(2 * time.Second)
	for rel.count.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for background sync ticks, got %d reloads", rel.count.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	currentCount := rel.count.Load()
	time.Sleep(50 * time.Millisecond)
	if rel.count.Load() != currentCount {
		t.Fatalf("expected syncer to stop after context cancellation, got %d vs %d", rel.count.Load(), currentCount)
	}
}

func TestFluxTarballSyncer_FileSizeLimit(t *testing.T) {
	bigContent := bytes.Repeat([]byte("a"), 11*1024*1024)
	tooLargeTar := buildTarGz(t, []tar.Header{
		{Name: "content/big.iso", Mode: 0o644, Size: int64(len(bigContent)), Typeflag: tar.TypeReg},
	}, [][]byte{bigContent})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"too-large"`)
		_, _ = w.Write(tooLargeTar)
	}))
	defer srv.Close()

	s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatalf("expected error for file exceeding 10 MiB, got nil")
	}
}

func TestFluxTarballSyncer_TotalSizeLimit(t *testing.T) {
	chunk := bytes.Repeat([]byte("b"), 9*1024*1024) // 9 MiB per file (< 10 MiB)
	var hdrs []tar.Header
	var bodies [][]byte
	for i := 0; i < 6; i++ { // 6 * 9 MiB = 54 MiB (> 50 MiB total limit)
		hdrs = append(hdrs, tar.Header{
			Name:     fmt.Sprintf("content/file%d.bin", i),
			Mode:     0o644,
			Size:     int64(len(chunk)),
			Typeflag: tar.TypeReg,
		})
		bodies = append(bodies, chunk)
	}

	tooLargeTar := buildTarGz(t, hdrs, bodies)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"total-too-large"`)
		_, _ = w.Write(tooLargeTar)
	}))
	defer srv.Close()

	s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatalf("expected error for total size exceeding 50 MiB, got nil")
	}
}

func TestFluxTarballSyncer_ReloadErrorRollback(t *testing.T) {
	payload := buildTarGz(t, []tar.Header{
		{Name: "content/posts/broken.md", Mode: 0o644, Size: 6, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("broken")})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"broken"`)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	workDir := t.TempDir()
	seedDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(seedDir, "seed.txt"), []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	relErr := errors.New("indexing failed")
	rel := &mockReloader{err: relErr}
	s := syncer.NewFluxTarballSyncer(srv.URL, workDir, rel)
	if err := s.SeedIfEmpty(seedDir); err != nil {
		t.Fatal(err)
	}
	initialActive := s.ActiveContentDir()

	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatalf("expected error when Reloader fails, got nil")
	}

	// Active content dir should still be initial seed dir
	if s.ActiveContentDir() != initialActive {
		t.Fatalf("expected active content dir rolled back to %q, got %q", initialActive, s.ActiveContentDir())
	}

	// Workspace should have no lingering rev-* staging directories
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "rev-") {
			t.Fatalf("expected rev-* directory to be cleaned up after error, found %s", entry.Name())
		}
	}
}

func TestFluxTarballSyncer_StagingCleanup(t *testing.T) {
	workDir := t.TempDir()
	seedDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(seedDir, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}

	var rev atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := rev.Add(1)
		payload := buildTarGz(t, []tar.Header{
			{Name: "content/posts/rev.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg},
		}, [][]byte{[]byte(fmt.Sprintf("%04d", current))})
		w.Header().Set("ETag", fmt.Sprintf(`"rev-%d"`, current))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	rel := &mockReloader{}
	s := syncer.NewFluxTarballSyncer(srv.URL, workDir, rel)
	if err := s.SeedIfEmpty(seedDir); err != nil {
		t.Fatal(err)
	}
	seedActive := s.ActiveContentDir()

	// First sync
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("sync 1 failed: %v", err)
	}
	firstActive := s.ActiveContentDir()
	if firstActive == seedActive {
		t.Fatalf("expected active dir to change from seed")
	}

	// Seed directory must still exist
	if _, err := os.Stat(seedActive); err != nil {
		t.Fatalf("seed directory was unexpectedly deleted: %v", err)
	}

	// Second sync
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("sync 2 failed: %v", err)
	}
	secondActive := s.ActiveContentDir()
	if secondActive == firstActive {
		t.Fatalf("expected active dir to change on sync 2")
	}

	// First rev directory must be cleaned up
	if _, err := os.Stat(firstActive); !os.IsNotExist(err) {
		t.Fatalf("expected previous staging dir %s to be deleted, stat err=%v", firstActive, err)
	}

	// Second rev directory and seed directory must exist
	if _, err := os.Stat(secondActive); err != nil {
		t.Fatalf("current staging dir %s does not exist: %v", secondActive, err)
	}
	if _, err := os.Stat(seedActive); err != nil {
		t.Fatalf("seed directory was deleted after sync 2: %v", err)
	}
}
