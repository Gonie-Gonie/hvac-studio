# Development Plan

Last updated: 2026-06-03

This plan folds the Component-Node-System UX flow into the runtime-first repository direction. The product is not a drag-and-drop HVAC library. It is a Python-based component-node-system authoring and runtime tool for building equipment modeling and control research.

## Product UX Thesis

The user defines component-node-system structure and parameter schema in the GUI, edits only the component calculation body in a component-aware Python editor, then reuses the same system for one-case runs, time-series simulation, batch simulation, model validation, calibration, optimization, Python SDK calls, CLI integration, and runtime-only export.

The primary workflow is:

```text
New Project
-> Python environment selection
-> Component creation
-> Component node definition
-> Parameter/state definition
-> Function body editing
-> System canvas composition
-> Node-to-node connection
-> Public input/output mapping
-> Validate
-> Run one case
-> Dataset import
-> Model validation
-> Calibration
-> Optimization
-> Export
```

## Operating Priorities

1. Stabilize runner/worker/source-of-truth files before GUI.
2. Keep GUI as authoring UX over `project.bcsproj`, `graph.json`, component files, schemas, datasets, parameter sets, scenarios, and run records.
3. Preserve user freedom in component logic, node count/meaning, parameters, system composition, and runtime mode.
4. Make every milestone testable by CLI/golden examples before attaching GUI.
5. Commit and push after each coherent test-green unit.
6. Build the Studio as a complete workspace shell first, then progressively connect panel behavior.
7. Release Windows-first as a portable installed tool while keeping engine/project/schema formats OS-independent.
8. Treat user-facing documentation as a product surface: explain workflow and runtime behavior, not only button usage.

## Milestone 0: Repository And Release Foundation

Status: complete. Verified on 2026-06-03 with `scripts/release/test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup`.

Already present:

- Repo-local setup for Go, uv, uv-managed Python, and `.venv`.
- Runtime core skeleton.
- `bcs-runner validate/run`.
- Python worker JSONL protocol.
- First scalar example.
- Minimal Windows runtime release package.
- Go-hosted Studio shell and Windows portable package script.
- Bundled Python runtime copied into Windows portable and runtime-only packages.
- Source templates under `templates/` for Studio-created projects/components.
- CI workflow for non-release test runs.
- Release package smoke test in CI for main and manual runs.
- Project-specific Python package lock/freeze support on top of bundled `runtime/python`.
- Explicit Go platform boundary for path, process, runtime Python, and executable naming decisions.

Near-term additions:

- None remaining for Milestone 0.

Acceptance criteria:

- Fresh clone can run `scripts/dev/setup.ps1`.
- `scripts/dev/test-fast.ps1` passes using repo-local tools.
- `scripts/release/test-runtime-package.ps1` builds, expands, and smoke-tests a runtime zip.
- `scripts/release/test-portable-package.ps1` builds, expands, and smoke-tests a portable Studio zip.
- `scripts/release/test-release-candidate.ps1` is the local release gate for fast checks plus both package smoke tests.
- Portable packages include real project/component templates rather than placeholder directories.

## Milestone 1: Runtime Core Contract

Status: complete. Verified on 2026-06-03 with `scripts/dev/test-fast.ps1` and `scripts/release/test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup`.

Goal: make the file contract robust enough for GUI and SDK to build on.

Scope:

- `project.bcsproj` and `graph.json` structural validation.
- Component, node, connection, system, public input, public output schemas.
- Explicit public IO endpoint mapping.
- Clear validation errors with component/node references.
- Exit code taxonomy for CLI:
  - `0`: success
  - `1`: validation error
  - `2`: runtime error
  - `3`: input schema error
  - `4`: Python worker error
  - `5`: license/runtime error
- One-case run result format with component outputs, states, context, and execution order.
- Public interface schema export through `bcs-runner schema`.

Acceptance criteria:

- Invalid graph cases have golden validation errors for algebraic loops, public IO endpoint mapping, and structural node errors.
- Algebraic loop detection reports the involved components.
- Missing public inputs and missing declared outputs fail with actionable messages.
- CLI errors return documented exit codes for validation, input, runtime, Python worker, and license/runtime categories.

