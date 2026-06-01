# Create Component

Use components when you need to add a new calculation unit to a project.

## Current Studio Behavior

Workspace projects can create a scalar Python component template. Studio writes:

```text
graph.json
components/<component_id>.py
```

New components are not silently added to the runnable system. Adding a component to a system is an explicit action.

Workspace component input and output nodes can be added from the Inspector. If the component is already in the runnable system, new input nodes are exposed as public inputs and added to the default input file; new output nodes are exposed as public outputs.

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
