# Data Validation

Model validation compares simulated outputs against measured or reference data.

## Workflow

1. Import a dataset.
2. Detect columns.
3. Map dataset columns to public inputs.
4. Map observed columns to public outputs.
5. Run simulations.
6. Compute validation metrics.
7. Inspect high-error timesteps.

In Studio, open the Artifacts workspace, enter a local CSV path, choose the
delimiter and encoding options, and select `Import Data`. Studio copies the CSV
into the project `datasets/` folder, normalizes it to the runner CSV format,
shows a header preview, infers basic column types, records the dataset SHA256
checksum, and suggests public input/output column matches. The dataset preview
includes a mapping editor for time column, public input columns, observed output
columns, and column unit hints. Select `Create Mapping` from the dataset preview
to save a validation mapping without editing JSON.

The saved artifacts use CSV datasets and mapping files. See `examples/005_chiller_plant_like_system`:

```text
datasets/plant_validation.csv
validation/mappings/plant_validation.json
```

The mapping connects dataset columns to public inputs and observed outputs:

```json
{
  "dataset": "datasets/plant_validation.csv",
  "dataset_checksum": "<sha256>",
  "time_column": "time",
  "missing_value_policy": "error",
  "input_columns": {
    "building_load_kw": "building_load_kw"
  },
  "observed_output_columns": {
    "total_power_kw": "measured_total_power_kw"
  },
  "unit_hints": {
    "building_load_kw": "kW",
    "measured_total_power_kw": "kW"
  }
}
```

Missing values are blank cells or common markers such as `NA`, `N/A`, `NaN`,
`null`, and `none`. Studio lets you choose the mapping policy before selecting
`Create Mapping`:

| Policy | Behavior |
|---|---|
| `error` | Stop validation at the first required missing value. Older `fail_fast` mappings are read as `error`. |
| `drop` | Skip rows with any missing mapped input, observed output, or time value. |
| `fill` | Fill missing mapped values with the previous value for that dataset column, or `0` if no previous value exists. |
| `ignore_output_rows` | Skip rows with missing observed output values while still requiring mapped inputs and time values. |

## Metrics

Implemented metrics:

- RMSE
- MAE
- MBE
- CVRMSE
- R2

Run from the CLI:

```powershell
bcs-runner.exe validate-data `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --mapping validation/mappings/plant_validation.json `
  --parameter-set parameter_sets/high_efficiency.json
```

In Studio, projects with saved mappings show a `Validation` section in the Project tree and enable the `Data` command. The result appears in the Results panel with evaluated row count, input row count, skipped rows, filled value count, metrics, high-error rows, and output-level plots for measured vs simulated values, scatter, residuals, and residual histograms. Select a high-error row to inspect the timestep component inputs, component outputs, node values, connection values, and state snapshot.

When you run validation again for the same dataset and mapping, Studio keeps the
previous validation result as the comparison baseline and shows a `Parameter Set
Comparison` table with RMSE, MAE, and R2 deltas. This is intended for comparing
baseline, calibrated, and scenario-specific parameter sets without changing the
saved graph.

For workspace projects, Studio saves Data command results under `validation/runs/` and shows them in the Project tree as `Validation Runs`. CLI users can do the same with `bcs-runner validate-data --save-record`.

Validation should not automatically change parameters. Calibration is the workflow that estimates parameters from observed data.
