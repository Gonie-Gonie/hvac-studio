# ADR 0001: File-Based Source Of Truth

## Status

Accepted

## Context

The design script states that neither the GUI nor Python object instances should be the source of truth. The product must support GUI authoring, CLI execution, SDK calls, batch simulations, optimization loops, and runtime-only delivery from the same model.

## Decision

Use `project.bcsproj`, `graph.json`, component source files, schema files, and environment lock files as the source of truth.

## Consequences

- The GUI edits files and asks the runner to validate/run them.
- Python SDK calls the runner instead of reimplementing simulation.
- Execution order is compiled from the graph, not inferred from object references.
- Future export packages can carry the same source-of-truth files.

