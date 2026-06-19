# HVAC Studio

HVAC Studio is the working repository for a Component-Node System Studio: a Python-first building-system modeling authoring/runtime tool.

The product goal is to let researchers define equipment, controls, surrogate models, objectives, and custom solvers in Python, while the runtime manages component-node contracts, graph validation, execution order, reproducibility, and delivery.

The release strategy is Windows-first: the initial supported platform is Windows 10/11 x64, distributed first as a portable zip. The engine, project format, graph schema, and component schema should remain OS-independent so macOS can be offered through an experimental support package.

## Current Development Surface

HVAC Studio is currently a runtime-backed Studio workspace plus CLI, SDK,
examples, and package gates:

- `project.bcsproj` and `graph.json` as source-of-truth files.
- User-defined Python components with `initialize(params, context)` and `evaluate(inputs, state, params, context)`.
- A persistent Python worker using JSONL over stdio.
- A Go `bcs-runner` CLI with `validate`, `run`, `run-series`, `serve`, `schema`, `migrate`, `validate-data`, `calibrate`, and `optimize`.
- A Python `bcs_sdk` client that wraps `bcs-runner serve` for repeated evaluations, async/pool evaluation, and helpers for validation, calibration, optimization, batches, schemas, and export manifests.
- Golden examples that behave as regression assets, including scalar, generated-wrapper, stateful time-series, plant workflow, optimization, runtime-only, vectorized, external executable, solver boundary, unit conversion, composite, ANN asset, and RC/ANN composition cases.
- A Wails-based Studio desktop UI that opens examples, creates workspace projects and components, edits component contracts and Python source, manages parameters and default inputs, imports datasets, creates validation/calibration/optimization setups, runs workflows, saves records, and exports runtime packages.

Studio is an authoring and inspection surface over the same source-of-truth
files and runner-backed workflows used by the CLI and SDK; it should not grow a
separate simulation engine.

## Repository Map

```text
tools/go/            Go runner, compiler, scheduler, runtime packages, and Studio host
python/bcs_worker/   Python component evaluator process
python/bcs_sdk/      Python wrapper around the runner
schema/              JSON schemas for project, graph, protocol, input, output
examples/            Runnable model examples and golden tests
scripts/dev/         Local development and verification scripts
runtime/             Runtime packaging manifest
templates/           Source templates for Studio-created projects/components
docs/                User guide, status, release notes, architecture notes, and ADRs
```

## Quick Start

Bootstrap the repo-local toolchain:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\setup.ps1
```

Validate the first example:

```powershell
.\scripts\dev\test-runner.ps1
```

Run the fast checks:

```powershell
.\scripts\dev\test-fast.ps1
```

Run every runnable example and compare against its golden output:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-examples.ps1
```

Run the scripted acceptance walkthroughs:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-acceptance-walkthroughs.ps1
```

Run validation golden cases:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-validation.ps1
```

Launch the Studio workspace:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\run-studio.ps1
```

The development launcher opens the Wails Studio desktop app. Use `-Server` when you only want the local Studio HTTP API for automation.

If PowerShell script execution is disabled on your machine, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

See [docs/setup.md](docs/setup.md) for the repo-local Go/Python layout.

Read the current status and release process when changing release scope:

```text
docs/status.md
docs/release.md
```

`docs/status.md` is the maintainer-facing snapshot for what works now, what is
not supported yet, where retained local build zips live, and which generated
folders are safe to clean. `docs/release.md` owns package scope, release gates,
checksums, provenance, signing/trust notes, and GitHub Release procedure.

Read the user guide:

```text
docs/user/index.md
```

Build and smoke-test a full local release candidate:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev
```

Run only the release upgrade rehearsal:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-upgrade-rehearsal.ps1 -Version 0.1.0-dev
```

Build and smoke-test only the portable Studio release package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
```

The portable package includes root-level `HVAC Studio.exe`, `bin/studio.exe`, `bcs-runner.exe`, `bcs-env.exe`, examples, and a bundled `runtime/python` for included example and workspace project runs.
Users launch the app by double-clicking `HVAC Studio.exe`; automation can run `bin\studio.exe --server`.
Use `bin\bcs-env.exe check` inside a package to verify the bundled Python runtime, worker, schemas, examples, and executables.
Studio-created projects live under `projects/`; workspace edits persist to project artifacts such as `graph.json`, `inputs/`, `components/`, `scenarios/`, `runs/`, and `exports/`.
Runtime exports under `projects/<name>/exports/runtime_package/` include workflow scripts such as `check-env.ps1`, `run-default.ps1`, `run-scenario.ps1`, `run-batch.ps1`, `validate-data.ps1`, `calibrate.ps1`, `optimize.ps1`, and `serve.ps1` when those artifacts exist, plus `docs/CLI_Guide.md` and `sdk-example.py`. Run scripts write result JSON under `outputs/` and component-log diagnostic bundles under `outputs/logs/`.

