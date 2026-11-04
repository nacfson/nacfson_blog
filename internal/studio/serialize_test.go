package studio

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
)

func sampleDoc() *Document {
	return &Document{
		Frontmatter: Frontmatter{
			Title:   `Running "AI" Agents: On K3s — 한국어 ✓`,
			Slug:    "ai-agents",
			Date:    "2026-11-04",
			Topic:   "ai-agents",
			Project: "nacfson_pipeline",
			Summary: `Gateway "pattern": no keys.`,
			Tags:    []string{"ai-agents", "open bao", `q"uote`},
		},
		Body: "\nLeading blank line kept.\n\n## Section\n\n```benchmarkTable\n{\"columns\":[]}\n```\n",
	}
}

func TestSerialize_RoundTripsThroughProductionParser(t *testing.T) {
	doc := sampleDoc()
	data, err := SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	post, err := content.ValidatePost(data, "x.md")
	if err != nil {
		t.Fatalf("production parser rejected serialized post: %v\n%s", err, data)
	}
	f := doc.Frontmatter
	if post.Title != f.Title || post.Slug != f.Slug || post.Date != f.Date || string(post.Topic) != f.Topic ||
		post.Project != f.Project || post.Summary != f.Summary || !reflect.DeepEqual(post.Tags, f.Tags) {
		t.Errorf("mismatch:\n got %+v\nwant %+v", post.PostSummary, f)
	}
	if post.MarkdownBody != doc.Body {
		t.Errorf("body changed:\n got %q\nwant %q", post.MarkdownBody, doc.Body)
	}

	back, err := ParseDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, doc) {
		t.Errorf("ParseDocument(Serialize(doc)) != doc:\n got %+v\nwant %+v", back, doc)
	}
}

func TestSerialize_FixedOrderAndOptionalFields(t *testing.T) {
	doc := &Document{Frontmatter: Frontmatter{Title: "T", Slug: "t", Date: "2026-11-04", Topic: "open-source", Tags: []string{}}, Body: "b\n"}
	data, err := SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntitle: \"T\"\nslug: \"t\"\ndate: \"2026-11-04\"\ntopic: \"open-source\"\nsummary: \"\"\ntags: []\n---\n\nb\n"
	if string(data) != want {
		t.Errorf("got\n%s\nwant\n%s", data, want)
	}
}

func TestSerialize_Rejects(t *testing.T) {
	cases := map[string]func(*Document){
		"newline in title":   func(d *Document) { d.Frontmatter.Title = "a\nb" },
		"CR in summary":      func(d *Document) { d.Frontmatter.Summary = "a\rb" },
		"comma in tag":       func(d *Document) { d.Frontmatter.Tags = []string{"a,b"} },
		"empty tag":          func(d *Document) { d.Frontmatter.Tags = []string{" "} },
		"delimiter in extra": func(d *Document) { d.Frontmatter.Extra = []string{"---"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := sampleDoc()
			mutate(d)
			if _, err := SerializeDocument(d); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParse_PreservesUnknownKeysAndReadTime(t *testing.T) {
	in := "---\r\ntitle: 'Single'\r\nslug: s\r\ndate: \"2026-01-02\"\r\ntopic: \"zero-trust\"\r\n" +
		"draft: true\r\naliases:\r\n  - /old\r\ntags:\r\n  - a\r\n  - \"b\"\r\nreadTimeMinutes: 9\r\n---\r\nBody\r\n"
	doc, err := ParseDocument([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	f := doc.Frontmatter
	if f.Title != "Single" || f.Slug != "s" || f.ReadTimeMinutes != 9 || !reflect.DeepEqual(f.Tags, []string{"a", "b"}) {
		t.Errorf("parsed %+v", f)
	}
	if !reflect.DeepEqual(f.Extra, []string{"draft: true", "aliases:", "  - /old"}) {
		t.Errorf("extra = %q", f.Extra)
	}
	out, err := SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"readTimeMinutes: 9\n", "draft: true\n", "aliases:\n  - /old\n"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("serialized output missing %q:\n%s", want, out)
		}
	}
	if post, err := content.ValidatePost(out, "s.md"); err != nil || post.ReadTimeMinutes != 9 {
		t.Errorf("ValidatePost: %+v, %v", post, err)
	}
}

func TestParse_Errors(t *testing.T) {
	for _, in := range []string{"no frontmatter", "---\ntitle: x\n"} {
		if _, err := ParseDocument([]byte(in)); err == nil {
			t.Errorf("ParseDocument(%q): expected error", in)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Running AI Agents on K3s!": "running-ai-agents-on-k3s",
		"  --Hello,  World--  ":     "hello-world",
		"한국어 only":                  "only",
		"":                          "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Slugify(strings.Repeat("a", 100)); len(got) != 64 {
		t.Errorf("len = %d, want 64", len(got))
	}
}
