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
scenarios/
runs/
exports/
```

- `project.bcsproj` stores project metadata, entry system, default input, and environment settings.
- `graph.json` stores systems, components, nodes, connections, public inputs, public outputs, and parameters.
- `components/` stores user Python source.
- `inputs/` stores default run input files.
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

## Public IO Is The Contract

The public input/output schema is the shared contract across Studio, CLI, SDK, validation, calibration, optimization, and runtime-only delivery.

