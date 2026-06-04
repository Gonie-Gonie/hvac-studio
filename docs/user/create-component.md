# Create Component

Use components when you need to add a new calculation unit to a project.

## Current Studio Behavior

Workspace projects can create a Python component template from the Project panel by entering a component name, selecting a template, and pressing Add. Studio reads the selected template manifest and Python source from `templates/components/<template>/`, then writes:

```text
graph.json
components/<component_id>/component.json
components/<component_id>/wrapper.py
components/<component_id>/user_init.py
components/<component_id>/user_step.py
components/<component_id>/helpers.py
```

New components use the generated-wrapper layout by default. Studio and the runner own the wrapper contract, while the Code workspace opens `user_step.py` as the primary editable function body. Existing single-file components still load and can be edited for compatibility with older projects.

Available component templates include scalar, controller, stateful, data source, data sink, utility, vectorized, and external executable placeholder. Vectorized components use `execution_mode: "vectorized"` and may implement `evaluate_batch(inputs, state, params, context)` with the same `(outputs, state)` return contract as `evaluate`. The external executable template remains an authoring placeholder until that post-1.0 execution mode is implemented.

New components are not silently added to the runnable system. Adding a component to a system is an explicit action. Components that are not currently in the entry system are marked as unused in the Project tree and can be added with Use.

Workspace component input and output nodes can be added, edited, or deleted from the Inspector. New nodes can be created with display names, media, value types, units, required flags, and input defaults. Node IDs remain stable after creation, but the Inspector can edit the same metadata later. If the component is already in the runnable system, new input nodes are exposed as public inputs and added to the default input file; new output nodes are exposed as public outputs. Editing node metadata updates related public IO, and editing an input default updates the default input file. Deleting a node removes related public IO and connection references.

Workspace component parameter definitions and state definitions can also be edited from the Inspector. Parameter definitions carry workflow-facing metadata such as role, bounds, units, defaults, groups, descriptions, and visibility. State definitions document state keys used by generated-wrapper bodies and Code workspace completions.

Workspace component display names can be edited from the Inspector. Component IDs and Python class paths remain stable because connections, run records, and source files reference those IDs.

The Inspector can open the selected workspace component in the Code workspace so its Python source can be edited without losing the current component selection.

Existing workspace components can be duplicated from the Project tree or the Inspector. The duplicate copies the graph contract, parameters, and Python source into a new component artifact, but it is not automatically added to the runnable system.

Removing a component from a system cleans system membership, related connections, public IO, and default input entries, while keeping the component source artifact in the project. Deleting a component removes its graph entry and source file only after it is no longer used by a system.

## Component Contract

A component contract includes:

- component ID
- display name
- kind
- Python class
- input nodes
- output nodes
- parameters
- parameter definitions
- state definitions
- source layout metadata

Step components implement `evaluate(inputs, state, params, context)`.
Vectorized components implement `evaluate_batch(inputs, state, params, context)`
when one worker call should process array-shaped inputs and return
array-shaped outputs.

## Authoring Direction

The editor should let users define arbitrary inlet, outlet, and signal nodes. Built-in templates should help users start quickly, but they must not become the modeling boundary.
