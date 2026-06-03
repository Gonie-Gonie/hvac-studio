from __future__ import annotations

import json
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
