# External Executables

Use `external_executable` components when a component should run outside the
Python worker boundary, such as a compiled simulator, a script owned by another
team, or a small command-line adapter.

## Component Settings

External components declare:

```json
{
  "kind": "external_exe",
  "execution_mode": "external_executable",
  "parameters": {
    "command": "python",
    "args": ["components/external_gain/external_gain.py"],
    "timeout_ms": 5000
  }
}
```

`command` is the executable name or path. `args` is an array of command
arguments. Project-relative paths in `command` or `args` are resolved from the
project root by the external process itself when it receives them; the runner
sets the process working directory to the project root. `timeout_ms` bounds one
component evaluation.

## Stdin Request

The runner writes one JSON object to stdin:

```json
{
  "component_id": "external_gain",
  "inputs": {},
  "state": {},
  "params": {},
  "context": {}
}
```

The request schema is `schema/external-component-request.schema.json`.

## Stdout Response

The executable writes one JSON object to stdout:

```json
{
  "ok": true,
  "outputs": {},
  "state": {},
  "logs": [
    {"severity": "info", "message": "external call complete"}
  ]
}
```

`outputs` must contain all declared output nodes. `state` is optional and is
carried to the next evaluation when present. `logs` are attached to the normal
component log table, preserving optional fields such as `source`, `line`,
`column`, and `time`. Stderr is captured as error-severity component logs and
is tagged with the component, `external_executable` stage, stream, and current
timestep when available.

For failures, return:

```json
{
  "ok": false,
  "error": {
    "type": "ExternalError",
    "message": "explanation"
  }
}
```

The response schema is `schema/external-component-response.schema.json`.

See `examples/010_external_executable_component` for a runnable adapter.
