# Agent Working Memory

Last updated: 2026-06-19

This file is a compact maintainer note for agents working in this repository.
User-facing documentation lives under `docs/`, especially `docs/status.md` and
`docs/user/`. Release procedure lives in `docs/release.md`.

## Product Center

HVAC Studio is a Python-first component-node-system authoring and runtime tool
for building-system researchers. It lets users define components, nodes,
parameters, state, public inputs, and public outputs as project artifacts, then
run, validate, calibrate, optimize, integrate through SDK or external engines,
and export runtime-only packages.

The core is not a fixed HVAC component library. The core is preserving
user-defined Python modeling freedom while making contracts, graph execution,
runtime environment, and delivery reproducible.

## Source Of Truth

- `project.bcsproj`, `graph.json`, component source files, schema files,
  datasets, parameter sets, scenarios, setup artifacts, and saved records are
  source artifacts.
- Studio is an authoring and inspection surface over those artifacts.
- `bcs-runner` is the execution engine.
- The Python SDK is a thin client over runner commands and serve sessions.
- Runtime exports must work without a source checkout when the selected package
  profile includes the needed support files.

## Current Product Surface

- Studio opens projects, creates template-backed components, edits contracts and
  Python source, manages parameters, builds systems, runs cases, imports
  datasets, creates validation/calibration/optimization setups, opens saved
  records, and exports runtime packages.
- Generated-wrapper components keep Studio-owned wrapper code separate from the
  editable `user_step.py` body.
- Source checks, quick fixes, completions, snippets, gutter markers, and
  traceback mapping should stay aligned with runner behavior.
- Examples are regression assets. Keep scalar, generated-wrapper, stateful,
  plant workflow, optimization, runtime-only, vectorized, external executable,
  solver boundary, unit conversion, composite, ANN asset, and RC/ANN composition
  examples runnable.
- Release packages currently target Windows-first portable, Windows installer
  bundle, runtime-only package, experimental macOS support package, docs package,
  and SDK package.

## Design Rules

- Use Component, Node, System, Public Input, and Public Output consistently.
  Avoid wording like "Chiller node"; chiller is a component, inlet/outlet/signal
  points are nodes.
- Baseline graph and component artifacts must not be silently mutated by
  calibration, optimization, or validation results. Save named result artifacts
  and make apply/revert actions explicit.
- GUI workflows should be JSON-free for common paths. Raw JSON belongs in
  diagnostics and inspection surfaces.
- Problems should identify the most useful target: component, node, source line,
  artifact, command, or package path.
- Canvas, Inspector, Code, Run, Data, Parameters, Calibration, Optimization,
  Artifacts, and Export views should call the same persisted APIs rather than
  maintaining parallel state.
- `tools/go/internal/studio/static/js/app.js` should stay as orchestration glue.
  Extract focused modules for workspaces, result renderers, helpers, and shared
  UI behavior when a section grows large.

## Development Docs

- Use `docs/status.md` for what works, current package limits, retained build
  artifacts, cleanup guidance, and active development focus.
- Use `docs/release.md` for release commands, package scope, provenance,
  checksums, signing/trust notes, and release checklist.
- Use `docs/repository-design.md` for architecture boundaries.
- Keep `docs/user/` focused on workflows users can follow in the current
  product surface.
- Avoid adding separate roadmap archives. Fold enduring direction into
  `docs/status.md` or the relevant user/release page.

## Verification

Choose the narrowest meaningful check for the change, then broaden when the
change touches shared behavior, runtime contracts, packaging, or docs included
in packages.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-docs.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-product-wording.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup
```

For meaningful frontend UI changes, run the relevant Studio target and capture
the screenshot matrix before declaring the surface ready.

## Generated Outputs

Retained release-candidate zip artifacts live directly under `dist/`:

```text
dist/hvac-studio-0.1.0-dev-windows-amd64-portable.zip
dist/hvac-studio-0.1.0-dev-windows-amd64-installer.zip
dist/hvac-studio-runtime-0.1.0-dev-windows-amd64.zip
dist/hvac-studio-0.1.0-dev-macos-universal-experimental.zip
dist/hvac-studio-docs-0.1.0-dev.zip
dist/hvac-studio-sdk-0.1.0-dev.zip
```

Clean local intermediates after tests:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\clean-generated.ps1
```

The cleanup script preserves retained zip artifacts and removes local staging,
logs, caches, smoke output, build folders, and empty generated directories.

## Operating Rules

- Read the codebase before refactoring. Prefer existing local patterns and
  helpers.
- Keep edits scoped to the requested workflow and surrounding ownership
  boundary.
- Do not revert user changes. Work with a dirty tree carefully.
- Commit and push coherent units after tests pass unless the user asks to hold
  changes locally.
- Keep build/package outputs current when source changes affect release
  artifacts.

## Monitoring Checklist

- Are we accidentally turning the GUI into the model source of truth?
- Are Studio, CLI, SDK, and external-engine paths using the same runner
  behavior?
- Are examples still regression assets rather than disposable samples?
- Can a user complete the primary path without editing raw JSON?
- Do errors lead to the place the user can fix?
- Do exported folders run after being moved or unzipped elsewhere?
- Are package docs and retained zip artifacts current for the commit being
  discussed?
