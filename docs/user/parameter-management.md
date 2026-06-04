# Parameter Management

Parameters are model values used by component logic.

## Current Studio Behavior

Workspace component parameters can be added, edited, or deleted in the Parameter Manager and saved to `graph.json`. Component selectors show display names with stable IDs when they differ. Bundled examples are read-only through Studio write APIs.

The Inspector edits parameter definitions for the selected component. Definition fields include display name, current value, default, unit, role, bounds, group, description, and visibility. For generated-wrapper components, Studio also keeps `components/<component_id>/component.json` synchronized with the edited contract.

Parameter names should be Python-friendly identifiers because component source usually reads them through `params`.

## Parameter Roles

Component parameter definitions use roles so workflows can filter meaningful candidates:

- `fixed`
- `scenario_input`
- `calibration_target`
- `optimization_variable`
- `derived`

Visibility controls whether a parameter is meant to appear in Studio authoring surfaces. Hidden parameters remain part of the component contract and can still be used by source code or parameter sets.

## Parameter Sets

Parameter sets are saved JSON overlays under `parameter_sets/`. Applying a parameter set changes runtime values in memory for that run or validation job; it does not rewrite the baseline `graph.json`.

In Studio, selecting a parameter set in the Project tree or Run Inputs toolbar applies the same overlay to Run, Batch, and Data validation commands. Selecting `Baseline` clears the overlay.

Example names:

```text
default
calibrated_summer_2026
calibrated_winter_2026
optimization_result_001
```

Example shape:

```json
{
  "id": "high_efficiency",
  "components": {
    "chiller": {
      "cop": 6.8
    }
  }
}
```

Run with a parameter set:

```powershell
bcs-runner.exe run `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --input examples/005_chiller_plant_like_system/inputs/case01.json `
  --parameter-set parameter_sets/high_efficiency.json
```

Run, batch, and validation results record the parameter-set path when one is used.
