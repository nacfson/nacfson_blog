package main_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBlogServer_SmokeLogContractV3(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "blog-server")
	expectVersion := "v0.0.0-3f2a9c1"

	buildCmd := exec.Command("go", "build", "-ldflags", "-X main.version="+expectVersion, "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build blog-server: %v\n%s", err, out)
	}

	// Pick an ephemeral free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()

	seedDir, err := filepath.Abs("../../tests/fixtures/content")
	if err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	webOutDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webOutDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	var logBuf bytes.Buffer
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"PORT="+port,
		"CONTENT_WORKSPACE_DIR="+workDir,
		"SEED_CONTENT_DIR="+seedDir,
		"WEB_OUT_DIR="+webOutDir,
		"FLUX_ARTIFACT_URL=http://127.0.0.1:1/unreachable.tar.gz",
	)
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start blog-server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	baseURL := "http://127.0.0.1:" + port
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	client := &http.Client{Timeout: 2 * time.Second}

	// Wait for readiness
	ready := false
	for i := 0; i < 40; i++ {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/healthz", nil)
		req.Header.Set("traceparent", traceparent)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("blog-server did not become ready; logs:\n%s", logBuf.String())
	}

	// Normal request
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/posts", nil)
	req.Header.Set("traceparent", traceparent)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/posts failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /api/posts, got %d", resp.StatusCode)
	}

	// Handled error request (500 via ENAMETOOLONG on asset stat)
	longName := strings.Repeat("a", 260)
	errReq, _ := http.NewRequest(http.MethodGet, baseURL+"/content/assets/"+longName, nil)
	errReq.Header.Set("traceparent", traceparent)
	errResp, err := client.Do(errReq)
	if err != nil {
		t.Fatalf("GET /content/assets/<long> failed: %v", err)
	}
	_ = errResp.Body.Close()
	if errResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 on /content/assets/<long>, got %d", errResp.StatusCode)
	}

	// Graceful shutdown
	_ = cmd.Process.Signal(syscall.SIGTERM)
	_ = cmd.Wait()

	captured := logBuf.String()
	if strings.TrimSpace(captured) == "" {
		t.Fatal("captured log output is empty")
	}

	scanner := bufio.NewScanner(strings.NewReader(captured))
	timeRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
	var (
		lineCount      int
		sawStartup     bool
		sawAccess200   bool
		sawError500    bool
		sawShutdown    bool
	)

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d is not valid JSON: %q (err=%v)", lineCount, line, err)
		}
		for _, k := range []string{"time", "level", "msg", "service.name", "service.version"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("line %d missing required key %q: %s", lineCount, k, line)
			}
		}
		if !timeRe.MatchString(fmt.Sprint(m["time"])) {
			t.Fatalf("line %d invalid time format %v", lineCount, m["time"])
		}
		if m["service.name"] != "blog-web" {
			t.Fatalf("line %d expected service.name=blog-web, got %v", lineCount, m["service.name"])
		}
		if m["service.version"] != expectVersion {
			t.Fatalf("line %d expected service.version=%s, got %v", lineCount, expectVersion, m["service.version"])
		}

		msg := fmt.Sprint(m["msg"])
		lvl := fmt.Sprint(m["level"])
		if msg == "initializing blog-server" && lvl == "info" {
			sawStartup = true
		}
		if msg == "blog-server stopped gracefully" && lvl == "info" {
			sawShutdown = true
		}
		if m["url.path"] == "/api/posts" && m["http.response.status_code"] == float64(200) {
			sawAccess200 = true
			if m["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
				t.Fatalf("expected trace_id propagated on /api/posts, got %v", m["trace_id"])
			}
		}
		if m["http.response.status_code"] == float64(500) {
			sawError500 = true
			if lvl != "error" {
				t.Fatalf("expected level=error on 500 line, got %s", lvl)
			}
			if m["exception.type"] == nil || m["exception.message"] == nil {
				t.Fatalf("expected exception.type and exception.message on 500 line: %s", line)
			}
		}
	}

	if !sawStartup || !sawAccess200 || !sawError500 || !sawShutdown {
		t.Fatalf("missing expected smoke events (startup=%v access200=%v error500=%v shutdown=%v)\nlogs:\n%s",
			sawStartup, sawAccess200, sawError500, sawShutdown, captured)
	}
}
