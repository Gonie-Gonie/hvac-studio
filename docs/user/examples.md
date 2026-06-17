# Examples

The `examples/` folder is part of the product contract. It gives users a
learning path and gives releases repeatable smoke assets.

## Recommended Order

| Step | Example | What to learn |
| --- | --- | --- |
| 1 | `examples/001_scalar_component` | Public inputs, public outputs, Python component calls, and expected outputs. |
| 2 | `examples/003_feedforward_system` | Feed-forward system composition and connection validation. |
| 3 | `examples/008_generated_wrapper_component` | Generated-wrapper component authoring and export-safe source layout. |
| 4 | `examples/009_vectorized_component` | Vectorized array input/output execution through `evaluate_batch`. |
| 5 | `examples/010_external_executable_component` | External process execution through stdin/stdout JSON. |
| 6 | `examples/011_solver_boundary_component` | Internal iterative feedback inside an explicit solver boundary. |
| 7 | `examples/012_unit_conversion_component` | Explicit connection-level linear unit conversion and value-type checks. |
| 8 | `examples/013_composite_system` | Nested systems behind explicit public IO boundaries. |
| 9 | `examples/004_stateful_controller` | Controller state, native `run-series`, and serve-mode repeated evaluations. |
| 10 | `examples/005_chiller_plant_like_system` | Plant composition, dataset validation, parameter sets, calibration, and time-column validation rows. |
| 11 | `examples/006_optimization_case` | Grid-search optimization and SDK-style external search scripting. |
| 12 | `examples/007_runtime_only_package` | Runtime-only delivery shape and exported-package command style. |
| 13 | `examples/014_ahu_state_ann` | Feature mapping, ML component metadata, model assets, and export checksums. |
| 14 | `examples/015_rc_ahu_ann_composition` | RC zone load, ANN surrogate AHU, validation, calibration, optimization, SDK use, and model replacement. |

`examples/002_custom_component` is intentionally reserved for richer custom
inlet/outlet authoring notes and is not part of the runnable smoke set.

## Regression Role

The example smoke gate validates every runnable project, runs its default case,
compares the result against the expected output, and runs native time-series
goldens when an example provides `inputs/series01.json`. It also runs the plant
data validation mapping, the plant calibration setup, and the optimization setup:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-examples.ps1
```

Use this gate after editing model contracts, runtime execution, validation,
calibration, optimization, export packaging, or example files.

## Time-Column Validation

The plant example includes `datasets/plant_validation.csv` and
`validation/mappings/plant_validation.json`. The mapping names a `time_column`,
so validation results preserve each row's time value while still treating each
row as an independent case.

Use this as the measured-vs-simulated validation pattern. For sequential stateful
timestep execution, use `bcs-runner run-series` with a series input artifact such
as `examples/004_stateful_controller/inputs/series01.json`.

For array-shaped one-call execution, use `examples/009_vectorized_component`.
That example declares `execution_mode: "vectorized"` and routes the component
through `evaluate_batch`.

For command-line process integration, use
`examples/010_external_executable_component`. That example declares
`kind: "external_exe"` and sends component inputs/state/params/context to a
separate process on stdin.

For feedback behavior, use `examples/011_solver_boundary_component`. That
example declares `solver_boundary` metadata and performs fixed-point iteration
inside one component while the project graph remains acyclic.

For explicit unit conversion, use `examples/012_unit_conversion_component`. That
example declares `unit_conversion` on a connection, validates the target
component receives the converted value, and can be copied into a workspace to
try the Inspector preset editor.

For nested composition, use `examples/013_composite_system`. That example
declares a `kind: "composite"` wrapper whose input and output node IDs match the
child system public IO IDs. The `run-series` golden also shows nested state
carryover through the wrapper state.

For ML-backed composition, use `examples/014_ahu_state_ann`. That example maps
scalar system inputs into a deterministic feature object, loads JSON model
assets once during component initialization, and exposes the asset requirements
through `ml_metadata`, schema export, and runtime export manifests.

For a full practical workflow, use `examples/015_rc_ahu_ann_composition`. It
combines RC zone dynamics, ANN-backed AHU behavior, equipment aggregation,
dataset validation, calibration, optimization, an SDK script, and a
`ZoneLoadRC` to `ZoneLoadANN` replacement walkthrough.
