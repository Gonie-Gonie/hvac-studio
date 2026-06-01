from __future__ import annotations

from typing import Any, Callable

from .client import RunnerClient


def scalar_objective(
    client: RunnerClient,
    make_inputs: Callable[[list[float]], dict[str, Any]],
    output_name: str,
) -> Callable[[list[float]], float]:
    def objective(x: list[float]) -> float:
        result = client.evaluate(make_inputs(x))
        return float(result["outputs"][output_name])

    return objective

