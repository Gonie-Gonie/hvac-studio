# ADR 0002: Runner Before GUI

## Status

Accepted

## Context

The design script prioritizes stabilizing `bcs-runner`, `project.bcsproj`, `graph.json`, user-defined Python components, the Python worker, and golden examples before the Studio GUI.

## Decision

Build the repository around the runner/worker contract first. GUI scaffolding can exist, but it must not drive model semantics.

## Consequences

- `bcs-runner validate` and `bcs-runner run` are the first executable product slice.
- Examples become regression tests early.
- GUI, SDK, batch, serve, and export can reuse one runtime path.

