package studio

import (
	"strings"
	"testing"
)

func lintFile(fm, body string, opts LintOptions) []Issue {
	return Lint([]byte("---\n"+fm+"---\n\n"+body), "x.md", opts)
}

const goodFM = "title: \"T\"\nslug: \"t\"\ndate: \"2026-11-04\"\ntopic: \"ai-agents\"\nsummary: \"S\"\ntags: []\n"

func find(issues []Issue, sub string) *Issue {
	for i := range issues {
		if strings.Contains(issues[i].Message, sub) {
			return &issues[i]
		}
	}
	return nil
}

func TestLint_CleanPost(t *testing.T) {
	body := "Intro with `code`, **bold**, a snake_case_word and [link](https://x).\n\n## Section\n\n- item\n* item\n\n" +
		"```go\nfunc *x* <b> 1. | a | ![i](x)\n```\n\n```benchmarkChart\n{\"title\":\"t\",\"series\":[]}\n```\n"
	if issues := lintFile(goodFM, body, LintOptions{}); len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestLint_FrontmatterErrors(t *testing.T) {
	fm := "title: \"\"\nslug: \"bad slug\"\ndate: \"Nov 4\"\ntopic: \"pillar\"\nsummary: \"\"\n"
	issues := lintFile(fm, "body\n", LintOptions{})
	want := map[string]Severity{"title": SevError, "slug": SevError, "date": SevError, "topic": SevError, "summary": SevWarn}
	got := map[string]Severity{}
	for _, i := range issues {
		got[i.Field] = i.Severity
	}
	for field, sev := range want {
		if got[field] != sev {
			t.Errorf("field %s: severity %q, want %q (issues %+v)", field, got[field], sev, issues)
		}
	}
}

func TestLint_DuplicateSlug(t *testing.T) {
	issues := lintFile(goodFM, "b\n", LintOptions{SlugTaken: func(s string) bool { return s == "t" }})
	if i := find(issues, "already used"); i == nil || i.Field != "slug" || i.Severity != SevError {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLint_MissingDelimiter(t *testing.T) {
	issues := Lint([]byte("no frontmatter"), "x.md", LintOptions{})
	if len(issues) != 1 || issues[0].Severity != SevError {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLintBody_Rules(t *testing.T) {
	cases := []struct {
		name, body, msg string
		line            int
		sev             Severity
	}{
		{"bad block json", "a\n\n```benchmarkTable\n{nope}\n```\n", "benchmarkTable: invalid JSON", 3, SevError},
		{"image", "a\n![x](/y.png)\n", "Markdown images", 2, SevWarn},
		{"ordered", "a\n\n1. one\n", "Ordered lists", 3, SevWarn},
		{"pipe table", "| a | b |\n", "Pipe tables", 1, SevWarn},
		{"html", "x <div>y</div>\n", "Raw HTML", 1, SevWarn},
		{"h1", "# Title\n", "Body H1", 1, SevWarn},
		{"italic star", "an *emphasis* here\n", "Italics", 1, SevWarn},
		{"italic underscore", "an _emphasis_ here\n", "Italics", 1, SevWarn},
		{"bold code", "a\nOn **`x`** we\n", "Code inside bold", 2, SevWarn},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := LintBody(c.body, LintOptions{})
			i := find(issues, c.msg)
			if i == nil {
				t.Fatalf("missing %q in %+v", c.msg, issues)
			}
			if i.Line != c.line || i.Severity != c.sev {
				t.Errorf("got line %d sev %s, want line %d sev %s", i.Line, i.Severity, c.line, c.sev)
			}
		})
	}
}

func TestLintBody_NoFalsePositives(t *testing.T) {
	body := "**bold** and **more bold**\n\n* bullet one\n* bullet two\n\nsnake_case_name and 2*3*4\n\n" +
		"Inline `<div>` and `*x*` and `1. x` are code.\n"
	if issues := LintBody(body, LintOptions{}); len(issues) != 0 {
		t.Fatalf("false positives: %+v", issues)
	}
}

func TestLintBody_Refs(t *testing.T) {
	body := "```blogPicture\n{\"src\": \"/content/assets/a.svg\"}\n```\n\nSee /content/demos/missing.html.\n"
	issues := LintBody(body, LintOptions{RefExists: func(u string) bool { return u == "/content/assets/a.svg" }})
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "/content/demos/missing.html not found") || issues[0].Line != 5 {
		t.Fatalf("issues = %+v", issues)
	}
}
