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
  bin/
    studio.exe
    bcs-runner.exe
    bcs-env.exe
  python/
    bcs_worker/
    bcs_sdk/
  runtime/
  schema/
  examples/
  templates/
  docs/
  Start-Studio.ps1
  PACKAGE_README.md
  release-manifest.json
```

The runtime-only package is for delivery/external-engine integration and does not include the Studio GUI.

Both MVP packages still require Python 3.11+ on `PATH`. A later release must vendor `runtime/python` before claiming no external Python requirement.

## Local Release Test

From a clean checkout:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\setup.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

The portable package smoke test expands the zip, verifies `studio.exe`, `bcs-runner.exe`, and `bcs-env.exe`, runs the feed-forward example through the CLI, starts Studio locally, calls `/api/projects`, and runs `/api/run`.

The runtime package smoke test expands the zip and verifies each runnable example against `expected/output.json`.

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
- studio.exe
- bcs-runner.exe
- bcs-env.exe
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
