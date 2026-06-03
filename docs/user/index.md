# HVAC Studio User Guide

HVAC Studio is a Python-first component-node-system authoring and runtime tool for building-system modeling and control research.

It is not a fixed HVAC component library. Users define components, nodes, parameters, public inputs, public outputs, and Python calculation logic. The same project can then be validated, run, inspected, exported, and later reused from the GUI, CLI, Python SDK, and runtime-only packages.

## Typical Workflow

1. Create or open a project.
2. Define components and their nodes.
3. Define parameters and state.
4. Edit the Python component function body.
5. Compose a system from components.
6. Map public inputs and outputs.
7. Validate the graph.
8. Run one case.
9. Save scenarios and run records.
10. Import datasets for validation.
11. Calibrate parameters.
12. Optimize controls or design variables.
13. Export schemas or runtime packages.

## How To Read This Guide

- Start with [Quick Start](quick-start.md) if you want to run a model immediately.
- Read [Core Concepts](core-concepts.md) before creating custom components.
- Read [How It Works](how-it-works.md) to understand why project files, the runner, and Python worker exist.
- Read [Artifact Compatibility](artifact-compatibility.md) before depending on saved project files across releases.
- Use [CLI Runner](cli-runner.md) and [Export Runtime](export-runtime.md) when integrating with external tools.

## Shared Execution Model

Studio, CLI, SDK, and runtime packages use the same source-of-truth project files and runner. A model should not behave differently because it was launched from a different surface.
