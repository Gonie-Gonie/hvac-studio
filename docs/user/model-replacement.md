# Model Replacement

Model replacement creates a new component from a selected template while keeping
the original component artifact intact.

## Studio Workflow

1. Open a workspace project.
2. Select the component to replace.
3. Choose the replacement template in the Project panel.
4. Select `Replace` in the Inspector.
5. Edit the new component source in the Code workspace.
6. Run validation or simulation again.

If the original component is used by the entry system, Studio only rewires the
system when every referenced public input, public output, and connection endpoint
has the same node ID and direction on the replacement component. If a referenced
node is missing, the replacement is rejected and the original system remains
unchanged.

The original component and source files remain in the project. The replacement
gets its own component ID, source folder, wrapper, user step body, and metadata.
This makes replacement a reversible modeling workflow instead of a silent
overwrite.

## CLI And Export Behavior

Replacement writes normal project artifacts:

```text
graph.json
components/<replacement_id>/component.json
components/<replacement_id>/wrapper.py
components/<replacement_id>/user_step.py
```

After replacement, `bcs-runner validate`, `run`, `validate-data`, calibration,
optimization, and runtime export use the new entry-system component references.
The retained original component is still available for inspection or manual
reuse, but it is no longer part of the entry system when the replacement was
successfully rewired.
