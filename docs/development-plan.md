# Development Plan

Last updated: 2026-06-04

This document is the planning spine for HVAC Studio. It should explain why the product is built this way, what the closed MVP can do, and what should happen next without mixing old implementation notes into future commitments.

HVAC Studio is not a fixed drag-and-drop HVAC component library. It is a Python-first component-node-system authoring and runtime tool for equipment modeling, controls research, validation, calibration, optimization, SDK use, and runtime-only delivery.

## Product Thesis

The user defines component-node-system structure, node contracts, parameters, state, public inputs, and public outputs as project artifacts. The user edits Python component logic inside those contracts. The same project then runs through Studio, CLI, SDK, external engines, validation, calibration, optimization, and runtime-only packages.

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

## Design Philosophy

### File-Based Source Of Truth

The model is defined by files, not by GUI memory or Python object instances. The important source artifacts are:

- `project.bcsproj`
- `graph.json`
- `components/`
- `inputs/`
- `scenarios/`
- `runs/`
- `datasets/`
- `validation/mappings/`
- `parameter_sets/`
- `calibration/setups/`
- `optimization/setups/`
- `exports/`
- environment lock files

Studio edits and visualizes these files. The runner loads them. The SDK and runtime packages reuse them.

### Runner Before GUI

The runner/worker contract must stay ahead of the GUI. Every GUI command should call a runtime-backed API path rather than becoming a second modeling engine.

The dependency direction is:

```text
Studio GUI
Python SDK
External engine
        -> bcs-runner
        -> Go project/graph/compiler/runtime packages
        -> Python worker
        -> user Python components
```

### Explicit Public IO

System public inputs and outputs are explicit endpoint mappings. The runtime must never guess public IO by matching names.

```json
{
  "id": "total_power_kw",
  "component": "aggregator",
  "node": "total_power_kw"
}
```

This keeps Studio labels friendly while preserving a stable machine contract for CLI, SDK, external engines, validation, calibration, optimization, and delivery.

### User Python Owns Model Semantics

The runtime manages contracts, execution order, state passing, validation, packaging, and structured results. It does not interpret the physics inside user components.

Component authors should remain free to model physical equipment, controllers, data sources, data sinks, utilities, surrogate models, and research-specific logic.

### Contracts Protect Freedom

The more freedom user Python has, the more explicit the surrounding contract must be:

- input nodes
- output nodes
- parameter definitions
- state definitions
- execution mode
- public IO
- connection medium compatibility
- result shape

The editor can assist with snippets, completions, lint hints, and source checks, but persisted contracts should stay in project artifacts.

### One Runtime Path

Studio, CLI, SDK, package smoke tests, exported runtime folders, calibration, and optimization should all use the same runner/runtime behavior. Divergence between surfaces is treated as a bug.

### Reproducibility By Artifact

Research workflows should produce named artifacts instead of silently mutating baseline files:

- scenarios save input/context cases
- runs save inputs, context, outputs, traces, and parameter-set references
- parameter sets overlay baseline parameters at runtime
- validation mappings connect datasets to public IO
- calibration setups record objective, bounds, mapping, and base parameter set
- optimization setups record decision variables, objective, and base inputs
- exports record copied files, schema, commands, and environment assumptions

### Inspectability First

Execution must be understandable. Runtime results should include enough structure for Studio and external tools to inspect:

- public outputs
- component inputs and outputs
- node values
- connection values
- component state
- context
- execution order
- timings
- source/runtime error metadata

### Windows-First, OS-Independent Core

Initial distribution is Windows-first. The portable zip is the first supported user-facing package. The project files, graph schema, runner logic, and Python worker contract should remain OS-independent. OS-specific path, process, executable naming, packaged Python, installer, and signing concerns belong behind explicit platform/package boundaries.

### Documentation Is Product Surface

The user guide should explain how to operate the product and how the artifacts work together. It should not only list buttons. Documentation, examples, and package readmes are part of the product.

### Examples Are Regression Assets

Runnable examples are not demos alone. They are contract tests for the runner, packaging, SDK, validation, calibration, optimization, and delivery flows.

## Operating Rules

1. Stabilize source-of-truth files, runner behavior, and examples before broadening GUI features.
2. Keep GUI as an authoring and inspection surface over project artifacts.
3. Prefer CLI/golden/example coverage before wiring equivalent Studio controls.
4. Preserve baseline files unless the user explicitly edits them.
5. Save research outputs as named artifacts.
6. Keep release gates repeatable through local scripts and CI.
7. Keep Windows release behavior reproducible before starting installer/macOS work.
8. Review GUI readability with screenshots after meaningful UI changes.

## Current Alpha Baseline

Current release baseline and MVP closure point:

```text
v0.1.0-alpha.3
Windows portable zip
Windows runtime-only zip
Bundled Python runtime
Wails desktop Studio entrypoint
CLI runner and environment checker
Python worker and serve-backed Python SDK
Saved validation/calibration/optimization result records
Release provenance manifests
Runtime export of datasets, parameter sets, validation mappings, calibration setups, and optimization setups
Runnable examples 001, 003, 004, 005, 006, 007/model, 008
Markdown user guide source
```

