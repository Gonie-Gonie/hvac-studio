# Release Process

This project releases the runtime core first. A release is valid when it can produce and smoke-test a Windows runtime package containing:

- `bin/bcs-runner.exe`
- `python/bcs_worker`
- `python/bcs_sdk`
- `schema`
- `runtime`
- `docs`
- `examples/001_scalar_component`

The MVP package still requires Python 3.11+ on `PATH`. A future runtime-only package will vendor `runtime/python`.

## Local Release Test

From a clean checkout:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\setup.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

The package script writes:

```text
dist/hvac-studio-runtime-<version>-windows-amd64.zip
```

The release package test expands that zip in a temp directory and runs:

```powershell
.\bin\bcs-runner.exe validate --project .\examples\001_scalar_component\project.bcsproj
.\bin\bcs-runner.exe run --project .\examples\001_scalar_component\project.bcsproj --input .\examples\001_scalar_component\inputs\case01.json
```

It verifies that the public output `result` is `10.0`.

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
4. Builds `bcs-runner.exe`.
5. Packages the runtime zip.
6. Smoke-tests the zip.
7. Uploads the zip as a workflow artifact.
8. Creates a GitHub Release for tag pushes.

Manual dry runs are available through GitHub Actions `workflow_dispatch`. Manual runs upload an artifact; they only create a GitHub Release when `create_release` is selected.

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

## Release Checklist

- Update `CHANGELOG.md`.
- Run `scripts/dev/test-fast.ps1`.
- Run `scripts/release/test-runtime-package.ps1`.
- Commit and push all changes.
- Create and push a version tag.
- Confirm the GitHub Release contains the Windows runtime zip.

