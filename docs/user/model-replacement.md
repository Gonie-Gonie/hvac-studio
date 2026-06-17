# Model Replacement

Model replacement creates a new component from a selected template while keeping
the original component artifact intact.

## Studio Workflow

1. Open a workspace project.
2. Select the component to replace.
3. Choose the replacement template in the Project panel.
4. Review `Replacement Preview` in the Inspector.
5. Leave `Copy same-name parameters` enabled when the replacement should inherit
   matching parameter values from the original component.
6. Select `Replace And Validate`.
7. Edit the new component source in the Code workspace if needed.
8. Run the model again and use the Run Output comparison table to compare
   before/after public outputs.

The preview shows the old node to new node mapping table for public inputs,
public outputs, and connection endpoints that reference the selected component.
It also shows matching, missing, and new input/output/parameter IDs before any
files are written.

If the original component is used by the entry system, Studio only rewires the
system when every referenced public input, public output, and connection endpoint
has the same node ID and direction on the replacement component. If a referenced
node is missing, the replacement is rejected and the original system remains
unchanged. Broken mappings are shown in the Inspector preview before replacement
and returned as Problems if the API rejects the request.

The original component and source files remain in the project. The replacement
gets its own component ID, source folder, wrapper, user step body, and metadata.
This makes replacement a reversible modeling workflow instead of a silent
overwrite.

## Example: ZoneLoadRC To ZoneLoadANN

Use `examples/015_rc_ahu_ann_composition` as the source project and copy it into
the workspace before editing. Select `RC Zone Load`, choose `Zone Load ANN
Surrogate` in the Project panel, verify that the preview reports no missing
inputs or outputs, then select `Replace And Validate`.

The template preserves the `ZoneLoadRC` public contract:

```text
inputs: outdoor_temperature_c, solar_gain_kw, internal_gain_kw, zone_setpoint_c
outputs: zone_load_kw, zone_temperature_c, zone_setpoint_c, outdoor_temperature_c
```

After replacement, `zone_load_ann` becomes the entry-system component reference,
public inputs and outputs stay attached, compatible connections are rewired, and
same-name RC parameters such as `ua_kw_per_k` are copied when the mapping option
is enabled.

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
