# 005 Chiller Plant-Like System

This example is a compact plant-loop-like system with a load model, reset controller, chiller, pump, tower, and plant power aggregator.

It is intentionally small enough to inspect in Studio while still showing the main project artifacts:

- `graph.json`: components, public IO, and node-to-node connections
- `datasets/plant_validation.csv`: measured/reference columns for validation workflows
- `validation/mappings/plant_validation.json`: column mapping from dataset columns to public inputs/outputs
- `calibration/setups/chiller_cop_grid.json`: grid-search calibration setup
- `parameter_sets/*.json`: baseline and high-efficiency parameter sets
- `inputs/case01.json`: a runnable one-case input
- `docs/time-series-validation.md`: current CSV time-column validation pattern

Run it with:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate `
  --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj
go run .\cmd\bcs-runner run `
  --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj `
  --input ..\..\examples\005_chiller_plant_like_system\inputs\case01.json
go run .\cmd\bcs-runner validate-data `
  --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj `
  --mapping validation\mappings\plant_validation.json `
  --output ..\..\artifacts\plant-validation.json
go run .\cmd\bcs-runner calibrate `
  --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj `
  --setup calibration\setups\chiller_cop_grid.json `
  --output ..\..\artifacts\plant-calibration.json
Pop-Location
```

## Learning Notes

The validation dataset includes a `time` column. The mapping preserves that value
in validation row summaries, which makes this example the current tutorial for
time-indexed validation data. Each CSV row is still evaluated independently;
native sequential time-series state carryover is planned as a later engine
feature.

Use the high-efficiency parameter set to compare runtime overlays without
mutating `graph.json`, then use the calibration setup to estimate chiller-related
parameters from the same validation dataset.