Build and smoke-test the Windows installer bundle:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-installer-package.ps1 -Version 0.1.0-dev
```

The installer bundle wraps the portable zip, checks WebView2 runtime presence,
creates a Start Menu shortcut by default, supports optional user PATH
registration, and records the `.bcsproj` association policy without enabling it
until Studio supports project-file launch.

Build and smoke-test only the runtime-only release package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

Build and smoke-test the experimental macOS support package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-macos-package.ps1 -Version 0.1.0-dev
```

The macOS package is a support bundle with a package plan, prerequisite checks,
docs, schemas, examples, and signing/notarization caveats. It is not a signed
or notarized public macOS app.

See [docs/release.md](docs/release.md) for the GitHub Release process.
See [docs/release-trust.md](docs/release-trust.md) for signing, checksum,
license notice, dependency notice, support matrix, and release-note policy.

Run the runner directly:

```powershell
cd tools\go
go run .\cmd\bcs-runner validate --project ..\..\examples\001_scalar_component\project.bcsproj
go run .\cmd\bcs-runner run --project ..\..\examples\001_scalar_component\project.bcsproj --input ..\..\examples\001_scalar_component\inputs\case01.json
go run .\cmd\bcs-runner run --project ..\..\examples\009_vectorized_component\project.bcsproj --input ..\..\examples\009_vectorized_component\inputs\case01.json
go run .\cmd\bcs-runner run --project ..\..\examples\010_external_executable_component\project.bcsproj --input ..\..\examples\010_external_executable_component\inputs\case01.json
go run .\cmd\bcs-runner run --project ..\..\examples\011_solver_boundary_component\project.bcsproj --input ..\..\examples\011_solver_boundary_component\inputs\case01.json
go run .\cmd\bcs-runner run --project ..\..\examples\012_unit_conversion_component\project.bcsproj --input ..\..\examples\012_unit_conversion_component\inputs\case01.json
go run .\cmd\bcs-runner run --project ..\..\examples\013_composite_system\project.bcsproj --input ..\..\examples\013_composite_system\inputs\case01.json
go run .\cmd\bcs-runner run-series --project ..\..\examples\004_stateful_controller\project.bcsproj --input ..\..\examples\004_stateful_controller\inputs\series01.json --output ..\..\artifacts\series-output.json
go run .\cmd\bcs-runner run-series --project ..\..\examples\013_composite_system\project.bcsproj --input ..\..\examples\013_composite_system\inputs\series01.json --output ..\..\artifacts\composite-series-output.json
'{ "id": "case-1", "inputs": { "value": 4 }, "context": { "time": 0, "dt": 60 } }' | go run .\cmd\bcs-runner serve --project ..\..\examples\001_scalar_component\project.bcsproj
go run .\cmd\bcs-runner schema --project ..\..\examples\003_feedforward_system\project.bcsproj --output ..\..\examples\003_feedforward_system\outputs\schema.json
go run .\cmd\bcs-runner migrate --project ..\..\examples\001_scalar_component\project.bcsproj --output ..\..\artifacts\migration-report.json
go run .\cmd\bcs-runner validate-data --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj --mapping validation\mappings\plant_validation.json
go run .\cmd\bcs-runner run --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj --input ..\..\examples\005_chiller_plant_like_system\inputs\case01.json --parameter-set parameter_sets\high_efficiency.json
go run .\cmd\bcs-runner calibrate --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj --setup calibration\setups\chiller_cop_grid.json --output ..\..\artifacts\calibration-result.json
go run .\cmd\bcs-runner optimize --project ..\..\examples\006_optimization_case\project.bcsproj --setup optimization\setups\chw_setpoint_grid.json --output ..\..\artifacts\optimization-result.json
```

## Component Contract

```python
class MyComponent:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {}, state
```

Vectorized components declare `"execution_mode": "vectorized"` and can implement `evaluate_batch(inputs, state, params, context)` with the same `(outputs, state)` return contract. External executable components declare `kind: "external_exe"` and `execution_mode: "external_executable"`; the runner sends a JSON request to the configured process on stdin and reads a JSON response from stdout. The runtime does not interpret the physics inside the component. It validates the declared interface, calls the component, carries state, and serializes inputs/outputs across a stable boundary.

Graph-level feedback loops are not solved implicitly. Iterative behavior belongs inside an explicit solver boundary component that still exposes normal public inputs and outputs to the outer acyclic graph.

Connection-level unit conversion is explicit. Use `unit_conversion` on a connection for linear numeric conversions; the runtime also validates common `value_type` contracts such as float, integer, boolean, string, array, and object.

Composite components declare `kind: "composite"` and `composite.system`.
Their node IDs must match the child system public input/output IDs. The runner
evaluates the child system through the same public IO contract and stores nested
child state under the wrapper component state.
