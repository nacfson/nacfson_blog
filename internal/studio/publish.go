package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nacfson/nacfson_blog/internal/content"
)

var (
	ErrNothingToPublish = errors.New("nothing to publish: draft matches the publish branch")
	ErrConflict         = errors.New("draft conflicts with the publish branch")
	ErrNonContentPath   = errors.New("publish commit would touch paths outside content/")
	ErrPushRejected     = errors.New("push rejected by remote")
)

// PublishPlan is what the Publish dialog shows before the author confirms.
type PublishPlan struct {
	Key      string   `json:"key"`
	Path     string   `json:"path"`
	Branch   string   `json:"publishBranch"`
	Unsaved  bool     `json:"unsaved"`
	Issues   []Issue  `json:"issues"`
	Blocking bool     `json:"blocking"` // true if any issue is an error
	Files    []string `json:"files"`    // changed paths vs the publish branch
	Diff     string   `json:"diff"`     // unified diff vs the publish branch
	Steps    []string `json:"steps"`    // commands Publish will run
}

// PublishResult reports a successful publish.
type PublishResult struct {
	Commit string `json:"commit"`
	Branch string `json:"publishBranch"`
	Path   string `json:"path"`
	Slug   string `json:"slug"`
}

// ProgressFunc is called as each publish step starts.
type ProgressFunc func(step string)

func (m *Manager) publishWorktree() string { return m.worktreeDir(publishKey) }

// Plan fetches the publish branch and describes what Publish would do.
func (m *Manager) Plan(ctx context.Context, key string) (*PublishPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	_ = m.fetch(ctx) // stale is acceptable for a preview; Publish fetches again (fatal there)

	st, err := m.status(ctx, d)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(d.Worktree, filepath.FromSlash(d.Path)))
	if err != nil {
		return nil, err
	}
	issues := Lint(data, path.Base(d.Path), m.lintOptions(ctx, d))

	plan := &PublishPlan{
		Key: key, Path: d.Path, Branch: m.cfg.PublishBranch,
		Unsaved: st.State == StateUnsaved || st.State == StateNew,
		Issues:  issues, Steps: m.steps(d, st.State == StateUnsaved || st.State == StateNew),
	}
	for _, i := range issues {
		if i.Severity == SevError {
			plan.Blocking = true
		}
	}

	// Tracked changes (committed + uncommitted) vs the publish ref...
	diff, err := m.git.Query(ctx, d.Worktree, "diff", "-M", m.PublishRef(), "--", "content")
	if err != nil {
		return nil, err
	}
	names, err := m.git.Query(ctx, d.Worktree, "diff", "-M", "--name-only", m.PublishRef(), "--", "content")
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(diff)
	plan.Files = splitLines(names)
	// ...plus untracked files, which `git diff` doesn't show.
	untracked, err := m.git.Query(ctx, d.Worktree, "ls-files", "--others", "--exclude-standard", "--", "content")
	if err != nil {
		return nil, err
	}
	for _, f := range splitLines(untracked) {
		out, derr := m.git.Query(ctx, d.Worktree, "diff", "--no-index", "--", os.DevNull, f)
		if derr != nil && exitCode(derr) != 1 { // 1 = differences found
			return nil, derr
		}
		b.WriteString(out)
		plan.Files = append(plan.Files, f)
	}
	plan.Diff = b.String()
	return plan, nil
}

func (m *Manager) steps(d *Draft, unsaved bool) []string {
	pw := ".worktrees/studio-" + publishKey
	ref := m.cfg.Remote + "/" + m.cfg.PublishBranch
	var s []string
	if unsaved {
		s = append(s, fmt.Sprintf("git -C .worktrees/studio-%s commit (save)", d.Key))
	}
	return append(s,
		fmt.Sprintf("git fetch %s %s", m.cfg.Remote, m.cfg.PublishBranch),
		fmt.Sprintf("git -C %s reset --hard %s", pw, ref),
		fmt.Sprintf("git -C %s merge --squash %s", pw, d.Branch),
		fmt.Sprintf("git -C %s diff --cached --name-only   # must be content/** only", pw),
		fmt.Sprintf("validate all posts in %s/content", pw),
		fmt.Sprintf("git -C %s commit -m \"content: publish <slug>\"", pw),
		fmt.Sprintf("git -C %s push %s HEAD:%s", pw, m.cfg.Remote, m.cfg.PublishBranch),
		fmt.Sprintf("git worktree remove .worktrees/studio-%s && git branch -D %s", d.Key, d.Branch),
	)
}

func (m *Manager) lintOptions(ctx context.Context, d *Draft) LintOptions {
	return LintOptions{
		SlugTaken: func(slug string) bool {
			// Another post in this draft's worktree already uses slug?
			entries, err := os.ReadDir(filepath.Join(d.Worktree, "content", "posts"))
			if err != nil {
				return false
			}
			for _, e := range entries {
				p := postsPrefix + e.Name()
				if p == d.Path || e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(d.Worktree, filepath.FromSlash(p)))
				if err != nil {
					continue
				}
				if doc, err := ParseDocument(data); err == nil && doc.Frontmatter.Slug == slug {
					return true
				}
			}
			return false
		},
		RefExists: func(url string) bool {
			rel := strings.TrimPrefix(url, "/")
			if !strings.HasPrefix(rel, "content/") || strings.Contains(rel, "..") {
				return false
			}
			fi, err := os.Stat(filepath.Join(d.Worktree, filepath.FromSlash(rel)))
			return err == nil && !fi.IsDir()
		},
	}
}

