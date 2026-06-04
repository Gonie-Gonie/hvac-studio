# 006 Optimization Case

This runnable example exposes a single decision variable, `chw_setpoint_c`, and an objective output, `objective_kw`. It is designed for CLI and SDK optimization smoke tests.

Run one case:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner run `
  --project ..\..\examples\006_optimization_case\project.bcsproj `
  --input ..\..\examples\006_optimization_case\inputs\case01.json
go run .\cmd\bcs-runner optimize `
  --project ..\..\examples\006_optimization_case\project.bcsproj `
  --setup optimization\setups\chw_setpoint_grid.json `
  --output ..\..\artifacts\optimization-result.json
Pop-Location
```

The `scripts/grid_search.py` file shows the intended SDK workflow: keep the model interface stable and let external research code search candidate inputs.
