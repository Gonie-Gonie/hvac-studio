# SDK And External Engine Examples

This folder contains examples for tools that call the runner without using
Studio.

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
