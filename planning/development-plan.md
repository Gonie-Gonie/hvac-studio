# Development Plan

Last updated: 2026-06-18

This document is the planning spine for HVAC Studio. It records the current
product direction, release-candidate baseline, quality bar, and active work
streams without carrying older milestone narratives forward.

HVAC Studio is a Python-first component-node-system authoring and runtime tool
for equipment modeling, controls research, validation, calibration,
optimization, SDK use, and runtime-only delivery. It is not a fixed
drag-and-drop HVAC library.

## Product Thesis

Users define component-node-system structure, node contracts, parameters,
state, public inputs, and public outputs as project artifacts. Users then edit
Python component logic inside those contracts. The same project runs through
Studio, CLI, SDK, external engines, validation, calibration, optimization, and
runtime-only packages.

Primary workflow:

```text
New Project
-> Python environment selection
-> Component creation
-> Component node definition
-> Parameter/state definition
-> Python function editing
-> System canvas composition
-> Node-to-node connection
-> Public input/output mapping
-> Validate
-> Run one case
-> Save scenarios and runs
-> Dataset mapping
-> Model validation
-> Parameter sets
-> Calibration
-> Optimization
-> SDK / external engine use
-> Runtime export / release package
```

## Design Rules

1. File-backed artifacts are the source of truth.
2. Studio is an authoring and inspection surface over runner-backed behavior.
3. The Python SDK is a thin client over `bcs-runner serve`.
4. Public inputs and outputs are explicit endpoint mappings.
5. Research outputs are saved as named artifacts instead of silently mutating
   baseline files.
6. Runtime export folders must be runnable without a source checkout.
7. Examples are regression assets, not disposable samples.
8. Documentation, package readmes, and in-app help are product surface.
9. Windows package behavior stays reproducible before broader platform promises.
10. GUI readability is checked with screenshots after meaningful UI changes.

## Current Baseline

The current development baseline is the `0.1.0-dev` release-candidate gate. It
is a Windows-first development package set, not a signed stable public release.

Current retained package artifacts produced by the release-candidate gate:

```text
dist/hvac-studio-0.1.0-dev-windows-amd64-portable.zip
dist/hvac-studio-0.1.0-dev-windows-amd64-installer.zip
dist/hvac-studio-runtime-0.1.0-dev-windows-amd64.zip
dist/hvac-studio-0.1.0-dev-macos-universal-experimental.zip
dist/hvac-studio-docs-0.1.0-dev.zip
dist/hvac-studio-sdk-0.1.0-dev.zip
```

The release-candidate gate covers upgrade rehearsal, portable Studio,
installer bundle, runtime-only package, experimental macOS package,
documentation package, SDK package, provenance/checksum checks, workflow smoke
tests, runtime export smoke tests, and docs/manual generation.

## Working Product Surface

- Repo-local setup installs Go, uv, managed Python, and `.venv`.
- `bcs-runner` validates, runs, serves JSONL, runs batches and series,
  validates datasets, calibrates, optimizes, migrates compatible schemas, and
  exports public IO schemas.
- Studio opens projects, creates template-backed components, edits contracts
  and Python source, manages parameters, builds systems, runs cases, imports
  datasets, creates validation/calibration/optimization setups, opens saved
  records, and exports runtime packages.
- Generated-wrapper components protect Studio-owned wrapper code while saving
  user edits to `user_step.py`.
- Runtime tracebacks and source checks map users back to component source files
  where possible.
- Examples cover scalar components, feed-forward systems, stateful control,
  validation, calibration, optimization, generated wrappers, vectorized
  components, external executables, solver boundaries, unit conversion,
  composite systems, ANN assets, and RC/ANN composition.
- User docs build through MkDocs, Studio help links point at local docs,
  tutorial screenshots come from the screenshot matrix, and manual generation
  includes the user-guide pages.

## Definition Of Done

Every user-facing feature should satisfy the relevant parts of this checklist:

- Users can start the workflow from Studio without editing raw JSON.
- Required source artifacts are created explicitly.
- Studio-managed baseline artifacts are not changed silently.
- The workflow runs through runner, Studio API, CLI, or SDK paths.
- Success, failure, validation state, and generated output paths are visible.
- Errors identify component, node, source, artifact, or command location.
- The same workflow is reproducible from CLI or SDK when applicable.
- Runtime export keeps the workflow usable when package options include it.
- At least one example, smoke test, golden test, or walkthrough covers the path.
- User guide, package docs, or in-app help explain the workflow.