// LintDraft lints the draft's current file with repository context.
func (m *Manager) LintDraft(ctx context.Context, key string, data []byte) ([]Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	return Lint(data, path.Base(d.Path), m.lintOptions(ctx, d)), nil
}

// Publish squashes the draft onto the latest publish branch and pushes it
// directly (spec §3.2). It never force-pushes. On success the draft branch
// and worktree are removed.
func (m *Manager) Publish(ctx context.Context, key string, progress ProgressFunc) (*PublishResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if progress == nil {
		progress = func(string) {}
	}

	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	st, err := m.status(ctx, d)
	if err != nil {
		return nil, err
	}
	if st.State == StateUnsaved || st.State == StateNew {
		progress("save")
		res, err := m.save(ctx, key)
		if err != nil && !errors.Is(err, ErrNothingToCommit) {
			return nil, err
		}
		if res != nil {
			d.Path = res.Path
		}
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := m.publishOnce(ctx, d, progress)
		if err == nil {
			progress("cleanup")
			m.removeDraft(ctx, d.Key, d.Worktree)
			_ = m.fetch(ctx) // refresh origin/<pb> for the UI
			return res, nil
		}
		lastErr = err
		if !errors.Is(err, ErrPushRejected) {
			break
		}
	}
	return nil, lastErr
}

func (m *Manager) publishOnce(ctx context.Context, d *Draft, progress ProgressFunc) (*PublishResult, error) {
	pw := m.publishWorktree()

	progress("fetch")
	if err := m.fetch(ctx); err != nil {
		return nil, fmt.Errorf("fetch %s: %w", m.cfg.PublishBranch, err)
	}

	progress("reset")
	if err := m.resetPublishWorktree(ctx); err != nil {
		return nil, err
	}

	progress("squash")
	if _, err := m.git.Run(ctx, pw, "merge", "--squash", d.Branch); err != nil {
		conflicts, _ := m.git.Query(ctx, pw, "diff", "--name-only", "--diff-filter=U")
		_ = m.resetPublishWorktree(ctx)
		if files := splitLines(conflicts); len(files) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrConflict, strings.Join(files, ", "))
		}
		return nil, err
	}

	progress("check-paths")
	names, err := m.git.Query(ctx, pw, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	files := splitLines(names)
	if len(files) == 0 {
		_ = m.resetPublishWorktree(ctx)
		return nil, ErrNothingToPublish
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "content/") {
			_ = m.resetPublishWorktree(ctx)
			return nil, fmt.Errorf("%w: %s", ErrNonContentPath, f)
		}
	}

	progress("validate")
	if err := content.ValidateDir(filepath.Join(pw, "content")); err != nil {
		_ = m.resetPublishWorktree(ctx)
		return nil, &ValidationError{Err: err}
	}
	data, err := os.ReadFile(filepath.Join(pw, filepath.FromSlash(d.Path)))
	if err != nil {
		_ = m.resetPublishWorktree(ctx)
		return nil, err
	}
	post, err := content.ValidatePost(data, path.Base(d.Path))
	if err != nil {
		_ = m.resetPublishWorktree(ctx)
		return nil, &ValidationError{Err: err}
	}

	progress("commit")
	if _, err := m.git.Run(ctx, pw, "commit", "--quiet", "-m", "content: publish "+post.Slug); err != nil {
		_ = m.resetPublishWorktree(ctx)
		return nil, err
	}

	if m.beforePush != nil {
		m.beforePush()
	}
	progress("push")
	if _, err := m.git.Run(ctx, pw, "push", m.cfg.Remote, "HEAD:refs/heads/"+m.cfg.PublishBranch); err != nil {
		if isRejection(err) {
			return nil, fmt.Errorf("%w: %v", ErrPushRejected, err)
		}
		return nil, err
	}

	sha, err := m.git.Query(ctx, pw, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	return &PublishResult{Commit: strings.TrimSpace(sha), Branch: m.cfg.PublishBranch, Path: d.Path, Slug: post.Slug}, nil
}

// resetPublishWorktree points the detached publish worktree at the publish ref
// with a clean tree, creating it on first use.
func (m *Manager) resetPublishWorktree(ctx context.Context) error {
	pw := m.publishWorktree()
	if _, err := os.Stat(filepath.Join(pw, ".git")); err != nil {
		_, _ = m.git.Run(ctx, m.cfg.Repo, "worktree", "prune")
		if exists(pw) {
			return fmt.Errorf("%s exists but is not a worktree; move it aside", pw)
		}
		_, err := m.git.Run(ctx, m.cfg.Repo, "worktree", "add", "--detach", pw, m.PublishRef())
		return err
	}
	_, _ = m.git.Query(ctx, pw, "merge", "--abort")
	if _, err := m.git.Run(ctx, pw, "checkout", "--quiet", "--detach", "--force", m.PublishRef()); err != nil {
		return err
	}
	if _, err := m.git.Run(ctx, pw, "reset", "--quiet", "--hard", m.PublishRef()); err != nil {
		return err
	}
	_, err := m.git.Run(ctx, pw, "clean", "-fdq")
	return err
}

func isRejection(err error) bool {
	var ge *GitError
	if !errors.As(err, &ge) {
		return false
	}
	s := ge.Stderr
	return strings.Contains(s, "[rejected]") || strings.Contains(s, "non-fast-forward") ||
		strings.Contains(s, "fetch first")
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