Release artifacts produced by the release gate:

```text
dist/hvac-studio-0.1.0-alpha.3-windows-amd64-portable.zip
dist/hvac-studio-runtime-0.1.0-alpha.3-windows-amd64.zip
```

Primary verification commands:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-alpha.3 -SkipSetup
```

Current post-MVP release candidate after closing PM-301 through PM-308 and the Studio traceback path hotfix:

```text
v0.1.0-alpha.5
Windows portable zip
Windows installer preview zip
Windows runtime-only zip
Experimental macOS support zip
Pooled Python SDK evaluation
Composite, unit-conversion, solver-boundary, external executable, and vectorized examples
Hardened release candidate gate with package smoke failure propagation
Studio Python traceback source mapping hardened for Windows short/long path aliases
```

Release artifacts produced by the post-MVP release gate:

```text
dist/hvac-studio-0.1.0-alpha.5-windows-amd64-portable.zip
dist/hvac-studio-0.1.0-alpha.5-windows-amd64-installer.zip
dist/hvac-studio-runtime-0.1.0-alpha.5-windows-amd64.zip
dist/hvac-studio-0.1.0-alpha.5-macos-universal-experimental.zip
```

Post-MVP release verification command:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-alpha.5 -SkipSetup
```

## Milestone Status Summary

MVP is closed as of `v0.1.0-alpha.3`. Do not add new scope to the MVP. New work belongs in the post-MVP backlog and should be scheduled through alpha hardening, beta usability, 1.0 readiness, or post-1.0 expansion.

| Milestone | Status | Closure baseline |
| --- | --- | --- |
| 0. Repository and release foundation | Closed | Repo-local tools, CI, package scripts, release gate, provenance |
| 1. Runtime core contract | Closed | Strict project/graph loading, validation, exit codes, schema export |
| 2. Component authoring model | Closed | Component metadata, templates, generated-wrapper layout |
| 3. Feed-forward systems | Closed | Multi-component DAG execution, connection traces, medium rules |
| 4. Project Explorer and GUI shell | Closed | Studio workspace, project tree, canvas, inspector, workflow records |
| 5. Component-aware Python editor | Closed | Source loading, syntax highlight, source checks, snippets, completions |
| 6. Run/debug/inspect UX | Closed | Run workspace, traces, batch cases, reusable runtime session, serve mode |
| 7. Datasets and model validation | Closed | CSV mappings, metrics, high-error inspection, saved validation records |
| 8. Parameter manager and parameter sets | Closed | Parameter editing, runtime overlays, shared Studio selector |
| 9. Calibration | Closed | Grid search over parameter bounds, result parameter set, saved records |
| 10. Optimization | Closed | Grid search over public inputs, optimized scenario, saved records |
| 11. SDK and external integration | Closed | Python SDK backed by `bcs-runner serve` |
| 12. Runtime-only delivery | Closed | Runtime zip smoke tests, workflow artifact export, delivery example |
| 13. Installed Studio distribution | Closed | Windows portable zip, desktop entrypoint, alpha.3 release |
| 14. User guide and documentation release | Closed | Markdown guide, MkDocs config, optional HTML packaging |

MVP closure criteria:

- Every milestone has a file-backed artifact path that can be exercised without hidden Studio-only state.
- Core workflows run through CLI/runner APIs, with Studio acting as an authoring and inspection layer.
- Validation, calibration, optimization, parameter-set, SDK, and runtime-export workflows have saved or reproducible artifacts.
- Windows portable and runtime packages pass the local release gate, and `v0.1.0-alpha.3` is the release baseline.
- Remaining work changes depth, ergonomics, compatibility, or distribution maturity; it does not reopen MVP scope.

## Post-MVP Work Model

All MVP milestones are complete in the sense that each workflow has at least one file-backed, runner-backed, releasable path. The remaining work is post-MVP depth: making those paths comfortable for daily use, broader across runtime modes, easier to inspect, and stronger as release assets.

| Horizon | Meaning | Planning rule |
| --- | --- | --- |
| Alpha hardening | Stabilize contracts and remove obvious workflow friction | Prefer runner/API coverage before Studio polish |
| Beta usability | Make the workflows natural for real users | Add structured Studio views, screenshots, docs, and examples |
| 1.0 readiness | Make release, support, and compatibility promises credible | Freeze schemas, package docs, sign releases, and document migrations |
| Post-1.0 expansion | Add larger engine/platform capabilities | Keep new modes behind explicit contracts and examples |

Before a remaining item is considered done, it should have a source artifact, a runner/API path when applicable, a Studio affordance when user-facing, and at least one example, guide section, or smoke/golden check proportional to the risk.

## MVP Closure Baseline

### Milestone 0: Foundation

Goal: make the repository buildable, testable, and releasable on Windows.

Completed:

- Repo-local Go, uv, Python, and `.venv` setup.
- Go runner, Python worker, runtime skeleton, first scalar example.
- Windows portable and runtime package scripts.
- Bundled Python runtime support.
- CI fast checks and package smoke checks.
- `bcs-env check` for portable/runtime/export diagnostics.
- Local release candidate gate.
- Release provenance manifests with git metadata, tool versions, documentation status, and package file lists.

