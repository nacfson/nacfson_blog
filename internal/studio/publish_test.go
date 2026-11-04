package studio

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// pushFromElsewhere simulates another machine pushing to origin/main.
func (e *env) pushFromElsewhere(t *testing.T, rel, data, msg string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "--quiet", e.origin, other)
	writeFile(t, filepath.Join(other, filepath.FromSlash(rel)), data)
	git(t, other, "add", "-A")
	git(t, other, "commit", "--quiet", "-m", msg)
	git(t, other, "push", "--quiet", "origin", "main")
}

func (e *env) originLog(t *testing.T) []string {
	t.Helper()
	return strings.Split(git(t, e.origin, "log", "--format=%s", "main"), "\n")
}

func TestPublish_HappyPath(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "hello", "Hello")
	if err := e.m.Write(ctx, "hello", []byte(post("Hello", "hello", "2026-11-04", "ai-agents"))); err != nil {
		t.Fatal(err)
	}

	var steps []string
	res, err := e.m.Publish(ctx, "hello", func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.Slug != "hello" || res.Branch != "main" || res.Path != d.Path || len(res.Commit) != 40 {
		t.Errorf("result = %+v", res)
	}
	wantSteps := "save fetch reset squash check-paths validate commit push cleanup"
	if got := strings.Join(steps, " "); got != wantSteps {
		t.Errorf("steps = %q, want %q", got, wantSteps)
	}

	// Exactly one squashed commit on origin/main, touching only the post.
	if log := e.originLog(t); log[0] != "content: publish hello" || log[1] != "seed" {
		t.Errorf("origin log = %v", log)
	}
	if files := git(t, e.origin, "show", "--name-only", "--format=", "main"); files != d.Path {
		t.Errorf("published files = %q", files)
	}
	if git(t, e.origin, "rev-parse", "main") != res.Commit {
		t.Error("result commit is not origin/main")
	}

	// Draft cleaned up; local remote-tracking ref refreshed; post now listed as published.
	if e.m.branchExists(ctx, "hello") || exists(d.Worktree) {
		t.Error("draft branch/worktree not removed")
	}
	posts, _ := e.m.PublishedPosts(ctx)
	if len(posts) != 2 || posts[0].Summary.Slug != "hello" {
		t.Errorf("published posts = %+v", posts)
	}
	// The author's main checkout is untouched (still at seed, clean).
	if git(t, e.repo, "log", "-1", "--format=%s", "HEAD") != "seed" || git(t, e.repo, "status", "--porcelain") != "" {
		t.Error("main checkout was modified")
	}

	// The publish worktree is reused for the next publish.
	e.create(t, "second", "Second")
	if _, err := e.m.Publish(ctx, "second", nil); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if log := e.originLog(t); log[0] != "content: publish second" {
		t.Errorf("origin log = %v", log)
	}
}

func TestPublish_UpdateOfPublishedPost(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d, err := e.m.EditPublished(ctx, "existing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Publish(ctx, d.Key, nil); !errors.Is(err, ErrNothingToPublish) {
		t.Fatalf("unchanged publish: %v, want ErrNothingToPublish", err)
	}
	_ = e.m.Write(ctx, d.Key, []byte(post("Existing v2", "existing", "2026-10-24", "cloud-native")))
	if _, err := e.m.Publish(ctx, d.Key, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := git(t, e.origin, "show", "main:"+existingPath); !strings.Contains(got, "Existing v2") {
		t.Errorf("remote content = %s", got)
	}
}

func TestPublish_RetriesOnceAfterRejectedPush(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "race", "Race")

	calls := 0
	e.m.beforePush = func() {
		calls++
		if calls == 1 {
			e.pushFromElsewhere(t, "content/posts/2026-11-01-other.md", post("Other", "other", "2026-11-01", "open-source"), "content: publish other")
		}
	}
	if _, err := e.m.Publish(ctx, "race", nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if calls != 2 {
		t.Errorf("push attempts = %d, want 2", calls)
	}
	if log := e.originLog(t); strings.Join(log, "|") != "content: publish race|content: publish other|seed" {
		t.Errorf("origin log = %v (must be linear, no force-push)", log)
	}
}

func TestPublish_SecondRejectionSurfacesAndKeepsDraft(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "unlucky", "Unlucky")

	n := 0
	e.m.beforePush = func() {
		n++
		e.pushFromElsewhere(t, "content/posts/2026-11-0"+string(rune('0'+n))+"-x.md",
			post("X", "x"+string(rune('0'+n)), "2026-11-01", "open-source"), "elsewhere")
	}
	_, err := e.m.Publish(ctx, "unlucky", nil)
	if !errors.Is(err, ErrPushRejected) {
		t.Fatalf("err = %v, want ErrPushRejected", err)
	}
	for _, s := range e.originLog(t) {
		if strings.Contains(s, "unlucky") {
			t.Fatal("draft reached origin despite rejection")
		}
	}
	if st := e.state(t, "unlucky"); st.State != StateSaved {
		t.Errorf("draft state = %s, want saved (kept for retry)", st.State)
	}
}

