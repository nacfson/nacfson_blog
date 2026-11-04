package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nacfson/nacfson_blog/internal/content"
)

var (
	ErrDraftNotFound   = errors.New("draft not found")
	ErrDraftExists     = errors.New("draft already exists")
	ErrNothingToCommit = errors.New("nothing to commit")
	ErrInvalidKey      = errors.New("invalid draft key")
	ErrInvalidPath     = errors.New("invalid post path")
	ErrPostNotFound    = errors.New("post not found on publish branch")
)

// ValidationError wraps a post that the production parser (internal/content)
// rejects. Nothing is committed when Save returns one.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return "validation failed: " + e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

const (
	branchPrefix   = "studio/"
	worktreePrefix = "studio-"
	// publishKey is reserved: .worktrees/studio-publish is the publish worktree.
	publishKey = "publish"
	// Per-branch git config keys (removed automatically by `git branch -D`).
	cfgPath     = "studiopath"
	cfgScaffold = "studioscaffold"
	postsPrefix = "content/posts/"
)

var (
	keyRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)
	dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	fileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.md$`)
)

// State is the derived status of a draft (spec §3.4).
type State string

const (
	StateUnsaved State = "unsaved" // worktree differs from the last save
	StateNew     State = "new"     // nothing committed yet, not on the publish branch
	StateSaved   State = "saved"   // has local commits not on the publish branch
	StateLive    State = "live"    // identical to what is on the publish branch
)

// Config configures a Manager.
type Config struct {
	Repo          string // repository root (main checkout)
	Remote        string // default "origin"
	PublishBranch string // default "main"
	Log           Logger
	Now           func() time.Time
}

// Draft identifies one draft: branch studio/<Key> checked out at Worktree.
// Key is fixed at creation; Path follows slug/date changes on Save.
type Draft struct {
	Key      string `json:"key"`
	Branch   string `json:"branch"`
	Worktree string `json:"worktree"`
	Path     string `json:"path"` // repo-relative, slash-separated post file path
}

// DraftStatus is a Draft plus its derived state.
type DraftStatus struct {
	Draft
	State State `json:"state"`
	Ahead int   `json:"ahead"` // commits on the draft branch not on the publish branch
}

// PublishedPost is a post present on the remote publish branch.
type PublishedPost struct {
	Path    string              `json:"path"`
	Summary content.PostSummary `json:"summary"`
	Err     string              `json:"error,omitempty"` // set if the published file does not parse
}

// Manager owns all draft git state. Every mutating operation is serialized.
type Manager struct {
	cfg Config
	git *Git
	mu  sync.Mutex
}

// NewManager validates that cfg.Repo is the top level of a git repository.
func NewManager(ctx context.Context, cfg Config) (*Manager, error) {
	if cfg.Remote == "" {
		cfg.Remote = "origin"
	}
	if cfg.PublishBranch == "" {
		cfg.PublishBranch = "main"
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	abs, err := filepath.Abs(cfg.Repo)
	if err != nil {
		return nil, err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return nil, err
	}
	cfg.Repo = abs

	m := &Manager{cfg: cfg, git: NewGit(cfg.Log)}
	top, err := m.git.Query(ctx, abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%s is not a git repository: %w", abs, err)
	}
	if filepath.Clean(strings.TrimSpace(top)) != abs {
		return nil, fmt.Errorf("%s is not the repository root (root is %s)", abs, strings.TrimSpace(top))
	}
	if err := m.ensureWorktreesIgnored(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// ensureWorktreesIgnored keeps .worktrees/ out of the author's `git status`.
// If the repo's .gitignore doesn't already cover it, it is added to the
// local-only .git/info/exclude (never committed).
func (m *Manager) ensureWorktreesIgnored(ctx context.Context) error {
	if _, err := m.git.Query(ctx, m.cfg.Repo, "check-ignore", "-q", ".worktrees/"); err == nil {
		return nil
	}
	p, err := m.git.Query(ctx, m.cfg.Repo, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	excl := strings.TrimSpace(p)
	if !filepath.IsAbs(excl) {
		excl = filepath.Join(m.cfg.Repo, excl)
	}
	if err := os.MkdirAll(filepath.Dir(excl), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(excl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString("\n# blog-studio draft worktrees\n/.worktrees/\n")
	return errors.Join(werr, f.Close())
}

// Repo returns the absolute repository root.
func (m *Manager) Repo() string { return m.cfg.Repo }

// Git returns the runner (shared with the publisher).
func (m *Manager) Git() *Git { return m.git }

// PublishRef is the remote-tracking ref of the publish branch.
func (m *Manager) PublishRef() string {
	return "refs/remotes/" + m.cfg.Remote + "/" + m.cfg.PublishBranch
}

func (m *Manager) worktreeDir(key string) string {
	return filepath.Join(m.cfg.Repo, ".worktrees", worktreePrefix+key)
}

func branchOf(key string) string { return branchPrefix + key }

// ValidKey reports whether key is usable as a draft key / branch suffix.
func ValidKey(key string) bool { return keyRe.MatchString(key) && key != publishKey }

// PostPath returns the canonical file path for a post with this date and slug.
func PostPath(date, slug string) string { return postsPrefix + date + "-" + slug + ".md" }

func validatePostPath(p string) error {
	if p != path.Clean(p) || !strings.HasPrefix(p, postsPrefix) {
		return fmt.Errorf("%w: %q must be under %s", ErrInvalidPath, p, postsPrefix)
	}
	if name := strings.TrimPrefix(p, postsPrefix); strings.Contains(name, "/") || !fileRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidPath, p)
	}
	return nil
}

// ---------- queries ----------

func (m *Manager) branchExists(ctx context.Context, key string) bool {
	_, err := m.git.Query(ctx, m.cfg.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branchOf(key))
	return err == nil
}

func (m *Manager) configGet(ctx context.Context, key, name string) string {
	out, err := m.git.Query(ctx, m.cfg.Repo, "config", "--get", "branch."+branchOf(key)+"."+name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (m *Manager) configSet(ctx context.Context, key, name, value string) error {
	_, err := m.git.Query(ctx, m.cfg.Repo, "config", "branch."+branchOf(key)+"."+name, value)
	return err
}

// worktrees maps branch short name -> worktree path for all attached worktrees.
func (m *Manager) worktrees(ctx context.Context) (map[string]string, error) {
	out, err := m.git.Query(ctx, m.cfg.Repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	res := map[string]string{}
	var cur string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			res[strings.TrimPrefix(line, "branch refs/heads/")] = cur
		}
	}
	return res, nil
}

func (m *Manager) draftKeys(ctx context.Context) ([]string, error) {
	out, err := m.git.Query(ctx, m.cfg.Repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+branchPrefix)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if key := strings.TrimPrefix(line, branchPrefix); line != "" && key != line {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *Manager) get(ctx context.Context, key string) (*Draft, error) {
	if !ValidKey(key) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	if !m.branchExists(ctx, key) {
		return nil, fmt.Errorf("%w: %s", ErrDraftNotFound, key)
	}
	wts, err := m.worktrees(ctx)
	if err != nil {
		return nil, err
	}
	wt, ok := wts[branchOf(key)]
	if !ok {
		return nil, fmt.Errorf("%w: %s has no worktree (run reconciliation)", ErrDraftNotFound, key)
	}
	p := m.configGet(ctx, key, cfgPath)
	if err := validatePostPath(p); err != nil {
		return nil, fmt.Errorf("draft %s: %w", key, err)
	}
	return &Draft{Key: key, Branch: branchOf(key), Worktree: wt, Path: p}, nil
}

// Get returns the draft for key.
func (m *Manager) Get(ctx context.Context, key string) (*Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.get(ctx, key)
}

// Read returns the current (possibly unsaved) post file contents.
func (m *Manager) Read(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(d.Worktree, filepath.FromSlash(d.Path)))
}

// Status derives the draft state (spec §3.4).
func (m *Manager) Status(ctx context.Context, key string) (*DraftStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	return m.status(ctx, d)
}

func (m *Manager) status(ctx context.Context, d *Draft) (*DraftStatus, error) {
	porcelain, err := m.git.Query(ctx, d.Worktree, "status", "--porcelain", "--untracked-files=all", "--", "content")
	if err != nil {
		return nil, err
	}
	dirty := strings.TrimSpace(porcelain) != ""

	out, err := m.git.Query(ctx, m.cfg.Repo, "rev-list", "--count", m.PublishRef()+".."+d.Branch)
	if err != nil {
		return nil, err
	}
	ahead, _ := strconv.Atoi(strings.TrimSpace(out))

	_, catErr := m.git.Query(ctx, m.cfg.Repo, "cat-file", "-e", m.PublishRef()+":"+d.Path)
	onRemote := catErr == nil

	st := &DraftStatus{Draft: *d, Ahead: ahead}
	switch {
	case ahead == 0 && !onRemote:
		// Brand-new draft: still "new" while the file is just the untouched scaffold.
		if !dirty || m.isUntouchedScaffold(ctx, d, porcelain) {
			st.State = StateNew
		} else {
			st.State = StateUnsaved
		}
	case dirty:
		st.State = StateUnsaved
	case ahead == 0:
		st.State = StateLive
	default:
		_, diffErr := m.git.Query(ctx, m.cfg.Repo, "diff", "--quiet", m.PublishRef(), d.Branch, "--", "content")
		if diffErr == nil {
			st.State = StateLive // remote already has identical content
		} else {
			st.State = StateSaved
		}
	}
	return st, nil
}

func (m *Manager) isUntouchedScaffold(ctx context.Context, d *Draft, porcelain string) bool {
	want := m.configGet(ctx, d.Key, cfgScaffold)
	if want == "" || strings.TrimSpace(porcelain) != "?? "+d.Path {
		return false
	}
	got, err := m.git.Query(ctx, d.Worktree, "hash-object", "--", d.Path)
	return err == nil && strings.TrimSpace(got) == want
}

// List returns every draft with its status, sorted by key.
func (m *Manager) List(ctx context.Context) ([]DraftStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys, err := m.draftKeys(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]DraftStatus, 0, len(keys))
	for _, k := range keys {
		d, err := m.get(ctx, k)
		if err != nil {
			continue // surfaced by Reconcile
		}
		st, err := m.status(ctx, d)
		if err != nil {
			return nil, err
		}
		res = append(res, *st)
	}
	return res, nil
}

// PublishedPosts lists posts on the remote publish branch (as last fetched),
// newest first.
func (m *Manager) PublishedPosts(ctx context.Context) ([]PublishedPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishedPosts(ctx)
}

func (m *Manager) publishedPosts(ctx context.Context) ([]PublishedPost, error) {
	out, err := m.git.Query(ctx, m.cfg.Repo, "ls-tree", "-z", "--name-only", m.PublishRef(), "--", postsPrefix)
	if err != nil {
		if exitCode(err) == 128 { // no such ref yet
			return nil, nil
		}
		return nil, err
	}
	var res []PublishedPost
	for _, p := range strings.Split(out, "\x00") {
		if validatePostPath(p) != nil {
			continue
		}
		data, err := m.git.Query(ctx, m.cfg.Repo, "show", m.PublishRef()+":"+p)
		if err != nil {
			return nil, err
		}
		pp := PublishedPost{Path: p}
		if post, err := content.ValidatePost([]byte(data), path.Base(p)); err != nil {
			pp.Err = err.Error()
		} else {
			pp.Summary = post.PostSummary
		}
		res = append(res, pp)
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].Summary.Date > res[j].Summary.Date })
	return res, nil
}

// ---------- mutations ----------

// fetch updates the publish branch's remote-tracking ref. Offline is not fatal
// for creating drafts: the last fetched ref is used and the failure is logged.
func (m *Manager) fetch(ctx context.Context) error {
	_, err := m.git.Run(ctx, m.cfg.Repo, "fetch", m.cfg.Remote, m.cfg.PublishBranch)
	return err
}

func (m *Manager) uniqueKey(ctx context.Context, base string) string {
	key := base
	for n := 2; !ValidKey(key) || m.branchExists(ctx, key) || exists(m.worktreeDir(key)); n++ {
		key = fmt.Sprintf("%s-%d", base, n)
	}
	return key
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// addWorktree creates studio/<key> from the publish ref in its own worktree.
func (m *Manager) addWorktree(ctx context.Context, key, postPath string) (*Draft, error) {
	wt := m.worktreeDir(key)
	if _, err := m.git.Run(ctx, m.cfg.Repo, "worktree", "add", "--no-track", "-b", branchOf(key), wt, m.PublishRef()); err != nil {
		return nil, err
	}
	if err := m.configSet(ctx, key, cfgPath, postPath); err != nil {
		m.removeDraft(ctx, key, wt)
		return nil, err
	}
	return &Draft{Key: key, Branch: branchOf(key), Worktree: wt, Path: postPath}, nil
}

func (m *Manager) removeDraft(ctx context.Context, key, wt string) {
	if exists(wt) {
		_, _ = m.git.Run(ctx, m.cfg.Repo, "worktree", "remove", "--force", wt)
	}
	_, _ = m.git.Run(ctx, m.cfg.Repo, "branch", "-D", branchOf(key))
}

// CreateOptions describes a new draft. Key is a suggestion; it is made unique.
type CreateOptions struct {
	Key     string
	Path    string // e.g. content/posts/2026-11-04-my-slug.md
	Content []byte // initial scaffold (written, not committed)
}

// Create starts a new draft on its own branch and worktree (spec §3).
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !keyRe.MatchString(opts.Key) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidKey, opts.Key)
	}
	if err := validatePostPath(opts.Path); err != nil {
		return nil, err
	}
	_ = m.fetch(ctx) // best effort; logged by the runner

	key := m.uniqueKey(ctx, opts.Key)
	d, err := m.addWorktree(ctx, key, opts.Path)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(d.Worktree, filepath.FromSlash(d.Path))
	if exists(full) {
		m.removeDraft(ctx, key, d.Worktree)
		return nil, fmt.Errorf("%w: %s already exists on %s; edit the published post instead", ErrInvalidPath, d.Path, m.cfg.PublishBranch)
	}
	if err := m.writeAtomic(full, opts.Content); err != nil {
		m.removeDraft(ctx, key, d.Worktree)
		return nil, err
	}
	if h, err := m.git.Query(ctx, d.Worktree, "hash-object", "--", d.Path); err == nil {
		_ = m.configSet(ctx, key, cfgScaffold, strings.TrimSpace(h))
	}
	return d, nil
}

// EditPublished starts a draft for an existing post on the publish branch.
func (m *Manager) EditPublished(ctx context.Context, slug string) (*Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !ValidKey(slug) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidKey, slug)
	}
	_ = m.fetch(ctx)

	posts, err := m.publishedPosts(ctx)
	if err != nil {
		return nil, err
	}
	var p string
	for _, pp := range posts {
		if pp.Err == "" && pp.Summary.Slug == slug {
			p = pp.Path
			break
		}
	}
	if p == "" {
		return nil, fmt.Errorf("%w: %s", ErrPostNotFound, slug)
	}
	if keys, err := m.draftKeys(ctx); err == nil {
		for _, k := range keys {
			if m.configGet(ctx, k, cfgPath) == p {
				return nil, fmt.Errorf("%w: %s is already being edited on %s", ErrDraftExists, p, branchOf(k))
			}
		}
	}
	return m.addWorktree(ctx, m.uniqueKey(ctx, slug), p)
}

// Write autosaves the post file (atomic temp+rename, no git).
func (m *Manager) Write(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.get(ctx, key)
	if err != nil {
		return err
	}
	return m.writeAtomic(filepath.Join(d.Worktree, filepath.FromSlash(d.Path)), data)
}

// writeAtomic writes via a temp file kept outside every worktree (so a crash
// can never leave a stray file that `git add` would pick up), then renames.
func (m *Manager) writeAtomic(dst string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmpDir := filepath.Join(m.cfg.Repo, ".worktrees")
	f, err := os.CreateTemp(tmpDir, ".studio-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// SaveResult reports a successful Save.
type SaveResult struct {
	Commit string `json:"commit"`
	Path   string `json:"path"` // post path after any slug/date rename
}

// Save validates the post with the production parser, moves it to its
// canonical path if the slug or date changed, and commits everything under
// content/ on the draft branch. Nothing is committed if validation fails.
func (m *Manager) Save(ctx context.Context, key string) (*SaveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(d.Worktree, filepath.FromSlash(d.Path))
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	post, err := content.ValidatePost(data, path.Base(d.Path))
	if err != nil {
		return nil, &ValidationError{Err: err}
	}
	if !dateRe.MatchString(post.Date) {
		return nil, &ValidationError{Err: fmt.Errorf("date %q must be YYYY-MM-DD", post.Date)}
	}

	if want := PostPath(post.Date, post.Slug); want != d.Path {
		dst := filepath.Join(d.Worktree, filepath.FromSlash(want))
		if exists(dst) {
			return nil, &ValidationError{Err: fmt.Errorf("%s already exists", want)}
		}
		// Plain rename + `add -A` below is equivalent to `git mv`: git records
		// the rename by content similarity either way.
		if err := os.Rename(full, dst); err != nil {
			return nil, err
		}
		if err := m.configSet(ctx, key, cfgPath, want); err != nil {
			return nil, err
		}
		d.Path = want
	}

	if err := content.ValidateDir(filepath.Join(d.Worktree, "content")); err != nil {
		return nil, &ValidationError{Err: err}
	}

	if _, err := m.git.Run(ctx, d.Worktree, "add", "-A", "--", "content"); err != nil {
		return nil, err
	}
	if _, err := m.git.Query(ctx, d.Worktree, "diff", "--cached", "--quiet"); err == nil {
		return nil, ErrNothingToCommit
	} else if exitCode(err) != 1 {
		return nil, err
	}
	msg := fmt.Sprintf("draft(%s): save %s", post.Slug, m.cfg.Now().Format("15:04:05"))
	if _, err := m.git.Run(ctx, d.Worktree, "commit", "--quiet", "-m", msg); err != nil {
		return nil, err
	}
	sha, err := m.git.Query(ctx, d.Worktree, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, err
	}
	return &SaveResult{Commit: strings.TrimSpace(sha), Path: d.Path}, nil
}

// Discard deletes the draft's worktree (including unsaved changes) and branch.
// Callers must confirm with the user first.
func (m *Manager) Discard(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !ValidKey(key) {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	if !m.branchExists(ctx, key) {
		return fmt.Errorf("%w: %s", ErrDraftNotFound, key)
	}
	wts, err := m.worktrees(ctx)
	if err != nil {
		return err
	}
	if wt, ok := wts[branchOf(key)]; ok {
		if _, err := m.git.Run(ctx, m.cfg.Repo, "worktree", "remove", "--force", wt); err != nil {
			return err
		}
	}
	_, err = m.git.Run(ctx, m.cfg.Repo, "branch", "-D", branchOf(key))
	return err
}

// Reconcile repairs draft state at startup (spec §3.3): prunes stale worktree
// records, re-creates missing worktrees for studio/* branches, and infers a
// missing post path. Problems that need a human are returned as warnings.
func (m *Manager) Reconcile(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.git.Run(ctx, m.cfg.Repo, "worktree", "prune"); err != nil {
		return nil, err
	}
	keys, err := m.draftKeys(ctx)
	if err != nil {
		return nil, err
	}
	wts, err := m.worktrees(ctx)
	if err != nil {
		return nil, err
	}

	var warnings []string
	for _, key := range keys {
		if !ValidKey(key) {
			warnings = append(warnings, fmt.Sprintf("ignoring branch %s: unsupported name", branchOf(key)))
			continue
		}
		if _, ok := wts[branchOf(key)]; !ok {
			wt := m.worktreeDir(key)
			if exists(wt) {
				warnings = append(warnings, fmt.Sprintf("%s: %s exists but is not a registered worktree; move it aside", branchOf(key), wt))
				continue
			}
			if _, err := m.git.Run(ctx, m.cfg.Repo, "worktree", "add", wt, branchOf(key)); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: re-create worktree: %v", branchOf(key), err))
				continue
			}
		}
		if validatePostPath(m.configGet(ctx, key, cfgPath)) != nil {
			p, err := m.inferPath(ctx, key)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", branchOf(key), err))
				continue
			}
			if err := m.configSet(ctx, key, cfgPath, p); err != nil {
				return warnings, err
			}
		}
	}
	return warnings, nil
}

// inferPath finds the single post the draft branch changed since it forked.
func (m *Manager) inferPath(ctx context.Context, key string) (string, error) {
	out, err := m.git.Query(ctx, m.cfg.Repo, "diff", "--name-only", "--diff-filter=AMR",
		m.PublishRef()+"..."+branchOf(key), "--", postsPrefix)
	if err != nil {
		return "", err
	}
	var found []string
	for _, p := range strings.Split(strings.TrimSpace(out), "\n") {
		if validatePostPath(p) == nil {
			found = append(found, p)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("cannot determine post file (found %d candidates)", len(found))
	}
	return found[0], nil
}