## Milestone 2: Component Authoring Model

Status: complete. Verified on 2026-06-03 with `scripts/dev/test-fast.ps1`.

Goal: define how GUI-managed component contracts map to user-editable Python.

Implemented:

- Component graph/schema fields for category, execution mode, source layout, node presets, parameter definitions, and state definitions.
- Scalar project/component templates now seed the component authoring metadata.
- `generated_wrapper` source layout maps `component.json` and wrapper files to user-editable `user_init.py`, `user_step.py`, and `helpers.py`.
- Studio source loading/checking opens the user step body for generated-wrapper components and validates `step(inputs, state, params, context)`.
- Runtime export includes nested generated-wrapper component metadata, user body files, helpers, and wrapper adapter.
- `examples/008_generated_wrapper_component` verifies worker execution through a generated wrapper.

UX requirements:

- User creates a New Python Component by default.
- Component categories:
  - physical component
  - controller
  - data source
  - data sink
  - utility
  - composite wrapper
- Execution modes:
  - step-based
  - vectorized
  - initialization only
  - external executable
- Node presets:
  - water inlet/outlet
  - air inlet/outlet
  - control signal input
  - electric power output
  - scalar input/output
  - time-series input

Storage direction:

```text
components/
  custom_chiller/
    component.json
    user_init.py
    user_step.py
    helpers.py
```

The GUI may display a generated function scaffold, but the safe persisted boundary should separate generated contract metadata from user-editable function bodies.

Acceptance criteria:

- A component can declare arbitrary inlet/outlet/signal nodes.
- Parameters include display name, unit, default/current value, bounds, role, group, and description.
- States include name, unit, and initial value.
- Worker can execute a component body through a generated wrapper without exposing runtime-managed regions as editable code.

## Milestone 3: Feed-Forward Component-Node Systems

Status: complete. Verified on 2026-06-03 with `scripts/dev/test-fast.ps1`.

Goal: support multiple user-defined components connected through nodes.

Implemented:

- `examples/003_feedforward_system` runs an acyclic four-component graph through the runner and example smoke tests.
- Compiler validation requires connection sources to be declared output nodes and targets to be declared input nodes.
- Compiler topological ordering preserves feed-forward execution and reports algebraic loops with involved components.
- Medium compatibility follows the MVP UX rule: matching media pass, signal-to-physical connections pass with warnings, physical mismatches fail by default, and explicit medium overrides are structured in `graph.json`.
- Runtime results include `component_inputs`, `component_outputs`, `node_values`, and `connection_values` for inspection.
- Studio validation surfaces connection medium warnings in the Problems panel without blocking execution.

Scope:

- Build out `examples/003_feedforward_system`.
- Propagate connection values component-to-component.
- Validate source output node to target input node.
- Validate medium compatibility with warning/override planning.
- Preserve acyclic topological execution for MVP.

Connection UX rule:

```text
Allowed: water outlet node -> water inlet node
Warning: signal node -> water inlet node
Error by default: air outlet node -> water inlet node
```

Research override direction:

- Medium mismatch should support an explicit custom-connection override later.
- Overridden connections must be visibly marked in GUI and structured in `graph.json`.

Acceptance criteria:

- Feed-forward example runs through runner and package smoke test.
- Each connection has traceable source/target endpoint metadata.
- Runtime output can show node values for inspection.

## Milestone 4: Project Explorer And GUI Shell

Status: complete. Verified on 2026-06-03 with `scripts/dev/test-fast.ps1`.

Goal: create the first GUI surface without redefining runtime semantics.

Implemented:

- Go-hosted Studio shell in `tools/go/cmd/studio` with the planned top bar, Project tree, System canvas, Inspector, and bottom workspace panels.
- Start workspace for runtime status, project type visibility, workspace projects, and examples.
- Project Explorer sections for systems, components, Python source, datasets, parameter sets, runs, batches, scenarios, and export profiles.
- Scalar and feed-forward projects open through the same project loader and display graph systems, components, nodes, and canvas connections.
- Validate, Run, Batch, Schema, and Export actions call the existing compiler/runtime/server paths used by CLI-facing code.
- New/copy project, starter workspace creation, component creation, component inclusion, node/connection editing, canvas layout persistence, default run inputs, scenarios, saved runs, parameter editing, source checks, and runtime export are wired for workspace projects.
- The Serve command slot is visible in the shell and intentionally disabled until the later serve-mode runtime milestone.

