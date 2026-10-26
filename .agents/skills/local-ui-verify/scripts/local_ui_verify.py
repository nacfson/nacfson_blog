#!/usr/bin/env python3
"""Cross-agent Local UI Verification & Manual Browser Preview Runner.

Supports:
  - Automated Agent Verification (--mode verify, default):
    Builds/starts the local web server on an isolated ephemeral port, validates
    HTTP/API/CSP contracts, and executes headless Playwright Chromium checks
    (DOM rendering, interactive state transitions, zero console/page errors,
    and desktop/mobile screenshots).
  - Manual Browser Preview (--mode manual / --serve):
    Starts or reuses a persistent background preview server (default :8080) so
    a human developer can inspect and interact with the UI in their own browser.
  - Multi-Agent Harness Installer (--install-harnesses):
    Installs and synchronizes skill discovery and slash commands across
    Claude Code, OpenAI Codex, Oh-my-pi (omp), OpenCode, and Antigravity (agy).
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
SKILL_DIR = SCRIPT_DIR.parent
PROBE_SCRIPT = SCRIPT_DIR / "playwright_probe.mjs"

CLAUDE_OPENCODE_COMMAND_MD = """---
description: Run automated headless browser + HTTP/CSP UI verification (default) or launch a persistent local preview server (--mode manual) for interactive browser testing.
---

## User Input

```text
$ARGUMENTS
```

## Instructions

1. Follow the workflow in `.agents/skills/local-ui-verify/SKILL.md` (or `~/.agents/skills/local-ui-verify/SKILL.md`).
2. **Default (Automated Agent UI Verification)**:
   ```bash
   python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode verify --json
   ```
3. **Manual Browser Preview (When the user asks to test/preview in their own browser)**:
   ```bash
   python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode manual --port 8080
   ```
