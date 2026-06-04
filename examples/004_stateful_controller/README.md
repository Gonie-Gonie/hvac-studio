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
go run .\cmd\bcs-runner run-series `
  --project ..\..\examples\004_stateful_controller\project.bcsproj `
  --input ..\..\examples\004_stateful_controller\inputs\series01.json `
  --output ..\..\artifacts\004_stateful_controller-series.json
Pop-Location
```

`run-series` evaluates the steps in `inputs/series01.json` inside one runtime
session. The output aggregates each public output into arrays and reports
step-level state plus `final_states`, so the PI controller's integral term can
be inspected across timesteps. For lower-level repeated evaluations, use
`bcs-runner serve`.
