# Troubleshooting

## Validation Error

Validation errors usually mean a project artifact has an invalid reference or unsupported graph shape.

Common causes:

- system references an unknown component
- public input references an unknown node
- connection references an unknown node
- input node has multiple incoming connections
- algebraic loop detected

## Python Worker Error

Python worker errors happen while loading, initializing, or evaluating user Python code.

Common causes:

- syntax error
- import error
- class name mismatch
- missing required output node
- non-JSON-serializable return value

## Missing Public Input

If a required public input is missing, add it to the run input file or save the default input from Studio.

## Bundled Python

Portable packages include `runtime/python`. Release smoke tests constrain `PATH` to prove bundled Python is used.

Run this from a package root to confirm the bundled runtime is visible:

```powershell
.\bin\bcs-env.exe check --root . --json
```

## Studio Does Not Open

In a portable package, launch the root-level executable:

```text
HVAC Studio.exe
```

For scripted API automation, use:

```powershell
.\bin\studio.exe --server
```

If the Wails window reports a build-tag error, the package was not built through the release build path. Rebuild with the release scripts and verify with `scripts/release/test-release-candidate.ps1`.

## Runtime Export Does Not Run

From an export folder, check the export first:

```powershell
.\bin\bcs-env.exe check --root . --json
```

Then run the default case:

```powershell
powershell -ExecutionPolicy Bypass -File .\run-default.ps1
```

If source-check errors are reported, open the project in Studio, use the Code workspace Problems panel, fix the Python component source, save, and export again.