"""


def find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def is_port_listening(port: int, host: str = "127.0.0.1") -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.5)
        return sock.connect_ex((host, port)) == 0


def wait_for_http(url: str, timeout_sec: float = 15.0) -> bool:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=1.5) as resp:
                if 200 <= resp.status < 400:
                    return True
        except Exception:
            time.sleep(0.2)
    return False


def http_get(url: str) -> tuple[int, dict[str, str], str]:
    req = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=5.0) as resp:
            headers = {k.lower(): v for k, v in resp.headers.items()}
            body = resp.read().decode("utf-8", errors="replace")
            return resp.status, headers, body
    except urllib.error.HTTPError as err:
        headers = {k.lower(): v for k, v in err.headers.items()} if err.headers else {}
        body = err.read().decode("utf-8", errors="replace") if err.fp else ""
        return err.code, headers, body


def is_blog_repo(root: Path) -> bool:
    return (root / "cmd" / "blog-server" / "main.go").exists() and (root / "web" / "package.json").exists()


def ensure_blog_built(root: Path, binary_path: Path, rebuild: bool = False) -> None:
    web_out = root / "web" / "out" / "index.html"
    if rebuild or not web_out.exists():
        subprocess.run(["npm", "run", "build"], cwd=root / "web", check=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    subprocess.run(
        ["go", "build", "-o", str(binary_path), "./cmd/blog-server"],
        cwd=root,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )


def run_blog_http_checks(base_url: str) -> dict[str, Any]:
    checks: list[dict[str, Any]] = []

    def add_check(name: str, passed: bool, detail: str) -> None:
        checks.append({"name": name, "passed": bool(passed), "detail": detail})

    status, _, body = http_get(f"{base_url}/healthz")
    add_check("healthz_200", status == 200 and '"ok"' in body, f"HTTP {status} {body.strip()}")

    status, _, body = http_get(f"{base_url}/api/posts")
    posts_ok = False
    first_slug = "cilium-zero-plaintext-k3s"
    if status == 200:
        try:
            posts = json.loads(body)
            posts_ok = isinstance(posts, list) and len(posts) >= 1
            if posts_ok and "slug" in posts[0]:
                first_slug = posts[0]["slug"]
        except Exception:
            posts_ok = False
    add_check("api_posts_list", posts_ok, f"HTTP {status}, first_slug={first_slug}")

    status, _, body = http_get(f"{base_url}/api/posts/{first_slug}")
    add_check("api_post_detail", status == 200 and first_slug in body, f"HTTP {status} for /api/posts/{first_slug}")

    status, _, _ = http_get(f"{base_url}/api/competencies")
    add_check("api_competencies_removed_404", status == 404, f"HTTP {status}")

    status, headers, _ = http_get(f"{base_url}/content/demos/hubble-flow-sim.html")
    csp = headers.get("content-security-policy", "")
    nosniff = headers.get("x-content-type-options", "")
    add_check(
        "demo_strict_csp_and_nosniff",
        status == 200 and "connect-src 'none'" in csp and nosniff == "nosniff",
        f"HTTP {status}, CSP={csp}, X-Content-Type-Options={nosniff}",
    )

    status, _, _ = http_get(f"{base_url}/api/nonexistent-endpoint")
    add_check("unmatched_api_returns_404", status == 404, f"HTTP {status}")

    return {
        "all_passed": all(c["passed"] for c in checks),
        "checks": checks,
    }


def run_generic_http_checks(base_url: str) -> dict[str, Any]:
    status, headers, body = http_get(f"{base_url}/")
    passed = 200 <= status < 400 and len(body.strip()) > 0
    return {
        "all_passed": passed,
        "checks": [
            {
                "name": "root_http_200",
                "passed": passed,
                "detail": f"HTTP {status}, content-type={headers.get('content-type', 'unknown')}",
            }
        ],
    }


def run_verify_mode(
    root: Path,
    base_url: str | None,
    artifact_dir: Path,
    routes: str,
    rebuild: bool,
) -> dict[str, Any]:
    artifact_dir.mkdir(parents=True, exist_ok=True)
    blog_mode = is_blog_repo(root) and not base_url
    proc: subprocess.Popen[str] | None = None
    temp_dir_obj: tempfile.TemporaryDirectory[str] | None = None

    try:
        if blog_mode:
            temp_dir_obj = tempfile.TemporaryDirectory(prefix="blog-ui-verify-")
            temp_dir = Path(temp_dir_obj.name)
            binary_path = temp_dir / "blog-server"
            workspace_dir = temp_dir / "workspace"
            workspace_dir.mkdir(parents=True, exist_ok=True)
            ensure_blog_built(root, binary_path, rebuild=rebuild)

            port = find_free_port()
            env = os.environ.copy()
            env.update(
                {
                    "PORT": str(port),
                    "STORAGE_MODE": "folder",
                    "CONTENT_WORKSPACE_DIR": str(workspace_dir),
                    "SEED_CONTENT_DIR": str(root / "tests" / "fixtures" / "content"),
                    "WEB_OUT_DIR": str(root / "web" / "out"),
                }
            )
            proc = subprocess.Popen(
                [str(binary_path)],
                cwd=root,
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            target_url = f"http://127.0.0.1:{port}"
            if not wait_for_http(f"{target_url}/healthz", timeout_sec=15.0):
                raise RuntimeError("Timed out waiting for local blog-server /healthz")
            http_report = run_blog_http_checks(target_url)
            profile = "blog"
        else:
            target_url = (base_url or "http://127.0.0.1:8080").rstrip("/")
            if not wait_for_http(f"{target_url}/", timeout_sec=10.0):
                raise RuntimeError(f"Target URL {target_url} is not reachable")
            http_report = run_generic_http_checks(target_url)
            profile = "generic"

        probe_cmd = [
            "node",
            str(PROBE_SCRIPT),
            "--base-url",
            target_url,
            "--profile",
            profile,
            "--artifact-dir",
            str(artifact_dir),
        ]
        if routes:
            probe_cmd.extend(["--routes", routes])

        probe_res = subprocess.run(probe_cmd, cwd=root, capture_output=True, text=True, check=False)
        try:
            browser_report = json.loads(probe_res.stdout)
        except Exception:
            browser_report = {
                "all_passed": False,
                "assertions": [],
                "console_errors": [probe_res.stderr or probe_res.stdout],
                "page_errors": [],
                "failed_requests": [],
                "screenshots": [],
            }

        overall_passed = bool(http_report["all_passed"] and browser_report.get("all_passed"))
        return {
            "status": "PASS" if overall_passed else "FAIL",
            "mode": "verify",
            "profile": profile,
            "base_url": target_url,
            "artifact_dir": str(artifact_dir),
            "http_checks": http_report,
            "browser_checks": browser_report,
        }
    finally:
        if proc is not None and proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
        if temp_dir_obj is not None:
            temp_dir_obj.cleanup()


def state_file_for_port(port: int) -> Path:
    return Path(f"/tmp/local-ui-verify-{port}.json")


def run_manual_mode(root: Path, port: int, rebuild: bool) -> dict[str, Any]:
    state_file = state_file_for_port(port)
    base_url = f"http://localhost:{port}"

    if is_port_listening(port) and wait_for_http(f"http://127.0.0.1:{port}/healthz", timeout_sec=2.0):
        return {
            "status": "RUNNING",
            "mode": "manual",
            "reused_existing": True,
            "port": port,
            "urls": {
                "home": f"{base_url}/",
                "healthz": f"{base_url}/healthz",
                "api_posts": f"{base_url}/api/posts",
            },
        }

    if not is_blog_repo(root):
        raise RuntimeError(f"Manual auto-start requires nacfson_blog layout at {root}")

    runtime_dir = Path(f"/tmp/local-ui-verify-runtime-{port}")
    if runtime_dir.exists():
        shutil.rmtree(runtime_dir)
    runtime_dir.mkdir(parents=True, exist_ok=True)
    binary_path = runtime_dir / "blog-server"
    workspace_dir = runtime_dir / "workspace"
    workspace_dir.mkdir(parents=True, exist_ok=True)
    log_path = runtime_dir / "server.log"

    ensure_blog_built(root, binary_path, rebuild=rebuild)

    env = os.environ.copy()
    env.update(
        {
            "PORT": str(port),
            "STORAGE_MODE": "folder",
            "CONTENT_WORKSPACE_DIR": str(workspace_dir),
            "SEED_CONTENT_DIR": str(root / "content"),
            "WEB_OUT_DIR": str(root / "web" / "out"),
        }
    )

    log_fp = open(log_path, "w", encoding="utf-8")
    proc = subprocess.Popen(
        [str(binary_path)],
        cwd=root,
        env=env,
        stdout=log_fp,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )

    if not wait_for_http(f"http://127.0.0.1:{port}/healthz", timeout_sec=15.0):
        raise RuntimeError(f"Manual preview server failed to start on port {port}. Check {log_path}")

    payload = {
        "status": "RUNNING",
        "mode": "manual",
        "reused_existing": False,
        "pid": proc.pid,
        "port": port,
        "log_path": str(log_path),
        "runtime_dir": str(runtime_dir),
        "urls": {
            "home": f"{base_url}/",
            "healthz": f"{base_url}/healthz",
            "api_posts": f"{base_url}/api/posts",
        },
    }
    state_file.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    return payload


def run_status_mode(port: int) -> dict[str, Any]:
    listening = is_port_listening(port)
    healthy = wait_for_http(f"http://127.0.0.1:{port}/healthz", timeout_sec=1.5) if listening else False
    state_file = state_file_for_port(port)
    meta = json.loads(state_file.read_text(encoding="utf-8")) if state_file.exists() else {}
    return {
        "status": "RUNNING" if (listening and healthy) else "STOPPED",
        "port": port,
        "pid": meta.get("pid"),
        "urls": meta.get(
            "urls",
            {
                "home": f"http://localhost:{port}/",
            },
        ),
    }


def run_stop_mode(port: int) -> dict[str, Any]:
    state_file = state_file_for_port(port)
    killed_pid = None
    if state_file.exists():
        meta = json.loads(state_file.read_text(encoding="utf-8"))
        pid = meta.get("pid")
        if isinstance(pid, int):
            try:
                os.kill(pid, signal.SIGTERM)
                killed_pid = pid
            except ProcessLookupError:
                pass
        runtime_dir = meta.get("runtime_dir")
        if runtime_dir and Path(runtime_dir).exists():
            shutil.rmtree(runtime_dir, ignore_errors=True)
        state_file.unlink(missing_ok=True)
    return {
        "status": "STOPPED",
        "port": port,
        "killed_pid": killed_pid,
    }


def copy_skill_tree(src_skill_dir: Path, dst_skill_dir: Path) -> None:
    if dst_skill_dir.resolve() == src_skill_dir.resolve():
        return
    if dst_skill_dir.is_symlink() or dst_skill_dir.is_file():
        dst_skill_dir.unlink()
    elif dst_skill_dir.exists():
        shutil.rmtree(dst_skill_dir)
    dst_skill_dir.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(src_skill_dir, dst_skill_dir)


def install_global_harnesses(global_home: Path) -> dict[str, Any]:
    skill_targets = [
        global_home / ".agents" / "skills" / "local-ui-verify",
        global_home / ".claude" / "skills" / "local-ui-verify",
        global_home / ".codex" / "skills" / "local-ui-verify",
        global_home / ".omp" / "skills" / "local-ui-verify",
        global_home / ".omp" / "agent" / "skills" / "local-ui-verify",
        global_home / ".opencode" / "skills" / "local-ui-verify",
        global_home / ".config" / "opencode" / "skills" / "local-ui-verify",
        global_home / ".gemini" / "config" / "skills" / "local-ui-verify",
        global_home / ".gemini" / "antigravity" / "skills" / "local-ui-verify",
    ]
    for target in skill_targets:
        copy_skill_tree(SKILL_DIR, target)

    command_targets = [
        global_home / ".claude" / "commands" / "local-ui-verify.md",
        global_home / ".omp" / "commands" / "ui.verify.md",
        global_home / ".omp" / "agent" / "commands" / "ui.verify.md",
        global_home / ".opencode" / "commands" / "local-ui-verify.md",
        global_home / ".config" / "opencode" / "commands" / "local-ui-verify.md",
    ]
    for cmd_file in command_targets:
        cmd_file.parent.mkdir(parents=True, exist_ok=True)
        cmd_file.write_text(CLAUDE_OPENCODE_COMMAND_MD, encoding="utf-8")

    return {
        "status": "INSTALLED",
        "global_home": str(global_home),
        "agents_covered": ["claude-code", "codex", "omp", "opencode", "antigravity"],
        "skill_targets": [str(p) for p in skill_targets],
        "command_targets": [str(p) for p in command_targets],
    }


def format_markdown(result: dict[str, Any]) -> str:
    mode = result.get("mode")
    if mode == "manual" or result.get("status") in {"RUNNING", "STOPPED"}:
        lines = [
            f"### Local UI Preview (`{result.get('status')}`)",
            f"- **Port**: `{result.get('port')}`",
        ]
        urls = result.get("urls") or {}
        for label, url in urls.items():
            lines.append(f"- **{label}**: {url}")
        return "\n".join(lines)

    lines = [
        f"### Local UI Verification Report (`{result.get('status')}`)",
        f"- **Base URL**: `{result.get('base_url')}`",
        f"- **Artifact Dir**: `{result.get('artifact_dir')}`",
        "",
        "#### HTTP & Security Header Checks",
    ]
    for chk in result.get("http_checks", {}).get("checks", []):
        mark = "PASS" if chk["passed"] else "FAIL"
        lines.append(f"- [{mark}] `{chk['name']}` — {chk['detail']}")

    lines.append("")
    lines.append("#### Headless Browser (Playwright Chromium) Checks")
    bchk = result.get("browser_checks", {})
    for chk in bchk.get("assertions", []):
        mark = "PASS" if chk["passed"] else "FAIL"
        lines.append(f"- [{mark}] `{chk['name']}` — {chk['detail']}")
    lines.append(f"- Console Errors: `{len(bchk.get('console_errors', []))}`")
    lines.append(f"- Page Errors: `{len(bchk.get('page_errors', []))}`")
    for shot in bchk.get("screenshots", []):
        lines.append(f"- Screenshot: `{shot}`")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="Cross-agent local UI verifier and manual preview server manager.")
    parser.add_argument(
        "--mode",
        choices=["verify", "manual", "status", "stop"],
        default="verify",
        help="verify (default headless+HTTP check), manual (persistent browser preview), status, or stop.",
    )
    parser.add_argument("--serve", action="store_true", help="Alias for --mode manual.")
    parser.add_argument("--root", default=".", help="Repository root directory.")
    parser.add_argument("--base-url", default=None, help="Target existing URL instead of starting local blog-server.")
    parser.add_argument("--port", type=int, default=8080, help="Port for --mode manual/status/stop (default: 8080).")
    parser.add_argument(
        "--artifact-dir",
        default="/tmp/local-ui-verify-artifacts",
        help="Directory to write headless browser screenshots.",
    )
    parser.add_argument("--routes", default="", help="Comma-separated routes for generic profile.")
    parser.add_argument("--rebuild", action="store_true", help="Force rebuild of web/out before starting server.")
    parser.add_argument(
        "--install-harnesses",
        action="store_true",
        help="Install skill and command files across global directories for Claude Code, Codex, omp, OpenCode, and Antigravity.",
    )
    parser.add_argument(
        "--global-home",
        default=str(Path.home()),
        help="Home directory target for --install-harnesses (default: ~).",
    )
    parser.add_argument("--json", action="store_true", help="Emit JSON output.")
    args = parser.parse_args()

    if args.install_harnesses:
        res = install_global_harnesses(Path(args.global_home).resolve())
        if args.json:
            print(json.dumps(res, indent=2))
        else:
            print(f"Installed local-ui-verify across {', '.join(res['agents_covered'])} in {res['global_home']}")
        return 0

    mode = "manual" if args.serve else args.mode
    root = Path(args.root).resolve()

    if mode == "verify":
        result = run_verify_mode(root, args.base_url, Path(args.artifact_dir).resolve(), args.routes, args.rebuild)
        exit_code = 0 if result["status"] == "PASS" else 1
    elif mode == "manual":
        result = run_manual_mode(root, args.port, args.rebuild)
        exit_code = 0
    elif mode == "status":
        result = run_status_mode(args.port)
        exit_code = 0 if result["status"] == "RUNNING" else 1
    else:
        result = run_stop_mode(args.port)
        exit_code = 0

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(format_markdown(result))
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
