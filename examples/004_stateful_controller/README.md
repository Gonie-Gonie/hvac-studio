# 004 Stateful Controller

This example shows a step-based Python controller whose state is carried through the runtime result. The controller integrates supply-temperature error and returns a chilled-water setpoint.

Run it with:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate `
  --project ..\..\examples\004_stateful_controller\project.bcsproj
go run .\cmd\bcs-runner run `
  --project ..\..\examples\004_stateful_controller\project.bcsproj `
  --input ..\..\examples\004_stateful_controller\inputs\case01.json
Pop-Location
```

For repeated evaluations with state preserved inside one process, use `bcs-runner serve`.
