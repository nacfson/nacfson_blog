package syncer_test

import (
	"archive/tar"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/syncer"
)

// fakeFlux mimics source-controller: tarballs are served only at
// /<revision>.tar.gz (a "latest.tar.gz" path 404s), and the Kubernetes API
// reports the current revision in status.artifact.
type fakeFlux struct {
	t         *testing.T
	rev       atomic.Value // string
	downloads atomic.Int32
	apiCalls  atomic.Int32
	srv       *httptest.Server
	bodies    map[string][]byte
}

func newFakeFlux(t *testing.T) *fakeFlux {
	f := &fakeFlux{t: t, bodies: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/source.toolkit.fluxcd.io/v1/namespaces/flux-system/gitrepositories/blog", func(w http.ResponseWriter, r *http.Request) {
		f.apiCalls.Add(1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		rev := f.rev.Load().(string)
		if rev == "" {
			_, _ = w.Write([]byte(`{"status":{}}`))
			return
		}
		fmt.Fprintf(w, `{"status":{"artifact":{"url":"%s/artifacts/%s.tar.gz","revision":"main@sha1:%s"}}}`, f.srv.URL, rev, rev)
	})
	mux.HandleFunc("/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/artifacts/")
		body, ok := f.bodies[strings.TrimSuffix(name, ".tar.gz")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f.downloads.Add(1)
		_, _ = w.Write(body)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	f.rev.Store("")
	return f
}

func (f *fakeFlux) publish(rev, post string) {
	f.bodies[rev] = buildTarGz(f.t, []tar.Header{
		{Name: "repo/content/posts/p.md", Mode: 0o644, Size: int64(len(post)), Typeflag: tar.TypeReg},
	}, [][]byte{[]byte(post)})
	f.rev.Store(rev)
}

func newResolvingSyncer(t *testing.T, f *fakeFlux) *syncer.FluxTarballSyncer {
	s := syncer.NewFluxTarballSyncer("", t.TempDir(), &mockReloader{})
	s.SetResolver(syncer.GitRepositoryResolver(f.srv.URL, "flux-system", "blog",
		func() (string, error) { return "tok", nil }, f.srv.Client()))
	return s
}

func readPost(t *testing.T, s *syncer.FluxTarballSyncer) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.ActiveContentDir(), "posts", "p.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestResolver_FollowsRevisionNamedArtifactsWithoutRedownloading(t *testing.T) {
	f := newFakeFlux(t)
	f.publish("aaa", "first")
	s := newResolvingSyncer(t, f)

	if updated, err := s.SyncOnce(context.Background()); err != nil || !updated {
		t.Fatalf("first sync: updated=%v err=%v", updated, err)
	}
	if got := readPost(t, s); got != "first" {
		t.Fatalf("got %q", got)
	}

	updated, err := s.SyncOnce(context.Background())
	if err != nil || updated {
		t.Fatalf("unchanged revision: updated=%v err=%v", updated, err)
	}
	if n := f.downloads.Load(); n != 1 {
		t.Fatalf("unchanged revision must not download again, downloads=%d", n)
	}

	f.publish("bbb", "second")
	if updated, err := s.SyncOnce(context.Background()); err != nil || !updated {
		t.Fatalf("new revision: updated=%v err=%v", updated, err)
	}
	if got := readPost(t, s); got != "second" {
		t.Fatalf("got %q", got)
	}
	if n := f.downloads.Load(); n != 2 {
		t.Fatalf("downloads=%d, want 2", n)
	}
}

func TestResolver_ErrorsLeaveCurrentContent(t *testing.T) {
	f := newFakeFlux(t) // no artifact stored yet
	s := newResolvingSyncer(t, f)
	if _, err := s.SyncOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "no stored artifact") {
		t.Fatalf("want 'no stored artifact' error, got %v", err)
	}

	bad := syncer.NewFluxTarballSyncer("", t.TempDir(), &mockReloader{})
	bad.SetResolver(syncer.GitRepositoryResolver(f.srv.URL, "flux-system", "blog",
		func() (string, error) { return "wrong", nil }, f.srv.Client()))
	if _, err := bad.SyncOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want 403 error, got %v", err)
	}
}

func TestResolver_FailedReloadRetriesSameRevision(t *testing.T) {
	f := newFakeFlux(t)
	f.publish("aaa", "first")
	rel := &mockReloader{err: fmt.Errorf("boom")}
	s := syncer.NewFluxTarballSyncer("", t.TempDir(), rel)
	s.SetResolver(syncer.GitRepositoryResolver(f.srv.URL, "flux-system", "blog",
		func() (string, error) { return "tok", nil }, f.srv.Client()))

	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatal("expected reload error")
	}
	rel.err = nil
	if updated, err := s.SyncOnce(context.Background()); err != nil || !updated {
		t.Fatalf("retry after reload failure: updated=%v err=%v", updated, err)
	}
}

func TestInClusterGitRepositoryResolver_RejectsBadRef(t *testing.T) {
	for _, ref := range []string{"", "nameonly", "/x", "x/"} {
		if _, err := syncer.InClusterGitRepositoryResolver(ref); err == nil {
			t.Errorf("ref %q: expected error", ref)
		}
	}
}
