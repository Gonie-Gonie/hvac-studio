# Troubleshooting

## Structured Errors

CLI JSON errors, Studio API errors, runner `serve` errors, and SDK `RunnerError.error` use the same `hvac-studio.error.v1` payload:

```json
{
  "schema": "hvac-studio.error.v1",
  "code": 3,
  "kind": "input",
  "message": "missing required public input: value",
  "problems": []
}
```

The `kind` values follow the documented CLI exit-code taxonomy: `validation`, `runtime`, `input`, `python_worker`, and `license_runtime`. Studio may include `problems` entries with `component_id`, `node_id`, `source`, `line`, and `column` when it can link an error back to a project artifact. For CLI commands, pass `--error-format json` before the subcommand or set `BCS_RUNNER_ERROR_FORMAT=json`.

In Studio, start with the Problems, Run, Results, and Logs panels. Use the
Diagnostics tab when support or CLI comparison requires the raw JSON payload.

## Validation Error

Validation errors usually mean a project artifact has an invalid reference or unsupported graph shape.

Common causes:

- system references an unknown component
- public input references an unknown node
- connection references an unknown node
- input node has multiple incoming connections
- algebraic loop detected; wrap feedback behavior in a solver boundary component
- value does not match the node `value_type`
- source and target units differ without an explicit `unit_conversion`

## Python Worker Error

Python worker errors happen while loading, initializing, or evaluating user Python code.

When a worker traceback points into a project component source file, Studio includes `component_id`, `source`, and `line` in the structured problem payload. Open the Problems panel or Code workspace to jump to that line.

Common causes:

- syntax error
- import error
- class name mismatch
- missing required output node
- non-JSON-serializable return value

## Missing Public Input

If a required public input is missing, add it to the run input file or save the default input from Studio.

## Bundled Python

Portable packages include `runtime/python`. Release smoke tests constrain `PATH` to prove bundled Python is used.

Run this from a package root to confirm the bundled runtime is visible:

```powershell
.\bin\bcs-env.exe check --root . --json
```

## Studio Does Not Open

In a portable package, launch the root-level executable:

```text
HVAC Studio.exe
```

For scripted API automation, use:

```powershell
.\bin\studio.exe --server
```

If the Wails window reports a build-tag error, the package was not built through the release build path. Rebuild with the release scripts and verify with `scripts/release/test-release-candidate.ps1`.

## Runtime Export Does Not Run

From an export folder, check the export first:

```powershell
.\bin\bcs-env.exe check --root . --json
```

Then run the default case:

```powershell
powershell -ExecutionPolicy Bypass -File .\run-default.ps1
```

The script writes the run result to `outputs\latest.json` and component logs to
`outputs\logs\latest-logs.json` unless you pass `-Output` or `-LogBundle`.

If source-check errors are reported, open the project in Studio, use the Code workspace Problems panel, fix the Python component source, save, and export again.