Primary layout:

```text
Top Bar: Project | Validate | Run | Batch | Serve | Export
Left: Project Tree
Center: System Canvas
Right: Inspector
Bottom: Problems | Logs | Python Console | Results | Schema
```

Project Explorer objects:

- Systems
- Components
- Python Source
- Datasets
- Parameter Sets
- Runs
- Scenarios
- Export Profiles

Start page:

- New Project
- Open Project
- Recent Projects
- Example Projects
- Runtime/Python Environment Status

Project types:

- Empty System
- Python Component Project (default)
- HVAC System Template
- Runtime-only Imported Project

Acceptance criteria:

- GUI can open scalar and feed-forward examples and display systems/components/nodes.
- Validate, Run, and Schema buttons use the same runtime/compiler path as the CLI runner. Validate also checks Python component source contracts.
- GUI can create a workspace project from the scalar Python component template.
- Fresh Studio sessions create/open an editable starter workspace when no workspace project exists.
- GUI can add a workspace Python component template from `templates/components/` to `graph.json` and `components/` without changing the runnable system yet.
- GUI can explicitly add a workspace component to the entry system with generated public IO and default input values.
- Inspector can jump from a selected workspace component to its Code workspace source editor.
- GUI can create a node-to-node connection between workspace system components from the canvas or Inspector and persist it to `graph.json`.
- GUI can select persisted canvas connection lines and remove them through the same Inspector/API path.
- GUI can drag workspace canvas components and persist view layout to `studio/layout.json`.
- GUI can load and save a workspace project's `default_input` run values.
- GUI can save current run inputs as workspace `scenarios/*.json` artifacts and reload them into Run Inputs.
- Runs from workspace projects are saved as `runs/run-*.json` records.
- Saved run records can be reopened from the Project tree and shown in Results.
- Parameter Manager can edit workspace component parameters and persist them to `graph.json`.
- Problems, results, schema, logs, inspector, parameters, run output, and runtime export workspaces are visible in the active shell.
- Export button can write a workspace `exports/runtime_package/` artifact with manifest, public IO schema, source-of-truth project files, a first-run script, and packaged runner/Python support when available.
- `bcs-env check` can diagnose exported runtime folders as `runtime-export` packages.
- Problems panel links validation, run, and batch-case messages to graph or source locations where possible, including inferred component links for runtime errors.
- Source save returns source-check feedback and execution actions stop on saved source-check errors.
- Source checks warn when Python source does not visibly reference required graph inputs or declared outputs.
- Run, batch, and export APIs reject saved source-check errors server-side.
- Code workspace snippets generate evaluate skeletons from all selected component input/output nodes.
- Code workspace shows source-check issue rows in the contract panel and can focus line-specific problems.
- Source checks load draft Python source to catch import and class-load errors before run/export.
- Python editor supports save/check shortcuts and line-based indent/outdent.
- Code workspace can run the project after source edits through the normal save/check/run path.
- Code workspace shows selected component last-run inputs and outputs alongside source contract context.
- System canvas shows latest run input/output values on component node endpoints.
- Studio marks last-run values stale when runtime-affecting inputs, source, parameters, nodes, or connections change.
- API coverage pins the edit-source, connect-components, run-result propagation workflow.

## Milestone 5: Component-Aware Python Editor

Status: complete for the MVP editor surface. Verified on 2026-06-03 with `scripts/dev/test-go.ps1` and capture review under `artifacts/ux-captures/`.

Goal: let the user edit component logic with contract-aware help while protecting generated regions.

Implemented:

- Studio Code workspace loads component source, keeps bundled examples read-only, and saves workspace source through the project API.
- The editor has lightweight Python syntax highlighting, line numbers, bracket status, tab/shift-tab indentation, Enter auto indentation, save/check shortcuts, and contract-derived snippets/completions.
- The contract panel shows runtime-managed signatures, inputs, outputs, parameters, context/state completions, source-check rows, and latest run values for the selected component.
- Source checks validate class/function signatures, Python syntax, graph input/output references, import/load failures, and undefined-name hints without executing component evaluation.
- Run, batch, and export actions flush dirty source and stop on saved source-check errors through both GUI and server API paths.

