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
5. Start the Python worker when the project contains user Python components.
6. Initialize components.
7. Evaluate components in order, including vectorized calls, external executable calls, and nested composite system calls when a component declares those modes.
8. Collect public outputs.
9. Return or save structured results.

## Python Worker Executes User Code

User Python runs in a worker process. This keeps the runtime boundary explicit and makes future repeated evaluation, optimization, and external engine integration more stable.

External executable components run as separate processes. The runner sends one
JSON request on stdin and reads one JSON response on stdout, while still owning
graph validation, execution order, state carryover, logs, and public outputs.

Solver boundary components are normal runner components with explicit
`solver_boundary` metadata. They can perform internal iterations, but the outer
project graph is still compiled and inspected as an acyclic graph.

## SDK Wraps Serve Mode

The Python SDK is a client for `bcs-runner serve`. `RunnerClient.start(...)` keeps the runner process alive and sends repeated JSON requests to the same compiled project/session:

```python
from bcs_sdk import RunnerClient

with RunnerClient.start("project.bcsproj", runner="bcs-runner.exe") as client:
    result = client.evaluate({"value": 4}, {"time": 0, "dt": 60})
```

Async scripts can call `await client.evaluate_async(...)`; the request still
goes through the same `RunnerClient` and runner process. For independent
high-volume cases, use `RunnerPool` to keep a bounded number of serve sessions
alive:

```python
from bcs_sdk import RunnerPool

cases = [{"inputs": {"value": value}} for value in [1, 2, 3]]

with RunnerPool.start("project.bcsproj", runner="bcs-runner.exe", workers=2, request_timeout=30) as pool:
    results = pool.evaluate_many(cases)
```

Each pool worker owns one runner session and sends requests serially to that
session. Use `run-series` for sequential stateful timestep runs; pooled
evaluation is intended for independent candidate evaluations.

For file-backed workflows, `RunnerClient` also provides helpers such as `run_validation`, `run_calibration`, `run_optimization`, `run_batch`, and `export_schema`. Model helpers can load parameter sets, scenarios, and runtime export manifests without hand-writing project-relative JSON path plumbing. These helpers still shell out to `bcs-runner`; the SDK is not a second execution engine.

## Public IO Is The Contract

The public input/output schema is the shared contract across Studio, CLI, SDK, validation, calibration, optimization, and runtime-only delivery.
