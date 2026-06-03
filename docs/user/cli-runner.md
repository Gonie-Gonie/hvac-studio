# CLI Runner

The CLI runner uses the same runtime path as Studio.

## Validate

```powershell
bcs-runner.exe validate --project project.bcsproj
```

## Run

```powershell
bcs-runner.exe run `
  --project project.bcsproj `
  --input input.json `
  --output output.json
```

## Schema

```powershell
bcs-runner.exe schema `
  --project project.bcsproj `
  --output schema.json
```

## Serve

Serve mode keeps the graph compiled and Python components loaded for repeated evaluations:

```powershell
@'
{"id":"case-1","inputs":{"value":4},"context":{"time":0,"dt":60}}
{"id":"case-2","inputs":{"value":5},"context":{"time":60,"dt":60}}
{"id":"stop","type":"shutdown"}
'@ | bcs-runner.exe serve --project project.bcsproj
```

Each input line is a JSON request. Each output line is a JSON response with `ok`, `id`, and either `result` or `error`. Component state is preserved inside the live serve session until shutdown.

## Exit Codes

```text
0 = success
1 = validation error
2 = runtime error
3 = input schema/error
4 = Python worker error
5 = license/runtime error
```

## Principle

If GUI and CLI use the same project, parameter set, and input, their results should match.
