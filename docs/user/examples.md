# Examples

The `examples/` folder is part of the product contract. It gives users a
learning path and gives releases repeatable smoke assets.

## Recommended Order

| Step | Example | What to learn |
| --- | --- | --- |
| 1 | `examples/001_scalar_component` | Public inputs, public outputs, Python component calls, and expected outputs. |
| 2 | `examples/003_feedforward_system` | Feed-forward system composition and connection validation. |
| 3 | `examples/008_generated_wrapper_component` | Generated-wrapper component authoring and export-safe source layout. |
| 4 | `examples/004_stateful_controller` | Controller state and serve-mode repeated evaluations. |
| 5 | `examples/005_chiller_plant_like_system` | Plant composition, dataset validation, parameter sets, calibration, and time-column validation rows. |
| 6 | `examples/006_optimization_case` | Grid-search optimization and SDK-style external search scripting. |
| 7 | `examples/007_runtime_only_package` | Runtime-only delivery shape and exported-package command style. |

`examples/002_custom_component` is intentionally reserved for future richer
custom inlet/outlet authoring notes and is not part of the runnable smoke set.

## Regression Role

The example smoke gate validates every runnable project, runs its default case,
and compares the result against the expected output. It also runs the plant data
validation mapping, the plant calibration setup, and the optimization setup:

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

Use this as the current time-indexed validation pattern. Native sequential
time-series execution with state carryover is a separate post-1.0 engine item.
