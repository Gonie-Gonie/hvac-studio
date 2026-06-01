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

