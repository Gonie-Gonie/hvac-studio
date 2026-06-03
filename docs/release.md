# Release Process

HVAC Studio is released as a Windows-first installed/portable engineering tool. The engine, project files, graph schema, and component schema must remain OS-independent, but the initial release target is deliberately narrow.

```text
Primary supported platform:
- Windows 10/11 x64

Future / experimental platform:
- macOS after MVP

Development policy:
- Windows-first release
- Cross-platform-ready architecture
- OS-specific path, process, runtime, and packaging logic isolated behind explicit boundaries
```

## Release Artifacts

Current release scripts produce two Windows artifacts:

```text
dist/hvac-studio-<version>-windows-amd64-portable.zip
dist/hvac-studio-runtime-<version>-windows-amd64.zip
```

Package scripts now keep `dist/` focused on final zip artifacts by default. The expanded staging folders are removed after compression; pass `-KeepStage` to `scripts/release/package-portable.ps1` or `scripts/release/package-runtime.ps1` when an expanded package folder is needed for manual inspection.

Standalone Studio build checks write the latest smoke executable to:

```text
dist/build/latest/studio/hvac-studio.exe
```

That folder is cleaned before each `scripts/dev/test-studio.ps1` run, so it shows only the most recent build check.

The portable Studio package is the first user-facing distribution:

```text
hvac-studio-<version>-windows-amd64-portable/
  HVAC Studio.exe
  bin/
    studio.exe
    bcs-runner.exe
    bcs-env.exe
  python/
    bcs_worker/
    bcs_sdk/
  runtime/
    python/
  schema/
  examples/
  projects/
  templates/
  docs/
  Start-Studio.ps1
  Start-Studio.cmd
  Run-Smoke-Example.ps1
  PACKAGE_README.md
  release-manifest.json
  release-provenance.json
```

The runtime-only package is for delivery/external-engine integration and does not include the Studio GUI.

Both MVP packages include `bin/bcs-env.exe` and a bundled Python runtime under `runtime/python`, copied from the repo-local setup toolchain. Included examples run without system Python on `PATH`. Projects can declare `environment.lockfile` in `project.bcsproj`; package and export flows preserve that lockfile, and `bcs-env.exe check` reports missing declared lockfiles.

`bcs-env.exe check` verifies the package root, bundled Python runtime, Python worker, SDK, schemas, examples, project/component templates, scalar component template manifest/source consistency, project Python lockfiles, and required executables. Release smoke tests call it with `--json` before running examples or Studio API checks.

Studio desktop binaries are built through `scripts/release/build-studio.ps1` with Wails production tags: `-tags desktop,production`. A plain `go build` can compile but show a Wails runtime error dialog instead of opening the app window.

User documentation source lives under `docs/user/`. Package scripts always include the Markdown docs under `docs/`. If `mkdocs` is available in the build environment, they also build offline HTML under `docs/site/`; otherwise they write `docs/HTML_BUILD_SKIPPED.md` so the package explains why only Markdown docs are present. PDF manual generation remains a later release task.

Each package also includes `release-provenance.json`, which records the package name, version, runtime id, git metadata, tool versions, documentation packaging status, and package file list.

## Local Release Test

From a clean checkout:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev
```

For repeated local runs after setup has already completed:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup
```

For debugging one package path at a time:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

The portable package smoke test expands the zip, verifies `HVAC Studio.exe`, `bin/studio.exe`, `bcs-runner.exe`, `bcs-env.exe`, and `runtime/python/python.exe`, constrains `PATH` so system Python is not used, runs `bcs-env.exe check --json`, briefly launches the Wails desktop entrypoint, runs the feed-forward example through the CLI, starts `bin/studio.exe --server`, and exercises the Studio API workflow.

Current portable Studio smoke coverage:

- Lists bundled examples through `/api/projects`.
- Runs the feed-forward example through `/api/run`.
- Creates a workspace project under `projects/`.
- Reads and saves workspace component Python source.
- Adds a Python component template and explicitly includes it in the entry system.
- Saves component parameters to `graph.json`.
- Saves default run inputs to the project `default_input` file.
- Saves a scenario under `scenarios/`.
- Runs saved scenarios as a batch and reopens the saved `runs/batch-*.json` record.
- Runs the workspace project and writes `runs/run-*.json`.
- Reopens the saved run record through `/api/project/run`.
- Writes `exports/runtime_package/manifest.json`, copied project files, public IO schema, runner tools, packaged Python runtime, README, and `run-default.ps1`.
- Runs the exported project through the exported `bin/bcs-runner.exe`.
- Runs the exported `run-default.ps1`.
- Runs exported `bin/bcs-env.exe check --root <export>` and verifies `runtime-export` mode.

The root-level `HVAC Studio.exe` opens the Wails desktop app without launching a browser or binding a normal-use TCP port. The included `Start-Studio.ps1` remains available for scripted launches, and server/API automation should use `bin\studio.exe --server` or `Start-Studio.ps1 -Server`.

Studio-created projects are written under `projects/` by default. Workspace project runs are saved as `runs/run-*.json` inside each project.

The runtime package smoke test expands the zip, constrains `PATH`, and verifies each runnable example against `expected/output.json`.

## Installer Scope

Installer packaging is intentionally later than portable zip packaging.

Portable zip first:

- easier internal research distribution
- easier debugging
- no Program Files or start-menu assumptions
- suitable for MVP and lab testing

Installer later:

- Program Files installation
- Start Menu registration
- optional project folder association
- WebView2/runtime checks
- code signing policy

## GitHub Release

GitHub Releases are created by `.github/workflows/release.yml`.

Release trigger:

```powershell
git tag v0.1.0-alpha.1
git push origin main
git push origin v0.1.0-alpha.1
```

The workflow:

1. Checks out the repository with tags.
2. Runs repo-local setup.
3. Runs `test-fast`.
4. Builds and smoke-tests the Windows portable Studio package.
5. Builds and smoke-tests the Windows runtime-only package.
6. Uploads both zips as workflow artifacts.
7. Creates a GitHub Release for tag pushes.

Manual dry runs are available through GitHub Actions `workflow_dispatch`. Manual runs upload artifacts; they only create a GitHub Release when `create_release` is selected.

## Required Permissions

The release workflow uses:

```yaml
permissions:
  contents: write
```

`contents: write` is required because the workflow creates a GitHub Release with the repository `GITHUB_TOKEN`.

## Versioning Rule

- Tags should use `vMAJOR.MINOR.PATCH` or prerelease forms like `v0.1.0-alpha.1`.
- Package filenames omit the leading `v`.
- Untagged local packages use `0.1.0-dev-<shortsha>` unless a version is passed explicitly.

## Roadmap

```text
v0.1
- Windows portable zip
- HVAC Studio.exe
- bin/studio.exe
- bcs-runner.exe
- bcs-env.exe
- bundled Python runtime
- MVP Python worker/source package
- simple example project

v0.2
- Windows installer
- WebView2/runtime checks
- optional project folder association

v0.3
- Runtime-only export
- CLI delivery package
- run / batch mode stabilization
- MkDocs HTML and PDF user guide release assets

v0.4
- Python SDK workflow
- serve mode
- optimization example

v1.0
- Stable Windows release
- validation / calibration / optimization workflow cleanup

v1.x
- macOS experimental release target
- macOS packaging, codesign, and notarization review
```

## Runtime Exit Codes

The runner uses stable exit code categories for external engines:

```text
0 = success
1 = validation error
2 = runtime error
3 = input schema/error
4 = Python worker error
5 = license/runtime error
```

## Release Checklist

- Update `CHANGELOG.md`.
- Run `scripts/release/test-release-candidate.ps1 -Version <version>`.
- Commit and push all changes.
- Create and push a version tag.
- Confirm the GitHub Release contains both Windows zips.
