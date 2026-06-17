from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any


def load_project(path: str | Path) -> dict[str, Any]:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def load_graph(path: str | Path) -> dict[str, Any]:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def project_root(project: str | Path) -> Path:
    path = Path(project)
    return path.parent if path.name.endswith(".bcsproj") else path


def load_project_artifact(project: str | Path, relative_path: str | Path) -> dict[str, Any]:
    return json.loads((project_root(project) / relative_path).read_text(encoding="utf-8"))


def load_parameter_set(project: str | Path, relative_path: str | Path) -> dict[str, Any]:
    return load_project_artifact(project, relative_path)


def load_scenario(project: str | Path, relative_path: str | Path) -> dict[str, Any]:
    return load_project_artifact(project, relative_path)


def load_export_manifest(project: str | Path, profile: str = "runtime_package") -> dict[str, Any]:
    return load_project_artifact(project, Path("exports") / profile / "manifest.json")


@dataclass(frozen=True)
class RuntimeExport:
    """Convenience view over an exported runtime package."""

    root: Path
    manifest: dict[str, Any]

    def artifact_path(self, relative_path: str | Path | None) -> Path:
        if not relative_path:
            return self.root
        path = Path(relative_path)
        return path if path.is_absolute() else self.root / path

    @property
    def project_path(self) -> Path:
        return self.artifact_path(self.manifest.get("project_path") or "project/project.bcsproj")

    @property
    def graph_path(self) -> Path:
        return self.artifact_path(self.manifest.get("graph_path") or "project/graph.json")

    @property
    def runner_path(self) -> Path:
        return self.artifact_path(self.manifest.get("runner") or "bin/bcs-runner.exe")

    @property
    def runtime_python_path(self) -> Path:
        return self.artifact_path(self.manifest.get("runtime_python") or "runtime/python/python.exe")

    @property
    def public_schema_path(self) -> Path:
        return self.artifact_path(self.manifest.get("interface_schema") or "schema/public-io.json")

    @property
    def serve_request_schema_path(self) -> Path:
        return self.root / "schema" / "serve-request.schema.json"

    @property
    def serve_response_schema_path(self) -> Path:
        return self.root / "schema" / "serve-response.schema.json"

    def runner_client(self, persistent: bool = True, request_timeout: float | None = None):
        from .client import RunnerClient

        return RunnerClient(
            project=self.project_path,
            runner=self.runner_path,
            persistent=persistent,
            request_timeout=request_timeout,
        )


def load_runtime_export(root: str | Path) -> RuntimeExport:
    export_root = Path(root)
    manifest = json.loads((export_root / "manifest.json").read_text(encoding="utf-8"))
    return RuntimeExport(root=export_root, manifest=manifest)
