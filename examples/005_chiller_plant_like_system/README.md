# 005 Chiller Plant-Like System

This example is a compact plant-loop-like system with a load model, reset controller, chiller, pump, tower, and plant power aggregator.

It is intentionally small enough to inspect in Studio while still showing the main project artifacts:

- `graph.json`: components, public IO, and node-to-node connections
- `datasets/plant_validation.csv`: measured/reference columns for validation workflows
- `validation/mappings/plant_validation.json`: column mapping from dataset columns to public inputs/outputs
- `parameter_sets/*.json`: baseline and high-efficiency parameter sets
- `inputs/case01.json`: a runnable one-case input

Run it with:

```powershell
go run ./tools/go/cmd/bcs-runner validate --project examples/005_chiller_plant_like_system/project.bcsproj
go run ./tools/go/cmd/bcs-runner run --project examples/005_chiller_plant_like_system/project.bcsproj --input examples/005_chiller_plant_like_system/inputs/case01.json
go run ./tools/go/cmd/bcs-runner validate-data --project examples/005_chiller_plant_like_system/project.bcsproj --mapping validation/mappings/plant_validation.json
```
