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
| Stateful controller | `004_stateful_controller` | Step component state, controller logic, native `run-series`, and serve-mode repeated evaluations. |
| Plant workflow | `005_chiller_plant_like_system` | Plant-like composition, dataset validation, parameter sets, calibration, and CSV time columns. |
| Optimization | `006_optimization_case` | Grid-search optimization and SDK-style external search scripting. |
| Runtime-only delivery | `007_runtime_only_package` | A packaged delivery layout with a runnable model and CLI guide. |
| Generated wrapper authoring | `008_generated_wrapper_component` | Generated-wrapper component source layout and export coverage. |
| Vectorized component | `009_vectorized_component` | Array input/output execution through the vectorized worker contract. |
| External executable | `010_external_executable_component` | A component that calls an external process through stdin/stdout JSON. |
| Solver boundary | `011_solver_boundary_component` | Internal iterative feedback inside one explicit solver boundary component. |
| Unit conversion | `012_unit_conversion_component` | Explicit connection-level linear unit conversion and value-type checks. |

## Smoke Coverage

Run all example smoke tests from the repo root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-examples.ps1
```

The smoke script:

- validates every `project.bcsproj` under `examples/`
- runs each example's `inputs/case01.json`
- compares the run result against `expected/output.json`
- runs native time-series examples when `inputs/series01.json` exists
- runs the plant validation mapping from `005_chiller_plant_like_system`
- runs the plant calibration setup from `005_chiller_plant_like_system`
- runs the optimization setup from `006_optimization_case`

## Time-Series Boundary

Use `bcs-runner run-series` for native sequential timestep runs with state
carryover. The `004_stateful_controller` example includes `inputs/series01.json`
and a golden `expected/series_output.json`.

Use `009_vectorized_component` for the vectorized execution mode. It shows one
component receiving an array public input and returning an array public output
through `evaluate_batch`.

Use `010_external_executable_component` for the external executable mode. It
shows one component invoking a separate command and carrying state through the
runner-managed request/response contract.

Use `011_solver_boundary_component` for feedback behavior. It keeps the project
graph acyclic and performs fixed-point iteration inside a declared solver
boundary component.

Use `012_unit_conversion_component` for unit conversion. It shows a connection
converting W to kW before the target component receives its input.

Use CSV data validation with an explicit `time_column` for
measured-vs-simulated comparisons. Each validation row is still treated as one
independent model evaluation, and `validate-data` does not carry hidden state
between rows.
