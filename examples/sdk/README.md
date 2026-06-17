# SDK And External Engine Examples

This folder contains examples for tools that call the runner without using
Studio.

## Python SDK Workflows

The SDK is a thin runner wrapper, not a second simulation implementation. Use
`RunnerClient.start(...)` for a persistent `bcs-runner serve` session, or create
`RunnerClient(..., persistent=False)` for one-shot CLI-backed workflows. The same
client exposes:

- `evaluate(...)` and `evaluate_async(...)`
- `run_batch(...)`
- `validate_project(...)`
- `export_schema(...)`
- `run_validation(...)`
- `run_calibration(...)`
- `run_optimization(...)`

Model helpers load project-owned artifacts such as parameter sets, scenarios,
runtime export manifests, and exported runtime packages:

```python
from bcs_sdk import RunnerClient, load_parameter_set, load_runtime_export

project = "examples/005_chiller_plant_like_system/project.bcsproj"
parameter_set = load_parameter_set(project, "parameter_sets/baseline.json")

with RunnerClient.start(project, request_timeout=30) as client:
    result = client.evaluate(
        {
            "building_load_kw": 600,
            "base_chw_setpoint_c": 7,
            "condenser_entering_temp_c": 32,
        },
        {"time": 0, "dt": 60},
        parameter_set="parameter_sets/baseline.json",
    )

export = load_runtime_export("exports/runtime_package")
with export.runner_client(request_timeout=30) as client:
    schema = client.export_schema()
```

For pooled optimization-style evaluations, see
`examples/006_optimization_case/scripts/grid_search.py`; it uses `RunnerPool`
so each worker owns a persistent serve process.

## JSONL Serve Requests

`serve-requests.jsonl` is a smoke-tested request stream for
`examples/001_scalar_component/project.bcsproj`.

Run it from the repository root:

```powershell
Push-Location .\tools\go
Get-Content -Encoding UTF8 ..\..\examples\sdk\serve-requests.jsonl |
  go run .\cmd\bcs-runner serve --project ..\..\examples\001_scalar_component\project.bcsproj
Pop-Location
```

The stream sends two successful evaluations, one structured error case, and a
shutdown request.

Reusable protocol schemas live at:

- `schema/serve-request.schema.json`
- `schema/serve-response.schema.json`

## Raw Python Subprocess

`raw_serve_subprocess.py` shows the same protocol without importing
`bcs_sdk`. Pass the runner path as the first argument when the runner is not on
`PATH`. From a source checkout, run the development command from the Go module
root:

```powershell
Push-Location .\tools\go
python ..\..\examples\sdk\raw_serve_subprocess.py go run .\cmd\bcs-runner
Pop-Location
```
