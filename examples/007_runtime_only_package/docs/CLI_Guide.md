# Runtime-Only CLI Guide

This example mirrors the intended delivery shape for a model that can run without Studio.

Validate:

```powershell
.\bin\bcs-runner.exe validate --project .\model\project.bcsproj
```

Run:

```powershell
powershell -ExecutionPolicy Bypass -File .\examples\run_once.ps1
```

Check the packaged environment:

```powershell
.\bin\bcs-env.exe check --root . --json
```
