# Release Process

## Local Build

```powershell
.\scripts\build.ps1
```

This builds and smoke-tests the Windows portable package, then installs the
validated application into `dist/latest/`. Double-click
`dist/latest/HVAC Studio.exe`; its support folders must remain beside it.
Intermediate output and the temporary ZIP live in `.tmp/` and are cleaned
after success. All generated output is Git-ignored.

## Package Scope

Windows 10/11 x64 is the primary supported platform. Development and prerelease
packages may be unsigned. Stable public installers require signing and recorded
verification. The macOS artifact is an experimental support bundle, without a
signed or notarized native app. Linux packages, automatic updates, and project
file association launch are not supported.

Versioned release scripts write into `dist/releases/`:

| Package | Filename |
| --- | --- |
| Windows Studio portable | `hvac-studio-<version>-windows-amd64-portable.zip` |
| Windows installer bundle | `hvac-studio-<version>-windows-amd64-installer.zip` |
| Windows runtime only | `hvac-studio-runtime-<version>-windows-amd64.zip` |
| Experimental macOS support | `hvac-studio-<version>-macos-universal-experimental.zip` |
| Offline documentation | `hvac-studio-docs-<version>.zip` |
| Python SDK and worker | `hvac-studio-sdk-<version>.zip` |

These are separate release outputs; normal cleanup removes them after local
work. Preserve or upload them before running cleanup when preparing a release.
`-KeepStage` on portable/runtime packaging retains expanded staging for inspection.

The portable ZIP includes `HVAC Studio.exe`, runner/environment tools in `bin/`,
bundled `runtime/python/`, the worker and SDK, schemas, examples, templates, and
offline docs. Use `bin\studio.exe --server` for API automation. Workspace projects
live in `projects/` and retain their own source and workflow records.

The runtime-only package carries execution support without Studio. Both runtime
packages must pass `bin\bcs-env.exe check --root . --json` with system Python
removed from `PATH`. The check covers runtime files, schemas, templates,
examples, executables, and project-declared Python lockfiles.

Desktop binaries require the Wails `desktop,production` build tags used by
`scripts/release/build-studio.ps1`. Runtime exports include selected project
artifacts, support files, and workflow scripts; see
[Runtime export](user/export-runtime.md).

## Verification

Run the complete release gate, or use `-SkipSetup` after local setup:

```powershell
.\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev
```

The gate covers fast checks, screenshots, upgrade compatibility, package builds,
and portable, installer, runtime, macOS support, docs, SDK, and trust checks.
`-SkipScreenshots` is available when Edge/Chrome is unavailable. Individual gates:

```powershell
.\scripts\release\test-upgrade-rehearsal.ps1 -Version 0.1.0-dev
.\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
.\scripts\release\test-installer-package.ps1 -Version 0.1.0-dev
.\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
.\scripts\release\test-macos-package.ps1 -Version 0.1.0-dev
.\scripts\release\test-docs-package.ps1 -Version 0.1.0-dev
.\scripts\release\test-sdk-package.ps1 -Version 0.1.0-dev
```

Portable smoke tests launch the desktop entrypoint, run the bundled CLI, exercise
Studio project/component/parameter/scenario/record APIs, export a workspace,
and run the exported CLI, workflow scripts, and SDK. The runtime gate compares
included examples with golden outputs. The upgrade gate works on a temporary
copy and checks compatible `0.1.x` project migration, validation, and execution.
Installer tests verify the payload and run `install.ps1 -PlanOnly`.

## Offline Documentation

Packages include Markdown sources, a MkDocs HTML site at `docs/site/`, and
Markdown/PDF manuals under `docs/manual/`. The docs package also contains
`docs/version.json`. Documentation checks validate navigation, local links,
Studio help targets, and manual coverage.

```powershell
.\scripts\release\build-docs-manual.ps1 -Version 0.1.0-dev
```

Standalone manual output goes to `.tmp/docs/manual/`. The builder uses `pandoc`
when installed, otherwise its plain-text PDF fallback. `manual-build.json`
records the generation mode. Package smoke tests require the HTML, Markdown,
and PDF outputs.

## Installer

The PowerShell installer bundle wraps the portable payload. It defaults to a
per-user installation at `%LOCALAPPDATA%\Programs\HVAC Studio`, supports a
custom directory, checks WebView2, creates a Start Menu shortcut, and optionally
registers `bin/` on the user PATH. It records the `.bcsproj` association policy
without enabling launch support. Contents include the payload ZIP, installer
manifest, install/uninstall scripts, and release metadata.

## Trust and Provenance

Every package includes `release-manifest.json`, `release-trust.json`, and the
legal notices. Expanded runtime packages include `release-provenance.json`
(version, runtime ID, Git state, tools, docs status, file inventory) and
`release-checksums.json` (SHA-256 content hashes). Release workflows generate
`SHA256SUMS.txt` for final archives. Smoke tests verify these assets and the
installer payload checksum.

Signing state must agree with package contents and release notes. Experimental
macOS packages record unsigned and not-notarized status in the release manifest
and `macos/package-plan.json`.

- [License notices](legal/license-notices.md)
- [Dependency notices](legal/dependency-notices.md)
- [Support matrix](legal/support-matrix.md)
- [Release notes policy](legal/release-notes-policy.md)

## Publish

`.github/workflows/release.yml` builds and tests the package set, uploads ZIPs
and checksums, and creates GitHub Releases for version tag pushes. Manual
`workflow_dispatch` runs publish only when `create_release` is selected. The
workflow needs `contents: write` for release creation.

1. Update `CHANGELOG.md` and confirm the support/signing statements.
2. Run the release-candidate gate for the intended version.
3. Commit the reviewed changes and push them.
4. Create and push a `vMAJOR.MINOR.PATCH` or prerelease tag.
5. Verify the GitHub Release contains the complete package set and checksums.

Package names omit the leading `v`. Untagged local packages use
`0.1.0-dev-<shortsha>` unless a version is supplied. Alpha, beta, release-candidate,
and development versions are marked prereleases; stable tags are not.