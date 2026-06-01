# HVAC Studio

HVAC Studio is the working repository for a Component-Node System Studio: a Python-first building-system modeling authoring/runtime tool.

The product goal is to let researchers define equipment, controls, surrogate models, objectives, and custom solvers in Python, while the runtime manages component-node contracts, graph validation, execution order, reproducibility, and delivery.

## Current Focus

The first stable slice is the runtime core:

- `project.bcsproj` and `graph.json` as source-of-truth files.
- User-defined Python components with `initialize(params, context)` and `evaluate(inputs, state, params, context)`.
- A persistent Python worker using JSONL over stdio.
- A Go `bcs-runner` CLI with `validate` and `run`.
- Golden examples that behave as regression assets.

GUI, optimization SDK, export packaging, and solver boundaries are planned on top of this runner.

## Repository Map

```text
app/studio/          Later Wails/React Studio GUI
tools/go/            Go runner, compiler, scheduler, runtime packages
python/bcs_worker/   Python component evaluator process
python/bcs_sdk/      Python wrapper around the runner
schema/              JSON schemas for project, graph, protocol, input, output
examples/            Runnable model examples and future golden tests
scripts/dev/         Local development and verification scripts
runtime/             Runtime packaging manifest
docs/                Architecture notes and ADRs
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

If PowerShell script execution is disabled on your machine, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

See [docs/setup.md](docs/setup.md) for the repo-local Go/Python layout.

Build and smoke-test the minimal runtime release package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

See [docs/release.md](docs/release.md) for the GitHub Release process.

Run the runner directly:

```powershell
cd tools\go
go run .\cmd\bcs-runner validate --project ..\..\examples\001_scalar_component\project.bcsproj
go run .\cmd\bcs-runner run --project ..\..\examples\001_scalar_component\project.bcsproj --input ..\..\examples\001_scalar_component\inputs\case01.json
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

The runtime does not interpret the physics inside the component. It validates the declared interface, calls the component, carries state, and serializes inputs/outputs across a stable boundary.