Post-MVP backlog:

- Keep local setup, CI, package smoke tests, and release gate behavior aligned so a clean machine can reproduce the same result.
- Add clearer remediation messages for missing Go, uv, Wails, Python, WebView2, and bundled-runtime issues.
- Periodically test bootstrap from an empty dependency cache and document the expected setup time.
- Keep installer work behind Milestone 13 until portable behavior and runtime export diagnostics remain stable.

### Milestone 1: Runtime Core Contract

Goal: make project and graph files robust enough for every other surface.

Completed:

- Strict JSON loading for `project.bcsproj` and `graph.json`.
- Graph validation for systems, components, nodes, connections, and public IO.
- Algebraic loop detection.
- Actionable validation/input/runtime/Python-worker error classes.
- CLI exit code taxonomy.
- One-case run results with public outputs, component IO, states, context, and execution order.
- `bcs-runner schema`.

Post-MVP backlog:

- Version `project.bcsproj`, `graph.json`, component manifests, mappings, parameter sets, calibration setups, optimization setups, and export manifests with migration notes.
- Publish a stable structured error schema for Studio, CLI, SDK, and external-engine consumers.
- Expand fixture coverage for malformed public IO, duplicate identifiers, medium mismatches, missing files, invalid parameter overlays, and Python worker failures.
- Add richer value-type and unit compatibility checks without making the runtime interpret user physics.
- Define compatibility guarantees: what can change in alpha, what must migrate in beta, and what becomes stable for 1.0.

### Milestone 2: Component Authoring Model

Goal: separate GUI-managed contracts from user-editable Python logic.

Completed:

- Component categories and execution modes.
- Node presets.
- Parameter and state definitions.
- Scalar component template.
- Generated-wrapper example with user body files.
- Runtime export of component metadata and source.

Post-MVP backlog:

- Make the generated-wrapper layout the default new-component path while retaining migration support for existing single-file components.
- Add Studio controls for source layout selection, state definitions, parameter roles, bounds, units, defaults, and visibility.
- Add templates for controller, stateful component, data source, data sink, utility component, vectorized execution, external executable execution, and solver boundary components.
- Generate clearer docstrings and contract comments that explain what the user may edit and what Studio owns.
- Add authoring checks that compare component metadata, function signatures, defaults, and generated wrapper assumptions.

### Milestone 3: Feed-Forward Systems

Goal: support user-defined components connected through nodes.

Completed:

- Runnable feed-forward example.
- Topological execution order.
- Connection value propagation.
- Source/target node validation.
- Medium compatibility warnings/errors and explicit overrides.
- Node and connection traces in run results.

Post-MVP backlog:

- Visibly mark medium compatibility, warnings, and explicit overrides on the canvas and Inspector.
- Improve fan-out, fan-in, and long-path readability with connection labels, hover values, and conflict markers.
- Add optional connection annotations for design intent, expected units, operating range, and notes.
- Write an ADR for feedback loops and solver boundaries before implementing any loop-solving behavior.
- Keep composite-system and time-series expansion behind explicit public IO and runner/compiler contracts.

### Milestone 4: Studio Shell

Goal: create the authoring shell without redefining runtime semantics.

Completed:

- Wails/Go-hosted Studio shell.
- Project tree, System canvas, Inspector, Start workspace, bottom panels.
- Workspace project creation/copy.
- Component creation, inclusion, duplication, deletion.
- Node and connection editing.
- Canvas layout persistence.
- Default inputs, scenarios, runs, batches, export records.
- Parameter Manager.
- Runtime export.
- Source checks before run/batch/export.
- Saved validation, calibration, and optimization result records.
- Project tree summaries and reopen APIs for workflow result records.
- Shared parameter-set selector for Run, Batch, and Data validation.

Post-MVP backlog:

- Replace remaining raw JSON result surfaces with structured views for datasets, validation runs, parameter sets, calibration results, optimization results, exports, and logs.
- Add first-class artifact editors for mappings, scenarios, runs, batches, parameter sets, calibration setups, optimization setups, and export manifests.
- Add project tree search/filter, artifact grouping, empty states, rename/delete affordances, and safer dirty-state prompts.
- Define undo/redo scope for canvas edits, Inspector edits, source edits, and artifact creation.
- Continue screenshot-based readability passes for project tree density, command bars, canvas contrast, Inspector tables, bottom panels, and small-window behavior.

### Milestone 5: Python Editor

Goal: help users edit component logic while protecting runtime-managed contracts.

Completed:

- Code workspace source loading/saving.
- Read-only examples.
- Syntax highlighting, line numbers, bracket status, auto indentation, indent/outdent.
- Save/check/run shortcuts.
- Contract panel.
- Snippets and contract-derived completions.
- Source checks for signatures, syntax, imports, class loading, visible IO references, and undefined-name hints.

Post-MVP backlog:

- Add formatting support that preserves generated-wrapper boundaries and user-editable regions.
- Add hover documentation for public IO, parameters, state, context, return shape, and common runtime helpers.
- Improve completions from component contracts, parameter definitions, state definitions, and known context keys.
- Add quick fixes for missing outputs, typo-like references, signature drift, unused public IO, and invalid imports where safe.
- Map Python syntax, import, runtime, and source-check errors back to editor lines with stable problem markers.
- Add editor-focused tests for wrapper layout, read-only examples, save/check/run shortcuts, and diagnostics.

### Milestone 6: Run, Debug, Inspect

Goal: make execution understandable.

Completed:

- Reusable runtime session.
- `bcs-runner serve` JSONL protocol.
- Component timings and total duration.
- Run workspace summary, outputs, selected component values, batch cases, trace tables.
- Canvas/Inspector/Code integration for latest run values.
- Stale result marking after runtime-affecting edits.

Post-MVP backlog:

- Add a native time-series runner contract with explicit input series, timestep/context handling, state carryover, and result shape.
- Add run comparison for baseline vs scenario vs parameter set vs calibration result vs optimization result.
- Add trace timelines, compact value charts, component timing bars, and connection-value history when data is time-indexed.
- Persist richer run records with runner version, graph checksum, parameter-set path, scenario path, source freshness, and selected artifacts.
- Add structured component log capture and display with severity, component, step/time, and source location when available.
- Improve cancellation, timeout, stale-result, and failed-run recovery UX.

### Milestone 7: Data Validation

Goal: compare simulated outputs against measured/reference data.

Completed:

- CSV dataset artifacts.
- Saved validation mapping artifacts.
- `bcs-runner validate-data`.
- Metrics: RMSE, MAE, MBE, CVRMSE, R2.
- Row summaries and high-error row inspection.
- Studio Project tree dataset/validation sections and `Data` command.
- Saved validation run records with Studio and CLI support.

Post-MVP backlog:

- Build a dataset import and mapping workflow with column detection, preview, unit hints, missing-value policy, and public IO matching.
- Support JSON/JSONL and weather-style time-series formats after CSV remains stable.
- Enrich validation run records with dataset checksum, graph checksum, runner version, and source freshness.
- Add measured-vs-simulated plots, scatter plots, residual plots, histograms, operating-range summaries, and row-level navigation.
- Add validation comparison across parameter sets, calibration results, and scenarios.
- Document data expectations, missing-data handling, row ordering, and reproducibility rules.

### Milestone 8: Parameter Sets

Goal: make parameters first-class research objects.

Completed:

- Workspace parameter editing.
- Saved `parameter_sets/*.json` artifacts.
- `run --parameter-set` runtime overlay.
- `validate-data --parameter-set` runtime overlay.
- Studio run/validation APIs accept `parameter_set_path`.
- Run results and records preserve parameter-set path.
- Studio Run, Batch, and Data validation workflows share one active parameter-set selector.

Post-MVP backlog:

- Add a parameter-set editor with filters by component, role, unit, changed-only, calibratable, optimizable, and validation status.
- Add diff/apply/revert views for baseline graph parameters, parameter sets, calibration outputs, and imported sets.
- Add parameter-set selection to calibration, optimization, SDK, and generated export scripts consistently.
- Define project/system/scenario/run-level parameter precedence and document replay behavior.
- Support derived/read-only parameters only after the artifact format and validation rules are explicit.
- Add import/export helpers for CSV and JSON parameter libraries.

### Milestone 9: Calibration

Goal: estimate parameters from observed data.

Completed:

- Saved `calibration/setups/*.json`.
- `bcs-runner calibrate`.
- Grid search.
- Weighted RMSE objective.
- Base parameter set support.
- Numeric parameter bounds.
- Result parameter set saving without overwriting `graph.json`.
- Saved calibration result records with Studio and CLI support.

Post-MVP backlog:

- Build a Studio calibration setup editor for dataset mapping, target outputs, candidate parameters, bounds, weights, base parameter set, and stopping rules.
- Add a candidate-grid preview with estimated run count, invalid-bound warnings, and parameter role filters.
- Add non-grid algorithms such as differential evolution and least squares behind a stable algorithm contract.
- Enrich calibration result records with objective history, failed candidates, runtime metadata, and source artifact checksums.
- Add calibration result comparison, apply-to-parameter-set, and report export workflows.
- Provide a tutorial that starts from a noisy dataset and ends with a saved parameter set.

### Milestone 10: Optimization

Goal: optimize controls or design variables against objectives.

Completed:

- Saved `optimization/setups/*.json`.
- `bcs-runner optimize`.
- Grid search over public input decision variables.
- Base inputs and context.
- Objective output with min/max sense.
- Optimized scenario saving.
- Saved optimization result records with Studio and CLI support.

Post-MVP backlog:

- Build a Studio optimization setup editor for decision variables, objective outputs, sense, constraints, base inputs, base scenario, and base parameter set.
- Support component-parameter decision variables in addition to public input decision variables.
- Add constraints for bounds, output limits, feasibility checks, and invalid-run penalties.
- Enrich optimization result records with objective history, failed candidates, scenario output, and runtime metadata.
- Add CSV export, scenario export, and SDK script export for repeatable optimization studies.
- Add multi-objective and scenario/batch optimization only after single-objective artifacts are stable.

