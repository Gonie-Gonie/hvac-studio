# 004 Stateful Controller

This example shows a step-based Python controller whose state is carried through the runtime result. The controller integrates supply-temperature error and returns a chilled-water setpoint.

Run it with:

```powershell
go run ./tools/go/cmd/bcs-runner validate --project examples/004_stateful_controller/project.bcsproj
go run ./tools/go/cmd/bcs-runner run --project examples/004_stateful_controller/project.bcsproj --input examples/004_stateful_controller/inputs/case01.json
```

For repeated evaluations with state preserved inside one process, use `bcs-runner serve`.
