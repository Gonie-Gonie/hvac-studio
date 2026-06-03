# Development Plan

Last updated: 2026-06-04

This document is the planning spine for HVAC Studio. It should explain why the product is built this way, what the current alpha baseline can do, and what should happen next without mixing old implementation notes into future commitments.

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

Current release baseline:

```text
v0.1.0-alpha.2
Windows portable zip
Windows runtime-only zip
Bundled Python runtime
Wails desktop Studio entrypoint
CLI runner and environment checker
Python worker and serve-backed Python SDK
Runnable examples 001, 003, 004, 005, 006, 007/model, 008
Markdown user guide source
```

Release artifacts produced by the release gate:

```text
dist/hvac-studio-0.1.0-alpha.2-windows-amd64-portable.zip
dist/hvac-studio-runtime-0.1.0-alpha.2-windows-amd64.zip
```

Primary verification commands:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-alpha.2 -SkipSetup
```

## Milestone Status Summary

| Milestone | Status | Current MVP slice |
| --- | --- | --- |
| 0. Repository and release foundation | Complete | Repo-local tools, CI, package scripts, release gate |
| 1. Runtime core contract | Complete | Strict project/graph loading, validation, exit codes, schema export |
| 2. Component authoring model | Complete | Component metadata, templates, generated-wrapper layout |
| 3. Feed-forward systems | Complete | Multi-component DAG execution, connection traces, medium rules |
| 4. Project Explorer and GUI shell | Complete | Studio workspace, project tree, canvas, inspector, APIs |
| 5. Component-aware Python editor | Complete | Source loading, syntax highlight, source checks, snippets, completions |
| 6. Run/debug/inspect UX | Complete | Run workspace, traces, batch cases, reusable runtime session, serve mode |
| 7. Datasets and model validation | Complete MVP | CSV mappings, metrics, high-error inspection, Data command |
| 8. Parameter manager and parameter sets | Complete MVP | Parameter editing, runtime overlays, parameter-set references |
| 9. Calibration | Complete MVP | Grid search over parameter bounds, result parameter set |
| 10. Optimization | Complete MVP | Grid search over public inputs, optimized scenario |
| 11. SDK and external integration | Complete MVP | Python SDK backed by `bcs-runner serve` |
| 12. Runtime-only delivery | Complete MVP | Runtime zip smoke tests, export artifacts, delivery-layout example |
| 13. Installed Studio distribution | Complete MVP | Windows portable zip and desktop entrypoint |
| 14. User guide and documentation release | Complete MVP | Markdown guide and MkDocs source config |

## Remaining Work Model

All MVP milestones are complete in the sense that each workflow has at least one file-backed, runner-backed, releasable path. The remaining work is depth: making those paths comfortable for daily use, broader across runtime modes, easier to inspect, and stronger as release assets.

| Horizon | Meaning | Planning rule |
| --- | --- | --- |
| Alpha hardening | Stabilize contracts and remove obvious workflow friction | Prefer runner/API coverage before Studio polish |
| Beta usability | Make the workflows natural for real users | Add structured Studio views, screenshots, docs, and examples |
| 1.0 readiness | Make release, support, and compatibility promises credible | Freeze schemas, package docs, sign releases, and document migrations |
| Post-1.0 expansion | Add larger engine/platform capabilities | Keep new modes behind explicit contracts and examples |

Before a remaining item is considered done, it should have a source artifact, a runner/API path when applicable, a Studio affordance when user-facing, and at least one example, guide section, or smoke/golden check proportional to the risk.

## Completed MVP Milestones

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

Next depth:

- Add a release provenance manifest that records git SHA, version, tool versions, bundled Python version, and package contents.
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

Next depth:

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

Next depth:

- Make the generated-wrapper layout the default new-component path while retaining migration support for existing single-file components.
- Add Studio controls for source layout selection, state definitions, parameter roles, bounds, units, defaults, and visibility.
- Add templates for controller, stateful component, data source, data sink, utility component, external executable placeholder, and vectorized placeholder.
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

Next depth:

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

Next depth:

- Replace remaining raw JSON result surfaces with structured views for datasets, validation runs, parameter sets, calibration results, optimization results, exports, and logs.
- Add first-class artifact openers and editors for mappings, scenarios, runs, batches, parameter sets, calibration setups, optimization setups, and export manifests.
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

Next depth:

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

Next depth:

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

Next depth:

- Build a dataset import and mapping workflow with column detection, preview, unit hints, missing-value policy, and public IO matching.
- Support JSON/JSONL and weather-style time-series formats after CSV remains stable.
- Persist validation run records with mapping, dataset checksum, parameter set, metrics, high-error rows, and runner version.
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

Next depth:

- Add a parameter-set editor with filters by component, role, unit, changed-only, calibratable, optimizable, and validation status.
- Add diff/apply/revert views for baseline graph parameters, parameter sets, calibration outputs, and imported sets.
- Add parameter-set selection to run, batch, validation, calibration, optimization, SDK, and export workflows consistently.
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

Next depth:

- Build a Studio calibration setup editor for dataset mapping, target outputs, candidate parameters, bounds, weights, base parameter set, and stopping rules.
- Add a candidate-grid preview with estimated run count, invalid-bound warnings, and parameter role filters.
- Add non-grid algorithms such as differential evolution and least squares behind a stable algorithm contract.
- Persist calibration result records with objective history, best parameters, failed candidates, runtime metadata, and source artifact checksums.
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

Next depth:

- Build a Studio optimization setup editor for decision variables, objective outputs, sense, constraints, base inputs, base scenario, and base parameter set.
- Support component-parameter decision variables in addition to public input decision variables.
- Add constraints for bounds, output limits, feasibility checks, and invalid-run penalties.
- Persist optimization result records with objective history, best candidate, failed candidates, scenario output, and runtime metadata.
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

Next depth:

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

Next depth:

- Generate a model-specific CLI guide during export that lists public IO, available scenarios, parameter sets, mappings, calibration setups, optimization setups, and smoke commands.
- Include validation, calibration, optimization, parameter-set, scenario, batch, and run-record artifacts in export manifests with checksums.
- Generate run, batch, validation, calibration, and optimization scripts appropriate to the exported model.
- Add a runtime logging folder convention and diagnostic bundle command.
- Add runtime package provenance, license notices, dependency notices, and environment compatibility checks.
- Keep runtime-only folders runnable without Studio and without hidden references to the source checkout.

### Milestone 13: Studio Distribution

Goal: release a Windows-first engineering tool.

Completed:

- Windows portable zip.
- Wails desktop entrypoint `HVAC Studio.exe`.
- `bin/studio.exe --server` for automation.
- Package smoke coverage for Studio API and runner examples.
- Runtime/export environment checks.
- `v0.1.0-alpha.1` and `v0.1.0-alpha.2` release tags.

Next depth:

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
- `mkdocs.yml` source configuration.
- Runtime-only example CLI guide.

Next depth:

- Build offline MkDocs HTML docs as release assets and include them in portable/runtime packages where appropriate.
- Add PDF manual generation once the Markdown guide structure is stable.
- Add in-app help links from Start, Project tree, System canvas, Inspector, Code, Run, Data, Parameters, Calibration, Optimization, Export, and Settings.
- Add screenshot-backed tutorials for component authoring, system building, validation, parameter sets, calibration, optimization, SDK use, and runtime-only delivery.
- Add a "concept map" page that explains how project artifacts relate to Studio screens, CLI commands, SDK calls, and exported packages.
- Keep docs versioned with releases and record behavior differences for alpha/beta/stable builds.

## Near-Term Update Sequence

1. Alpha hardening release: finish schema/error compatibility notes, release provenance, artifact openers, parameter-set selector consistency, saved validation/calibration/optimization result records, and docs build packaging.
2. Beta usability release: add dataset mapping UI, parameter-set diff editor, calibration setup editor, optimization setup editor, validation/result plots, and screenshot-backed tutorials.
3. 1.0 candidate release: freeze public artifact schemas, package offline docs, add installer/signing/provenance flow, complete migration docs, and run clean-machine release rehearsals.
4. Post-1.0 expansion: add native time-series execution, vectorized/external-executable component modes, loop solver boundaries, broader SDK helpers, and experimental non-Windows packages.

## Post-MVP Roadmap

### Track A: Runtime Contract Hardening

- Schema versioning and migrations.
- First-class time-series execution.
- Vectorized component execution mode.
- External executable component mode.
- Richer unit/value-type validation.
- Solver-boundary design for loops.
- Stable structured error schema for all commands.

### Track B: Studio Workflow Depth

- Dataset import and mapping editor.
- Validation results workspace with plots.
- Parameter set diff/editor.
- Calibration setup/result workspace.
- Optimization setup/result workspace.
- Better source editor diagnostics and quick fixes.
- Serve/session status workspace.

### Track C: Research And SDK

- SDK parameter-set support.
- SDK batch and scenario helpers.
- SDK validation/calibration/optimization wrappers.
- Example notebooks/scripts for research loops.
- External-engine integration examples with stable JSON contracts.

### Track D: Delivery And Release

- Runtime export docs generated per model.
- MkDocs HTML site packaged with releases.
- PDF manual.
- Windows installer.
- Signing and release provenance.
- macOS experimental package after Windows portable/installer stability.

### Track E: Examples And Templates

- HVAC plant template.
- Controller template.
- Stateful time-series example.
- Dataset validation tutorial.
- Calibration tutorial.
- Optimization tutorial.
- Runtime-only delivery tutorial.

## Release Gates

Per coherent implementation unit:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

For release candidates:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version <version> -SkipSetup
```

For GUI readability changes:

- start Studio server or desktop build
- capture desktop and mobile screenshots
- check Project tree, command bars, Inspector, canvas, tables, code editor, and results for overlap/truncation
- keep captures under ignored `artifacts/`

## Explicit Non-Goals For The Current MVP

- A fixed built-in HVAC component library as the main product.
- GUI-only model semantics.
- Python SDK reimplementation of the engine.
- Hidden mutation of baseline parameters during validation/calibration/optimization.
- Unplanned macOS support before Windows package behavior is stable.
- Feedback-loop solving without an explicit solver-boundary design.
