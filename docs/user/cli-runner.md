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

Vectorized components use the same `run` command. The runner dispatches
components with `execution_mode: "vectorized"` through the worker's
`evaluate_batch` contract:

```powershell
bcs-runner.exe run `
  --project examples/009_vectorized_component/project.bcsproj `
  --input examples/009_vectorized_component/inputs/case01.json `
  --output vectorized-output.json
```

External executable components also use `run`. The runner sends a JSON request
to the configured command on stdin and reads one JSON response from stdout:

```powershell
bcs-runner.exe run `
  --project examples/010_external_executable_component/project.bcsproj `
  --input examples/010_external_executable_component/inputs/case01.json `
  --output external-output.json
```

## Run Time Series

`run-series` evaluates ordered steps inside one runtime session, so component
state carries from one step to the next. Put shared context such as `dt` at the
top level and per-step context such as `time` inside each step:

```powershell
bcs-runner.exe run-series `
  --project examples/004_stateful_controller/project.bcsproj `
  --input examples/004_stateful_controller/inputs/series01.json `
  --output series-output.json
```

The result contains `series[]` step records, `final_states`, and `outputs`
aggregated as arrays for plotting or export. This is the native sequential path
for stateful timestep studies; `validate-data` remains the measured-vs-simulated
workflow for dataset rows.

## Schema

```powershell
bcs-runner.exe schema `
  --project project.bcsproj `
  --output schema.json
```

## Migration Report

`migrate` checks artifact schema compatibility and writes a machine-readable
report. For the current `0.1.x` line, compatible projects do not need rewriting.

```powershell
bcs-runner.exe migrate `
  --project project.bcsproj `
  --output migration-report.json
```

The command exits successfully when `project.bcsproj` and `graph.json` are inside
the supported compatibility line. It exits with a validation error when an
artifact is missing `schema_version`, has an invalid version, or uses an
unsupported major/minor version. The report still records which artifact needs a
manual migration. `--write` is accepted for documented migrations, but
currently writes no changes when no migration is needed.

## Data Validation

`validate-data` runs a saved dataset mapping and writes metrics plus high-error rows:

```powershell
bcs-runner.exe validate-data `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --mapping validation/mappings/plant_validation.json `
  --parameter-set parameter_sets/high_efficiency.json `
  --save-record `
  --output validation-result.json
```

The mapping and parameter-set paths are project-relative. The result includes RMSE, MAE, MBE, CVRMSE, R2, row summaries, and inspect data for the highest-error rows. `--save-record` also writes a project artifact under `validation/runs/`.
Validation mappings can set `missing_value_policy` to `error`, `drop`, `fill`,
or `ignore_output_rows`; the result reports evaluated, skipped, and filled row
handling so CLI output matches the Studio Data view.

## Calibration

`calibrate` runs a saved calibration setup. The current implemented algorithm is grid search:

```powershell
bcs-runner.exe calibrate `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --setup calibration/setups/chiller_cop_grid.json `
  --save-parameter-set parameter_sets/calibrated_chiller_cop.json `
  --save-record `
  --output calibration-result.json
```

The setup points to a validation mapping, an optional base parameter set, an objective, and parameter bounds. Saving the parameter set writes a new parameter set without changing baseline `graph.json`. `--save-record` also writes the calibration result under `calibration/results/`.

## Optimization

`optimize` runs a saved optimization setup. The current implemented path supports grid search over public inputs:

```powershell
bcs-runner.exe optimize `
  --project examples/006_optimization_case/project.bcsproj `
  --setup optimization/setups/chw_setpoint_grid.json `
  --save-scenario scenarios/optimized_setpoint.json `
  --save-record `
  --output optimization-result.json
```

The result reports candidate objectives, best inputs, best outputs, and the saved scenario path. `--save-record` also writes the optimization result under `optimization/results/`.

## Saved Records

Saved validation, calibration, and optimization records include a `provenance` object. It records the workflow provenance schema, runner version, package version when available, project and graph schema versions, SHA-256 checksums for `project.bcsproj` and `graph.json`, component source checksums, and checksums for the workflow artifacts used by that record such as mappings, datasets, parameter sets, setups, saved parameter sets, and saved scenarios.

## Serve

Serve mode keeps the graph compiled and Python components loaded for repeated evaluations:

```powershell
@'
{"id":"case-1","inputs":{"value":4},"context":{"time":0,"dt":60}}
{"id":"case-2","inputs":{"value":5},"context":{"time":60,"dt":60}}
{"id":"stop","type":"shutdown"}
'@ | bcs-runner.exe serve --project project.bcsproj
```

Each input line is a JSON request. Each output line is a JSON response with `ok`, `id`, and either `result` or `error`. Component state is preserved inside the live serve session until shutdown. Use `run-series` when the repeated evaluations are a saved time-indexed artifact. See [External Engine Protocol](external-engine-protocol.md) for the schema, shutdown request, timeout guidance, and smoke-tested JSONL example.

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
