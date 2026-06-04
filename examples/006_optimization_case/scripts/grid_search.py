from __future__ import annotations

from pathlib import Path

from bcs_sdk import RunnerPool


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    candidates = [6.0 + index * 0.25 for index in range(9)]
    cases = [
        {
            "inputs": {
                "building_load_kw": 500.0,
                "chw_setpoint_c": setpoint,
            }
        }
        for setpoint in candidates
    ]
    best: tuple[float, float] | None = None

    with RunnerPool.start(project=root / "project.bcsproj", workers=2, request_timeout=30) as pool:
        for setpoint, result in zip(candidates, pool.evaluate_many(cases)):
            objective = float(result["outputs"]["objective_kw"])
            if best is None or objective < best[1]:
                best = (setpoint, objective)

    assert best is not None
    print(f"best_setpoint_c={best[0]:.2f} objective_kw={best[1]:.2f}")


if __name__ == "__main__":
    main()
