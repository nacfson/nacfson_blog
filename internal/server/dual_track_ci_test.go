package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualTrackPipelineInvariants(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	wfBytes, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "publish-ghcr-image.yml"))
	if err != nil {
		t.Fatalf("missing workflow file: %v", err)
	}
	wf := string(wfBytes)
	for _, required := range []string{`"web/**"`, `"cmd/**"`, `"internal/**"`, `"Dockerfile"`, `linux/amd64,linux/arm64`} {
		if !strings.Contains(wf, required) {
			t.Errorf("workflow missing required token %s", required)
		}
	}
	if strings.Contains(wf, `"content/**"`) {
		t.Errorf("workflow must NOT trigger Docker builds on content/** changes")
	}

	dfBytes, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("missing Dockerfile: %v", err)
	}
	df := string(dfBytes)
	for _, required := range []string{"10001:10001", "/app/blog-server", "/app/web/out", "/app/seed-content", "EXPOSE 8080"} {
		if !strings.Contains(df, required) {
			t.Errorf("Dockerfile missing required directive %q", required)
		}
	}
}
