# Agent Working Memory

Last updated: 2026-06-02

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
  - `tools/go/cmd/studio` hosts the current Wails desktop entrypoint; `app/studio` can remain future React/product notes if needed.

## Active Design Decisions

- Public system input/output mappings should be explicit objects with `id`, `component`, and `node`. The design script shows simple string examples, but the runtime needs explicit endpoint mapping to avoid guessing.
- The initial runner supports feed-forward acyclic systems. Cycles should produce a clear algebraic-loop validation error.
- The Python worker uses JSONL over stdio for the MVP.
- During development, the Go runner finds `python/bcs_worker` from the repo tree and adds it plus the project root to `PYTHONPATH`.
- Python user components may use arbitrary internal logic, but inputs/outputs/states must be JSON-serializable across the worker boundary.
- Python component returned output keys must match declared component output nodes exactly; missing and undeclared output keys are runtime contract errors.
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
- Newly created projects should come from real source templates under `templates/projects/`, not hardcoded GUI/server mock data. The scalar project template is the first canonical template.
- When a component is explicitly added to a system, the Studio should keep the runnable path intact by creating public IO mappings and extending `default_input` for new required public inputs.
- When a node-to-node connection targets a previously public input, that input should stop being public and be removed from default input artifacts so the graph remains valid.
- Saved runs are project artifacts. The GUI should be able to reopen `runs/run-*.json` records and use them for Results and Inspector state, not just show the latest transient response.
- Export profiles should write concrete project artifacts under `exports/` before becoming full package builders. The first connected profile is `exports/runtime_package/`.
- Runtime export should include a copied `exports/runtime_package/project/` source-of-truth project artifact, not only a manifest, before growing into a full runner/Python package builder.
- Runtime export should carry `schema/public-io.json` beside the project artifact so external users can see the delivered input/output contract without opening Studio.
- Validation problems should carry structured metadata where possible. Even simple inferred `component_id` links are useful because they keep the Problems panel connected to the graph authoring surface.
- Problems panel rows should navigate to the most specific authoring surface available: source line for Python problems, otherwise the selected component on the system canvas.
- Python source editing must respect ownership: examples are read-only, workspace project component files can be saved, and the graph contract remains in `graph.json`.
- Direct Python authoring belongs in a first-class Code workspace with component contract context, source drafts, explicit save/revert/check actions, and snippets; source files remain the source of truth.
- Scenarios should start as explicit project artifacts under `scenarios/`, created from current run inputs/context, before adding richer dataset and validation workflows.
- Saved scenarios should be reusable immediately in Studio by reopening them into the Run Inputs panel.
- Studio execution actions should flush workspace model edits that affect runtime behavior, especially component parameters and Python source, before invoking run/export paths.
- Removing a Studio connection should restore the target input as editable public IO and reinsert its default input value when no other incoming connection owns that target.
- Deleting a component node must clean related public IO, default inputs, and connections; if deleting an output removes an upstream connection, restore the still-existing target input as public IO.
- Parameter Manager should let workspace users create Python-friendly parameter keys, not only edit template-created keys, so source edits and graph parameters can evolve together.
- Parameter deletion is a graph edit only; it should preserve other pending parameter edits before removing the selected key and still reject bundled examples.
- Examples should remain read-only, but Studio needs a first-class copy-to-workspace path so users can turn an example into an editable project without manually duplicating files.
- Removing a component from a runnable system should not delete its source artifact; it should clean system membership, touching connections, public IO, and default inputs so the graph remains valid and reversible.
- Deleting a component artifact is allowed only after it is out of every system and has no connection references; then graph entry and unshared source file are removed together.
- Duplicating a component should copy its graph contract, parameters, and Python source as a new unused component; system assembly remains explicit through `Use`.
- Source checks should catch obvious authoring errors before execution: expected class name, `evaluate`/`initialize` signatures, return-shape hints, and Python syntax when a runtime is available.
- Studio project validation should combine graph compilation with Python source contract checks for all `user_python` components, so Validate reflects the actual component-node-system authoring contract.
- Batch execution should start with saved scenarios and write explicit `runs/batch-*.json` artifacts before adding dataset-scale orchestration.
- Component management should separate stable component IDs/classes from editable display labels until full refactoring/rename support exists.
- Portable smoke coverage should keep exercising the connected Studio workflow, not just server startup. The current smoke path covers project copy/creation, node creation/deletion, source editing/checking, component creation/duplication/inclusion/removal/deletion, connection creation/removal, parameter creation/deletion, input/scenario/batch/run/export artifacts, and bundled Python execution.
- Release packaging must work from untagged development checkouts by falling back to a dev version with the current short SHA.
- The portable Studio package should have a user-facing root `HVAC Studio.exe` that opens a native Wails desktop window without launching a browser or binding a normal-use TCP port; `bin/studio.exe --server` remains the automation/server-only entrypoint.
- Wails desktop binaries must be built with the Wails production tags (`-tags desktop,production`); plain `go build` can produce a runtime error dialog even when compilation succeeds.
- User-facing package tools should not be placeholders. `bcs-env.exe check` is now the package self-diagnostic for Python runtime, worker, SDK, schema, examples, and entrypoints, and release smoke tests should keep using it.
- Studio static frontend should stay modular under `tools/go/internal/studio/static/js/`: shared state/API/DOM/format helpers and focused workspace renderers should be extracted instead of growing one monolithic `app.js`.
- Studio UI should show only implemented workflow surfaces during development. Future dataset/validation/calibration/optimization areas belong in the plan/docs until runtime-backed artifacts and actions exist, so the running app stays understandable and honest.
- Workspace detail views should render real artifact state, not only raw JSON. Keep JSON panes for inspection, but pair them with concise tables for records, exported files, paths, and statuses.
- User documentation is part of the product. Keep Markdown source under `docs/user/`, explain both user workflows and the internal execution model users need to reason correctly, and plan for MkDocs HTML, in-app help, PDF manual, and release assets.

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
- Are unimplemented future UI surfaces hidden until they have source-of-truth files and runtime-backed actions?
- Do GUI edit APIs reject examples/templates unless the user has created or copied them into `projects/`?
- Are user guide pages explaining source-of-truth files, runner behavior, public IO, and Python worker boundaries clearly enough for model authors?