### Milestone 11: SDK And External Integration

Goal: let research code and external engines use the same runner.

Completed:

- `bcs_sdk.RunnerClient.start(...)` launches `bcs-runner serve`.
- SDK keeps the runner alive across repeated evaluations.
- Context-manager shutdown.
- SDK returns the same structured runner result.
- Optimization example script uses the SDK.

Post-MVP backlog:

- Add SDK helpers for parameter sets, scenarios, batches, validation mappings, calibration setups, optimization setups, and runtime exports.
- Add typed SDK exceptions that preserve runner error code, command, source location, component, node, and raw diagnostic payload.
- Add async or pooled evaluation helpers for external optimization engines while keeping the runner contract single-source.
- Publish stable JSON examples for external tools that do not use the Python SDK.
- Add notebooks/scripts for parameter sweeps, validation loops, calibration, optimization, and co-simulation harnesses.
- Keep SDK semantics as a thin client over `bcs-runner serve`; avoid a second execution engine.

### Milestone 12: Runtime-Only Delivery

Goal: deliver runnable models without Studio.

Completed:

- Runtime package zip with runner, env checker, worker, SDK, docs, schemas, examples, and bundled Python.
- Runtime package smoke tests.
- Studio runtime export folder with project files, public IO schema, scripts, manifest, runner tools, and packaged runtime when available.
- Runtime-only delivery-layout example.
- Runtime exports include datasets, parameter sets, validation mappings, calibration setups, and optimization setups.

Post-MVP backlog:

- Generate a model-specific CLI guide during export that lists public IO, available scenarios, parameter sets, mappings, calibration setups, optimization setups, and smoke commands.
- Include run-record, batch-record, validation-result, calibration-result, and optimization-result artifacts in export manifests when selected, with checksums.
- Generate run, batch, validation, calibration, and optimization scripts appropriate to the exported model.
- Add a runtime logging folder convention and diagnostic bundle command.
- Add license notices, dependency notices, stronger checksums, and environment compatibility notes.
- Keep runtime-only folders runnable without Studio and without hidden references to the source checkout.

### Milestone 13: Studio Distribution

Goal: release a Windows-first engineering tool.

Completed:

- Windows portable zip.
- Wails desktop entrypoint `HVAC Studio.exe`.
- `bin/studio.exe --server` for automation.
- Package smoke coverage for Studio API and runner examples.
- Runtime/export environment checks.
- `v0.1.0-alpha.1`, `v0.1.0-alpha.2`, and `v0.1.0-alpha.3` release tags.

Post-MVP backlog:

- Add installer packaging after portable zip behavior, environment checks, and release gates are stable.
- Add WebView2/runtime detection with user-facing remediation and packaged fallback policy.
- Define code signing, checksum, provenance, and release-note requirements for public builds.
- Add Start menu integration, optional PATH registration, and file association policy for `.bcsproj`.
- Add an update policy that distinguishes alpha zip releases, beta installer releases, and future stable channels.
- Keep macOS experimental packaging behind Windows portable/installer stability and OS-independent core checks.

### Milestone 14: User Guide And Documentation

Goal: help users build correct models, not just press buttons.

Completed:

- Markdown guide under `docs/user/`.
- Quick start, concepts, internals, authoring, Python editing, system building, parameters, run/inspect, validation, calibration, optimization, CLI, export, troubleshooting, glossary.
- Screenshot-backed Studio tutorial map for component authoring, system building, validation, parameter sets, calibration, optimization, SDK use, and runtime-only delivery.
- `mkdocs.yml` source configuration.
- Runtime-only example CLI guide.
- Package scripts include Markdown docs and optionally build MkDocs HTML when `mkdocs` is available.

Post-MVP backlog:

- Make offline MkDocs HTML docs a required release asset once `mkdocs` is installed in CI/release environments.
- Add PDF manual generation once the Markdown guide structure is stable.
- Add in-app help links from Start, Project tree, System canvas, Inspector, Code, Run, Data, Parameters, Calibration, Optimization, Export, and Settings.
- Keep screenshot-backed tutorials current as Studio workspaces change.
- Add a "concept map" page that explains how project artifacts relate to Studio screens, CLI commands, SDK calls, and exported packages.
- Keep docs versioned with releases and record behavior differences for alpha/beta/stable builds.

## Post-MVP Backlog

Backlog IDs are the planning source for post-MVP work. Milestone numbers remain historical closure context; new work should reference a `PM-*` backlog item instead of reopening an MVP milestone. This section is the canonical list of known unimplemented work after MVP closure.

### P0: Alpha Hardening

Status: closed in the current post-MVP alpha hardening branch. These items stabilized contracts, records, and repeated user workflows enough to move the next workstream to P1 beta usability.

