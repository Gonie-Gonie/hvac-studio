# Optimization

Optimization changes public inputs or design parameters to minimize or maximize an objective while respecting constraints.

## Optimization Versus Calibration

Calibration changes model parameters to match data.

Optimization changes decision variables to improve an objective.

## Workflow

1. Select a scenario or dataset.
2. Select decision variables.
3. Select an objective output.
4. Define constraints.
5. Run optimization.
6. Save the result as scenario, parameter set, script, or CSV.

The current implemented path uses a saved grid-search setup over public inputs. See:

```text
examples/006_optimization_case/optimization/setups/chw_setpoint_grid.json
```

Run it from the CLI:

```powershell
bcs-runner.exe optimize `
  --project examples/006_optimization_case/project.bcsproj `
  --setup optimization/setups/chw_setpoint_grid.json `
  --save-scenario scenarios/optimized_setpoint.json `
  --save-record
```

The result includes each candidate objective, the best inputs, best outputs, and the saved scenario path. `--save-record` writes a reproducible result artifact under `optimization/results/`.

## SDK Path

Advanced optimization can use Python SDK scripts that call the same runner as
Studio. `examples/006_optimization_case/scripts/grid_search.py` uses
`RunnerPool.start(...)` to keep a bounded number of `bcs-runner serve` sessions
alive and evaluate independent candidate setpoints from Python. Use
`request_timeout` on the client or pool so an external search loop does not wait
forever on an unresponsive runner process.
