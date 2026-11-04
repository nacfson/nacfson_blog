package studio

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestNormalizeAssetName(t *testing.T) {
	cases := map[string]string{
		"My Diagram (v2).SVG":  "my-diagram-v2.svg",
		"../../etc/passwd.png": "passwd.png",
		`C:\Users\x\photo.JPG`: "photo.jpg",
		"한글.png":               "file.png",
		".hidden.svg":          "hidden.svg",
	}
	for in, want := range cases {
		if got := NormalizeAssetName(in); got != want {
			t.Errorf("NormalizeAssetName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddAsset(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "pics", "Pics")

	res, err := e.m.AddAsset(ctx, "pics", "Arch Diagram.svg", strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "/content/assets/arch-diagram.svg" || res.Kind != "assets" {
		t.Errorf("res = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(d.Worktree, "content/assets/arch-diagram.svg")); err != nil {
		t.Fatal(err)
	}

	// Collisions get a numeric suffix.
	res2, err := e.m.AddAsset(ctx, "pics", "arch diagram.svg", strings.NewReader("<svg/>"))
	if err != nil || res2.URL != "/content/assets/arch-diagram-2.svg" {
		t.Fatalf("collision: %+v, %v", res2, err)
	}

	if _, err := e.m.AddAsset(ctx, "pics", "photo.png", bytes.NewReader(pngHeader)); err != nil {
		t.Fatalf("png: %v", err)
	}

	// Staged by the next Save, along with the post.
	if _, err := e.m.Save(ctx, "pics"); err != nil {
		t.Fatal(err)
	}
	files := git(t, d.Worktree, "show", "--name-only", "--format=", "HEAD")
	for _, want := range []string{"content/assets/arch-diagram.svg", "content/assets/photo.png", d.Path} {
		if !strings.Contains(files, want) {
			t.Errorf("commit missing %s:\n%s", want, files)
		}
	}
}

func TestAddAsset_Rejects(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "rej", "Rej")

	cases := []struct {
		name, file string
		data       []byte
		want       error
	}{
		{"exe", "x.exe", []byte("MZ"), ErrAssetType},
		{"fake png", "x.png", []byte("<html>not an image</html>"), ErrAssetType},
		{"fake svg", "x.svg", []byte("hello"), ErrAssetType},
		{"too large", "big.png", append(append([]byte{}, pngHeader...), make([]byte, MaxAssetBytes)...), ErrAssetTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := e.m.AddAsset(ctx, "rej", c.file, bytes.NewReader(c.data)); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestAddAsset_ContentBudget(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := e.create(t, "full", "Full")
	// Fill content/ close to the 50 MiB limit (sparse file: no disk cost).
	f, err := os.Create(filepath.Join(d.Worktree, "content/assets/huge.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxContentBytes - 100); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, err := e.m.AddAsset(ctx, "full", "p.png", bytes.NewReader(append(pngHeader, make([]byte, 200)...))); !errors.Is(err, ErrContentFull) {
		t.Fatalf("err = %v, want ErrContentFull", err)
	}
}

func TestAddAsset_DemoWarnings(t *testing.T) {
	e := newEnv(t)
	html := "<html>\n<script src=\"https://cdn.example/x.js\"></script>\n<script>fetch('/api')</script>\n<form></form>\n</html>"
	res, err := e.m.AddAsset(context.Background(), e.create(t, "demo", "Demo").Key, "Sim.html", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "/content/demos/sim.html" || len(res.Warnings) != 3 {
		t.Fatalf("res = %+v", res)
	}
	if clean := LintDemo([]byte("<html><script>const x = 1;</script><img src=\"/content/assets/a.svg\"></html>")); len(clean) != 0 {
		t.Errorf("false positives: %+v", clean)
	}
}

func TestResolveContentFile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, "res", "Res")
	if _, err := e.m.AddAsset(ctx, "res", "a.svg", strings.NewReader("<svg/>")); err != nil {
		t.Fatal(err)
	}
	if p, err := e.m.ResolveContentFile(ctx, "res", "assets", "a.svg"); err != nil || !strings.HasSuffix(p, "content/assets/a.svg") {
		t.Fatalf("resolve = %q, %v", p, err)
	}
	for _, bad := range []struct{ kind, rel string }{
		{"assets", "../posts/x.md"}, {"assets", "/etc/passwd"}, {"assets", "."}, {"posts", "x.md"}, {"assets", "missing.svg"},
	} {
		if _, err := e.m.ResolveContentFile(ctx, "res", bad.kind, bad.rel); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("ResolveContentFile(%s, %s) = %v, want ErrNotExist", bad.kind, bad.rel, err)
		}
	}
}