Required editor features:

- Python syntax highlighting.
- Bracket matching.
- Auto indentation.
- Syntax errors.
- Undefined variable hints.
- Component contract autocomplete.
- Node name autocomplete.
- Variable name autocomplete.
- Parameter name autocomplete.
- Pre-run lint.
- Runtime error location display.

Recommended editor features:

- Type-hint based completion.
- Unit display.
- Generated docstring.
- Formatting.
- Quick fix.
- Hover documentation.

Generated/protected areas:

- import policy
- class wrapper
- function signature
- input/output binding
- state/parameter loading
- return format

User-editable area:

- The calculation body for `step(t, dt, inputs, params, state)` or the equivalent component function.

Acceptance criteria:

- Studio Python panel can load component source files and save workspace edits while examples remain read-only.
- Editing nodes/parameters/states updates the contract panel, snippets, and completions from the current graph metadata.
- Runtime-managed class/function signatures are checked before run/export, and generated-wrapper projects expose only user body files as editable source.
- Autocomplete/completion suggestions can insert `inputs["..."]`, `params["..."]`, `state["..."]`, `context["..."]`, output-return entries, and local input variables from component contract metadata.

## Milestone 6: Run, Debug, And Inspect UX

Status: complete for one-case, batch, and JSONL serve MVP. Verified on 2026-06-03 with `scripts/dev/test-go.ps1` and Studio capture review.

Goal: make execution understandable for model authors.

Implemented:

- Runtime execution uses a reusable session that compiles the graph, starts the Python worker, loads components, initializes state, and evaluates repeatedly.
- `bcs-runner serve` exposes a JSONL request/response loop for repeated external evaluations while preserving component state inside the live session.
- Run results include public outputs, component inputs/outputs, node values, connection values, states, context, execution order, per-component timings, and total duration.
- Studio Run workspace shows latest run summary, public outputs, selected component values, batch cases, output preview, execution trace, connection values, and node values.
- Canvas, Inspector, Code workspace, Problems, and Results all consume the same structured runtime result and problem metadata.

Run modes:

- one case
- time-series
- batch cases
- serve mode
- Python SDK

Debug/inspect outputs:

- component execution order
- node values
- component logs
- selected timestep component inputs/outputs
- parameters
- states
- execution time

Acceptance criteria:

- Runner output has enough structured data for the GUI Results and Inspect panels.
- Runtime errors preserve component ID, node ID when applicable, and Python traceback/location.
- Serve mode compiles graph and imports Python components once, then evaluates repeatedly.

## Milestone 7: Datasets And Model Validation

Status: complete for CSV mapping and metrics MVP. Verified on 2026-06-03 with `go test ./cmd/bcs-runner ./internal/modelvalidation ./internal/studio`.

Implemented:

- `examples/005_chiller_plant_like_system` includes a CSV validation dataset and saved mapping artifact.
- `bcs-runner validate-data` runs project-relative mappings and writes structured metrics.
- Validation metrics include RMSE, MAE, MBE, CVRMSE, and R2.
- Validation results include row summaries and high-error row inspection data with component/node/connection traces.
- Studio Project tree lists datasets, validation mappings, and parameter sets; the `Data` command runs the first saved mapping and shows the result.

Goal: connect measured/reference datasets to system public inputs/outputs.

Dataset import UX:

- Import CSV/weather-like files.
- Detect columns.
- Map dataset columns to system public inputs.
- Map observed output columns to system public outputs.

Validation metrics:

- RMSE
- MAE
- MBE
- CVRMSE
- R2

Plots:

- measured vs simulated time-series
- scatter plot
- residual plot
- error histogram
- error by operating range

Acceptance criteria:

- Dataset mapping is saved in source-of-truth project artifacts.
- Validation run writes structured metrics and links high-error timesteps to inspectable node/component values.

## Milestone 8: Parameter Manager And Parameter Sets

