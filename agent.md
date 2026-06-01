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
- The Windows portable and runtime MVP zips should include `runtime/python` copied from repo-local setup. Included examples must run without system Python on `PATH`.
- Project-specific third-party package locking/freezing remains a later environment-management milestone.
- The first user-facing app release should be a Windows 10/11 x64 portable Studio zip. Installer packaging comes after portable behavior is reproducible.
- macOS is a future experimental release target after MVP. Keep engine, project files, graph schema, and component schema OS-independent, but do not let macOS packaging slow the Windows MVP.
- OS-specific path, process, runtime, executable naming, installer, signing, and packaging logic should be isolated behind platform/release boundaries.
- The UX development plan is tracked in `docs/development-plan.md`. It folds in the Component-Node-System UX flow: project creation, component/node/parameter/state authoring, protected Python function-body editing, system canvas, validation, run/debug, datasets, validation, calibration, optimization, SDK, and runtime-only delivery.
- GUI component editing should eventually show a generated scaffold but persist contract metadata separately from user-editable function bodies, e.g. `component.json`, `user_init.py`, `user_step.py`, and helpers.
- Dataset, parameter set, scenario, run record, validation, calibration, and optimization artifacts must become source-of-truth project objects rather than transient GUI state.
- Studio GUI work should start from the full product workspace shape, then connect behavior incrementally. Avoid ambiguous tiny demo screens that obscure the intended workflow.
- Studio-created projects should live under `projects/` in the portable package. Workspace runs should be persisted as `runs/run-*.json` records inside the project.
- GUI edits should persist to source-of-truth artifacts immediately and explicitly. Current write scope starts with workspace-only component parameters saved to `graph.json`; bundled examples remain read-only through the Studio API.
- Studio run input fields should come from the project's `default_input` file when available, not from hardcoded sample values. Saving run inputs writes back to that source artifact for workspace projects.
- Newly created components should first be persisted as source artifacts (`graph.json` plus `components/<id>.py`) without silently changing system execution. System membership, connections, and public IO should be explicit authoring actions.
- When a component is explicitly added to a system, the Studio should keep the runnable path intact by creating public IO mappings and extending `default_input` for new required public inputs.
- Saved runs are project artifacts. The GUI should be able to reopen `runs/run-*.json` records and use them for Results and Inspector state, not just show the latest transient response.
- Export profiles should write concrete project artifacts under `exports/` before becoming full package builders. The first connected profile is `exports/runtime_package/manifest.json`.
- Validation problems should carry structured metadata where possible. Even simple inferred `component_id` links are useful because they keep the Problems panel connected to the graph authoring surface.
- Python source editing must respect ownership: examples are read-only, workspace project component files can be saved, and the graph contract remains in `graph.json`.
- Scenarios should start as explicit project artifacts under `scenarios/`, created from current run inputs/context, before adding richer dataset and validation workflows.
- Portable smoke coverage should keep exercising the connected Studio workflow, not just server startup. The current smoke path covers project creation, source editing, component creation/inclusion, parameter/input/scenario/run/export artifacts, and bundled Python execution.

## Monitoring Checklist

- Are we accidentally turning the GUI into the model source of truth?
- Are we adding built-in HVAC models before the runner/worker contract is stable?
- Are we relying on global Python instances, callbacks, or side effects for graph execution?
- Are error messages actionable for researchers connecting component nodes?
- Can examples be run as regression tests?
- Can a future optimization loop keep the runner alive instead of starting Python repeatedly?
- Are setup scripts keeping tool caches inside the repo-local ignored directories rather than leaking assumptions into the user's global environment?
- After a coherent unit is tested, did we commit and push before starting the next unit?
- Does every release package get smoke-tested after expansion, not just built?
- Do release smoke tests constrain `PATH` so they prove bundled Python is being used?
- Are Windows portable, runtime-only, and future installer packages clearly separated?
- Are we accidentally hardcoding Windows-specific behavior into engine/compiler/runtime packages instead of release/platform code?
- Are UX features being staged so runtime support and golden examples exist before GUI polish?
- Does the GUI shell preserve the complete Studio workflow even while individual features are still being connected?
- Do GUI edit APIs reject examples/templates unless the user has created or copied them into `projects/`?
