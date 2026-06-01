# Quick Start

This page gets a first-time user from an included example to a successful one-case run.

## 1. Launch Studio

From the repository:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\run-studio.ps1
```

Open the printed local URL, usually:

```text
http://127.0.0.1:5174
```

## 2. Open An Example

Select an example project from the Project dropdown. The system canvas shows the entry system, its components, and visible node endpoints.

## 3. Inspect Components

Select a component in the canvas or project tree. The Inspector shows:

- component ID and class
- input nodes
- output nodes
- parameters
- latest run inputs and outputs after execution

## 4. Run One Case

Use the Run button. Studio sends the current project and run inputs to the same runtime path used by `bcs-runner`.

## 5. Check Results

Open the Results panel. For workspace projects, runs are saved as `runs/run-*.json` and can be reopened from the Project tree.

## 6. Create A Workspace Project

Use New to create a workspace project under `projects/`. Workspace projects can be edited; bundled examples are read-only through Studio write APIs.

