package content_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
)

func TestFolderStore_LoadsAndFiltersPosts(t *testing.T) {
	root, err := filepath.Abs("../../content")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore failed: %v", err)
	}

	posts, err := store.ListPosts(context.Background(), "", "")
	if err != nil || len(posts) == 0 {
		t.Fatalf("expected at least 1 seeded post, got %d (err=%v)", len(posts), err)
	}
	if posts[0].Slug != "cilium-zero-plaintext-k3s" {
		t.Errorf("expected slug cilium-zero-plaintext-k3s, got %q", posts[0].Slug)
	}

	filtered, err := store.ListPosts(context.Background(), "project-updates", "Cilium")
	if err != nil || len(filtered) != 1 {
		t.Fatalf("expected 1 post matching pillar+tag, got %d (err=%v)", len(filtered), err)
	}

	post, err := store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if post.MarkdownBody == "" {
		t.Errorf("expected non-empty MarkdownBody")
	}

	matrix, err := store.GetCompetencyMatrix(context.Background())
	if err != nil || len(matrix) == 0 {
		t.Fatalf("expected non-empty competency matrix, err=%v", err)
	}
}

func TestFolderStore_RejectsInvalidSlugsAndMissingPosts(t *testing.T) {
	root, _ := filepath.Abs("../../content")
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}
	for _, bad := range []string{"../secret", "a/b", `a\b`, "bad\x00slug", ""} {
		if _, err := store.GetPost(context.Background(), bad); !errors.Is(err, content.ErrInvalidSlug) {
			t.Errorf("slug %q: expected ErrInvalidSlug, got %v", bad, err)
		}
	}
	if _, err := store.GetPost(context.Background(), "non-existent-post"); !errors.Is(err, content.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestFolderStore_RejectsInvalidFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	badMD := "---\ntitle: \"Missing Slug and Bad Pillar\"\ndate: \"2026-10-24\"\npillar: \"invalid-pillar\"\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(tmp, "posts", "bad.md"), []byte(badMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := content.NewFolderStore(func() string { return tmp }); err == nil {
		t.Fatalf("expected NewFolderStore to fail on invalid frontmatter")
	}
}

func TestFolderStore_Sorting(t *testing.T) {
	tmp := t.TempDir()
	postsDir := filepath.Join(tmp, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	post1 := "---\ntitle: \"Older Post\"\nslug: \"older-post\"\ndate: \"2026-01-01\"\npillar: \"tech-opinions\"\n---\nBody 1\n"
	post2 := "---\ntitle: \"Newer Post\"\nslug: \"newer-post\"\ndate: \"2026-10-20\"\npillar: \"tech-opinions\"\n---\nBody 2\n"
	post3 := "---\ntitle: \"Middle Post\"\nslug: \"middle-post\"\ndate: \"2026-05-15\"\npillar: \"tech-opinions\"\n---\nBody 3\n"

	if err := os.WriteFile(filepath.Join(postsDir, "p1.md"), []byte(post1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postsDir, "p2.md"), []byte(post2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postsDir, "p3.md"), []byte(post3), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := content.NewFolderStore(func() string { return tmp })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}

	posts, err := store.ListPosts(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(posts))
	}

	expectedOrder := []string{"newer-post", "middle-post", "older-post"}
	for i, exp := range expectedOrder {
		if posts[i].Slug != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, posts[i].Slug)
		}
	}
}

func TestFolderStore_ConcurrentAccessAndReload(t *testing.T) {
	root, err := filepath.Abs("../../content")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				_, _ = store.ListPosts(context.Background(), "", "")
				_, _ = store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
				_, _ = store.GetCompetencyMatrix(context.Background())
			}
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = store.Reload()
			}
			done <- true
		}()
	}

	for i := 0; i < 13; i++ {
		<-done
	}
}

func TestFolderStore_CompetencyMatrixCachingAndValidation(t *testing.T) {
	tmp := t.TempDir()
	postsDir := filepath.Join(tmp, "posts")
	compDir := filepath.Join(tmp, "competencies")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(compDir, 0o755); err != nil {
		t.Fatal(err)
	}

	postMD := "---\ntitle: \"Sample\"\nslug: \"sample\"\ndate: \"2026-10-24\"\npillar: \"tech-opinions\"\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(postsDir, "sample.md"), []byte(postMD), 0o644); err != nil {
		t.Fatal(err)
	}

	matrixPath := filepath.Join(compDir, "matrix.json")
	if err := os.WriteFile(matrixPath, []byte(`{"domains":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := content.NewFolderStore(func() string { return tmp })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}

	// 1. Initial read succeeds
	data, err := store.GetCompetencyMatrix(context.Background())
	if err != nil || string(data) != `{"domains":[]}` {
		t.Fatalf("expected matrix data, got err=%v data=%s", err, string(data))
	}

	// 2. Remove file from disk; GetCompetencyMatrix should still serve from in-memory cache
	if err := os.Remove(matrixPath); err != nil {
		t.Fatal(err)
	}
	cachedData, err := store.GetCompetencyMatrix(context.Background())
	if err != nil || string(cachedData) != `{"domains":[]}` {
		t.Fatalf("expected cached matrix data after file deletion, got err=%v", err)
	}

	// 3. Reload after deletion records error on GetCompetencyMatrix without failing Reload
	if err := store.Reload(); err != nil {
		t.Fatalf("Reload should not fail when matrix.json is missing: %v", err)
	}
	if _, err := store.GetCompetencyMatrix(context.Background()); err == nil {
		t.Fatalf("expected error from GetCompetencyMatrix when matrix.json is missing")
	}

	// 4. Reload with invalid JSON fails
	if err := os.WriteFile(matrixPath, []byte(`{ not valid json `), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(); err == nil {
		t.Fatalf("expected Reload to fail on invalid matrix JSON")
	}
}


