# Create Component

Use components when you need to add a new calculation unit to a project.

## Current Studio Behavior

Workspace projects can create a scalar Python component template. Studio reads the template manifest and Python source from `templates/components/scalar/`, then writes:

```text
graph.json
components/<component_id>.py
```

New components are not silently added to the runnable system. Adding a component to a system is an explicit action.

Workspace component input and output nodes can be added or deleted from the Inspector. If the component is already in the runnable system, new input nodes are exposed as public inputs and added to the default input file; new output nodes are exposed as public outputs. Deleting a node removes related public IO and connection references.

Workspace component display names can be edited from the Inspector. Component IDs and Python class paths remain stable because connections, run records, and source files reference those IDs.

The Inspector can open the selected workspace component in the Code workspace so its Python source can be edited without losing the current component selection.

Existing workspace components can be duplicated from the Inspector. The duplicate copies the graph contract, parameters, and Python source into a new component artifact, but it is not automatically added to the runnable system.

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
- future state schema

## Authoring Direction

The long-term editor should let users define arbitrary inlet, outlet, and signal nodes. Built-in templates should help users start quickly, but they must not become the modeling boundary.
