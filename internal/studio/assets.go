package studio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Limits mirror internal/syncer/flux_tarball.go so anything the studio
// accepts can also be synced to the cluster.
const (
	MaxAssetBytes   int64 = 10 * 1024 * 1024
	MaxContentBytes int64 = 50 * 1024 * 1024
)

var (
	ErrAssetType     = errors.New("unsupported file type")
	ErrAssetTooLarge = errors.New("file too large")
	ErrContentFull   = errors.New("content/ would exceed 50 MiB")
)

var assetKinds = map[string]string{
	".svg": "assets", ".png": "assets", ".jpg": "assets", ".jpeg": "assets",
	".webp": "assets", ".gif": "assets", ".html": "demos",
}

// AssetResult describes a stored upload.
type AssetResult struct {
	URL      string  `json:"url"`  // e.g. /content/assets/diagram.svg
	Path     string  `json:"path"` // repo-relative
	Kind     string  `json:"kind"` // "assets" or "demos"
	Size     int64   `json:"size"`
	Warnings []Issue `json:"warnings,omitempty"`
}

var nonNameChars = regexp.MustCompile(`[^a-z0-9._-]+`)

// NormalizeAssetName lowercases name and keeps only [a-z0-9._-].
func NormalizeAssetName(name string) string {
	name = strings.ToLower(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	base = strings.Trim(nonNameChars.ReplaceAllString(base, "-"), "-._")
	if base == "" {
		base = "file"
	}
	return base + ext
}

// AddAsset stores an uploaded image (content/assets/) or HTML demo
// (content/demos/) in the draft's worktree. It is committed by the next Save.
func (m *Manager) AddAsset(ctx context.Context, key, filename string, r io.Reader) (*AssetResult, error) {
	name := NormalizeAssetName(filename)
	ext := filepath.Ext(name)
	kind, ok := assetKinds[ext]
	if !ok {
		return nil, fmt.Errorf("%w: %q (allowed: svg, png, jpg, jpeg, webp, gif, html)", ErrAssetType, ext)
	}

	data, err := io.ReadAll(io.LimitReader(r, MaxAssetBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxAssetBytes {
		return nil, fmt.Errorf("%w: %s exceeds 10 MiB", ErrAssetTooLarge, name)
	}
	if err := sniff(ext, data); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.get(ctx, key)
	if err != nil {
		return nil, err
	}
	total, err := dirSize(filepath.Join(d.Worktree, "content"))
	if err != nil {
		return nil, err
	}
	if total+int64(len(data)) > MaxContentBytes {
		return nil, fmt.Errorf("%w (currently %d bytes)", ErrContentFull, total)
	}

	dir := filepath.Join(d.Worktree, "content", kind)
	base := strings.TrimSuffix(name, ext)
	for n := 2; exists(filepath.Join(dir, name)); n++ {
		name = fmt.Sprintf("%s-%d%s", base, n, ext)
	}
	if err := m.writeAtomic(filepath.Join(dir, name), data); err != nil {
		return nil, err
	}

	res := &AssetResult{
		URL:  "/content/" + kind + "/" + name,
		Path: "content/" + kind + "/" + name,
		Kind: kind,
		Size: int64(len(data)),
	}
	if kind == "demos" {
		res.Warnings = LintDemo(data)
	}
	return res, nil
}

func sniff(ext string, data []byte) error {
	switch ext {
	case ".svg":
		head := strings.ToLower(string(data[:min(len(data), 1024)]))
		if !strings.Contains(head, "<svg") {
			return fmt.Errorf("%w: file is not an SVG", ErrAssetType)
		}
	case ".html":
	default:
		if ct := http.DetectContentType(data); !strings.HasPrefix(ct, "image/") {
			return fmt.Errorf("%w: content looks like %s, not an image", ErrAssetType, ct)
		}
	}
	return nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			fi, err := d.Info()
			if err != nil {
				return err
			}
			total += fi.Size()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

var demoRules = []struct {
	re  *regexp.Regexp
	msg string
}{
	{regexp.MustCompile(`\bfetch\s*\(`), "fetch() is blocked by the demo CSP (connect-src 'none')"},
	{regexp.MustCompile(`\bXMLHttpRequest\b`), "XMLHttpRequest is blocked by the demo CSP"},
	{regexp.MustCompile(`\bWebSocket\b`), "WebSocket is blocked by the demo CSP"},
	{regexp.MustCompile(`\bEventSource\b`), "EventSource is blocked by the demo CSP"},
	{regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*["']?(?:https?:)?//`), "External resources are blocked: inline them or use /content/assets/"},
	{regexp.MustCompile(`(?i)<form\b`), "Forms cannot submit inside the sandboxed iframe"},
}

// LintDemo warns about features the sandboxed demo iframe will block.
func LintDemo(data []byte) []Issue {
	s := string(data)
	var issues []Issue
	for _, r := range demoRules {
		if loc := r.re.FindStringIndex(s); loc != nil {
			issues = append(issues, Issue{Severity: SevWarn, Line: strings.Count(s[:loc[0]], "\n") + 1, Message: r.msg})
		}
	}
	return issues
}

// ResolveContentFile maps a /content/assets/... or /content/demos/... URL to a
// file inside the draft's worktree, rejecting traversal. Used for previews.
func (m *Manager) ResolveContentFile(ctx context.Context, key, kind, rel string) (string, error) {
	if kind != "assets" && kind != "demos" {
		return "", fs.ErrNotExist
	}
	m.mu.Lock()
	d, err := m.get(ctx, key)
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	root := filepath.Join(d.Worktree, "content", kind)
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fs.ErrNotExist
	}
	full := filepath.Join(root, clean)
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", fs.ErrNotExist
	}
	return full, nil
}
