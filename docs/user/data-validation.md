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

The current implemented path uses CSV datasets and saved mapping files. See `examples/005_chiller_plant_like_system`:

```text
datasets/plant_validation.csv
validation/mappings/plant_validation.json
```

The mapping connects dataset columns to public inputs and observed outputs:

```json
{
  "dataset": "datasets/plant_validation.csv",
  "time_column": "time",
  "input_columns": {
    "building_load_kw": "building_load_kw"
  },
  "observed_output_columns": {
    "total_power_kw": "measured_total_power_kw"
  }
}
```

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
  --mapping validation/mappings/plant_validation.json
```

In Studio, projects with saved mappings show a `Validation` section in the Project tree and enable the `Data` command. The result appears in the Results panel.

Validation should not automatically change parameters. Calibration is the workflow that estimates parameters from observed data.
