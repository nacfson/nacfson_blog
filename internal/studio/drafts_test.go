package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------- fixtures ----------

func post(title, slug, date, topic string) string {
	return fmt.Sprintf("---\ntitle: %q\nslug: %q\ndate: %q\ntopic: %q\nsummary: \"s\"\ntags: [\"t\"]\n---\nBody of %s.\n",
		title, slug, date, topic, slug)
}

const existingPath = "content/posts/2026-10-24-existing.md"

// isolateGit keeps the user's global/system git config (signing, hooks,
// templates) out of the tests and supplies a commit identity.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Studio Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "studio@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Studio Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "studio@example.invalid")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, p, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

type env struct {
	origin string // bare remote
	repo   string // author's main checkout
	m      *Manager
	events *eventLog
}

type eventLog struct {
	mu sync.Mutex
	ev []Event
}

func (l *eventLog) add(e Event) { l.mu.Lock(); l.ev = append(l.ev, e); l.mu.Unlock() }
func (l *eventLog) commands() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.ev {
		out = append(out, strings.Join(e.Args, " "))
	}
	return out
}

// newEnv creates origin (bare, branch main) with one published post, clones
// it as the author's repo, and returns a Manager for that clone.
func newEnv(t *testing.T) *env {
	t.Helper()
	isolateGit(t)
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	seed := filepath.Join(base, "seed")
	repo := filepath.Join(base, "repo")

	git(t, base, "init", "--quiet", "--bare", "-b", "main", origin)
	git(t, base, "init", "--quiet", "-b", "main", seed)
	writeFile(t, filepath.Join(seed, existingPath), post("Existing", "existing", "2026-10-24", "cloud-native"))
	writeFile(t, filepath.Join(seed, "content/assets/.gitkeep"), "")
	writeFile(t, filepath.Join(seed, "README.md"), "seed\n")
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "--quiet", "-m", "seed")
	git(t, seed, "push", "--quiet", origin, "main")
	git(t, base, "clone", "--quiet", origin, repo)

	ev := &eventLog{}
	m, err := NewManager(context.Background(), Config{
		Repo: repo, Log: ev.add,
		Now: func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return &env{origin: origin, repo: m.Repo(), m: m, events: ev}
}

func (e *env) create(t *testing.T, key, title string) *Draft {
	t.Helper()
	d, err := e.m.Create(context.Background(), CreateOptions{
		Key:     key,
		Path:    PostPath("2026-11-04", key),
		Content: []byte(post(title, key, "2026-11-04", "ai-agents")),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return d
}

func (e *env) state(t *testing.T, key string) *DraftStatus {
	t.Helper()
	st, err := e.m.Status(context.Background(), key)
	if err != nil {
		t.Fatalf("Status(%s): %v", key, err)
	}
	return st
}

// ---------- tests ----------

func TestNewManager_RejectsNonRoot(t *testing.T) {
	e := newEnv(t)
	if _, err := NewManager(context.Background(), Config{Repo: filepath.Join(e.repo, "content")}); err == nil {
		t.Fatal("expected error for a subdirectory")
	}
	if _, err := NewManager(context.Background(), Config{Repo: t.TempDir()}); err == nil {
		t.Fatal("expected error for a non-repository")
	}
}

func TestCreate_BranchWorktreeAndScaffold(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "agents", "Agents")

	if d.Branch != "studio/agents" || d.Worktree != filepath.Join(e.repo, ".worktrees", "studio-agents") {
		t.Fatalf("unexpected draft %+v", d)
	}
	if got := git(t, d.Worktree, "rev-parse", "--abbrev-ref", "HEAD"); got != "studio/agents" {
		t.Errorf("worktree HEAD = %s", got)
	}
	// --no-track: drafts never get an upstream, so they can't be pushed by accident.
	if out, err := exec.Command("git", "-C", e.repo, "config", "--get", "branch.studio/agents.remote").Output(); err == nil {
		t.Errorf("draft branch has upstream %q", out)
	}
	if _, err := os.Stat(filepath.Join(d.Worktree, filepath.FromSlash(d.Path))); err != nil {
		t.Fatalf("scaffold not written: %v", err)
	}
	if st := e.state(t, "agents"); st.State != StateNew || st.Ahead != 0 {
		t.Errorf("state = %s ahead=%d, want new/0", st.State, st.Ahead)
	}

	// Author's main checkout is untouched.
	if got := git(t, e.repo, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("main checkout moved to %s", got)
	}
	if got := git(t, e.repo, "status", "--porcelain"); got != "" {
		t.Errorf("main checkout dirty: %q", got)
	}

	got, err := e.m.Get(ctx, "agents")
	if err != nil || *got != *d {
		t.Errorf("Get = %+v, %v; want %+v", got, err, d)
	}
}

func TestCreate_UniqueAndReservedKeys(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "dup", "One")

	d2, err := e.m.Create(ctx, CreateOptions{Key: "dup", Path: PostPath("2026-11-04", "dup-two"), Content: []byte("x")})
	if err != nil || d2.Key != "dup-2" {
		t.Fatalf("second draft = %+v, %v; want key dup-2", d2, err)
	}
	d3, err := e.m.Create(ctx, CreateOptions{Key: "publish", Path: PostPath("2026-11-04", "publish"), Content: []byte("x")})
	if err != nil || d3.Key != "publish-2" {
		t.Fatalf("reserved key: %+v, %v; want publish-2", d3, err)
	}
}

func TestCreate_RejectsBadInput(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	cases := []CreateOptions{
		{Key: "../x", Path: PostPath("2026-11-04", "x")},
		{Key: "-x", Path: PostPath("2026-11-04", "x")},
		{Key: "ok", Path: "content/assets/x.md"},
		{Key: "ok", Path: "content/posts/../../x.md"},
		{Key: "ok", Path: "content/posts/sub/x.md"},
		{Key: "ok", Path: "content/posts/x.txt"},
		{Key: "ok", Path: existingPath}, // would clobber a published post
	}
	for _, c := range cases {
		if _, err := e.m.Create(ctx, c); err == nil {
			t.Errorf("Create(%+v): expected error", c)
		}
	}
	keys, _ := e.m.draftKeys(ctx)
	if len(keys) != 0 {
		t.Errorf("failed creates left branches behind: %v", keys)
	}
}

func TestWriteSave_StatusTransitions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "flow", "Flow")

	if err := e.m.Write(ctx, "flow", []byte(post("Flow v2", "flow", "2026-11-04", "ai-agents"))); err != nil {
		t.Fatal(err)
	}
	if st := e.state(t, "flow"); st.State != StateUnsaved {
		t.Fatalf("after edit: %s, want unsaved", st.State)
	}
	if b, _ := e.m.Read(ctx, "flow"); !strings.Contains(string(b), "Flow v2") {
		t.Fatalf("Read returned %q", b)
	}

	res, err := e.m.Save(ctx, "flow")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if res.Commit == "" || res.Path != d.Path {
		t.Errorf("SaveResult = %+v", res)
	}
	if msg := git(t, d.Worktree, "log", "-1", "--format=%s"); msg != "draft(flow): save 12:00:00" {
		t.Errorf("commit message %q", msg)
	}
	if st := e.state(t, "flow"); st.State != StateSaved || st.Ahead != 1 {
		t.Errorf("after save: %s ahead=%d, want saved/1", st.State, st.Ahead)
	}
	if _, err := e.m.Save(ctx, "flow"); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("second save: %v, want ErrNothingToCommit", err)
	}

	// Draft commits never reach the remote.
	if out := git(t, e.origin, "branch", "--list", "studio/*"); out != "" {
		t.Errorf("origin has draft branches: %s", out)
	}
	// No temp files leaked into the worktree.
	if out := git(t, d.Worktree, "status", "--porcelain"); out != "" {
		t.Errorf("worktree dirty after save: %q", out)
	}
}

