# Export Runtime

Runtime export prepares a project for delivery to external tools or users who do not need Studio.

## Current Studio Behavior

The Export button can write:

```text
exports/runtime_package/manifest.json
exports/runtime_package/README.md
exports/runtime_package/check-env.ps1
exports/runtime_package/run-default.ps1
exports/runtime_package/run-scenario.ps1
exports/runtime_package/run-batch.ps1
exports/runtime_package/validate-data.ps1
exports/runtime_package/calibrate.ps1
exports/runtime_package/optimize.ps1
exports/runtime_package/serve.ps1
exports/runtime_package/sdk-example.py
exports/runtime_package/optimize-sdk.py
exports/runtime_package/bin/bcs-runner.exe
exports/runtime_package/bin/bcs-env.exe
exports/runtime_package/docs/CLI_Guide.md
exports/runtime_package/project/project.bcsproj
exports/runtime_package/project/graph.json
exports/runtime_package/project/components/
exports/runtime_package/project/assets/
exports/runtime_package/project/inputs/
exports/runtime_package/project/scenarios/
exports/runtime_package/project/datasets/
exports/runtime_package/project/parameter_sets/
exports/runtime_package/project/validation/
exports/runtime_package/project/calibration/
exports/runtime_package/project/optimization/
exports/runtime_package/runtime/python/
exports/runtime_package/schema/public-io.json
exports/runtime_package/schema/serve-request.schema.json
exports/runtime_package/schema/serve-response.schema.json
```

Runtime export copies the source-of-truth project files needed by the runner, including parameter sets, inputs, scenarios, and component sources. Export options let you include or omit datasets with validation mappings, calibration setups, optimization setups, ML assets, Python SDK examples, and generated records. The default Studio selection keeps these workflow artifacts self-contained. Export always writes a public input/output schema, serve protocol schemas, and model-specific CLI guide for consumers, generates Windows scripts for the workflows present in the package, and records the exported files plus public IO, execution order, command list, workflow artifact lists, model asset paths, ML validation report summaries, option selections, and SHA-256 checksums in the manifest. When Studio is running from a portable/runtime package, export also copies the packaged runner tools and Python runtime; `python/bcs_sdk` is copied when SDK examples are selected.

Export profiles appear in the Project tree. Before export, selecting the ready runtime package profile opens the Export workspace preview. After export, selecting the saved profile reopens the saved manifest so the exported folder, file list, public IO, command list, self-check command, record count, and paths can be inspected after the original export action has completed.

From the export folder:

```text
powershell -ExecutionPolicy Bypass -File .\run-default.ps1
powershell -ExecutionPolicy Bypass -File .\run-scenario.ps1 -Input project\inputs\case01.json
powershell -ExecutionPolicy Bypass -File .\run-batch.ps1
powershell -ExecutionPolicy Bypass -File .\validate-data.ps1
powershell -ExecutionPolicy Bypass -File .\calibrate.ps1
powershell -ExecutionPolicy Bypass -File .\optimize.ps1
powershell -ExecutionPolicy Bypass -File .\serve.ps1 -RequestFile requests.jsonl -Output outputs\serve-responses.jsonl
powershell -ExecutionPolicy Bypass -File .\check-env.ps1 -Json
.\runtime\python\python.exe .\sdk-example.py
.\runtime\python\python.exe .\optimize-sdk.py
```

The Studio Export workspace includes toggles for datasets, calibration setups, optimization setups, ML assets, SDK examples, and generated records. When `Records` is selected, Studio copies generated run, batch, validation, calibration, and optimization records into the package. Generated records are listed separately in `manifest.json` under `run_records`, `batch_records`, `validation_records`, `calibration_records`, and `optimization_records`.

## Target Runtime Package Shape

```text
DeliveredModel/
  bin/
    bcs-runner.exe
    bcs-env.exe
  model/
    project.bcsproj
    graph.json
    components/
    schema/
  runtime/
    python/
  examples/
    input.json
    run_once.ps1
    run_batch.ps1
  docs/
    UserGuide.pdf
    CLI_Guide.pdf
```

## Delivery Requirements

- no external Python installation requirement
- clear input/output schema
- example input/output files
- structured errors and exit codes
- smoke test after package expansion

`examples/007_runtime_only_package` mirrors this layout with a runnable `model/project.bcsproj`, first-run script, and concise CLI guide.
