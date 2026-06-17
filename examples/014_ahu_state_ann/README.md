# 014 AHU State ANN

This example shows the minimum ML-backed component workflow:

- `FeatureMapper` maps public scalar inputs into a deterministic feature object.
- `AHUStateANN` loads project-owned JSON model assets during initialize.
- `ml_metadata` records the model file, feature schema, target schema, validation report, valid ranges, and package requirements.
- Runtime export copies the assets and records their checksums in the export manifest.

Run it with:

```powershell
Push-Location .\tools\go
go run .\cmd\bcs-runner validate `
  --project ..\..\examples\014_ahu_state_ann\project.bcsproj
go run .\cmd\bcs-runner run `
  --project ..\..\examples\014_ahu_state_ann\project.bcsproj `
  --input ..\..\examples\014_ahu_state_ann\inputs\case01.json
Pop-Location
```
