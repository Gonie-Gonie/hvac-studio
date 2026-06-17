# External Executable Component

This example shows the `external_executable` execution mode. The runner invokes
an external process for the component, sends one JSON request on stdin, and
expects one JSON response on stdout.

The request contains:

```json
{
  "component_id": "external_gain",
  "inputs": {},
  "state": {},
  "params": {},
  "context": {}
}
```

The response must contain an `outputs` object and may contain `state`, `logs`,
or `ok: false` with an error object. This example emits one stderr line and one
structured info log so Studio can show both external stderr capture and
source-tagged component logs.

Run it from the repo root:

```powershell
cd tools/go
go run ./cmd/bcs-runner run --project ../../examples/010_external_executable_component/project.bcsproj --input ../../examples/010_external_executable_component/inputs/case01.json
```
