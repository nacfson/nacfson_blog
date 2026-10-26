package content_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
)

func TestFolderStore_LoadsAndFiltersPosts(t *testing.T) {
	root, err := filepath.Abs("../../tests/fixtures/content")
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

	filtered, err := store.ListPosts(context.Background(), "cloud-native", "Cilium")
	if err != nil || len(filtered) != 1 {
		t.Fatalf("expected 1 post matching topic+tag, got %d (err=%v)", len(filtered), err)
	}

	post, err := store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if post.MarkdownBody == "" {
		t.Errorf("expected non-empty MarkdownBody")
	}
}

func TestFolderStore_RejectsInvalidSlugsAndMissingPosts(t *testing.T) {
	root, _ := filepath.Abs("../../tests/fixtures/content")
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
	badMD := "---\ntitle: \"Missing Slug and Bad Topic\"\ndate: \"2026-10-24\"\ntopic: \"invalid-topic\"\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(tmp, "posts", "bad.md"), []byte(badMD), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := content.NewFolderStore(func() string { return tmp })
	if err == nil {
		t.Fatalf("expected NewFolderStore to fail on invalid frontmatter")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Errorf("expected error message to contain filename bad.md, got %v", err)
	}
}

func TestFolderStore_GetPost_TagsSliceIsolation(t *testing.T) {
	root, err := filepath.Abs("../../tests/fixtures/content")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore failed: %v", err)
	}

	post1, err := store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	origCount := len(post1.Tags)
	post1.Tags = append(post1.Tags, "MutatedTag")

	post2, err := store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if len(post2.Tags) != origCount {
		t.Fatalf("modifying returned tags slice mutated the store: expected %d, got %d", origCount, len(post2.Tags))
	}
}

func TestFolderStore_Sorting(t *testing.T) {
	tmp := t.TempDir()
	postsDir := filepath.Join(tmp, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	post1 := "---\ntitle: \"Older Post\"\nslug: \"older-post\"\ndate: \"2026-01-01\"\ntopic: \"ai-agents\"\n---\nBody 1\n"
	post2 := "---\ntitle: \"Newer Post\"\nslug: \"newer-post\"\ndate: \"2026-10-20\"\ntopic: \"ai-agents\"\n---\nBody 2\n"
	post3 := "---\ntitle: \"Middle Post\"\nslug: \"middle-post\"\ndate: \"2026-05-15\"\ntopic: \"ai-agents\"\n---\nBody 3\n"

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
	root, err := filepath.Abs("../../tests/fixtures/content")
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

func TestFolderStore_EmptyPostsDirectory(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := content.NewFolderStore(func() string { return tmp })
	if err != nil {
		t.Fatalf("NewFolderStore on empty posts dir: %v", err)
	}
	posts, err := store.ListPosts(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts, got %d", len(posts))
	}
}

func TestFolderStore_TopicValidationAndFiltering(t *testing.T) {
	mk := func(t *testing.T, topicLine string) error {
		t.Helper()
		tmp := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmp, "posts"), 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\ntitle: \"T\"\nslug: \"t\"\ndate: \"2026-10-25\"\n" + topicLine + "---\nBody\n"
		if err := os.WriteFile(filepath.Join(tmp, "posts", "t.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := content.NewFolderStore(func() string { return tmp })
		return err
	}

	for _, topic := range []string{"cloud-native", "zero-trust", "ai-agents", "open-source"} {
		if err := mk(t, "topic: \""+topic+"\"\n"); err != nil {
			t.Errorf("topic %q should be valid, got %v", topic, err)
		}
	}
	if err := mk(t, ""); err == nil || !strings.Contains(err.Error(), "topic") {
		t.Errorf("expected missing topic to fail with a topic error, got %v", err)
	}
	if err := mk(t, "pillar: \"project-updates\"\n"); err == nil {
		t.Errorf("expected legacy pillar-only frontmatter to fail validation")
	}

	tmp := t.TempDir()
	postsDir := filepath.Join(tmp, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := "---\ntitle: \"A\"\nslug: \"a\"\ndate: \"2026-10-25\"\ntopic: \"zero-trust\"\n---\nA\n"
	b := "---\ntitle: \"B\"\nslug: \"b\"\ndate: \"2026-10-24\"\ntopic: \"open-source\"\n---\nB\n"
	_ = os.WriteFile(filepath.Join(postsDir, "a.md"), []byte(a), 0o644)
	_ = os.WriteFile(filepath.Join(postsDir, "b.md"), []byte(b), 0o644)
	store, err := content.NewFolderStore(func() string { return tmp })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}
	got, err := store.ListPosts(context.Background(), "zero-trust", "")
	if err != nil || len(got) != 1 || got[0].Slug != "a" || got[0].Topic != content.TopicZeroTrust {
		t.Fatalf("expected only post a with topic zero-trust, got %+v (err=%v)", got, err)
	}
}
