# Parameter Management

Parameters are model values used by component logic.

## Current Studio Behavior

Workspace component parameters can be added, edited, or deleted in the Parameter Manager and saved to `graph.json`. Component selectors show display names with stable IDs when they differ. Bundled examples are read-only through Studio write APIs.

Parameter names should be Python-friendly identifiers because component source usually reads them through `params`.

## Parameter Roles

Future workflows should distinguish:

- fixed parameter
- scenario input
- calibration target
- optimization variable
- derived parameter

## Parameter Sets

Calibration and optimization should not overwrite baseline values by default. Results should be saved as named parameter sets.

Example names:

```text
default
calibrated_summer_2026
calibrated_winter_2026
optimization_result_001
```
