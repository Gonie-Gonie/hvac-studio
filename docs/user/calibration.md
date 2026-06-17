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

In Studio, open `Artifacts` and use `Cal Setup` to create a setup without
editing JSON. The editor lets you select the validation mapping, target outputs
and weights, candidate parameters, bounds, base parameter set, and grid-search
algorithm. Candidate filters cover role, component, unit, and bounds presence;
the selected candidate count and expected grid run count update before saving.

The saved setup can also be inspected or reproduced from JSON. See:

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
calibration results show a before/after parameter table and explicit actions for
the saved parameter set. `Use for Runs` activates the saved parameter set without
rewriting the baseline graph, `Revert Active` clears that runtime overlay, and
`Apply Parameter Set` is the deliberate graph-writing path. `Validation
Before/After` reruns the mapping with the base and saved parameter sets and shows
the same validation plots and metric deltas used by Data validation. The
candidate table can be exported as CSV for spreadsheet review, and `Export
Report` downloads a Markdown report with the objective summary, parameter
changes, and candidate table. `--save-record` writes a reproducible result
artifact under `calibration/results/`.

## Important Rule

Calibration results should not overwrite baseline parameters by default. They should be reproducible from dataset mapping, objective settings, bounds, and base parameter set.