## Active Work Streams

### P0: Release-Candidate Stewardship

Keep the release-candidate gate green and keep retained artifacts easy to trust.

Current maintenance:

- Keep `test-fast.ps1`, acceptance walkthroughs, package smoke tests, and CI
  aligned.
- Keep `dist/` focused on final zip artifacts and remove local staging outputs
  with `scripts/dev/clean-generated.ps1`.
- Keep package provenance, checksums, release trust metadata, license notices,
  support matrix, and package-internal docs in sync.
- Keep package scripts quiet enough for daily development while preserving logs.

### P1: Product Surface And Acceptance

Make the main workflows feel like one coherent application.

Current maintenance:

- Remove prototype-facing wording and unavailable affordances from Studio,
  package docs, examples, scripts, and release assets.
- Keep Start, System, Code, Run, Data, Parameters, Calibration, Optimization,
  Artifacts, and Export workspaces complete with empty, busy, success, failure,
  and recovery states.
- Keep structured views as the primary decision surface while retaining raw JSON
  and logs for diagnostics.
- Maintain scripted acceptance walkthroughs from first project through runtime
  package use.

### P2: Component Authoring And Source Editing

Keep component authoring understandable without requiring schema knowledge.

Current maintenance:

- Keep generated-wrapper as the default new-component authoring layout.
- Keep templates, source layout metadata, parameter roles, state definitions,
  units, bounds, defaults, and visibility editable from Studio.
- Keep source checks, quick fixes, completions, snippets, gutter markers, and
  traceback mapping aligned with runtime behavior.
- Keep duplicated, renamed, deleted, exported, and migrated components from
  corrupting source paths or Studio-owned wrapper files.

### P3: Data, ML, Calibration, And Optimization

Make research workflows artifact-backed and inspectable.

Current maintenance:

- Keep dataset import, preview, mapping, validation metrics, high-error rows,
  and saved validation records usable without raw JSON editing.
- Keep ML/ANN assets packaged with feature schemas, target schemas, validation
  reports, checksums, and export manifest summaries.
- Keep parameter sets, calibration setups/results, optimization setups/results,
  and generated result artifacts protected from hidden baseline mutation.
- Keep RC/ANN/equipment composition examples and walkthroughs exercising run,
  series, validation, calibration, optimization, SDK, and export paths.

### P4: Runtime Delivery And SDK

Keep exported models and external integrations self-contained.

Current maintenance:

- Keep runtime exports carrying project files, public IO schema, manifests,
  checksums, commands, runner tools, docs, and optional workflow records.
- Keep generated run, batch, validation, calibration, and optimization scripts
  accurate for the selected export profile.
- Keep `bcs-env check` useful for portable packages, runtime packages, and
  exported model folders.
- Keep SDK helpers thin over runner commands and serve sessions, including
  typed errors and repeated-evaluation helpers.

### P5: Platform And Release Trust

Keep support promises explicit.

Current maintenance:

- Keep Windows portable and installer bundles reproducible.
- Keep experimental macOS support clearly marked and package-smoke tested.
- Keep signing, notarization, installer replacement, update channels, file
  association, and broader OS support behind explicit release-trust decisions.
- Keep docs clear that HVAC Studio does not certify physics, equipment
  performance, or control safety.

## Known Boundaries

- No signed stable Windows installer release yet.
- No signed or notarized native macOS application yet.
- No Linux package yet.
- No automatic updater yet.
- `.bcsproj` file association launch flow is not enabled yet.
- Feedback-loop solving requires explicit solver-boundary components.
- HVAC physics, equipment performance, and control safety remain user/model
  responsibilities.

## Release Gates

Per coherent implementation unit:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

For release candidates:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup
```

For GUI readability changes:

- Run the relevant Studio server or desktop build.
- Capture the screenshot matrix with `scripts/dev/test-screenshot-matrix.ps1`.
- Check Project tree, command bars, Inspector, canvas, tables, code editor, and
  result panels for overlap, truncation, and layout shifts.
- Keep captures under ignored `artifacts/`.

For product-surface changes:

- Run `scripts/dev/test-product-wording.ps1`.
- Run an acceptance walkthrough that reaches runtime export.
- Verify generated outputs are clear and baseline edits are explicit.
- Clean local generated outputs with `scripts/dev/clean-generated.ps1`.
