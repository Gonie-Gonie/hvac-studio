# Python SDK

The Python SDK is a thin wrapper around `bcs-runner`. It does not reimplement
simulation logic; it starts the same runner used by Studio and the CLI, sends
requests, and returns runner results and structured errors.

## Install Path

Runtime and portable packages include the SDK under `python/bcs_sdk`. In a source
checkout, set `PYTHONPATH` to that folder or run through the development scripts,
which add it automatically.

## Persistent Evaluation

Use `RunnerClient.start(...)` when you want a persistent `bcs-runner serve`
process. The client is a context manager and closes the serve process with a
shutdown request:

```python
from bcs_sdk import RunnerClient

with RunnerClient.start("project.bcsproj", runner="bcs-runner.exe", request_timeout=30) as client:
    result = client.evaluate({"value": 4}, {"time": 0, "dt": 60})
    print(result["outputs"])
```

`evaluate_async(...)` exposes the same call from asyncio workflows without
changing the execution engine:

```python
result = await client.evaluate_async({"value": 4}, {"time": 0, "dt": 60})
```

## One-Shot Workflows

Use a non-persistent client when you want CLI-backed one-shot commands:

```python
from bcs_sdk import RunnerClient

client = RunnerClient("project.bcsproj", runner="bcs-runner.exe", persistent=False, request_timeout=30)
schema = client.export_schema()
validation = client.run_validation("validation/mappings/case.json")
calibration = client.run_calibration("calibration/setups/case.json")
optimization = client.run_optimization("optimization/setups/case.json")
```

The same timeout applies to persistent serve requests and one-shot workflow
commands. Runner failures raise `RunnerError`; when the runner returns a
structured error, `RunnerError.error`, `RunnerError.schema`, `RunnerError.kind`,
and `RunnerError.code` preserve that payload.

## Batches And Pools

`run_batch(...)` evaluates independent cases through the runner and returns
per-case success or structured failure records. For larger independent candidate
sets, use `RunnerPool` so each worker owns a persistent serve process:

```python
from bcs_sdk import RunnerPool

cases = [{"inputs": {"value": value}, "context": {"time": 0, "dt": 60}} for value in [3, 4, 5]]

with RunnerPool.start("project.bcsproj", runner="bcs-runner.exe", workers=2, request_timeout=30) as pool:
    results = pool.evaluate_many(cases)
```

The optimization example at
`examples/006_optimization_case/scripts/grid_search.py` uses this pooled pattern.

## Project Helpers

Model helper functions load project-owned artifacts without manual path plumbing:

```python
from bcs_sdk import load_parameter_set, load_runtime_export, load_scenario

parameters = load_parameter_set("project.bcsproj", "parameter_sets/baseline.json")
scenario = load_scenario("project.bcsproj", "scenarios/case01.json")

export = load_runtime_export("exports/runtime_package")
with export.runner_client(request_timeout=30) as client:
    result = client.evaluate(scenario["inputs"], scenario.get("context") or {})
```

Use the raw JSONL protocol only when integrating a non-Python tool. For that path,
see [External Engine Protocol](external-engine-protocol.md).
