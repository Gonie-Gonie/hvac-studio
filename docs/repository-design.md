# Repository Design

## Design Center

The repository is centered on the runtime core: every product surface calls the
same engine and persists the same source-of-truth files.

```text
Studio GUI
Python SDK
External engine
        -> bcs-runner
        -> Go runtime/compiler
        -> Python worker
        -> user-defined Python components
```

Studio is a product surface over the same project artifacts and runner APIs. It
edits and visualizes source-of-truth files; it does not become a separate
modeling engine.

The Studio implementation is a Go-hosted web workspace embedded in
`tools/go/cmd/studio`. Its panels are backed by the same validation, run,
dataset, calibration, optimization, source-check, and export paths used by CLI
and SDK workflows.

Distribution is Windows-first, but source-of-truth files and engine packages should stay OS-independent. OS-specific path, process, runtime, executable naming, installer, signing, and packaging behavior should be isolated instead of leaking through compiler/runtime code.

The UX-driven milestone plan lives in `planning/development-plan.md`. That plan adds the component-node-system authoring flow, component-aware Python editor, datasets, validation, calibration, optimization, SDK, and runtime-only delivery sequence on top of this runtime-first architecture.

## Source Of Truth

The model source of truth is file-based:

- `project.bcsproj`
- `graph.json`
- `components/*.py`
- `schema/*.json`
- environment lock files

The runner loads these files, validates the graph, compiles a feed-forward execution plan, and calls user Python components through the worker.

## MVP Runtime Flow

```text
project load
graph load
schema/structural validation
connection validation
algebraic loop detection
execution plan compile
Python worker start
component import
component initialize
component evaluate in topological order
result write
```

## Boundary Rules

- Component logic belongs to user Python.
- Node schema is the public contract.
- Connection validation belongs to the runtime.
- Execution order belongs to the graph compiler/scheduler.
- Feedback loops require explicit solver boundary components.

## Repository Layers

```text
tools/go/internal/project       project and graph loading
tools/go/internal/graph         graph indexing helpers
tools/go/internal/compiler      validation and execution plan compilation
tools/go/internal/pythonworker  JSONL stdio worker client
tools/go/internal/runtime       run orchestration
tools/go/internal/studio        local Studio web host and API
tools/go/internal/platform      OS-specific path/process/runtime boundary
python/bcs_worker               user component host process
python/bcs_sdk                  runner wrapper for research workflows
schema                          JSON Schema contracts
examples                        regression assets
```