| ID | Area | Completed outcome | Done when |
| --- | --- | --- | --- |
| PM-001 | Runtime contract | Schema versioning and migration notes for project, graph, component, mapping, parameter-set, calibration, optimization, export, and result-record artifacts | Schemas carry versions, compatibility notes exist, and tests cover old/new fixture loading |
| PM-002 | Runtime contract | Stable structured error schema across CLI, Studio, SDK, serve, validation, calibration, optimization, and export | Every command can emit machine-readable error kind, location, component/node/source metadata, and docs explain the shape |
| PM-003 | Reproducibility | Enriched result records with graph checksum, source freshness, dataset checksum, runner version, package version, and artifact references | Run, batch, validation, calibration, and optimization records can be replayed or rejected with clear mismatch diagnostics |
| PM-004 | Studio artifacts | Structured artifact browser for datasets, mappings, parameter sets, validation records, calibration results, optimization results, scenarios, batches, runs, and exports | Project tree rows open structured views instead of raw JSON where user decisions are expected |
| PM-005 | Studio artifacts | Artifact rename/delete/copy policy with dirty-state and undo/redo boundaries | User-facing artifact edits are reversible or clearly confirmed, and generated records remain protected |
| PM-006 | Data validation | Dataset import and mapping UI with preview, column detection, unit hints, missing-value policy, and public IO matching | A user can create a dataset mapping in Studio without hand-editing JSON |
| PM-007 | Data validation | Validation plots and comparison views | Measured-vs-simulated, scatter, residual, histogram, operating-range, and parameter-set comparison views exist |
| PM-008 | Parameters | Parameter-set editor, filters, diff/apply/revert, import/export, and precedence rules | Users can inspect and apply parameter differences without editing JSON; docs define precedence |
| PM-009 | Calibration | Studio calibration setup and result workspace | Users can select mapping, targets, candidate parameters, bounds, base parameter set, run calibration, compare results, and apply saved parameter sets |
| PM-010 | Optimization | Studio optimization setup and result workspace | Users can define decision variables, objective, base inputs/scenario/parameter set, run optimization, inspect candidates, and save scenarios/scripts |
| PM-011 | SDK | SDK helpers for parameter sets, scenarios, batches, validation, calibration, optimization, runtime export, and typed exceptions | Python scripts can call the same workflows without manually assembling CLI JSON |
| PM-012 | Runtime export | Generated export commands for run, batch, validation, calibration, and optimization plus optional inclusion of selected result records | Exported folders are self-describing for every workflow they include |
| PM-013 | Release | Required docs HTML build in CI, alpha/beta GitHub prerelease marking, checksums, and stronger provenance validation | Release workflow fails if required docs/provenance/checksum artifacts are missing and prerelease tags are marked correctly |

P0 closure notes:

- Contract and reproducibility work is backed by schema compatibility checks, structured errors, and workflow provenance/checksums in saved records.
- Studio now has a structured artifact workspace, dataset preview with public IO mapping suggestions, mapping creation from datasets, parameter-set diff/apply flows, calibration/optimization run actions, and structured result summaries with raw JSON retained.
- Python SDK helpers cover repeated evaluation, one-shot runs with parameter sets, validation, calibration, optimization, batches, schema export, parameter/scenario loading, export manifest loading, and typed runner errors.
- Runtime exports generate workflow scripts, list commands in the manifest, and can include selected generated records.
- Release gates require MkDocs HTML, package provenance, package-internal checksums, CI checksum assets, and prerelease marking for alpha/beta/rc/dev tags.

### P1: Beta Usability

These items make the existing workflows feel like a product rather than a set of connected primitives.

| ID | Area | Unimplemented item | Done when |
| --- | --- | --- | --- |
| PM-101 | Component authoring | Done: new Studio components default to generated-wrapper source directories while existing single-file components continue to load, edit, duplicate, delete, and export | Keep compatibility coverage as schemas evolve |
| PM-102 | Component authoring | Done: Studio exposes template/source layout metadata and Inspector controls for state definitions plus parameter roles, bounds, units, defaults, current values, groups, descriptions, and visibility | Component contracts can be authored in Studio without direct graph editing |
| PM-103 | Component templates | Done: controller, stateful, data source, data sink, utility, vectorized, external executable, solver boundary, and scalar generated-wrapper templates are selectable and smoke-tested | Keep templates packaged and aligned with runtime support boundaries |
| PM-104 | Canvas | Done: medium badges, override/warning/mismatch markers, connection annotations, fan-in/fan-out anchor spreading, and long/backtracking path markers | Keep screenshot-based readability checks as canvas density grows |
| PM-105 | Python editor | Done: lightweight formatting, hover text, contract/state/context completions, common source-check quick fixes, gutter problem markers, generated-wrapper `step` snippets, and Python traceback-to-line mapping | Keep diagnostics aligned as worker/runtime protocols evolve |
| PM-106 | Run/inspect | Done: component timing bars, run-to-run public output comparison, component stdout/stderr logs, configurable run/batch timeout/cancel controls, and failed-run/batch problem summaries are in place | Keep time-indexed trace timelines aligned with PM-301 native time-series work |
| PM-107 | Documentation | Done: Studio serves local docs, links major workspaces plus workflow result headers to relevant user-guide pages, and ships screenshot-backed walkthroughs for the main authoring, workflow, SDK, and delivery paths | Keep docs links and screenshots current as UI changes |
| PM-108 | Examples | Done: examples now document the learning path for dataset validation, calibration, optimization, runtime-only delivery, controller, plant, generated-wrapper, vectorized execution, external executable execution, solver boundaries, unit conversion, composite systems, and current CSV time-column workflows, with smoke coverage for run, validation, calibration, optimization, and series paths | Keep examples current as runtime contracts and tutorials evolve |

