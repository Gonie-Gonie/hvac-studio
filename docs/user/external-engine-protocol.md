# External Engine Protocol

External optimizers, co-simulation harnesses, and non-Python tools can call the
same runner used by Studio and the Python SDK through `bcs-runner serve`.

Serve mode is a UTF-8 JSON Lines protocol: one JSON request per stdin line, one
JSON response per stdout line. The process keeps the project graph compiled and
keeps component sessions alive until shutdown or process exit.

## Start Serve Mode

```powershell
bcs-runner.exe serve --project .\examples\001_scalar_component\project.bcsproj
```

For repository smoke tests, the same protocol is exercised with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-serve-protocol.ps1
```

That smoke test runs both the raw JSONL request stream and the raw Python
subprocess example so SDK-free integrations keep the same request, response,
error, and shutdown behavior.

## Request Schema

Reusable JSON Schema file: `schema/serve-request.schema.json`.

Evaluation request:

```json
{"id":"case-1","inputs":{"value":4},"context":{"time":0,"dt":60}}
```

Shutdown request:

```json
{"id":"stop","type":"shutdown"}
```

Fields:

| Field | Required | Description |
|---|---:|---|
| `id` | No | Caller-owned correlation ID copied into the response when present. |
| `type` | No | Use `shutdown` to ask the runner to close cleanly. |
| `inputs` | Yes for evaluation | Public input values using the project public input IDs. |
| `context` | No | Per-request context such as `time`, `dt`, candidate ID, or optimizer metadata. |

Batching is line-oriented: send multiple evaluation requests as multiple JSONL
lines. Use `run-series` instead when the sequence itself is a saved time-indexed
artifact.

## Response Schema

Reusable JSON Schema file: `schema/serve-response.schema.json`.

Successful evaluation response:

```json
{"id":"case-1","ok":true,"result":{"outputs":{"result":10}}}
```

Structured error response:

```json
{
  "id": "bad-missing-input",
  "ok": false,
  "error": {
    "schema": "hvac-studio.error.v1",
    "code": 3,
    "kind": "input",
    "message": "missing required public input: value"
  }
}
```

Shutdown response:

```json
{"id":"stop","ok":true,"message":"shutdown"}
```

Errors are per request. Invalid JSON or a failed evaluation returns `ok:false`
and the serve loop continues. Initialization failures before the loop starts are
reported through the normal CLI exit path.

## Timeout Behavior

`bcs-runner serve` does not currently take a per-request timeout flag. External
callers should enforce their own timeout, terminate the serve process if it
expires, and start a new serve process for subsequent work. The Python SDK
exposes this as `RunnerClient(..., request_timeout=seconds)` for persistent
serve requests and one-shot workflow commands.

## PowerShell Example

```powershell
Get-Content -Encoding UTF8 .\examples\sdk\serve-requests.jsonl |
  bcs-runner.exe serve --project .\examples\001_scalar_component\project.bcsproj
```

## Raw Python Subprocess Example

Use `examples/sdk/raw_serve_subprocess.py` when you need the wire protocol
without importing `bcs_sdk`:

```powershell
python .\examples\sdk\raw_serve_subprocess.py bcs-runner.exe
```

From a source checkout, run the development command from the Go module root:

```powershell
Push-Location .\tools\go
python ..\..\examples\sdk\raw_serve_subprocess.py go run .\cmd\bcs-runner
Pop-Location
```

For Python projects, prefer `RunnerClient` and `RunnerPool` unless you are
testing a non-SDK integration.
