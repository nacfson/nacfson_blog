package content_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
)

const validPost = "---\n" +
	"title: \"Running \"AI\" Agents: On K3s\"\n" +
	"slug: \"ai-agents-on-k3s\"\n" +
	"date: \"2026-11-04\"\n" +
	"topic: \"ai-agents\"\n" +
	"project: \"nacfson_pipeline\"\n" +
	"summary: \"Gateway pattern.\"\n" +
	"tags: [\"ai-agents\", \"openbao\"]\n" +
	"---\n" +
	"Body text.\n"

func TestValidatePost_Valid(t *testing.T) {
	post, err := content.ValidatePost([]byte(validPost), "2026-11-04-ai-agents-on-k3s.md")
	if err != nil {
		t.Fatalf("ValidatePost: %v", err)
	}
	if post.Slug != "ai-agents-on-k3s" || post.Topic != content.TopicAIAgents {
		t.Errorf("unexpected post: slug=%q topic=%q", post.Slug, post.Topic)
	}
	// Inner quotes are kept raw: the parser only strips the outer pair.
	if post.Title != `Running "AI" Agents: On K3s` {
		t.Errorf("title = %q", post.Title)
	}
	if len(post.Tags) != 2 || post.ReadTimeMinutes != 1 {
		t.Errorf("tags=%v readTime=%d", post.Tags, post.ReadTimeMinutes)
	}
	if post.MarkdownBody != "Body text.\n" {
		t.Errorf("body = %q", post.MarkdownBody)
	}
}

func TestValidatePost_AcceptsCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(validPost, "\n", "\r\n")
	if _, err := content.ValidatePost([]byte(crlf), "crlf.md"); err != nil {
		t.Fatalf("CRLF post rejected: %v", err)
	}
}

func TestValidatePost_Rejects(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(validPost, old, new, 1) }
	cases := []struct {
		name     string
		data     string
		wantErr  string
		wantSlug bool // expect errors.Is(err, ErrInvalidSlug)
	}{
		{"no frontmatter", "Body only\n", "missing leading frontmatter", false},
		{"unclosed frontmatter", "---\ntitle: \"x\"\n", "missing closing frontmatter", false},
		{"missing title", replace("title: \"Running \"AI\" Agents: On K3s\"\n", ""), "title is required", false},
		{"missing slug", replace("slug: \"ai-agents-on-k3s\"\n", ""), "slug is required", false},
		{"bad slug", replace("ai-agents-on-k3s", "../etc/passwd"), "invalid slug", true},
		{"slug with space", replace("ai-agents-on-k3s", "ai agents"), "invalid slug", true},
		{"missing date", replace("date: \"2026-11-04\"\n", ""), "date is required", false},
		{"missing topic", replace("topic: \"ai-agents\"\n", ""), "invalid topic", false},
		{"unknown topic", replace("\"ai-agents\"\nproject", "\"project-updates\"\nproject"), "invalid topic", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := content.ValidatePost([]byte(tc.data), "draft.md")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
			if tc.wantSlug != errors.Is(err, content.ErrInvalidSlug) {
				t.Errorf("errors.Is(err, ErrInvalidSlug) = %v, want %v", !tc.wantSlug, tc.wantSlug)
			}
			// Frontmatter errors name the file so the author can find it.
			if strings.Contains(tc.wantErr, "required") && !strings.Contains(err.Error(), "draft.md") {
				t.Errorf("error %q does not mention filename", err)
			}
		})
	}
}

func writeRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, "posts", name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestValidateDir_FixtureContent(t *testing.T) {
	root, err := filepath.Abs("../../tests/fixtures/content")
	if err != nil {
		t.Fatal(err)
	}
	if err := content.ValidateDir(root); err != nil {
		t.Fatalf("fixture content should validate: %v", err)
	}
}

func TestValidateDir_EmptyPostsDirIsValid(t *testing.T) {
	if err := content.ValidateDir(writeRoot(t, nil)); err != nil {
		t.Fatalf("empty posts dir: %v", err)
	}
}

func TestValidateDir_IgnoresNonMarkdown(t *testing.T) {
	root := writeRoot(t, map[string]string{
		"a.md":      validPost,
		".gitkeep":  "",
		"notes.txt": "not a post",
	})
	if err := content.ValidateDir(root); err != nil {
		t.Fatalf("ValidateDir: %v", err)
	}
}

func TestValidateDir_Rejects(t *testing.T) {
	t.Run("duplicate slug across files", func(t *testing.T) {
		root := writeRoot(t, map[string]string{"a.md": validPost, "b.md": validPost})
		err := content.ValidateDir(root)
		if err == nil || !strings.Contains(err.Error(), "duplicate post slug") {
			t.Fatalf("expected duplicate slug error, got %v", err)
		}
	})
	t.Run("one invalid post fails the whole index", func(t *testing.T) {
		bad := strings.Replace(validPost, "ai-agents-on-k3s", "other", 1)
		bad = strings.Replace(bad, "topic: \"ai-agents\"", "topic: \"nope\"", 1)
		root := writeRoot(t, map[string]string{"good.md": validPost, "bad.md": bad})
		err := content.ValidateDir(root)
		if err == nil || !strings.Contains(err.Error(), "bad.md") {
			t.Fatalf("expected error naming bad.md, got %v", err)
		}
	})
	t.Run("missing posts directory", func(t *testing.T) {
		if err := content.ValidateDir(t.TempDir()); err == nil {
			t.Fatal("expected error for missing posts/")
		}
	})
}

// ValidateDir must agree with what the server would do on Reload.
func TestValidateDir_MatchesFolderStore(t *testing.T) {
	other := strings.Replace(validPost, "ai-agents-on-k3s", "second-post", 1)
	badTopic := strings.Replace(other, "topic: \"ai-agents\"", "topic: \"pillar\"", 1)
	cases := map[string]map[string]string{
		"valid":     {"a.md": validPost, "b.md": other},
		"duplicate": {"a.md": validPost, "b.md": validPost},
		"bad topic": {"a.md": validPost, "b.md": badTopic},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeRoot(t, files)
			validateErr := content.ValidateDir(root)
			_, storeErr := content.NewFolderStore(func() string { return root })
			if (validateErr == nil) != (storeErr == nil) {
				t.Fatalf("ValidateDir err=%v but NewFolderStore err=%v", validateErr, storeErr)
			}
		})
	}
}
