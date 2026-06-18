# 002 Custom Component

This runnable example shows a user-defined Python component with several custom
inlet and outlet nodes. The model is a small air mixing box: outdoor air and
return air are mixed according to a requested outdoor-air fraction, with the
component exposing temperature, airflow, and CO2 nodes as first-class public IO.

Run it from the repository root:

```powershell
cd tools\go
go run .\cmd\bcs-runner validate --project ..\..\examples\002_custom_component\project.bcsproj
go run .\cmd\bcs-runner run --project ..\..\examples\002_custom_component\project.bcsproj --input ..\..\examples\002_custom_component\inputs\case01.json
```

The example is included in `scripts/dev/test-examples.ps1`, so edits to custom
component node metadata, public IO mapping, or worker execution are checked by
the normal example smoke gate.
