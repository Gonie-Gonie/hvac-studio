"""JSONL protocol helpers for the worker boundary."""

from __future__ import annotations

from dataclasses import asdict, is_dataclass
from pathlib import Path
from typing import Any


def ensure_project_root(project_root: str) -> str:
    path = Path(project_root).resolve()
    if not path.exists():
        raise FileNotFoundError(f"project_root does not exist: {path}")
    return str(path)


def to_jsonable(value: Any) -> Any:
    if value is None or isinstance(value, (str, int, float, bool)):
        return value
    if isinstance(value, list):
        return [to_jsonable(item) for item in value]
    if isinstance(value, tuple):
        return [to_jsonable(item) for item in value]
    if isinstance(value, dict):
        return {str(key): to_jsonable(item) for key, item in value.items()}
    if hasattr(value, "to_dict") and callable(value.to_dict):
        return to_jsonable(value.to_dict())
    if is_dataclass(value):
        return to_jsonable(asdict(value))
    raise TypeError(f"value is not JSON serializable by the worker: {type(value).__name__}")

