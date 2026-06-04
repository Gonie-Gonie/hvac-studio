# Vectorized Component

This example shows the post-MVP `vectorized` execution mode. The component
accepts an array public input and returns an array public output from a single
worker call.

Run it from the repo root:

```powershell
cd tools/go
go run ./cmd/bcs-runner run --project ../../examples/009_vectorized_component/project.bcsproj --input ../../examples/009_vectorized_component/inputs/case01.json
```

Expected public output:

```json
{
  "results": [2.0, 4.0, 7.0]
}
```
