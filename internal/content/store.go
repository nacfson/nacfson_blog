package content

import (
	"context"
	"errors"
)

// Topic represents one of the four interest topics a post belongs to.
type Topic string

const (
	TopicCloudNative Topic = "cloud-native"
	TopicZeroTrust   Topic = "zero-trust"
	TopicAIAgents    Topic = "ai-agents"
	TopicOpenSource  Topic = "open-source"
)

// PostSummary contains metadata for post listings and navigation.
type PostSummary struct {
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Date            string   `json:"date"`
	Topic           Topic    `json:"topic"`
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

// ContentStore provides access to blog posts.
type ContentStore interface {
	ListPosts(ctx context.Context, topic, tag string) ([]PostSummary, error)
	GetPost(ctx context.Context, slug string) (*Post, error)
}

var (
	// ErrPostNotFound is returned when a requested post slug does not exist.
	ErrPostNotFound = errors.New("post not found")

	// ErrInvalidSlug is returned when a requested post slug contains invalid characters or path traversal.
	ErrInvalidSlug = errors.New("invalid post slug")
)