### P2: 1.0 Readiness

Status: closed in the current post-MVP 1.0-readiness branch. These items make support promises, installation, compatibility, release trust, docs packaging, and release rehearsal reproducible enough to move expansion work to P3.

| ID | Area | Unimplemented item | Done when |
| --- | --- | --- | --- |
| PM-201 | Compatibility | Done: project and graph schema compatibility are frozen to the documented `0.1.x` line, `bcs-runner migrate` emits machine-readable compatibility reports, and docs define migration/freeze behavior | Add concrete rewrite migrations only when a future incompatible schema exists |
| PM-202 | Distribution | Done: Windows installer bundle wraps the portable payload with manifest-driven WebView2 checks, Start Menu install plan, optional user PATH registration, disabled `.bcsproj` association policy, update-channel metadata, and separate installer smoke coverage | Replace script bundle with signed MSI/MSIX only after signing/provenance policy is complete |
| PM-203 | Release trust | Done: packages include release-trust metadata, license/dependency notices, support matrix, release-note policy, checksum/provenance verification, installer payload checksum checks, and unsigned prerelease/stable signing boundaries | Add real Authenticode signing and generated third-party notice bundles before stable public installer release |
| PM-204 | Documentation | Done: offline HTML docs are required package assets, docs carry version metadata, a concept map ties artifacts to Studio/CLI/SDK/export surfaces, and a consolidated manual/PDF build script is available with optional PDF generation | Make PDF generation mandatory only when the release environment includes a stable PDF toolchain |
| PM-205 | Rehearsal | Done: the release candidate gate runs fast checks, upgrade rehearsal, portable smoke, installer smoke, runtime smoke, runtime export smoke, docs/provenance/checksum checks, and clean package artifact verification | Keep bounded server smoke tests and clean-machine release rehearsals green as packaging evolves |

### P3: Post-1.0 Expansion

These items broaden the engine. They should not block beta or 1.0 unless a real user commitment requires them.

| ID | Area | Unimplemented item | Done when |
| --- | --- | --- | --- |
| PM-301 | Time-series | Done: native `bcs-runner run-series`, `/api/run-series`, timestep/context merging, state carryover, series result shape, stateful controller golden coverage, and Studio Run workspace series preview plots | Keep richer artifact editors and saved series records as future workflow depth |
| PM-302 | Execution modes | Done: vectorized components dispatch through the worker `evaluate_batch` contract, keep the standard `(outputs, state)` return shape, expose metadata in Studio, and are covered by template, docs, and `009_vectorized_component` smoke coverage | Keep deeper batching/vector algebra helpers as future workflow depth |
| PM-303 | Execution modes | Done: external executable components run a bounded process per evaluation, exchange stdin/stdout JSON, carry state through runner-managed requests, capture stderr/log records, publish request/response schemas, and ship with example smoke coverage | Keep long-lived external adapters and pooled process lifecycles as future workflow depth |
| PM-304 | Solvers | Done: ADR 0004 keeps project graphs acyclic, loop errors point users to solver boundary components, graph/component schemas support `solver_boundary` metadata and `solver` category, Studio templates preserve the metadata, and `011_solver_boundary_component` covers the runnable pattern | Keep richer nonlinear solver libraries and solver diagnostics behind explicit component contracts |
| PM-305 | Units | Done: connections support explicit linear `unit_conversion`, Studio Inspector edits common presets and custom factor/offset values with preview, compiler diagnostics warn on unit mismatch without conversion, runtime applies numeric conversion before target evaluation, traces record before/after values, common node `value_type` contracts are validated, and `012_unit_conversion_component` covers the runnable pattern | Keep richer unit libraries and domain-specific dimensional analysis optional and explicit |
| PM-306 | Composition | Done: composite components wrap child systems through explicit public IO node IDs, compiler validation rejects boundary mismatches and recursion, the runtime evaluates nested sessions with state carryover, and `013_composite_system` covers run and series smoke paths | Keep richer Studio authoring controls and visual nested-system navigation as future workflow depth |
| PM-307 | SDK scale | Done: `RunnerClient.evaluate_async(...)` supports asyncio callers, `RunnerPool` keeps a bounded number of persistent serve sessions for independent high-volume evaluations, request timeouts prevent unbounded SDK waits, and the optimization SDK example uses pooled candidates | Keep richer process health telemetry and distributed-worker adapters as future workflow depth |
| PM-308 | Platforms | Done: release scripts build and smoke-test an experimental macOS support package with package plan, platform prerequisite checks, release provenance/checksums, GitHub artifact wiring, and explicit signing/notarization caveats | Build native signed/notarized macOS apps only on macOS after platform smoke infrastructure exists |

