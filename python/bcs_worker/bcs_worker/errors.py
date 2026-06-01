"""Structured worker errors."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class WorkerError(Exception):
    error_type: str
    message: str
    traceback: str = ""

    def to_dict(self) -> dict[str, str]:
        return {
            "type": self.error_type,
            "message": self.message,
            "traceback": self.traceback,
        }

