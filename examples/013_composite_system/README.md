# Composite System

This example shows an explicit nested public IO boundary. `MainSystem` contains
one `kind: "composite"` wrapper component. The wrapper points to `GainSystem`,
and its input/output node IDs match `GainSystem` public input/output IDs.

The runner evaluates the child system as a nested session while the outer graph
still sees one normal DAG component. The series input also verifies nested state
carryover through the wrapper component state.

Run it from the repo root:

```powershell
cd tools/go
go run ./cmd/bcs-runner run --project ../../examples/013_composite_system/project.bcsproj --input ../../examples/013_composite_system/inputs/case01.json
go run ./cmd/bcs-runner run-series --project ../../examples/013_composite_system/project.bcsproj --input ../../examples/013_composite_system/inputs/series01.json
```
