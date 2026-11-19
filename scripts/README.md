# scripts

## Overview
Developer convenience scripts for launching local tooling in the repository. Currently provides `open-studio.sh`, which resolves the repository root from the script's canonical path, builds `bin/blog-studio` via `make studio` if it is missing or not executable, opens `http://${ADDR}` in the desktop browser after a 1-second delay, and replaces the shell process with `./bin/blog-studio`.

## Interface
- `scripts/open-studio.sh`: Executable Bash launcher script; accepts no CLI arguments or flags and invokes `./bin/blog-studio --repo . --addr "$ADDR" --publish-branch "$BRANCH"` `[open-studio.sh:exec]`
- `STUDIO_ADDR`: Optional environment variable (default `127.0.0.1:4321`) assigned to `ADDR` and passed to `xdg-open "http://${ADDR}"` and `./bin/blog-studio --addr "$ADDR"` `[open-studio.sh:STUDIO_ADDR]`
- `STUDIO_PUBLISH_BRANCH`: Optional environment variable (default `main`) assigned to `BRANCH` and passed to `./bin/blog-studio --publish-branch "$BRANCH"` `[open-studio.sh:STUDIO_PUBLISH_BRANCH]`
- Exit status: Exits immediately on command error, unset variable, or pipeline failure via `set -euo pipefail`, suppresses `xdg-open` errors via `|| true`, and transfers process exit status to `./bin/blog-studio` via `exec` `[open-studio.sh:set -euo pipefail]`, `[open-studio.sh:xdg-open]`, `[open-studio.sh:exec]`

## Invariants
- Strict Bash execution options `-e`, `-u`, and `-o pipefail` are enabled `[open-studio.sh:set -euo pipefail]`
- `ROOT` is always resolved as the parent directory (`..`) of the symlink-resolved script path (`readlink -f "${BASH_SOURCE[0]}"`), and the working directory is changed to `ROOT` before checking, building, or running `bin/blog-studio` `[open-studio.sh:SCRIPT_PATH]`, `[open-studio.sh:ROOT]`, `[open-studio.sh:cd]`
- `ADDR` is never empty; defaults to `127.0.0.1:4321` when `STUDIO_ADDR` is unset or empty `[open-studio.sh:ADDR]`
- `BRANCH` is never empty; defaults to `main` when `STUDIO_PUBLISH_BRANCH` is unset or empty `[open-studio.sh:BRANCH]`
- `bin/blog-studio` must be executable (`[ -x bin/blog-studio ]`) before launch; otherwise `make studio` is executed first `[open-studio.sh:make studio]`
- `./bin/blog-studio` is always invoked with `--repo .` (resolved at `ROOT`), `--addr "$ADDR"`, and `--publish-branch "$BRANCH"` `[open-studio.sh:exec]`

## Side Effects
- Spawns `readlink -f` and `dirname` subprocesses to resolve `SCRIPT_PATH` and `ROOT` `[open-studio.sh:SCRIPT_PATH]`, `[open-studio.sh:ROOT]`
- Changes the shell working directory to `ROOT` via `cd "$ROOT"` `[open-studio.sh:cd]`
- Checks executable permission on `bin/blog-studio` and conditionally executes `make studio` to build the binary `[open-studio.sh:make studio]`
- Spawns a background subshell `(sleep 1; xdg-open "http://${ADDR}" >/dev/null 2>&1 || true) &` that sleeps for 1 second and opens `http://${ADDR}` via `xdg-open` with stdout/stderr redirected to `/dev/null` `[open-studio.sh:xdg-open]`
- Replaces the current shell process with `./bin/blog-studio --repo . --addr "$ADDR" --publish-branch "$BRANCH"` via `exec` `[open-studio.sh:exec]`

## Verification
- Run: `bash -n scripts/open-studio.sh`
