# HVAC Studio Examples

Examples are both learning material and regression assets. Runnable examples are
validated by `scripts/dev/test-examples.ps1`; workflow examples also exercise
data validation, calibration, optimization, SDK-oriented scripting, and
runtime-only delivery paths.

## Learning Path

| Path | Example | Use it for |
| --- | --- | --- |
| First scalar run | `001_scalar_component` | A minimal public input, Python component, public output, and expected output. |
| Custom component placeholder | `002_custom_component` | Reserved design notes for richer custom inlet/outlet authoring. |
| Feed-forward system | `003_feedforward_system` | Multiple connected components and system-level public IO. |
| Stateful controller | `004_stateful_controller` | Step component state, controller logic, and serve-mode repeated evaluations. |
| Plant workflow | `005_chiller_plant_like_system` | Plant-like composition, dataset validation, parameter sets, calibration, and CSV time columns. |
| Optimization | `006_optimization_case` | Grid-search optimization and SDK-style external search scripting. |
| Runtime-only delivery | `007_runtime_only_package` | A packaged delivery layout with a runnable model and CLI guide. |
| Generated wrapper authoring | `008_generated_wrapper_component` | Generated-wrapper component source layout and export coverage. |

## Smoke Coverage

Run all example smoke tests from the repo root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-examples.ps1
```

The smoke script:

- validates every `project.bcsproj` under `examples/`
- runs each example's `inputs/case01.json`
- compares the run result against `expected/output.json`
- runs the plant validation mapping from `005_chiller_plant_like_system`
- runs the plant calibration setup from `005_chiller_plant_like_system`
- runs the optimization setup from `006_optimization_case`

## Time-Series Boundary

The current stable time-indexed workflow is CSV data validation with an explicit
`time_column`. Each row is treated as one independent model evaluation and the
time value is preserved in validation rows. Native sequential time-series state
carryover is planned separately under PM-301, so examples should not imply hidden
stateful iteration through `validate-data`.
