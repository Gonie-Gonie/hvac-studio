# 001 Scalar Component

This is the first golden example for the runtime core.

It runs a single user-defined Python component:

```text
public input `value` -> Gain component -> public output `result`
```

The component multiplies `value` by the `gain` parameter.

Run it from the repository root:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate `
  --project ..\..\examples\001_scalar_component\project.bcsproj
go run .\cmd\bcs-runner run `
  --project ..\..\examples\001_scalar_component\project.bcsproj `
  --input ..\..\examples\001_scalar_component\inputs\case01.json
Pop-Location
```