Status: complete for parameter editing and parameter-set runtime overlay MVP. Verified on 2026-06-03 with `go test ./cmd/bcs-runner ./internal/parameterset ./internal/studio`.

Implemented:

- Studio Parameter Manager can edit workspace component parameters in `graph.json`.
- Project tree lists saved `parameter_sets/*.json` artifacts.
- `bcs-runner run --parameter-set` applies a saved parameter set in memory without overwriting baseline `graph.json`.
- `bcs-runner validate-data --parameter-set` evaluates validation datasets against a saved parameter set.
- Studio run and validation APIs accept `parameter_set_path`; run records and runtime results preserve the parameter-set path.

Goal: make parameters first-class research objects.

Parameter hierarchy:

- project parameters
- system parameters
- component parameters
- scenario parameters
- calibration parameters
- optimization variables

Parameter roles:

- fixed
- scenario input
- calibration target
- optimization variable
- derived parameter

Parameter sets:

- default
- calibrated cases
- design cases
- optimization results

Each run record must include:

- project version
- graph version
- parameter set
- input dataset/scenario
- output location

Acceptance criteria:

- Parameter sets can be saved without overwriting baseline values.
- Runs are reproducible from recorded parameter set and input artifacts.
- Parameter table supports filtering by component, role, unit, parameter set, changed-only, calibratable, and optimizable.

## Milestone 9: Calibration

Status: complete for grid-search CLI/runtime MVP. Verified on 2026-06-03 with `go test ./cmd/bcs-runner ./internal/calibration`.

Implemented:

- Saved calibration setup artifact under `calibration/setups/*.json`.
- `bcs-runner calibrate --setup` runs grid search against a saved validation mapping.
- Calibration setup supports base parameter set, weighted RMSE objective, numeric parameter bounds, and step size.
- Calibration result reports initial objective, best objective, changed parameters, candidate objectives, and best parameter set.
- `--save-parameter-set` writes a new parameter set without overwriting baseline `graph.json`.

Goal: estimate model parameters from observed data.

Calibration setup:

- Select dataset.
- Select target outputs.
- Select objective, initially weighted RMSE.
- Select calibration parameters and bounds.
- Choose algorithm:
  - grid search
  - differential evolution
  - scipy least_squares
  - Bayesian optimization later

Result behavior:

- Show initial/final objective.
- Show changed parameters.
- Default action saves as a new parameter set.
- Optional report export.

Acceptance criteria:

- Calibration does not overwrite current parameters by default.
- Calibration can be reproduced from dataset mapping, objective settings, parameter bounds, and base parameter set.

## Milestone 10: Optimization

Status: complete for public-input grid-search CLI/runtime MVP. Verified on 2026-06-03 with `go test ./cmd/bcs-runner ./internal/optimization`.

Implemented:

- Saved optimization setup artifact under `optimization/setups/*.json`.
- `bcs-runner optimize --setup` runs grid search over public input decision variables.
- Optimization setup supports base inputs, context, objective output, min/max/step bounds, and min/max sense.
- Optimization result reports candidate objectives, best inputs, best outputs, and saved scenario path.
- `--save-scenario` writes an optimized scenario artifact for later runs.

Goal: optimize control inputs or design parameters against objectives and constraints.

Optimization differs from calibration:

- Calibration estimates model parameters to match data.
- Optimization changes public inputs or design parameters to minimize/maximize an objective.

Optimization setup:

- Base parameter set.
- Decision variables from public inputs or component parameters.
- Objective, initially a public output such as `total_power`.
- Constraints from public outputs.
- Dataset/scenario.

Optimization modes:

- single scenario
- time-series
- batch scenario
- external Python through SDK

Acceptance criteria:

- GUI supports basic optimization.
- Advanced optimization can export a Python SDK script.
- Results can be saved as scenario, parameter set, Python script, and CSV.

## Milestone 11: SDK And External Engine Integration

Goal: make research and delivery paths use the same runner.

Principles:

- `bcs_sdk` is not a simulation engine.
- `bcs_sdk` wraps `bcs-runner.exe serve`.
- GUI, CLI, SDK, and external engines use the same project files and runner.

Acceptance criteria:

- SDK can keep runner alive across repeated evaluations.
- Example optimization script uses SDK and the scalar/feed-forward examples.
- External engine calls receive stable JSON outputs and exit codes.

## Milestone 12: Runtime-Only Delivery Package

Goal: deliver models without GUI.

Target package shape:

```text
DeliveredModel/
  bin/
    bcs-runner.exe
    bcs-env.exe
  model/
    project.bcsproj
    graph.json
    components/
    schema/
  runtime/
    python/
  examples/
    input.json
    run_once.ps1
    run_batch.ps1
  docs/
    CLI_Guide.md
```

Delivery requirements:

- No external Python installation requirement.
- Clear input/output schema.
- Example input/output.
- Logs.
- Clear exit codes.
- Structured errors for external engines.
- Package environment check through `bcs-env.exe check`.

Acceptance criteria:

- Release package vendors Python runtime or a frozen project environment.
- Package smoke test validates and runs without system Python.
- Runtime and portable smoke tests run `bcs-env.exe check --json` before runner/API checks.
- CLI guide and schemas are included in the package.

## Milestone 13: Installed Studio Distribution

Goal: release HVAC Studio as a Windows-first installed/portable engineering tool.

Distribution order:

- Windows portable zip first.
- Windows installer after the portable package is stable.
- macOS experimental package after MVP.

Portable package target:

```text
hvac-studio-<version>-windows-amd64-portable/
  HVAC Studio.exe
  bin/
    studio.exe
    bcs-runner.exe
    bcs-env.exe
  runtime/
    python/
  python/
    bcs_worker/
    bcs_sdk/
  schema/
  examples/
  templates/
  docs/
```

Acceptance criteria:

- Portable package launches Studio from `HVAC Studio.exe` as a Wails desktop app without launching a browser or binding a normal-use TCP port.
- Portable package can create projects under `projects/`.
- Studio-created projects are copied from source templates under `templates/projects/`.
- CLI runner validates and runs included examples using bundled `runtime/python`.
- `bcs-env.exe check` verifies packaged Python, worker, schemas, examples, and entrypoints.
- Package smoke test exercises Studio API and runner CLI after zip expansion.
- Installer work does not start until portable zip behavior is reproducible.
- macOS packaging remains a deliberate post-MVP release target, not an implicit promise.

## Milestone 14: User Guide, In-App Help, And Documentation Release

Status: started with Markdown source under `docs/user/` and an initial `mkdocs.yml`.

Goal: give users enough conceptual and procedural guidance to build correct component-node-system models.

Documentation thesis:

- The User Guide should explain how users operate the program.
- It should also explain how project files, `graph.json`, public IO, the runner, and the Python worker cooperate internally.
- It should avoid low-level implementation details that do not help users model correctly, such as worker protocol messages, WebView internals, or scheduler implementation details.

Source structure:

```text
docs/user/
  index.md
  quick-start.md
  core-concepts.md
  how-it-works.md
  create-component.md
  edit-python-function.md
  build-system.md
  parameter-management.md
  run-simulation.md
  data-validation.md
  calibration.md
  optimization.md
  cli-runner.md
  export-runtime.md
  troubleshooting.md
  glossary.md
```

Target build flow:

```text
Markdown source
-> MkDocs HTML site
-> in-app Help Viewer / offline docs
-> PDF manual
-> GitHub Release assets
```

In-app help mapping direction:

- Component Editor -> `docs/user/create-component.md`
- Python Function Editor -> `docs/user/edit-python-function.md`
- System Canvas -> `docs/user/build-system.md`
- Parameter Manager -> `docs/user/parameter-management.md`
- Validation Workspace -> `docs/user/data-validation.md`
- Calibration Workspace -> `docs/user/calibration.md`
- Optimization Workspace -> `docs/user/optimization.md`
- CLI Export -> `docs/user/export-runtime.md`

Acceptance criteria:

- User Guide Markdown source exists and is linked from README. Started.
- MkDocs config can build an offline HTML user guide. Started.
- Release scripts can package HTML docs and a PDF manual as release assets.
- Portable Studio can open relevant local help pages from major workspaces.
- Runtime-only packages include concise user-facing CLI/runtime documentation.
