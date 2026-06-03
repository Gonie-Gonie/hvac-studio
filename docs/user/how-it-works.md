# How It Works

## Studio Is An Authoring Tool

Studio edits and visualizes project artifacts. It is not a separate simulation engine.

When Studio validates or runs a model, it uses the same compiler/runtime path as the CLI runner.

## Project Files Are The Source Of Truth

A project is defined by files:

```text
project.bcsproj
graph.json
components/
inputs/
requirements.lock.txt
scenarios/
runs/
exports/
```

- `project.bcsproj` stores project metadata, entry system, default input, and environment settings, including the optional Python lockfile path.
- `graph.json` stores systems, components, nodes, connections, public inputs, public outputs, and parameters.
- `components/` stores user Python source.
- `inputs/` stores default run input files.
- `requirements.lock.txt` or another declared lockfile stores the frozen project Python package set.
- `scenarios/` stores reusable input/context cases.
- `runs/` stores saved execution records.
- `exports/` stores export manifests and later package outputs.

## Runner Executes The System

Execution follows this shape:

1. Load `project.bcsproj`.
2. Load `graph.json`.
3. Validate component, node, connection, and public IO references.
4. Build an execution order.
5. Start the Python worker.
6. Initialize components.
7. Evaluate components in order.
8. Collect public outputs.
9. Return or save structured results.

## Python Worker Executes User Code

User Python runs in a worker process. This keeps the runtime boundary explicit and makes future repeated evaluation, optimization, and external engine integration more stable.

## SDK Wraps Serve Mode

The Python SDK is a client for `bcs-runner serve`. `RunnerClient.start(...)` keeps the runner process alive and sends repeated JSON requests to the same compiled project/session:

```python
from bcs_sdk import RunnerClient

with RunnerClient.start("project.bcsproj", runner="bcs-runner.exe") as client:
    result = client.evaluate({"value": 4}, {"time": 0, "dt": 60})
```

For file-backed workflows, `RunnerClient` also provides helpers such as `run_validation`, `run_calibration`, `run_optimization`, `run_batch`, and `export_schema`. Model helpers can load parameter sets, scenarios, and runtime export manifests without hand-writing project-relative JSON path plumbing. These helpers still shell out to `bcs-runner`; the SDK is not a second execution engine.

## Public IO Is The Contract

The public input/output schema is the shared contract across Studio, CLI, SDK, validation, calibration, optimization, and runtime-only delivery.
