"""Small helper objects user components may import."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class Flow:
    """JSON-friendly flow-like value with open-ended properties."""

    values: dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_mapping(cls, values: dict[str, Any]) -> "Flow":
        return cls(dict(values))

    def updated(self, **changes: Any) -> "Flow":
        next_values = dict(self.values)
        next_values.update(changes)
        return Flow(next_values)

    def to_dict(self) -> dict[str, Any]:
        return dict(self.values)

