package syncer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxFileSize  int64 = 10 * 1024 * 1024 // 10 MiB per file limit
	maxTotalSize int64 = 50 * 1024 * 1024 // 50 MiB total extracted limit
)

// Reloader defines an interface for notifying content stores when new
// content has been extracted and is ready to be loaded.
type Reloader interface {
	Reload() error
}

// FluxTarballSyncer polls FluxCD's in-cluster source-controller for artifact
// updates, extracts the content directory from the tarball, and triggers
// reloads on the configured Reloader.
type FluxTarballSyncer struct {
	artifactURL  string
	workspaceDir string
	reloader     Reloader
	httpClient   *http.Client

	syncMu    sync.Mutex
	mu        sync.RWMutex
	activeDir string
	etag      string
}

// NewFluxTarballSyncer creates a new FluxTarballSyncer.
func NewFluxTarballSyncer(artifactURL, workspaceDir string, reloader Reloader) *FluxTarballSyncer {
	return &FluxTarballSyncer{
		artifactURL:  artifactURL,
		workspaceDir: workspaceDir,
		reloader:     reloader,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// ActiveContentDir returns the currently active content root directory.
func (s *FluxTarballSyncer) ActiveContentDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeDir
}

// SeedIfEmpty seeds the workspace with content from seedDir if the syncer
// does not yet have an active content directory initialized.
func (s *FluxTarballSyncer) SeedIfEmpty(seedDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeDir != "" {
		return nil
	}

	targetDir := filepath.Join(s.workspaceDir, "seed")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create seed directory: %w", err)
	}

	if seedDir != "" && seedDir != targetDir {
		if err := copyDir(seedDir, targetDir); err != nil {
			return fmt.Errorf("failed to copy seed directory: %w", err)
		}
	}

	s.activeDir = targetDir
	return nil
}

// SyncOnce fetches the latest tarball artifact from artifactURL (if modified),
// extracts content into a new revision staging directory, validates archive entries
// against zip-slip and size limits, switches the active content directory,
// triggers the reloader, and removes the old staging directory.
func (s *FluxTarballSyncer) SyncOnce(ctx context.Context) (bool, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	if s.artifactURL == "" {
		return false, fmt.Errorf("artifactURL is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.artifactURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create http request: %w", err)
	}

	s.mu.RLock()
	currentETag := s.etag
	s.mu.RUnlock()

	if currentETag != "" {
		req.Header.Set("If-None-Match", currentETag)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("http fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return false, fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gzr.Close()

	if err := os.MkdirAll(s.workspaceDir, 0o755); err != nil {
		return false, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	stagingDir := filepath.Join(s.workspaceDir, fmt.Sprintf("rev-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return false, fmt.Errorf("failed to create staging directory: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	tr := tar.NewReader(gzr)
	var totalExtracted int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("error reading tar stream: %w", err)
		}

		relPath, isContent := extractContentPath(hdr.Name)
		if !isContent {
			continue
		}

		// Reject any non-regular file or directory (symlinks, devices, fifos, etc.)
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA && hdr.Typeflag != tar.TypeDir {
			return false, fmt.Errorf("unsupported entry type %v for %s", hdr.Typeflag, hdr.Name)
		}

		// Prevent zip-slip path traversal
		cleanedRel := filepath.Clean(relPath)
		if cleanedRel == ".." || strings.HasPrefix(cleanedRel, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanedRel) {
			return false, fmt.Errorf("path traversal attempt detected: %s", hdr.Name)
		}

		destPath := filepath.Join(stagingDir, cleanedRel)
		relToStaging, err := filepath.Rel(stagingDir, destPath)
		if err != nil || relToStaging == ".." || strings.HasPrefix(relToStaging, ".."+string(filepath.Separator)) {
			return false, fmt.Errorf("path traversal attempt detected: %s", hdr.Name)
		}

		if hdr.Typeflag == tar.TypeDir {
			if cleanedRel != "." {
				if err := os.MkdirAll(destPath, 0o755); err != nil {
					return false, fmt.Errorf("failed to create directory %s: %w", destPath, err)
				}
			}
			continue
		}

		// Regular file
		if hdr.Size > maxFileSize {
			return false, fmt.Errorf("file size %d exceeds 10 MiB limit: %s", hdr.Size, hdr.Name)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return false, fmt.Errorf("failed to create parent dir for %s: %w", destPath, err)
		}

		perm := hdr.FileInfo().Mode().Perm()
		if perm == 0 {
			perm = 0o644
		}

		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			return false, fmt.Errorf("failed to create file %s: %w", destPath, err)
		}

		limitReader := io.LimitReader(tr, maxFileSize+1)
		n, copyErr := io.Copy(outFile, limitReader)
		closeErr := outFile.Close()
		if copyErr != nil {
			return false, fmt.Errorf("failed to write file %s: %w", destPath, copyErr)
		}
		if closeErr != nil {
			return false, fmt.Errorf("failed to close file %s: %w", destPath, closeErr)
		}
		if n > maxFileSize {
			return false, fmt.Errorf("file exceeded 10 MiB limit: %s", hdr.Name)
		}

		totalExtracted += n
		if totalExtracted > maxTotalSize {
			return false, fmt.Errorf("extracted content exceeded 50 MiB total limit")
		}
	}

	// Atomically swap the active directory pointer and update ETag
	s.mu.Lock()
	oldActive := s.activeDir
	oldETag := s.etag
	s.activeDir = stagingDir
	s.etag = resp.Header.Get("ETag")
	s.mu.Unlock()

	// Notify reloader
	if s.reloader != nil {
		if err := s.reloader.Reload(); err != nil {
			// Roll back active dir and etag on reload error
			s.mu.Lock()
			s.activeDir = oldActive
			s.etag = oldETag
			s.mu.Unlock()
			return false, fmt.Errorf("reloader failed: %w", err)
		}
	}

	success = true

	// Clean up previous staging directory if it was an ephemeral rev-* directory
	if oldActive != "" && oldActive != stagingDir {
		cleanOld := filepath.Clean(oldActive)
		base := filepath.Base(cleanOld)
		if strings.HasPrefix(base, "rev-") && filepath.Dir(cleanOld) == filepath.Clean(s.workspaceDir) {
			_ = os.RemoveAll(cleanOld)
		}
	}

	return true, nil
}

// Start begins periodic polling of artifact updates at the specified interval.
// Polling stops when ctx is cancelled.
func (s *FluxTarballSyncer) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.SyncOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("flux syncer periodic update error: %v", err)
				}
			}
		}
	}()
}

// extractContentPath determines if an archive path belongs inside the content
// directory and returns its path relative to content/.
func extractContentPath(name string) (string, bool) {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")

	if strings.HasPrefix(name, "content/") {
		rel := strings.TrimPrefix(name, "content/")
		if rel == "" {
			return ".", true
		}
		return rel, true
	}
	if idx := strings.Index(name, "/content/"); idx != -1 {
		rel := name[idx+len("/content/"):]
		if rel == "" {
			return ".", true
		}
		return rel, true
	}
	if name == "content" || strings.HasSuffix(name, "/content") {
		return ".", true
	}
	return "", false
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("seed source is not a directory: %s", src)
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	return err
}
