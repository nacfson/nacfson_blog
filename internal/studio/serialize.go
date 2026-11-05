package studio

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Frontmatter is the editable metadata of a post (spec §4). Parsing is
// structural only: a draft may hold values the production parser rejects
// (empty title, unknown topic); Lint/ValidatePost report those.
type Frontmatter struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Date            string   `json:"date"`
	Topic           string   `json:"topic"`
	Project         string   `json:"project"`
	Summary         string   `json:"summary"`
	Tags            []string `json:"tags"`
	ReadTimeMinutes int      `json:"readTimeMinutes,omitempty"`
	// Extra holds unknown frontmatter lines verbatim so they survive a round trip.
	Extra []string `json:"extra,omitempty"`
}

// Document is a post file split into frontmatter and Markdown body.
type Document struct {
	Frontmatter Frontmatter `json:"frontmatter"`
	Body        string      `json:"body"`
}

var ErrFrontmatter = errors.New("invalid frontmatter")

// ParseDocument splits a post file the same way internal/content does
// (leading "---\n", closing "\n---\n", CRLF normalized, one leading body
// newline dropped) without validating values.
func ParseDocument(data []byte) (*Document, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("%w: missing leading --- delimiter", ErrFrontmatter)
	}
	rest := s[4:]
	var fm, body string
	if i := strings.Index(rest, "\n---\n"); i != -1 {
		fm, body = rest[:i], strings.TrimPrefix(rest[i+5:], "\n")
	} else if strings.HasSuffix(rest, "\n---") {
		fm = rest[:len(rest)-4]
	} else {
		return nil, fmt.Errorf("%w: missing closing --- delimiter", ErrFrontmatter)
	}

	doc := &Document{Body: body, Frontmatter: Frontmatter{Tags: []string{}}}
	f := &doc.Frontmatter
	var current string // key owning following "- item" lines
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			if current == "tags" {
				if item := unquote(strings.TrimPrefix(trimmed, "- ")); item != "" {
					f.Tags = append(f.Tags, item)
				}
			} else if current == "" {
				f.Extra = append(f.Extra, line)
			}
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			f.Extra = append(f.Extra, line)
			current = ""
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		current = key
		switch key {
		case "title":
			f.Title = unquote(val)
		case "slug":
			f.Slug = unquote(val)
		case "date":
			f.Date = unquote(val)
		case "topic":
			f.Topic = unquote(val)
		case "project":
			f.Project = unquote(val)
		case "summary":
			f.Summary = unquote(val)
		case "readtimeminutes":
			f.ReadTimeMinutes, _ = strconv.Atoi(unquote(val))
		case "tags":
			if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				for _, p := range strings.Split(val[1:len(val)-1], ",") {
					if item := unquote(strings.TrimSpace(p)); item != "" {
						f.Tags = append(f.Tags, item)
					}
				}
			}
		default:
			f.Extra = append(f.Extra, line)
			current = "" // unknown key: keep its "- item" lines with it
		}
	}
	return doc, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// SerializeDocument writes a post file in the fixed order of spec §4.
// Values are wrapped in double quotes WITHOUT escaping: internal/content only
// strips the outer pair, so inner quotes must stay raw.
func SerializeDocument(doc *Document) ([]byte, error) {
	f := doc.Frontmatter
	fields := map[string]string{
		"title": f.Title, "slug": f.Slug, "date": f.Date, "topic": f.Topic,
		"project": f.Project, "summary": f.Summary,
	}
	for name, v := range fields {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("%w: %s must be a single line", ErrFrontmatter, name)
		}
	}
	for _, t := range f.Tags {
		if strings.ContainsAny(t, ",\r\n") || strings.TrimSpace(t) == "" {
			return nil, fmt.Errorf("%w: tag %q must be non-empty without commas or newlines", ErrFrontmatter, t)
		}
	}

	var b strings.Builder
	q := func(s string) string { return `"` + s + `"` }
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", q(f.Title))
	fmt.Fprintf(&b, "slug: %s\n", q(f.Slug))
	fmt.Fprintf(&b, "date: %s\n", q(f.Date))
	fmt.Fprintf(&b, "topic: %s\n", q(f.Topic))
	if f.Project != "" {
		fmt.Fprintf(&b, "project: %s\n", q(f.Project))
	}
	fmt.Fprintf(&b, "summary: %s\n", q(f.Summary))
	tags := make([]string, len(f.Tags))
	for i, t := range f.Tags {
		tags[i] = q(strings.TrimSpace(t))
	}
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
	if f.ReadTimeMinutes > 0 {
		fmt.Fprintf(&b, "readTimeMinutes: %d\n", f.ReadTimeMinutes)
	}
	for _, line := range f.Extra {
		if strings.ContainsAny(line, "\r\n") || strings.TrimSpace(line) == "---" {
			return nil, fmt.Errorf("%w: invalid preserved line %q", ErrFrontmatter, line)
		}
		b.WriteString(line + "\n")
	}
	// Blank line after the delimiter: the parser drops exactly one leading
	// newline, so the body round-trips byte for byte.
	b.WriteString("---\n\n")
	b.WriteString(strings.ReplaceAll(doc.Body, "\r\n", "\n"))
	return []byte(b.String()), nil
}

// Slugify turns a title into a slug accepted by internal/content.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}
