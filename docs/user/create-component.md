# Create Component

Use components when you need to add a new calculation unit to a project.

## Current Studio Behavior

Workspace projects can create a Python component template from the Project panel by entering a component name, selecting a template, and pressing New Component. Use New ML Component as the direct path for an ML Inference component; it selects the ML template, creates the wrapper and model/schema assets, and records `ml_metadata` automatically. Studio reads the selected template manifest and Python source from `templates/components/<template>/`, then writes:

```text
graph.json
components/<component_id>/component.json
components/<component_id>/wrapper.py
components/<component_id>/user_init.py
components/<component_id>/user_step.py
components/<component_id>/helpers.py
```

New components use the generated-wrapper layout by default. Studio and the runner own the wrapper contract, while the Code workspace opens `user_step.py` as the primary editable function body. Source save APIs write only that editable user step for generated-wrapper components, leaving `wrapper.py` and component metadata as Studio-managed runtime artifacts. Existing single-file components still load and can be edited for compatibility with older projects.

Available component templates include scalar, controller, stateful, data source, data sink, utility, feature mapper, ML inference, vectorized, external executable, and solver boundary. Vectorized components use `execution_mode: "vectorized"` and may implement `evaluate_batch(inputs, state, params, context)` with the same `(outputs, state)` return contract as `evaluate`. ML inference components use `ml_metadata` to record model assets, feature schemas, target schemas, validation reports, valid ranges, and package requirements. External executable components use `kind: "external_exe"` with `execution_mode: "external_executable"` and run the process named by `parameters.command` and `parameters.args`. Solver boundary components use `category: "solver"` and `solver_boundary` metadata to keep iterative feedback behavior inside one component.

The Project panel can include a newly created component in the entry system immediately when Use in System is checked. When it is unchecked, the component is created as a source artifact only. Components that are not currently in the entry system are marked as unused in the Project tree and can be added with Use.

Workspace component input and output nodes can be added, edited, or deleted from the Inspector. New nodes can start from presets for water inlet/outlet, air inlet/outlet, control signal input, electric power output, scalar input/output, and time-series input, then be adjusted with display names, media, value types, units, required flags, and input defaults. Node IDs remain stable after creation, but the Inspector can edit the same metadata later. If the component is already in the runnable system, new input nodes are exposed as public inputs and added to the default input file; new output nodes are exposed as public outputs. Editing node metadata updates related public IO, and editing an input default updates the default input file. Deleting a node removes related public IO and connection references.

Workspace component parameter definitions and state definitions can also be edited from the Inspector. Parameter definitions carry workflow-facing metadata such as role, bounds, units, defaults, groups, descriptions, and visibility. State definitions document state keys used by generated-wrapper bodies and Code workspace completions.

Workspace component display names can be edited from the Inspector. Component IDs and Python class paths remain stable because connections, run records, and source files reference those IDs.

The Inspector can open the selected workspace component in the Code workspace so its Python source can be edited without losing the current component selection.

Existing workspace components can be duplicated from the Project tree or the Inspector. The duplicate copies the graph contract, parameters, and Python source into a new component artifact, but it is not automatically added to the runnable system.

Workspace components can also be replaced from the Inspector. Replacement creates
a new component from the selected Project panel template, keeps the original
component source, and rewires the entry system only when public IO and connection
node IDs are contract-compatible. See [Model Replacement](model-replacement.md).

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
- optional ML metadata and model asset paths

Step components implement `evaluate(inputs, state, params, context)`.
Vectorized components implement `evaluate_batch(inputs, state, params, context)`
when one worker call should process array-shaped inputs and return
array-shaped outputs.
External executable components do not run the generated Python wrapper. They
exchange JSON with the configured command through stdin/stdout.
Solver boundary components still run as normal components, but their internal
function owns the iteration method and stopping criteria.

## Authoring Direction

Component contracts are open-ended. Use templates for a fast starting point, then define the inlet, outlet, signal, parameter, and state contract that matches the model you are building.
