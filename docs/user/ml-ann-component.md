# ML/ANN Component

ML-backed components are normal runner components with additional metadata for
model files and feature contracts. The initial supported path is a Python
generated-wrapper component that loads project-owned assets during
`initialize(...)` and uses the cached state during `step(...)`.

## Component Metadata

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
    "valid_time_resolution": "step"
  }
}
```

Asset paths must be project-relative and stay inside the project root. The
runner validates that referenced files exist before running or exporting.

## Feature Mapping

Use a Feature Mapper component when raw system variables need a stable feature
object. The mapper should preserve a deterministic feature order and should
convert or clip values before the ML component receives them.

`examples/014_ahu_state_ann` uses this shape:

```text
public inputs -> FeatureMapper.features -> AHUStateANN.features -> public outputs
```

## Export Behavior

Runtime export copies the `assets/` directory, lists ML asset paths in
`manifest.json`, and records SHA-256 checksums for exported files. Schema export
also lists the ML asset requirements so external tools can prepare the same
files before running the model.
