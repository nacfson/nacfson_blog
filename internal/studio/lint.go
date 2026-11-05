package studio

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/nacfson/nacfson_blog/internal/content"
)

// Severity of a lint issue. Errors block Save and Publish; warnings never do.
type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warning"
)

// Issue is one finding. Field names a frontmatter field; Line is 1-based
// within the Markdown body (the part shown in the editor).
type Issue struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Field    string   `json:"field,omitempty"`
	Line     int      `json:"line,omitempty"`
}

// LintOptions supplies repository context. Nil funcs skip those checks.
type LintOptions struct {
	// SlugTaken reports whether another post already uses slug.
	SlugTaken func(slug string) bool
	// RefExists reports whether a /content/assets/... or /content/demos/... URL exists.
	RefExists func(url string) bool
}

// CustomBlocks are the fenced JSON block types rendered by MarkdownDispatcher.
var CustomBlocks = []string{"scrollHero", "blogPicture", "blogIframeTabs", "benchmarkChart", "benchmarkTable"}

// Same tokenizer shape as web/src/components/MarkdownDispatcher.tsx.
var fenceRe = regexp.MustCompile("(?m)^```([a-zA-Z0-9_-]*)\\r?\\n([\\s\\S]*?)\\r?\\n```(?:\\r?\\n|$)")

var (
	reImage    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reOrdered  = regexp.MustCompile(`(?m)^[ \t]*\d+\.[ \t]`)
	rePipe     = regexp.MustCompile(`(?m)^[ \t]*\|.*\|[ \t]*$`)
	reHTML     = regexp.MustCompile(`</?[a-zA-Z][^>\n]*>`)
	reH1       = regexp.MustCompile(`(?m)^# `)
	reBoldCode = regexp.MustCompile("\\*\\*`[^`\n]+`\\*\\*")
	reInline   = regexp.MustCompile("`[^`\n]*`")
	reRef      = regexp.MustCompile(`/content/(?:assets|demos)/[A-Za-z0-9._/-]+`)
	reItalic   = regexp.MustCompile(`([*_])([^\s*_](?:[^*_\n]*[^\s*_])?)([*_])`)
)

var validTopics = map[string]bool{
	string(content.TopicCloudNative): true, string(content.TopicZeroTrust): true,
	string(content.TopicAIAgents): true, string(content.TopicOpenSource): true,
}

// Lint checks a full post file (spec §5).
func Lint(data []byte, filename string, opts LintOptions) []Issue {
	doc, err := ParseDocument(data)
	if err != nil {
		return []Issue{{Severity: SevError, Message: err.Error()}}
	}
	var issues []Issue
	add := func(sev Severity, field string, line int, format string, a ...any) {
		issues = append(issues, Issue{Severity: sev, Field: field, Line: line, Message: fmt.Sprintf(format, a...)})
	}

	// Frontmatter errors: one message per field, mirroring internal/content.
	f := doc.Frontmatter
	if strings.TrimSpace(f.Title) == "" {
		add(SevError, "title", 0, "title is required")
	}
	switch {
	case f.Slug == "":
		add(SevError, "slug", 0, "slug is required")
	case !ValidKey(f.Slug) && f.Slug != publishKey:
		add(SevError, "slug", 0, "slug may only contain letters, digits, - and _")
	case opts.SlugTaken != nil && opts.SlugTaken(f.Slug):
		add(SevError, "slug", 0, "slug %q is already used by another post", f.Slug)
	}
	if !dateRe.MatchString(f.Date) {
		add(SevError, "date", 0, "date must be YYYY-MM-DD")
	}
	if !validTopics[f.Topic] {
		add(SevError, "topic", 0, "topic must be one of cloud-native, zero-trust, ai-agents, open-source")
	}
	// Belt and braces: anything else the production parser rejects.
	if len(issues) == 0 {
		if _, err := content.ValidatePost(data, filename); err != nil {
			add(SevError, "", 0, "%v", err)
		}
	}
	if strings.TrimSpace(f.Summary) == "" {
		add(SevWarn, "summary", 0, "summary is empty: the home-page card will have no description")
	}

	issues = append(issues, LintBody(doc.Body, opts)...)
	return issues
}

// LintBody checks the Markdown body only. Line numbers are 1-based in body.
func LintBody(body string, opts LintOptions) []Issue {
	var issues []Issue
	lineAt := func(off int) int { return strings.Count(body[:off], "\n") + 1 }

	// Custom block JSON.
	for _, m := range fenceRe.FindAllStringSubmatchIndex(body, -1) {
		lang := body[m[2]:m[3]]
		if !isCustomBlock(lang) {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(body[m[4]:m[5]]), &v); err != nil {
			issues = append(issues, Issue{Severity: SevError, Line: lineAt(m[0]),
				Message: fmt.Sprintf("%s: invalid JSON (%v)", lang, err)})
		}
	}

	// Blank out fenced blocks (keeping newlines) so prose rules ignore code.
	prose := fenceRe.ReplaceAllStringFunc(body, blankKeepNewlines)
	warn := func(off int, msg string) {
		issues = append(issues, Issue{Severity: SevWarn, Line: lineAt(off), Message: msg})
	}
	if loc := reBoldCode.FindStringIndex(prose); loc != nil {
		warn(loc[0], "Code inside bold renders literal backticks: use **text** or `code`, not both")
	}
	// Then blank inline code too.
	prose = reInline.ReplaceAllStringFunc(prose, blankKeepNewlines)

	rules := []struct {
		re  *regexp.Regexp
		msg string
	}{
		{reImage, "Markdown images don't render: use a blogPicture block"},
		{reOrdered, `Ordered lists render as a plain paragraph: use "-" bullets`},
		{rePipe, "Pipe tables don't render: use a benchmarkTable block"},
		{reHTML, "Raw HTML is shown as text, not rendered"},
		{reH1, "Body H1 duplicates the article title: start sections at ##"},
	}
	for _, r := range rules {
		if loc := r.re.FindStringIndex(prose); loc != nil {
			warn(loc[0], r.msg)
		}
	}
	if off := findItalic(prose); off >= 0 {
		warn(off, "Italics aren't supported: text will show literal * or _")
	}

	if opts.RefExists != nil {
		seen := map[string]bool{}
		for _, loc := range reRef.FindAllStringIndex(body, -1) {
			ref := strings.TrimRight(body[loc[0]:loc[1]], ".")
			if seen[ref] {
				continue
			}
			seen[ref] = true
			if !opts.RefExists(ref) {
				warn(loc[0], ref+" not found in this draft")
			}
		}
	}
	return issues
}

func isCustomBlock(lang string) bool {
	for _, b := range CustomBlocks {
		if b == lang {
			return true
		}
	}
	return false
}

func blankKeepNewlines(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return '\n'
		}
		return ' '
	}, s)
}

// findItalic returns the offset of the first *x* or _x_ span that is not part
// of **bold**, a "* " bullet, or a snake_case word; -1 if none.
func findItalic(s string) int {
	isWord := func(i int) bool {
		if i < 0 || i >= len(s) {
			return false
		}
		r := rune(s[i])
		return r == '*' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	for _, m := range reItalic.FindAllStringSubmatchIndex(s, -1) {
		open, close := s[m[2]:m[3]], s[m[6]:m[7]]
		if open != close || isWord(m[0]-1) || isWord(m[1]) {
			continue
		}
		return m[0]
	}
	return -1
}
