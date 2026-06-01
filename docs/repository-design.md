# Repository Design

## Design Center

The repository starts with the runtime core, because every future surface must call the same engine:

```text
Studio GUI
Python SDK
External engine
        -> bcs-runner
        -> Go runtime/compiler
        -> Python worker
        -> user-defined Python components
```

The GUI is intentionally delayed. It should edit and visualize source-of-truth files, not become a separate modeling engine.

The UX-driven milestone plan lives in `docs/development-plan.md`. That plan adds the component-node-system authoring flow, component-aware Python editor, datasets, validation, calibration, optimization, SDK, and runtime-only delivery sequence on top of this runtime-first architecture.

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
- Loops require explicit solver boundaries in later versions.

## Repository Layers

```text
tools/go/internal/project       project and graph loading
tools/go/internal/graph         graph indexing helpers
tools/go/internal/compiler      validation and execution plan compilation
tools/go/internal/pythonworker  JSONL stdio worker client
tools/go/internal/runtime       run orchestration
python/bcs_worker               user component host process
python/bcs_sdk                  runner wrapper for research workflows
schema                          JSON Schema contracts
examples                        regression assets
```
