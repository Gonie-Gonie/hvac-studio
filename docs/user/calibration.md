# Calibration

Calibration estimates model parameters to reduce the difference between simulated outputs and observed data.

## Workflow

1. Select a dataset.
2. Select target public outputs.
3. Select calibration parameters.
4. Set bounds.
5. Choose an objective.
6. Choose an algorithm.
7. Run calibration.
8. Save results as a new parameter set.

The current implemented path uses a saved setup JSON and grid search. See:

```text
examples/005_chiller_plant_like_system/calibration/setups/chiller_cop_grid.json
```

Run it from the CLI:

```powershell
bcs-runner.exe calibrate `
  --project examples/005_chiller_plant_like_system/project.bcsproj `
  --setup calibration/setups/chiller_cop_grid.json `
  --save-parameter-set parameter_sets/calibrated_chiller_cop.json `
  --save-record
```

The result includes the initial objective, best objective, changed parameters,
candidate objectives, and the new parameter set content. In Studio, saved
calibration results show a before/after parameter table and an explicit `Apply
Parameter Set` action when a calibrated parameter set was saved. `--save-record`
writes a reproducible result artifact under `calibration/results/`.

## Important Rule

Calibration results should not overwrite baseline parameters by default. They should be reproducible from dataset mapping, objective settings, bounds, and base parameter set.
