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
```

The runtime-only package is for delivery/external-engine integration and does not include the Studio GUI.

Both MVP packages include a bundled Python runtime under `runtime/python`, copied from the repo-local setup toolchain. Included examples run without system Python on `PATH`. Project-specific third-party package locking and dependency freezing are still later milestones.

User documentation source lives under `docs/user/`. The planned documentation release flow is Markdown source to MkDocs HTML, offline/in-app help, PDF manual, and GitHub Release assets. The source scaffold exists now; automated HTML/PDF packaging is still a later release task.

## Local Release Test

From a clean checkout:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\setup.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

The portable package smoke test expands the zip, verifies `HVAC Studio.exe`, `bin/studio.exe`, `bcs-runner.exe`, `bcs-env.exe`, and `runtime/python/python.exe`, constrains `PATH` so system Python is not used, briefly launches the Wails desktop entrypoint, runs the feed-forward example through the CLI, starts `bin/studio.exe --server`, and exercises the Studio API workflow.

Current portable Studio smoke coverage:

- Lists bundled examples through `/api/projects`.
- Runs the feed-forward example through `/api/run`.
- Creates a workspace project under `projects/`.
- Reads and saves workspace component Python source.
- Adds a Python component template and explicitly includes it in the entry system.
- Saves component parameters to `graph.json`.
- Saves default run inputs to the project `default_input` file.
- Saves a scenario under `scenarios/`.
- Runs the workspace project and writes `runs/run-*.json`.
- Reopens the saved run record through `/api/project/run`.
- Writes `exports/runtime_package/manifest.json`.

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
- Run `scripts/dev/test-fast.ps1`.
- Run `scripts/release/test-portable-package.ps1`.
- Run `scripts/release/test-runtime-package.ps1`.
- Commit and push all changes.
- Create and push a version tag.
- Confirm the GitHub Release contains both Windows zips.
