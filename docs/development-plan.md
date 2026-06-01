# Development Plan

Last updated: 2026-06-01

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

## Milestone 0: Repository And Release Foundation

Status: in progress.

Already present:

- Repo-local setup for Go, uv, uv-managed Python, and `.venv`.
- Runtime core skeleton.
- `bcs-runner validate/run`.
- Python worker JSONL protocol.
- First scalar example.
- Minimal Windows runtime release package.

Near-term additions:

- Add CI workflow for non-release test runs.
- Add release package smoke test to CI.
- Add `examples` golden comparison helper.

Acceptance criteria:

- Fresh clone can run `scripts/dev/setup.ps1`.
- `scripts/dev/test-fast.ps1` passes using repo-local tools.
- `scripts/release/test-runtime-package.ps1` builds, expands, and smoke-tests a runtime zip.

## Milestone 1: Runtime Core Contract

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

Acceptance criteria:

- Invalid graph cases have golden validation errors.
- Algebraic loop detection reports the involved components.
- Missing public inputs and missing declared outputs fail with actionable messages.
- CLI errors return documented exit codes. Started in code with typed runner errors for validation, input, runtime, Python worker, and license/runtime categories.

## Milestone 2: Component Authoring Model

Goal: define how GUI-managed component contracts map to user-editable Python.

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

Status: started with `examples/003_feedforward_system`.

Goal: support multiple user-defined components connected through nodes.

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

Goal: create the first GUI surface without redefining runtime semantics.

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

- GUI can open the scalar example and display systems/components/nodes.
- Validate and Run buttons call `bcs-runner`, not a separate engine.
- Problems panel links validation messages to graph or source locations where possible.

## Milestone 5: Component-Aware Python Editor

Goal: let the user edit component logic with contract-aware help while protecting generated regions.

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

- Editing nodes/parameters/states updates the generated scaffold.
- Editing scaffold-protected areas is blocked or recovered safely.
- Autocomplete can suggest `inputs["chw_in"]["temperature"]` style paths from component contract metadata.

## Milestone 6: Run, Debug, And Inspect UX

Goal: make execution understandable for model authors.

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

Acceptance criteria:

- Release package vendors Python runtime or a frozen project environment.
- Package smoke test validates and runs without system Python.
- CLI guide and schemas are included in the package.