func TestSave_InvalidPostCommitsNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "bad", "Bad")

	cases := map[string]string{
		"bad topic":      post("Bad", "bad", "2026-11-04", "pillar"),
		"no frontmatter": "just text\n",
		"loose date":     post("Bad", "bad", "Nov 4", "ai-agents"),
		"duplicate slug": post("Bad", "existing", "2026-11-05", "ai-agents"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := e.m.Write(ctx, "bad", []byte(data)); err != nil {
				t.Fatal(err)
			}
			_, err := e.m.Save(ctx, "bad")
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Save: %v, want ValidationError", err)
			}
			if st := e.state(t, "bad"); st.Ahead != 0 {
				t.Errorf("commit created despite validation error (ahead=%d)", st.Ahead)
			}
		})
	}
}

func TestSave_SlugAndDateChangeRenamesFile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "old-slug", "Rename")
	if _, err := e.m.Save(ctx, "old-slug"); err != nil {
		t.Fatal(err)
	}

	if err := e.m.Write(ctx, "old-slug", []byte(post("Rename", "new-slug", "2026-11-05", "ai-agents"))); err != nil {
		t.Fatal(err)
	}
	res, err := e.m.Save(ctx, "old-slug")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := "content/posts/2026-11-05-new-slug.md"
	if res.Path != want {
		t.Fatalf("path = %s, want %s", res.Path, want)
	}
	if _, err := os.Stat(filepath.Join(d.Worktree, filepath.FromSlash(d.Path))); !os.IsNotExist(err) {
		t.Errorf("old file still present: %v", err)
	}
	got, _ := e.m.Get(ctx, "old-slug")
	if got.Path != want || got.Branch != "studio/old-slug" {
		t.Errorf("Get after rename = %+v (branch name must stay fixed)", got)
	}
	if out := git(t, d.Worktree, "show", "--name-status", "-M", "--format=", "HEAD"); !strings.HasPrefix(out, "R") {
		t.Errorf("expected a rename in the commit, got %q", out)
	}
}

