# Repository Setup

The development environment should be reproducible from a fresh clone without relying on globally installed Go or Python.

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\setup.ps1
```

The setup script installs repo-local tools:

```text
.repo_tools/
  go/             Portable Go toolchain
  uv/             uv executable
  python/         uv-managed Python installations
  go-cache/       Go module/build cache
  uv-cache/       uv cache
  uv-tools/       uv tool storage

.venv/            Repo-local Python virtual environment
```

After setup, dev scripts load `scripts/dev/env.ps1`, which puts `.venv`, `.repo_tools/go/bin`, and `.repo_tools/uv` at the front of `PATH`. It also sets Go and uv caches inside `.repo_tools` so normal test/build commands do not depend on user-profile caches.

## Tool Versions

Pinned setup versions live in:

```text
scripts/dev/tool-versions.ps1
```

Update that file deliberately when the project decides to move toolchains.

## Common Commands

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\run-runner.ps1 validate --project .\examples\001_scalar_component\project.bcsproj
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\freeze-project-python.ps1 -Project .\examples\001_scalar_component\project.bcsproj
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\run-studio.ps1
```

`freeze-project-python.ps1` writes the project Python package snapshot to the project's declared `environment.lockfile`, or to `requirements.lock.txt` when the project has not declared one yet.

`run-studio.ps1` opens the Wails Studio desktop app by default. Add `-Server` to run only the local HTTP API for automation.

## Notes

- `setup.ps1` is currently Windows AMD64 focused.
- `uv` is used because it can install managed Python versions and create virtual environments without requiring a system Python.
- The runner still supports fallback to system `go` or `python` when setup has not been run, but the intended development path is repo-local.
