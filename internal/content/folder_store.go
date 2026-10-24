package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// FolderStore implements ContentStore by reading posts and metadata
// from the local filesystem rooted at activeRoot().
type FolderStore struct {
	activeRoot func() string
	mu         sync.RWMutex
	posts      map[string]Post
	summaries  []PostSummary
	matrixData []byte
	matrixErr  error
}

// Compile-time check that FolderStore implements ContentStore.
var _ ContentStore = (*FolderStore)(nil)

// NewFolderStore creates a new FolderStore with the given root path provider
// and performs an initial content reload.
func NewFolderStore(activeRoot func() string) (*FolderStore, error) {
	if activeRoot == nil {
		return nil, fmt.Errorf("activeRoot function is required")
	}
	s := &FolderStore{
		activeRoot: activeRoot,
		posts:      make(map[string]Post),
		summaries:  make([]PostSummary, 0),
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// Reload rescans the active content directory and re-indexes all markdown posts.
func (s *FolderStore) Reload() error {
	root := s.activeRoot()
	postsDir := filepath.Join(root, "posts")

	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return fmt.Errorf("read posts directory %s: %w", postsDir, err)
	}

	newPosts := make(map[string]Post)
	newSummaries := make([]PostSummary, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(postsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read post file %s: %w", filePath, err)
		}

		post, err := parsePost(string(data), entry.Name())
		if err != nil {
			return fmt.Errorf("parse post %s: %w", filePath, err)
		}

		if _, exists := newPosts[post.Slug]; exists {
			return fmt.Errorf("duplicate post slug %q found in %s", post.Slug, filePath)
		}

		newPosts[post.Slug] = *post
		newSummaries = append(newSummaries, post.PostSummary)
	}

	// Sort summaries descending by Date (newest first).
	// If dates are identical, sort alphabetically by Slug.
	sort.Slice(newSummaries, func(i, j int) bool {
		if newSummaries[i].Date != newSummaries[j].Date {
			return newSummaries[i].Date > newSummaries[j].Date
		}
		return newSummaries[i].Slug < newSummaries[j].Slug
	})

	// Load and cache competency matrix in memory if present.
	// If missing, do not fail Reload(), but record matrixErr so GetCompetencyMatrix returns it.
	var newMatrixData []byte
	var newMatrixErr error
	matrixPath := filepath.Join(root, "competencies", "matrix.json")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		newMatrixErr = fmt.Errorf("read competency matrix %s: %w", matrixPath, err)
	} else if !json.Valid(data) {
		return fmt.Errorf("invalid competency matrix JSON in %s", matrixPath)
	} else {
		newMatrixData = data
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.posts = newPosts
	s.summaries = newSummaries
	s.matrixData = newMatrixData
	s.matrixErr = newMatrixErr

	return nil
}

// ListPosts returns posts optionally filtered by pillar and tag, sorted newest first.
func (s *FolderStore) ListPosts(ctx context.Context, pillar, tag string) ([]PostSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PostSummary, 0, len(s.summaries))
	for _, summary := range s.summaries {
		if pillar != "" && !strings.EqualFold(string(summary.Pillar), pillar) {
			continue
		}
		if tag != "" && !hasTag(summary.Tags, tag) {
			continue
		}
		result = append(result, summary)
	}

	return result, nil
}

// GetPost retrieves a single post by slug, rejecting path traversal or invalid slugs.
func (s *FolderStore) GetPost(ctx context.Context, slug string) (*Post, error) {
	if !isValidSlug(slug) {
		return nil, ErrInvalidSlug
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	post, ok := s.posts[slug]
	if !ok {
		return nil, ErrPostNotFound
	}

	postCopy := post
	if post.Tags != nil {
		postCopy.Tags = append([]string(nil), post.Tags...)
	}
	return &postCopy, nil
}

// GetCompetencyMatrix returns the raw JSON content of matrix.json served from in-memory cache.
func (s *FolderStore) GetCompetencyMatrix(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.matrixErr != nil {
		return nil, s.matrixErr
	}

	cp := make([]byte, len(s.matrixData))
	copy(cp, s.matrixData)
	return cp, nil
}

// hasTag checks whether any tag matches target case-insensitively.
func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, target) {
			return true
		}
	}
	return false
}