### P4: Productization And Usability

These items are the next quality bar before calling the Studio experience beta-ready for real users. They remove prototype signals, make state and consequences explicit, and keep automation quiet enough that daily development feels professional.

| ID | Area | Unimplemented item | Done when |
| --- | --- | --- | --- |
| PM-401 | Product surface | Remove prototype-facing language and dead affordances from Studio, packaged docs, examples, scripts, and release assets | No visible `planned`, unavailable-feature placeholder, `mock`, `demo`, disabled future feature, or raw internal status appears in user-facing screens unless it is an intentional support label |
| PM-402 | User explicitness | Make every workflow show the active project, source artifact, selected inputs, parameter-set precedence, generated output path, and whether baseline files will change | Before running, saving, exporting, calibrating, optimizing, deleting, or applying parameters, the user can tell exactly what artifact will be read or written |
| PM-403 | Workflow ergonomics | Turn Start, System, Code, Run, Data, Parameters, Calibration, Optimization, Artifacts, and Export into complete task workspaces with clear empty states, inline validation, busy states, cancel/retry behavior, and stable keyboard focus | Common tasks can be completed without hand-editing JSON, opening docs in parallel, or guessing which panel owns the next action |
| PM-404 | Error recovery | Promote structured problems into navigable recovery actions across Studio, CLI, SDK, serve, and package smoke logs | Errors identify component/node/source/artifact location, explain the failing contract, preserve logs, and offer the next valid action without hiding raw diagnostics |
| PM-405 | Visual fit and polish | Run screenshot-backed desktop and mobile audits for dense projects, long names, failed runs, empty projects, large tables, and code diagnostics | No clipped labels, overlapping controls, awkward empty cells, inconsistent button language, or layout shifts remain in supported viewport sizes |
| PM-406 | Release first-run experience | Harden portable and installer first-run flows around WebView2, Python runtime, unsigned prerelease trust, docs availability, examples, update channel, and uninstall/reinstall behavior | A clean Windows machine can install/open/run/export without seeing development-only paths or unexplained console output |
| PM-407 | Quiet automation | Keep development, test, package smoke, Python worker, source-check, and external-executable subprocesses hidden unless the user explicitly launches a CLI or desktop window | `scripts/dev/test-fast.ps1` and release package tests do not flash transient `cmd`/Python/PowerShell windows; stdout/stderr still land in logs |
| PM-408 | Acceptance walkthroughs | Maintain scripted user walkthroughs and screenshot baselines for first project, component authoring, source error fix, run comparison, dataset validation, calibration, optimization, export, SDK use, and runtime package use | A release cannot advance if a walkthrough requires tribal knowledge, exposes prototype wording, or leaves the user unable to recover from an expected mistake |

P4 closure notes:

- Treat prototype polish issues as release blockers, not cosmetic backlog, when they appear in the first-run or main workflow path.
- Keep raw JSON and logs available for inspection, but never as the primary decision surface when a structured view exists.
- Keep future features out of primary UI until they can either work end to end or be represented as documented package/release support boundaries.
- Keep quiet automation changes behind shared platform helpers so Studio, runner, worker, examples, and release scripts do not diverge.

## Post-MVP Release Sequence

1. `v0.1.x-alpha`: complete P0 alpha hardening and keep release gates green.
2. `v0.1.x-alpha`: close P4 productization and usability blockers found during real walkthroughs.
3. `v0.2.0-beta`: ship P1/P4 Studio usability workflows and screenshot-backed docs without prototype-facing surfaces.
4. `v1.0.0-rc`: finish P2 compatibility, installer, provenance, docs, clean-machine rehearsals, and P4 acceptance walkthroughs.
5. `v1.x`: begin P3 engine/platform expansion behind explicit contracts.

## Release Gates

Per coherent implementation unit:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

For release candidates:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version <version> -SkipSetup
```

The release candidate gate includes a server-free upgrade rehearsal and bounded
package smoke tests. Server-based checks must use readiness loops, request
timeouts, and cleanup blocks so automation never waits indefinitely.

For GUI readability changes:

- start Studio server or desktop build
- capture desktop and mobile screenshots
- check Project tree, command bars, Inspector, canvas, tables, code editor, and results for overlap/truncation
- keep captures under ignored `artifacts/`

For productization/usability changes:

- search user-facing assets for prototype wording such as `planned`, unavailable-feature placeholders, `mock`, `demo`, `not implemented`, and unexplained `dev` labels
- verify Start, System, Code, Run, Artifacts, Parameters, and Export have useful empty, dirty, busy, success, and failure states
- run at least one walkthrough from project creation to export without hand-editing JSON
- verify test and package smoke automation does not open transient console windows except for explicitly launched desktop smoke windows

## Boundaries After MVP Closure

- A fixed built-in HVAC component library as the main product.
- GUI-only model semantics.
- Python SDK reimplementation of the engine.
- Hidden mutation of baseline parameters during validation/calibration/optimization.
- Unplanned macOS support before Windows package behavior is stable.
- Feedback-loop solving without an explicit solver-boundary design.
