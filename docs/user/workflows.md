# Workflows

All workflows use the same project, public IO, and runner. Start with
[modeling](modeling.md) to define a system and
[CLI reference](cli-runner.md) for exact command options.

## Run Simulation

Validate the graph before running. Studio's **Run** executes the current public
inputs, context, active parameter set, and saved component source. The input
toolbar shows types, units, defaults, and required fields. **Default** resets a
field; save inputs separately to keep them as project defaults.

Run/Batch requests default to a 30-second timeout. Adjust **Timeout** for longer
work. **Cancel** stops the active request while retaining the preceding result;
**Retry** repeats the workflow with current fields and selections.

Workspace runs save `runs/run-*.json`. Results show public outputs, component
inputs/outputs, node/connection values, states, execution order, timings, and
logs. The canvas and Inspector show the latest values; model changes mark them
stale until the next successful run. Converted connections retain both source
and target values.

Use Run comparisons for differences from the preceding result, saved records
for repeatable inspection, and CSV/JSON exports for analysis. Problems link
failures to components and source locations. Logs can be filtered and exported;
Diagnostics exposes the raw payload.

### Time Series

Use Studio **Series** or `bcs-runner run-series` with an input artifact containing
ordered `steps`. Each step contains public inputs and context; top-level context
is merged into each step and used during initialization. One session preserves
component state across timesteps.

The result includes output arrays, step-level `series[]` records, and
`final_states`. Studio plots numeric outputs, displays the step table, and uses
the final step for latest canvas values. The Series Input selector accepts a
saved series artifact or a short preview from current fields.

### Scenarios and Batches

Save the current inputs/context with **Scenario**; records live in `scenarios/`.
Reopen one from the Project tree to restore it. Editing inputs clears the active
scenario badge. Batch runs execute saved scenarios and save
`runs/batch-*.json`, including case status, output summaries, and errors.
The first successful case supplies latest component values for inspection.

### Persistent Evaluation

`bcs-runner serve` keeps a compiled graph and component state alive, receiving
one JSON request and returning one response per line. The
[Python SDK](python-sdk.md) wraps it with `RunnerClient`, async evaluation,
and `RunnerPool`. Use native time series for sequential state carryover and a
pool for independent candidate evaluations. See the
[protocol reference](external-engine-protocol.md).

## Data Validation

1. In **Artifacts**, import a local CSV with **Import Data**.
2. Review detected columns, time field, units, and checksum.
3. Map columns to public inputs and observed public outputs.
4. Select a missing-value policy and use **Evaluate Sample**.
5. Save with **Create Mapping**, then run **Data**.

Import accepts automatic detection, UTF-8/BOM, UTF-16 LE/BE, and CP949/EUC-KR.
Studio copies and normalizes the dataset into `datasets/` as UTF-8 comma CSV.
Mappings live in `validation/mappings/` and record the dataset, checksum,
time/input/output columns, unit hints, and missing-value policy.

| Policy | Behavior |
| --- | --- |
| `error` | Stop at the first required missing value; also accepts older `fail_fast` |
| `drop` | Skip rows missing mapped inputs, observed outputs, or time |
| `fill` | Use the preceding column value, or zero when none exists |
| `ignore_output_rows` | Skip missing observed outputs while requiring input/time values |

Blank cells and `NA`, `N/A`, `NaN`, `null`, or `none` count as missing values.
Rows are independent validation cases; use time series for stateful simulation.

```powershell
bcs-runner.exe validate-data --project examples/005_chiller_plant_like_system/project.bcsproj --mapping validation/mappings/plant_validation.json --parameter-set parameter_sets/high_efficiency.json --save-record
```

Results include RMSE, MAE, MBE, CVRMSE, R2, row/skip/fill counts, high-error rows,
measured-versus-simulated plots, scatter, and residuals. Select a high-error row
for component and state details. Repeated validation shows parameter-set metric
deltas. **Create Calibration Setup** carries the same mapping into calibration.
Workspace results and CLI `--save-record` persist under `validation/runs/`.
Mappings used by calibration setups are protected from deletion.

## Calibration

Calibration estimates model parameters from observations. In **Artifacts**, use
**Cal Setup**, select the validation mapping, target outputs and weights,
candidate parameters and bounds, base parameter set, algorithm, and stopping
rules. Saving requires valid bounds and at least one target and parameter.

Supported algorithms are `grid`, `differential_evolution`, and `least_squares`.
Candidate filters use role, component, unit, and bounds. Review estimated run
count and the max-candidate limit before starting.

```powershell
bcs-runner.exe calibrate --project examples/005_chiller_plant_like_system/project.bcsproj --setup calibration/setups/chiller_cop_grid.json --save-parameter-set parameter_sets/calibrated_chiller_cop.json --save-record
```

Results show initial/best objective, candidate objectives, changed parameters,
and the saved parameter set. Records live in `calibration/results/`.
**Use for Runs** activates the overlay; **Revert Active** clears it.
**Apply Parameter Set** deliberately writes the graph.

**Validation Before/After** and **Compare Existing Set** compare validation
metrics and plots. Export candidate CSV or a Markdown report for review.
The dataset mapping, bounds, objective, and base set keep the result reproducible.

## Optimization

Optimization varies decisions to improve an objective under constraints. In
**Artifacts**, use **Opt Setup** and select current inputs, project defaults, or
a saved scenario as the base. Choose the objective output, minimize/maximize,
base parameter set, decision bounds, and output constraints.

Variables can be public inputs, component parameters, or entry-system-scoped
parameters. Output constraints support `<=`, `>=`, and `==`.
Supported algorithms are `grid`, `differential_evolution`, and
`custom_sdk_script`; the latter pairs the saved setup with an exported
`RunnerClient` script for external search loops.

```powershell
bcs-runner.exe optimize --project examples/006_optimization_case/project.bcsproj --setup optimization/setups/chw_setpoint_grid.json --save-scenario scenarios/optimized_setpoint.json --save-record
```

Results show candidate feasibility, failed runs, constraint violations, best
decisions/outputs, and saved scenario or parameter-set paths. Records live in
`optimization/results/`. Open a saved scenario, activate a parameter overlay,
or explicitly apply values through the result actions. Export candidate CSV,
a Markdown report, or the SDK script for repeated studies.

Validation, calibration, and optimization preserve baseline project parameters
unless the user explicitly applies a saved parameter set.