func TestEditPublished(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	if _, err := e.m.EditPublished(ctx, "missing"); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("missing post: %v", err)
	}
	d, err := e.m.EditPublished(ctx, "existing")
	if err != nil {
		t.Fatalf("EditPublished: %v", err)
	}
	if d.Path != existingPath {
		t.Fatalf("path = %s", d.Path)
	}
	if st := e.state(t, d.Key); st.State != StateLive {
		t.Errorf("fresh edit: %s, want live", st.State)
	}
	if _, err := e.m.EditPublished(ctx, "existing"); !errors.Is(err, ErrDraftExists) {
		t.Errorf("second edit: %v, want ErrDraftExists", err)
	}

	if err := e.m.Write(ctx, d.Key, []byte(post("Existing v2", "existing", "2026-10-24", "cloud-native"))); err != nil {
		t.Fatal(err)
	}
	if st := e.state(t, d.Key); st.State != StateUnsaved {
		t.Errorf("edited: %s, want unsaved", st.State)
	}
	if _, err := e.m.Save(ctx, d.Key); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if st := e.state(t, d.Key); st.State != StateSaved || st.Ahead != 1 {
		t.Errorf("saved: %s/%d, want saved/1", st.State, st.Ahead)
	}
}

func TestPublishedPosts(t *testing.T) {
	e := newEnv(t)
	posts, err := e.m.PublishedPosts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Path != existingPath || posts[0].Summary.Slug != "existing" {
		t.Fatalf("PublishedPosts = %+v", posts)
	}
}

func TestDiscard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "gone", "Gone")
	_ = e.m.Write(ctx, "gone", []byte("unsaved work"))

	if err := e.m.Discard(ctx, "gone"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if exists(d.Worktree) {
		t.Error("worktree still exists")
	}
	if e.m.branchExists(ctx, "gone") {
		t.Error("branch still exists")
	}
	if v := e.m.configGet(ctx, "gone", cfgPath); v != "" {
		t.Errorf("branch config left behind: %q", v)
	}
	if err := e.m.Discard(ctx, "gone"); !errors.Is(err, ErrDraftNotFound) {
		t.Errorf("second discard: %v", err)
	}
}

func TestList(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "b-draft", "B")
	e.create(t, "a-draft", "A")
	if _, err := e.m.Save(ctx, "a-draft"); err != nil {
		t.Fatal(err)
	}
	list, err := e.m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Key != "a-draft" || list[0].State != StateSaved || list[1].State != StateNew {
		t.Fatalf("List = %+v", list)
	}
}

func TestReconcile_RecreatesMissingWorktreeAndPath(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "recover", "Recover")
	if _, err := e.m.Save(ctx, "recover"); err != nil {
		t.Fatal(err)
	}

	// Simulate a deleted worktree directory and lost path config.
	if err := os.RemoveAll(d.Worktree); err != nil {
		t.Fatal(err)
	}
	git(t, e.repo, "config", "--unset", "branch.studio/recover.studiopath")
	if _, err := e.m.Get(ctx, "recover"); err == nil {
		t.Fatal("expected Get to fail before reconciliation")
	}

	m2, err := NewManager(ctx, Config{Repo: e.repo})
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := m2.Reconcile(ctx)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Reconcile: warnings=%v err=%v", warnings, err)
	}
	got, err := m2.Get(ctx, "recover")
	if err != nil {
		t.Fatalf("Get after reconcile: %v", err)
	}
	if got.Path != d.Path || !exists(filepath.Join(got.Worktree, filepath.FromSlash(got.Path))) {
		t.Errorf("reconciled draft = %+v", got)
	}
}

func TestReconcile_WarnsOnUnregisteredDirectory(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	git(t, e.repo, "branch", "studio/orphan", "origin/main")
	writeFile(t, filepath.Join(e.repo, ".worktrees", "studio-orphan", "stray.txt"), "x")

	warnings, err := e.m.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not a registered worktree") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestEvents_LogMutationsOnly(t *testing.T) {
	e := newEnv(t)
	e.create(t, "logged", "Logged")
	if _, err := e.m.Save(context.Background(), "logged"); err != nil {
		t.Fatal(err)
	}
	cmds := strings.Join(e.events.commands(), "\n")
	for _, want := range []string{"fetch origin main", "worktree add --no-track -b studio/logged", "add -A -- content", "commit --quiet -m draft(logged): save 12:00:00"} {
		if !strings.Contains(cmds, want) {
			t.Errorf("event log missing %q:\n%s", want, cmds)
		}
	}
	for _, noisy := range []string{"rev-parse", "status --porcelain", "config --get"} {
		if strings.Contains(cmds, noisy) {
			t.Errorf("read-only query %q was logged:\n%s", noisy, cmds)
		}
	}
}

func TestGitError_CarriesStderrAndExitCode(t *testing.T) {
	isolateGit(t)
	_, err := NewGit(nil).Run(context.Background(), t.TempDir(), "rev-parse", "--verify", "no-such-ref")
	var ge *GitError
	if !errors.As(err, &ge) || ge.ExitCode() != 128 || ge.Stderr == "" {
		t.Fatalf("err = %#v", err)
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(dir); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: %v, want ErrLocked", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	l2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	_ = l2.Release()

	// A lock left by a dead process is replaced.
	writeFile(t, filepath.Join(dir, ".worktrees", "studio.lock"), "999999999\n")
	l3, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("stale lock not replaced: %v", err)
	}
	_ = l3.Release()
}
