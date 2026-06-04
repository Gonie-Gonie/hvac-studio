# Time-Column Validation Tutorial

This example uses the measured-vs-simulated time-column workflow: CSV validation
rows with an explicit `time_column`.

## Files

```text
datasets/plant_validation.csv
validation/mappings/plant_validation.json
```

The dataset has one row per sampled operating point:

```text
time,building_load_kw,base_chw_setpoint_c,condenser_entering_temp_c,measured_total_power_kw,measured_chw_supply_temp_c
0,450,7.0,29.0,91.2,7.12
60,600,7.0,32.0,142.5,7.08
120,700,6.8,34.0,172.4,6.98
```

The mapping tells the runner which CSV columns are inputs, which columns are
observed outputs, and which column should be carried into row summaries as time.

## Run

From the repository root:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate-data `
  --project ..\..\examples\005_chiller_plant_like_system\project.bcsproj `
  --mapping validation\mappings\plant_validation.json `
  --output ..\..\artifacts\plant-validation.json
Pop-Location
```

The result JSON includes:

- `row_count`
- `metrics`
- `rows[].time`
- simulated public outputs
- observed public outputs
- high-error row summaries

## Boundary

`validate-data` evaluates each CSV row independently. It is the right tool for
measured-vs-simulated checks, calibration setup data, and time-indexed reporting.
It does not carry hidden state between rows.

For sequential stateful timestep execution, use `bcs-runner run-series` with a
series input artifact. For repeated calls where an external script owns the loop,
use `bcs-runner serve` or the Python SDK.
