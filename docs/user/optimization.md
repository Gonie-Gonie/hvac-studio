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

In Studio, open `Artifacts` and use `Opt Setup` to create a saved grid-search
setup without editing JSON. The editor lets you select the objective output,
minimize or maximize, base parameter set, public-input and component-parameter
decision variables, bounds, output constraints, and estimated run count.

The saved setup uses public inputs or component parameters. A decision variable
can use:

- `kind: "public_input"` with `name`
- `kind: "component_parameter"` with `component` and `name`

Output constraints use `output`, `operator`, and `value`. Supported operators
are `<=`, `>=`, and `==`. Candidate rows record feasibility, failed runs, and
constraint violations without mutating the baseline graph.

See:

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

The result includes each candidate objective, the best inputs, best outputs, and
the saved scenario path. Studio result views show best decision variables, best
outputs, constraint status, candidate output comparison, export the candidate
table as CSV for spreadsheet review, download a Markdown report, and export a
Python SDK script that calls `RunnerClient.run_optimization(...)` with the same
setup. `--save-record` writes a reproducible result artifact under
`optimization/results/`.

For component-parameter studies, save the best parameter values separately:

```powershell
bcs-runner.exe optimize `
  --project examples/006_optimization_case/project.bcsproj `
  --setup optimization/setups/parameter_credit_grid.json `
  --save-parameter-set parameter_sets/optimized_credit.json
```

## SDK Path

Advanced optimization can use Python SDK scripts that call the same runner as
Studio. `examples/006_optimization_case/scripts/grid_search.py` uses
`RunnerPool.start(...)` to keep a bounded number of `bcs-runner serve` sessions
alive and evaluate independent candidate setpoints from Python. Use
`request_timeout` on the client or pool so an external search loop does not wait
forever on an unresponsive runner process.
