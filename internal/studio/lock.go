package studio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrLocked is returned when another live blog-studio process holds the repo lock.
var ErrLocked = errors.New("another blog-studio instance is running for this repository")

// Lock is a PID lock file at <repo>/.worktrees/studio.lock that prevents two
// studio processes from driving the same repository's git state concurrently.
type Lock struct {
	path string
}

// AcquireLock takes the repository lock, replacing it if the recorded PID is
// no longer running.
func AcquireLock(repo string) (*Lock, error) {
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	p := filepath.Join(dir, "studio.lock")

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d\n", os.Getpid())
			cerr := f.Close()
			if werr != nil || cerr != nil {
				_ = os.Remove(p)
				return nil, fmt.Errorf("write lock %s: %w", p, errors.Join(werr, cerr))
			}
			return &Lock{path: p}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create lock %s: %w", p, err)
		}

		data, _ := os.ReadFile(p)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && processAlive(pid) {
			return nil, fmt.Errorf("%w (pid %d, lock %s)", ErrLocked, pid, p)
		}
		// Stale lock from a crashed process.
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("remove stale lock %s: %w", p, err)
		}
	}
	return nil, fmt.Errorf("%w (lock %s)", ErrLocked, p)
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
