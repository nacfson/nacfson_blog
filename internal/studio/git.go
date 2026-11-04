// Package studio implements the local, git-backed Blog Studio: drafts live on
// studio/<key> branches in their own worktrees, Save is a local commit, and
// Publish (see publish.go) pushes a validated squash to the Flux-tracked branch.
//
// Spec: docs/superpowers/specs/2026-11-04-blog-studio-design.md
package studio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Event describes one git invocation. Mutating commands are reported to the
// Logger so the UI can show a "Git activity" log; read-only queries are not.
type Event struct {
	Time     time.Time     `json:"time"`
	Dir      string        `json:"dir"`
	Args     []string      `json:"args"`
	Output   string        `json:"output,omitempty"`
	Err      string        `json:"error,omitempty"`
	Duration time.Duration `json:"durationNs"`
}

// Logger receives git events. It must be safe for concurrent use.
type Logger func(Event)

// GitError is returned when git exits unsuccessfully.
type GitError struct {
	Args   []string
	Dir    string
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Stderr)
}

func (e *GitError) Unwrap() error { return e.Err }

// ExitCode returns git's exit status, or -1 if it did not exit normally.
func (e *GitError) ExitCode() int {
	var ee *exec.ExitError
	if errors.As(e.Err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// exitCode reports the git exit status carried by err (0 for nil, -1 if unknown).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ge *GitError
	if errors.As(err, &ge) {
		return ge.ExitCode()
	}
	return -1
}

// Git runs the git binary directly (never through a shell) with the user's
// own configuration, identity and credentials.
type Git struct {
	bin string
	env []string
	log Logger
}

// NewGit returns a runner that reports mutating commands to log (may be nil).
func NewGit(log Logger) *Git {
	return &Git{
		bin: "git",
		// Never block on an interactive credential prompt; keep output parseable.
		env: []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C"},
		log: log,
	}
}

// Run executes a mutating git command in dir and reports it to the logger.
func (g *Git) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return g.exec(ctx, dir, true, args)
}

// Query executes a read-only git command in dir without logging it.
func (g *Git) Query(ctx context.Context, dir string, args ...string) (string, error) {
	return g.exec(ctx, dir, false, args)
}

func (g *Git) exec(ctx context.Context, dir string, logIt bool, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, g.bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), g.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	out := stdout.String()

	var err error
	errMsg := ""
	if runErr != nil {
		errMsg = strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = runErr.Error()
		}
		err = &GitError{Args: append([]string(nil), args...), Dir: dir, Stderr: errMsg, Err: runErr}
	}

	if logIt && g.log != nil {
		combined := strings.TrimSpace(strings.TrimSpace(out) + "\n" + strings.TrimSpace(stderr.String()))
		g.log(Event{
			Time:     start,
			Dir:      dir,
			Args:     append([]string(nil), args...),
			Output:   combined,
			Err:      errMsg,
			Duration: time.Since(start),
		})
	}
	return out, err
}
