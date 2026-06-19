# ML/ANN Component

ML-backed components are normal runner components with additional metadata for
model files and feature contracts. The initial supported path is a Python
generated-wrapper component that loads project-owned assets during
`initialize(...)` and uses the cached state during `step(...)`.
The generated helper pipeline separates feature extraction, preprocessing,
inference, postprocessing, and output return so the default component can be
adapted without changing the wrapper contract.

## Component Metadata

In Studio, start from a workspace project and use the Project panel. Press
**New ML** to create an ML Inference component directly, or choose
**ML Inference Component** in the template selector and press **Add**. Studio
creates the generated wrapper plus `model.json`, `feature_schema.json`,
`target_schema.json`, and `validation_report.json` under the component folder,
then records those project-relative paths in `ml_metadata`.

After selecting the component, use the Inspector's **ML Assets** block to import
or replace the model file, optional input/output scaler files, feature schema,
target schema, training metadata, validation report, required packages, model
format, valid time resolution, and valid input ranges. Imported files are
written under the component folder and the graph plus component metadata file
are kept in sync.
Use **Apply Schema Nodes** after importing schema files to ensure the component
has the `features` object input and target-based output nodes. For components
already included in a system, added nodes also update related public IO.
When a validation report is present, the Inspector shows dataset, periods,
feature schema version, model checksum, time resolution, and metric values.

Use `ml_metadata` on the component entry in `graph.json` or the component
metadata file:

```json
{
  "ml_metadata": {
    "model_format": "custom",
    "model_file": "assets/ahu_state_ann/model.json",
    "feature_schema_file": "assets/ahu_state_ann/feature_schema.json",
    "target_schema_file": "assets/ahu_state_ann/target_schema.json",
    "validation_report_file": "assets/ahu_state_ann/validation_report.json",
    "required_packages": [],
    "valid_time_resolution": "step",
    "valid_input_ranges": {
      "outdoor_temperature_c": {
        "min": -20,
        "max": 45
      }
    }
  }
}
```

Asset paths must be project-relative and stay inside the project root. Graph
validation rejects absolute paths or paths that escape the project before run or
export. Missing files surface when the component loads assets or when export
copies and records the selected ML assets.

## Feature Mapping

Use a Feature Mapper component when raw system variables need a stable feature
object. The mapper should preserve a deterministic feature order and should
convert or clip values before the ML component receives them. The template
accepts an optional `feature_config` parameter where each feature can set a
different `source`, `scale`, `offset`, `min`, and `max`. Missing source values
raise `missing feature input: <name>` so Studio and CLI runs report the same
component error. After a run, selecting the Feature Mapper shows a Feature
Preview table; selecting the ML component shows the received feature object.
When an ML component has an unconnected `features` input and a Feature Mapper
with a `features` output is already in the entry system, Studio shows a
Feature Mapping Suggestion action that creates the connection.

`examples/014_ahu_state_ann` uses this shape:

```text
public inputs -> FeatureMapper.features -> AHUStateANN.features -> public outputs
```

## Export Behavior

Runtime export copies the `assets/` directory, lists ML asset paths in
`manifest.json`, records SHA-256 checksums for exported files, and includes
`ml_validation_reports` summaries for component-level validation metadata.
Schema export also lists the ML asset requirements so external tools can
prepare the same files before running the model.
