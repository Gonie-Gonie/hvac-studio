# Export Runtime

Runtime export prepares a project for delivery to external tools or users who do not need Studio.

## Current Studio Behavior

The Export button can write:

```text
exports/runtime_package/manifest.json
exports/runtime_package/bin/bcs-runner.exe
exports/runtime_package/bin/bcs-env.exe
exports/runtime_package/project/project.bcsproj
exports/runtime_package/project/graph.json
exports/runtime_package/project/components/
exports/runtime_package/project/inputs/
exports/runtime_package/project/scenarios/
exports/runtime_package/runtime/python/
exports/runtime_package/schema/public-io.json
```

This is the first connected runtime export artifact. It copies the source-of-truth project files needed by the runner, writes a public input/output schema for consumers, and records the exported files plus public IO and execution order in the manifest. When Studio is running from a portable/runtime package, export also copies the packaged runner tools and Python runtime into the export folder so the exported project can run without a system Python install.

## Target Runtime Package Shape

```text
DeliveredModel/
  bin/
    bcs-runner.exe
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
