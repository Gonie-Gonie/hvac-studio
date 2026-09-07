# HVAC Studio

HVAC Studio is a Python-first Component-Node System authoring and runtime tool
for building-system research. Define components in Python, connect their nodes
into systems, then run, validate, calibrate, optimize, or export the same project
through Studio, the CLI, and the Python SDK.

Windows 10/11 x64 is the primary platform. Portable builds include the Python
runtime; the macOS support bundle remains experimental.

## Start Developing

```powershell
.\scripts\dev\setup.ps1
.\scripts\dev\test-fast.ps1
.\scripts\dev\run-studio.ps1
```

If PowerShell blocks a script, invoke it with
`powershell -NoProfile -ExecutionPolicy Bypass -File <script>`.
The Studio launcher opens the desktop app; add `-Server` for HTTP API automation.

## Build

```powershell
.\scripts\build.ps1
```

The build command creates and smoke-tests the portable application, replaces
the previous local build, and cleans intermediate files. Double-click:

```text
dist/latest/HVAC Studio.exe
```

The complete portable application lives in the fixed, Git-ignored
`dist/latest/` folder. Keep its support folders beside the executable.
To check it, run `.\dist\latest\bin\bcs-env.exe check --root .\dist\latest --json`.

## Repository

| Path | Purpose |
| --- | --- |
| `go/` | Runner, compiler, runtime, and Studio host/UI |
| `python/` | Python worker and SDK |
| `schema/` | Project, graph, component, and protocol contracts |
| `examples/` | Runnable examples and regression fixtures |
| `templates/` | Project and component authoring templates |
| `scripts/dev/` | Setup, launchers, checks, and cleanup |
| `scripts/release/` | Packaging, release gates, and runtime manifest |
| `docs/` | User, integration, development, and release guides |
| `.toolchain/`, `.venv/` | Local tools and Python environment (ignored) |
| `.cache/`, `.tmp/` | Regenerable caches and intermediate files (ignored) |
| `dist/latest/` | Latest validated portable application (ignored) |

Project files such as `project.bcsproj`, `graph.json`, Python components,
datasets, and saved parameter sets are the source of truth. Studio edits those
artifacts and uses the same runner as the CLI and SDK.

## Documentation

- [Quick start](docs/user/quick-start.md)
- [Modeling](docs/user/modeling.md) and [workflows](docs/user/workflows.md)
- [CLI reference](docs/user/cli-runner.md) and [Python SDK](docs/user/python-sdk.md)
- [Development and cleanup](docs/development.md)
- [Architecture](docs/architecture.md) and [release process](docs/release.md)
- [Documentation index](docs/index.md)