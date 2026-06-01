# HVAC Studio

HVAC Studio is the working repository for a Component-Node System Studio: a Python-first building-system modeling authoring/runtime tool.

The product goal is to let researchers define equipment, controls, surrogate models, objectives, and custom solvers in Python, while the runtime manages component-node contracts, graph validation, execution order, reproducibility, and delivery.

The release strategy is Windows-first: the initial supported platform is Windows 10/11 x64, distributed first as a portable zip. The engine, project format, graph schema, and component schema should remain OS-independent so macOS can become an experimental post-MVP target.

## Current Focus

The first stable slice is the runtime core plus a real Studio workspace shell:

- `project.bcsproj` and `graph.json` as source-of-truth files.
- User-defined Python components with `initialize(params, context)` and `evaluate(inputs, state, params, context)`.
- A persistent Python worker using JSONL over stdio.
- A Go `bcs-runner` CLI with `validate` and `run`.
- Golden examples that behave as regression assets.
- A Go-hosted Studio web UI that opens examples, renders the system canvas, inspects components, validates, runs, and exports public schema.

The Studio UI is intentionally built as the full product workspace first. Individual panels can be wired up gradually without changing the source-of-truth files or inventing a separate simulation engine.

## Repository Map

```text
app/studio/          Studio GUI direction and future installed app notes
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

Run every runnable example and compare against its golden output:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-examples.ps1
```

Run validation golden cases:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-validation.ps1
```

Launch the Studio workspace:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\run-studio.ps1
```

Then open:

```text
http://127.0.0.1:5174
```

If PowerShell script execution is disabled on your machine, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1
```

See [docs/setup.md](docs/setup.md) for the repo-local Go/Python layout.

Read the milestone plan:

```text
docs/development-plan.md
```

Build and smoke-test the portable Studio release package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-portable-package.ps1 -Version 0.1.0-dev
```

Build and smoke-test the runtime-only release package:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\release\test-runtime-package.ps1 -Version 0.1.0-dev
```

See [docs/release.md](docs/release.md) for the GitHub Release process.

Run the runner directly:

```powershell
cd tools\go
go run .\cmd\bcs-runner validate --project ..\..\examples\001_scalar_component\project.bcsproj
go run .\cmd\bcs-runner run --project ..\..\examples\001_scalar_component\project.bcsproj --input ..\..\examples\001_scalar_component\inputs\case01.json
go run .\cmd\bcs-runner schema --project ..\..\examples\003_feedforward_system\project.bcsproj --output ..\..\examples\003_feedforward_system\outputs\schema.json
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
