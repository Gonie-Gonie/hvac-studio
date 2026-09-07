# Development

## Setup

Run from the repository root on Windows x64:

```powershell
.\scripts\dev\setup.ps1
```

Pinned versions live in `scripts/dev/tool-versions.ps1`. Setup installs Go, uv,
and managed Python into `.toolchain/` and creates `.venv/`; global installations
are unnecessary. Development scripts load `scripts/dev/env.ps1` to select these
tools and keep Go/uv caches inside `.cache/`.

| Local path | Contents | Cleanup |
| --- | --- | --- |
| `.toolchain/` | Installed Go, uv, and managed Python | Keep for development |
| `.venv/` | Repository Python environment | Keep for development |
| `.cache/` | Downloads and Go/uv caches | Recreated when needed |
| `.tmp/` | Builds, staging, docs HTML/PDF, smoke output, and logs | Disposable |
| `dist/latest/` | Latest validated portable application | Replaced by `scripts/build.ps1` |
| `dist/releases/` | Versioned packages created by release scripts | Removed by normal cleanup |

All these local paths are Git-ignored. Keep source fixtures, expected outputs,
schemas, dependency locks, and example model assets under version control.

## Run and Check

```powershell
.\scripts\dev\run-studio.ps1
.\scripts\dev\run-runner.ps1 validate --project .\examples\001_scalar_component\project.bcsproj
.\scripts\dev\test-fast.ps1
.\scripts\dev\test-examples.ps1
.\scripts\dev\test-docs.ps1
.\scripts\dev\test-product-wording.ps1
```

`run-studio.ps1 -Server` starts the local HTTP API for automation. Choose checks
that cover the changed behavior. Runtime or contract edits need the relevant Go
tests and example goldens; Studio UI changes also need the screenshot matrix:

```powershell
.\scripts\dev\test-screenshot-matrix.ps1
```

Screenshots are test output under `.tmp/`, not documentation source assets.
The example and acceptance gates exercise persisted project artifacts through
the same runtime used by Studio, CLI, and SDK.

To freeze a project's Python dependencies into its declared lockfile:

```powershell
.\scripts\dev\freeze-project-python.ps1 -Project .\examples\001_scalar_component\project.bcsproj
```

## Build and Clean

```powershell
.\scripts\build.ps1
.\scripts\dev\clean-generated.ps1 -Inventory
.\scripts\dev\clean-generated.ps1
.\scripts\dev\clean-generated.ps1 -Caches
```

The build command validates a portable package, extracts the application to
`dist/latest/`, and removes its temporary ZIP. Launch `dist/latest/HVAC Studio.exe`
directly. Cleanup removes intermediate outputs and release packages while
preserving the latest application and installed development tools. `-Inventory`
lists targets without deleting them; `-Caches` also clears regenerable caches.
Rebuilding preserves the complete `dist/latest/projects/` workspace.

Release packages and their broader smoke gates are documented in
[Release process](release.md). Documentation navigation lives in `mkdocs.yml`;
keep it and `scripts/release/build-docs-manual.ps1` aligned when changing pages.
