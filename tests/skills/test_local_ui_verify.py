import json
import os
import socket
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SKILL_MD = REPO_ROOT / ".agents" / "skills" / "local-ui-verify" / "SKILL.md"
SCRIPT_PATH = REPO_ROOT / ".agents" / "skills" / "local-ui-verify" / "scripts" / "local_ui_verify.py"


def find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class LocalUIVerifySkillTest(unittest.TestCase):
    def test_01_repo_multi_agent_harness_files_exist(self):
        """Verify all 5 AI agents (Claude Code, Codex, omp, OpenCode, Antigravity) discover the skill in-repo."""
        expected_paths = [
            REPO_ROOT / "AGENTS.md",
            REPO_ROOT / "CLAUDE.md",
            REPO_ROOT / "GEMINI.md",
            SKILL_MD,
            SCRIPT_PATH,
            REPO_ROOT / ".agents" / "skills" / "local-ui-verify" / "scripts" / "playwright_probe.mjs",
            REPO_ROOT / ".claude" / "skills" / "local-ui-verify" / "SKILL.md",
            REPO_ROOT / ".claude" / "commands" / "local-ui-verify.md",
            REPO_ROOT / ".codex" / "skills" / "local-ui-verify" / "SKILL.md",
            REPO_ROOT / ".omp" / "skills" / "local-ui-verify" / "SKILL.md",
            REPO_ROOT / ".omp" / "commands" / "ui.verify.md",
            REPO_ROOT / ".opencode" / "skills" / "local-ui-verify" / "SKILL.md",
            REPO_ROOT / ".opencode" / "commands" / "local-ui-verify.md",
        ]
        for path in expected_paths:
            self.assertTrue(path.exists(), f"Missing multi-agent skill/command file: {path}")

        skill_text = SKILL_MD.read_text(encoding="utf-8")
        self.assertIn("name: local-ui-verify", skill_text)
        self.assertIn("description: Use when", skill_text)

    def test_02_global_harness_installer_covers_all_five_agents(self):
        """Verify --install-harnesses populates global directories for Claude Code, Codex, omp, OpenCode, and Antigravity."""
        with tempfile.TemporaryDirectory() as tmp_home:
            res = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--install-harnesses",
                    "--global-home",
                    tmp_home,
                    "--json",
                ],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(res.returncode, 0, f"Installer failed: {res.stderr}\n{res.stdout}")
            payload = json.loads(res.stdout)
            self.assertEqual(payload["status"], "INSTALLED")

            home = Path(tmp_home)
            expected_global = [
                home / ".agents" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".claude" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".claude" / "commands" / "local-ui-verify.md",
                home / ".codex" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".omp" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".omp" / "agent" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".omp" / "commands" / "ui.verify.md",
                home / ".opencode" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".config" / "opencode" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".opencode" / "commands" / "local-ui-verify.md",
                home / ".gemini" / "config" / "skills" / "local-ui-verify" / "SKILL.md",
                home / ".gemini" / "antigravity" / "skills" / "local-ui-verify" / "SKILL.md",
            ]
            for path in expected_global:
                self.assertTrue(path.exists(), f"Missing global harness path: {path}")

    def test_03_automated_verify_mode_headless_and_http(self):
        """Verify default --mode verify runs HTTP/API/CSP + headless Playwright DOM/console/screenshot checks."""
        with tempfile.TemporaryDirectory() as artifact_dir:
            res = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--mode",
                    "verify",
                    "--root",
                    str(REPO_ROOT),
                    "--artifact-dir",
                    artifact_dir,
                    "--json",
                ],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(res.returncode, 0, f"Verify mode failed: {res.stderr}\n{res.stdout}")
            report = json.loads(res.stdout)
            self.assertEqual(report["status"], "PASS")
            self.assertTrue(report["http_checks"]["all_passed"])
            self.assertTrue(report["browser_checks"]["all_passed"])
            self.assertEqual(report["browser_checks"]["console_errors"], [])
            self.assertEqual(report["browser_checks"]["page_errors"], [])
            for shot in report["browser_checks"]["screenshots"]:
                self.assertTrue(Path(shot).exists(), f"Missing screenshot: {shot}")

    def test_04_manual_preview_lifecycle(self):
        """Verify --mode manual starts a background preview server for user browser testing, and status/stop work."""
        port = find_free_port()
        try:
            start_res = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--mode",
                    "manual",
                    "--root",
                    str(REPO_ROOT),
                    "--port",
                    str(port),
                    "--json",
                ],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(start_res.returncode, 0, f"Manual start failed: {start_res.stderr}\n{start_res.stdout}")
            start_data = json.loads(start_res.stdout)
            self.assertEqual(start_data["status"], "RUNNING")
            self.assertIn(f"http://localhost:{port}/", start_data["urls"]["home"])

            status_res = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--mode",
                    "status",
                    "--port",
                    str(port),
                    "--json",
                ],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(status_res.returncode, 0)
            status_data = json.loads(status_res.stdout)
            self.assertEqual(status_data["status"], "RUNNING")
        finally:
            stop_res = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--mode",
                    "stop",
                    "--port",
                    str(port),
                    "--json",
                ],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(stop_res.returncode, 0)
            stop_data = json.loads(stop_res.stdout)
            self.assertEqual(stop_data["status"], "STOPPED")


if __name__ == "__main__":
    unittest.main()
