package content

import (
	"context"
	"errors"
)

// Pillar represents one of the three core blog content pillars.
type Pillar string

const (
	PillarProjectUpdates   Pillar = "project-updates"
	PillarCompetencyGrowth Pillar = "competency-growth"
	PillarTechOpinions     Pillar = "tech-opinions"
)

// PostSummary contains metadata for post listings and navigation.
type PostSummary struct {
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Date            string   `json:"date"`
	Pillar          Pillar   `json:"pillar"`
	Project         string   `json:"project,omitempty"`
	Summary         string   `json:"summary"`
	Tags            []string `json:"tags"`
	ReadTimeMinutes int      `json:"readTimeMinutes"`
}

// Post represents a full blog post including its markdown content.
type Post struct {
	PostSummary
	MarkdownBody string `json:"markdownBody"`
}

// ContentStore provides access to posts and competency metadata.
type ContentStore interface {
	ListPosts(ctx context.Context, pillar, tag string) ([]PostSummary, error)
	GetPost(ctx context.Context, slug string) (*Post, error)
	GetCompetencyMatrix(ctx context.Context) ([]byte, error)
}

var (
	// ErrPostNotFound is returned when a requested post slug does not exist.
	ErrPostNotFound = errors.New("post not found")

	// ErrInvalidSlug is returned when a requested post slug contains invalid characters or path traversal.
	ErrInvalidSlug = errors.New("invalid post slug")
)
