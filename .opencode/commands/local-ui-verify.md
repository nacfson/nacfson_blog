---
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
