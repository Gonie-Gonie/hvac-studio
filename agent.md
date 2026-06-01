# Agent Working Memory

Last updated: 2026-06-01

## North Star

This repository is for a Component-Node System Studio: an installable authoring/runtime system for building-system researchers who define equipment and control models in Python, then connect, validate, run, optimize, and deliver those models through a stable component-node-system runtime.

The core is not an HVAC component library. The core is preserving user-defined Python modeling freedom while making connections, schemas, execution order, runtime environment, and delivery reproducible.

## Product Principles To Keep Checking

- User-defined component first, not component library first.
- `project.bcsproj`, `graph.json`, component source files, schema files, and environment locks are the source of truth.
- GUI is an authoring/viewing layer, not the source of truth.
- Python object references and setter side effects must not define execution order.
- User Python code is not translated to Go. The runtime manages the boundary and calls `initialize` / `evaluate`.
- A component owns calculation logic; nodes are connection points. Avoid wording like "Chiller node".
- Node schema is the component's external contract and should allow arbitrary inlet/outlet counts.
- The same model must run through GUI, CLI, SDK, batch, and serve paths using the same runner.
- Algebraic loops are detected in the MVP. They are not implicitly solved by recursive callbacks.
- Built-in components are convenience/templates, not the modeling boundary.

## Current Repository Direction

- Start with MVP 1 runtime core:
  - `project.bcsproj`
  - `graph.json`
  - user-defined Python component contract
  - Python worker
  - `bcs-runner validate`
  - `bcs-runner run`
  - JSON input/output
  - golden example assets
- Keep the monorepo shape from the design script:
  - `tools/go` for runner/runtime/compiler packages.
  - `python/bcs_worker` for the persistent Python component evaluator.
  - `python/bcs_sdk` for research/optimization wrappers around the runner.
  - `schema` for source-of-truth JSON schemas.
  - `examples` as regression assets, not demos only.
  - `app/studio` reserved for the later Wails/React GUI.

## Active Design Decisions

- Public system input/output mappings should be explicit objects with `id`, `component`, and `node`. The design script shows simple string examples, but the runtime needs explicit endpoint mapping to avoid guessing.
- The initial runner supports feed-forward acyclic systems. Cycles should produce a clear algebraic-loop validation error.
- The Python worker uses JSONL over stdio for the MVP.
- During development, the Go runner finds `python/bcs_worker` from the repo tree and adds it plus the project root to `PYTHONPATH`.
- Python user components may use arbitrary internal logic, but inputs/outputs/states must be JSON-serializable across the worker boundary.
- Fresh clones should be bootstrappable into a repo-local development environment. `scripts/dev/setup.ps1` installs Go, uv, uv-managed Python, and `.venv` inside the clone so normal development does not depend on user-profile toolchains.
- Dev/test/build scripts should load `scripts/dev/env.ps1` and prefer `.repo_tools` / `.venv` before falling back to system tools.
- Work should be committed and pushed at sensible milestones, especially after a test pass. Treat "test green -> commit -> push" as an operating rule unless the user says to hold changes locally.

## Monitoring Checklist

- Are we accidentally turning the GUI into the model source of truth?
- Are we adding built-in HVAC models before the runner/worker contract is stable?
- Are we relying on global Python instances, callbacks, or side effects for graph execution?
- Are error messages actionable for researchers connecting component nodes?
- Can examples be run as regression tests?
- Can a future optimization loop keep the runner alive instead of starting Python repeatedly?
- Are setup scripts keeping tool caches inside the repo-local ignored directories rather than leaking assumptions into the user's global environment?
- After a coherent unit is tested, did we commit and push before starting the next unit?
