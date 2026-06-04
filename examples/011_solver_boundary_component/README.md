# Solver Boundary Component

This example shows the feedback-loop policy used by HVAC Studio: the project
graph remains acyclic, and iterative feedback behavior lives inside an explicit
solver boundary component.

The component declares `solver_boundary` metadata and runs a small fixed-point
iteration internally. The runner still executes the system as a normal DAG.

Run it from the repo root:

```powershell
cd tools/go
go run ./cmd/bcs-runner run --project ../../examples/011_solver_boundary_component/project.bcsproj --input ../../examples/011_solver_boundary_component/inputs/case01.json
```
