# 006 Optimization Case

This runnable example exposes a single decision variable, `chw_setpoint_c`, and an objective output, `objective_kw`. It is designed for CLI and SDK optimization smoke tests.

Run one case:

```powershell
go run ./tools/go/cmd/bcs-runner run --project examples/006_optimization_case/project.bcsproj --input examples/006_optimization_case/inputs/case01.json
```

The `scripts/grid_search.py` file shows the intended SDK workflow: keep the model interface stable and let external research code search candidate inputs.