func TestPublish_ConflictAbortsCleanly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d, _ := e.m.EditPublished(ctx, "existing")
	_ = e.m.Write(ctx, d.Key, []byte(post("Mine", "existing", "2026-10-24", "cloud-native")))
	if _, err := e.m.Save(ctx, d.Key); err != nil {
		t.Fatal(err)
	}
	e.pushFromElsewhere(t, existingPath, post("Theirs", "existing", "2026-10-24", "cloud-native"), "edit elsewhere")
	before := git(t, e.origin, "rev-parse", "main")

	_, err := e.m.Publish(ctx, d.Key, nil)
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), existingPath) {
		t.Fatalf("err = %v, want ErrConflict naming the file", err)
	}
	if git(t, e.origin, "rev-parse", "main") != before {
		t.Error("remote changed")
	}
	if st := e.state(t, d.Key); st.State != StateSaved {
		t.Errorf("draft state = %s", st.State)
	}
	if out := git(t, e.m.publishWorktree(), "status", "--porcelain"); out != "" {
		t.Errorf("publish worktree left dirty: %q", out)
	}
}

func TestPublish_RejectsNonContentPaths(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "sneaky", "Sneaky")
	if _, err := e.m.Save(ctx, "sneaky"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(d.Worktree, "README.md"), "changed\n")
	git(t, d.Worktree, "commit", "--quiet", "-am", "touch readme")

	before := git(t, e.origin, "rev-parse", "main")
	if _, err := e.m.Publish(ctx, "sneaky", nil); !errors.Is(err, ErrNonContentPath) {
		t.Fatalf("err = %v, want ErrNonContentPath", err)
	}
	if git(t, e.origin, "rev-parse", "main") != before {
		t.Error("remote changed")
	}
}

func TestPublish_DuplicateSlugOnMainAborts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "clash", "Clash")
	if _, err := e.m.Save(ctx, "clash"); err != nil {
		t.Fatal(err)
	}
	// Someone else publishes a different file with the same slug first.
	e.pushFromElsewhere(t, "content/posts/2026-11-02-clash.md", post("Other", "clash", "2026-11-02", "open-source"), "content: publish clash elsewhere")
	before := git(t, e.origin, "rev-parse", "main")

	_, err := e.m.Publish(ctx, "clash", nil)
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), "duplicate post slug") {
		t.Fatalf("err = %v, want duplicate-slug ValidationError", err)
	}
	if git(t, e.origin, "rev-parse", "main") != before {
		t.Error("remote changed")
	}
}

func TestPublish_InvalidDraftCommitsNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "broken", "Broken")
	_ = e.m.Write(ctx, "broken", []byte(post("Broken", "broken", "2026-11-04", "nope")))
	before := git(t, e.origin, "rev-parse", "main")

	var ve *ValidationError
	if _, err := e.m.Publish(ctx, "broken", nil); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if git(t, e.origin, "rev-parse", "main") != before {
		t.Error("remote changed")
	}
}

func TestPlan(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "planned", "Planned")
	_ = e.m.Write(ctx, "planned", []byte(post("Planned", "planned", "2026-11-04", "ai-agents")+"\n1. ordered\n"))

	plan, err := e.m.Plan(ctx, "planned")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Unsaved || plan.Blocking || plan.Branch != "main" {
		t.Errorf("plan = %+v", plan)
	}
	if len(plan.Files) != 1 || plan.Files[0] != d.Path || !strings.Contains(plan.Diff, "+title: \"Planned\"") {
		t.Errorf("files=%v diff=%q", plan.Files, plan.Diff)
	}
	if find(plan.Issues, "Ordered lists") == nil {
		t.Errorf("issues = %+v", plan.Issues)
	}
	if !strings.Contains(strings.Join(plan.Steps, "\n"), "push origin HEAD:main") {
		t.Errorf("steps = %v", plan.Steps)
	}

	_ = e.m.Write(ctx, "planned", []byte(post("Planned", "existing", "2026-11-04", "ai-agents")))
	plan, _ = e.m.Plan(ctx, "planned")
	if !plan.Blocking || find(plan.Issues, "already used") == nil {
		t.Errorf("duplicate slug not blocking: %+v", plan.Issues)
	}
}
