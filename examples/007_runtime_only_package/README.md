# 007 Runtime-Only Package

This example shows the intended runtime-only delivery shape:

```text
bin/
model/
runtime/
examples/
docs/
```

The `model/` folder is a runnable project and participates in the example smoke tests. In a real delivery package, `bin/` contains `bcs-runner.exe` and `bcs-env.exe`, and `runtime/python/` contains the bundled Python runtime.

Run the model from the repo:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate `
  --project ..\..\examples\007_runtime_only_package\model\project.bcsproj
go run .\cmd\bcs-runner run `
  --project ..\..\examples\007_runtime_only_package\model\project.bcsproj `
  --input ..\..\examples\007_runtime_only_package\model\inputs\case01.json
Pop-Location
```
