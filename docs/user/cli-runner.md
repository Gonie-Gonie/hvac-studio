# CLI Runner

The CLI runner uses the same runtime path as Studio.

## Validate

```powershell
bcs-runner.exe validate --project project.bcsproj
```

## Run

```powershell
bcs-runner.exe run `
  --project project.bcsproj `
  --input input.json `
  --output output.json
```

Run with a saved parameter set without changing `graph.json`:

```powershell
bcs-runner.exe run `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --input examples/005_chiller_plant_like_system/inputs/case01.json `
  --parameter-set parameter_sets/high_efficiency.json `
  --output output.json
```

## Schema

```powershell
bcs-runner.exe schema `
  --project project.bcsproj `
  --output schema.json
```

## Data Validation

`validate-data` runs a saved dataset mapping and writes metrics plus high-error rows:

```powershell
bcs-runner.exe validate-data `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --mapping validation/mappings/plant_validation.json `
  --parameter-set parameter_sets/high_efficiency.json `
  --output validation-result.json
```

The mapping and parameter-set paths are project-relative. The result includes RMSE, MAE, MBE, CVRMSE, R2, row summaries, and inspect data for the highest-error rows.

## Serve

Serve mode keeps the graph compiled and Python components loaded for repeated evaluations:

```powershell
@'
{"id":"case-1","inputs":{"value":4},"context":{"time":0,"dt":60}}
{"id":"case-2","inputs":{"value":5},"context":{"time":60,"dt":60}}
{"id":"stop","type":"shutdown"}
'@ | bcs-runner.exe serve --project project.bcsproj
```

Each input line is a JSON request. Each output line is a JSON response with `ok`, `id`, and either `result` or `error`. Component state is preserved inside the live serve session until shutdown.

## Exit Codes

```text
0 = success
1 = validation error
2 = runtime error
3 = input schema/error
4 = Python worker error
5 = license/runtime error
```

## Principle

If GUI and CLI use the same project, parameter set, and input, their results should match.