// isValidSlug checks if a slug is safe (no path traversal, non-empty, alphanumeric + - and _).
func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := (c >= '0' && c <= '9')
		isAllowedPunct := c == '-' || c == '_'
		if !isAlpha && !isDigit && !isAllowedPunct {
			return false
		}
	}
	return true
}

// parsePost parses YAML frontmatter and markdown body from raw file contents.
func parsePost(content, filename string) (*Post, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, fmt.Errorf("missing leading frontmatter delimiter (---)")
	}

	rest := normalized[4:]
	endIdx := strings.Index(rest, "\n---\n")
	var frontmatter, markdownBody string
	if endIdx != -1 {
		frontmatter = rest[:endIdx]
		markdownBody = strings.TrimPrefix(rest[endIdx+5:], "\n")
	} else if strings.HasSuffix(rest, "\n---") {
		frontmatter = rest[:len(rest)-4]
		markdownBody = ""
	} else {
		return nil, fmt.Errorf("missing closing frontmatter delimiter (---)")
	}

	summary, err := parseFrontmatter(frontmatter, filename)
	if err != nil {
		return nil, err
	}

	if summary.ReadTimeMinutes <= 0 {
		words := len(strings.Fields(markdownBody))
		minutes := words / 200
		if minutes < 1 {
			minutes = 1
		}
		summary.ReadTimeMinutes = minutes
	}

	return &Post{
		PostSummary:  *summary,
		MarkdownBody: markdownBody,
	}, nil
}

// parseFrontmatter extracts and validates metadata fields from YAML frontmatter text.
func parseFrontmatter(fm, filename string) (*PostSummary, error) {
	lines := strings.Split(fm, "\n")
	summary := &PostSummary{
		Tags: []string{},
	}

	var currentKey string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Handle list item continuation (e.g., "  - tag")
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			item = unquote(item)
			if currentKey == "tags" && item != "" {
				summary.Tags = append(summary.Tags, item)
			}
			continue
		}

		key, val, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		currentKey = key

		switch key {
		case "title":
			summary.Title = unquote(val)
		case "slug":
			summary.Slug = unquote(val)
		case "date":
			summary.Date = unquote(val)
		case "pillar":
			summary.Pillar = Pillar(unquote(val))
		case "project":
			summary.Project = unquote(val)
		case "summary":
			summary.Summary = unquote(val)
		case "readtimeminutes":
			v, err := strconv.Atoi(unquote(val))
			if err == nil {
				summary.ReadTimeMinutes = v
			}
		case "tags":
			if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				inner := val[1 : len(val)-1]
				parts := strings.Split(inner, ",")
				for _, p := range parts {
					item := unquote(strings.TrimSpace(p))
					if item != "" {
						summary.Tags = append(summary.Tags, item)
					}
				}
			}
		}
	}

	// Validation
	if summary.Slug == "" {
		return nil, fmt.Errorf("%s: slug is required in post frontmatter", filename)
	}
	if !isValidSlug(summary.Slug) {
		return nil, fmt.Errorf("%s: invalid slug %q: %w", filename, summary.Slug, ErrInvalidSlug)
	}
	if summary.Title == "" {
		return nil, fmt.Errorf("%s: title is required in post frontmatter", filename)
	}
	if summary.Date == "" {
		return nil, fmt.Errorf("%s: date is required in post frontmatter", filename)
	}
	switch summary.Pillar {
	case PillarProjectUpdates, PillarCompetencyGrowth, PillarTechOpinions:
		// valid
	default:
		return nil, fmt.Errorf("%s: invalid pillar %q: must be one of %s, %s, %s",
			filename, summary.Pillar, PillarProjectUpdates, PillarCompetencyGrowth, PillarTechOpinions)
	}

	return summary, nil
}

// unquote strips matching leading and trailing single or double quotes.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
