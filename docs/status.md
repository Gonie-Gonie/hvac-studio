# Current Status

HVAC Studio is currently a Windows-first development build with a working
runtime, Studio UI, examples, documentation packaging, and release smoke gates.
It is not yet a signed stable public release.

## Working Now

- Repo-local setup installs Go, uv, managed Python, and a virtual environment
  without depending on global toolchains.
- `bcs-runner` validates, runs, serves JSONL requests, runs batches and series,
  validates datasets, calibrates, optimizes, migrates compatible schemas, and
  exports public IO schemas.
- Studio opens projects, creates components from templates, edits component
  contracts and Python source, manages parameters, builds systems, runs cases,
  imports datasets, creates validation/calibration/optimization setups, opens
  saved records, and exports runtime packages.
- Python SDK helpers call the runner instead of reimplementing simulation logic.
- Examples cover scalar components, feed-forward systems, stateful control,
  validation, calibration, optimization, generated wrappers, vectorized
  components, external executables, solver boundaries, unit conversion,
  composite systems, ANN assets, and RC/ANN composition.
- User docs build through MkDocs, Studio help links point at local docs, tutorial
  screenshots are generated from the screenshot matrix, and the manual/PDF build
  includes all user-guide pages.
- Release scripts create portable Studio, installer bundle, runtime-only,
  experimental macOS support, documentation, and SDK zip artifacts with
  provenance and checksums.

## Not Yet Supported

- Stable signed Windows installer release.
- Signed or notarized native macOS application.
- Linux package.
- Automatic updater.
- `.bcsproj` file association launch flow.
- Certification of HVAC physics, equipment performance, or control safety.

## Development Focus

- Keep the release-candidate gate green and keep retained artifacts easy to
  inspect.
- Keep Studio workflows coherent from component authoring through runtime
  export, with structured views before raw JSON.
- Keep generated-wrapper authoring, source checks, quick fixes, completions,
  and traceback mapping aligned with runner behavior.
- Keep datasets, ML assets, validation, calibration, optimization, and saved
  result artifacts usable without hidden baseline mutation.
- Keep runtime exports, generated scripts, `bcs-env check`, SDK helpers, and
  external-engine protocol examples self-contained.
- Keep support promises explicit for Windows packages, experimental macOS
  packages, signing, installer behavior, and platform boundaries.

## Build Outputs

Final package zips are kept directly under `dist/`:

```text
dist/hvac-studio-<version>-windows-amd64-portable.zip
dist/hvac-studio-<version>-windows-amd64-installer.zip
dist/hvac-studio-runtime-<version>-windows-amd64.zip
dist/hvac-studio-<version>-macos-universal-experimental.zip
dist/hvac-studio-docs-<version>.zip
dist/hvac-studio-sdk-<version>.zip
```

Retained zips are the last locally generated release-candidate artifacts. If
source files changed after they were built, rerun the release-candidate gate
before treating them as packages for the current `HEAD`.

The current checkout has these retained development package artifacts:

```text
dist/hvac-studio-0.1.0-dev-windows-amd64-portable.zip
dist/hvac-studio-0.1.0-dev-windows-amd64-installer.zip
dist/hvac-studio-runtime-0.1.0-dev-windows-amd64.zip
dist/hvac-studio-0.1.0-dev-macos-universal-experimental.zip
dist/hvac-studio-docs-0.1.0-dev.zip
dist/hvac-studio-sdk-0.1.0-dev.zip
```

Standalone Studio smoke builds write the latest executable to
`dist/build/latest/studio/hvac-studio.exe`. That directory is transient: it is
rewritten by `scripts/dev/test-studio.ps1` and removed by
`scripts/dev/clean-generated.ps1`.

Other transient local outputs include `artifacts/`, `bin/`, `dist/docs/`,
`.repo_tools` smoke/log/staging folders, `.tmp/`, Python `__pycache__/`
directories, Python package build folders, and empty generated directories left
by local smoke tests or legacy app scaffolding.
Run this after local checks when you want only tracked files and retained
package zips left behind:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\clean-generated.ps1
```

## Main Gates

Use these before treating a checkout or package as usable:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-screenshot-matrix.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup
```
