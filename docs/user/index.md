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

Use the guide in this order:

1. [Quick Start](quick-start.md), [Screenshot Tutorials](tutorials.md), and [Examples](examples.md) for the first successful run.
2. [Core Concepts](core-concepts.md), [Concept Map](concept-map.md), and [How It Works](how-it-works.md) before changing a model contract.
3. [Create Component](create-component.md), [Edit Python Function](edit-python-function.md), [Build System](build-system.md), and [Parameter Management](parameter-management.md) while authoring.
4. [Run Simulation](run-simulation.md), [Data Validation](data-validation.md), [Calibration](calibration.md), [Optimization](optimization.md), and [Export Runtime](export-runtime.md) for project workflows.
5. [Python SDK](python-sdk.md), [CLI Runner](cli-runner.md), and [External Engine Protocol](external-engine-protocol.md) for external integration.

Use [Model Replacement](model-replacement.md), [ML/ANN Component](ml-ann-component.md), [External Executables](external-executables.md), [Artifact Compatibility](artifact-compatibility.md), [Troubleshooting](troubleshooting.md), and [Glossary](glossary.md) as reference pages when those topics appear in a workflow.

## Shared Execution Model

Studio, CLI, SDK, and runtime packages use the same source-of-truth project files and runner. A model should not behave differently because it was launched from a different surface.
