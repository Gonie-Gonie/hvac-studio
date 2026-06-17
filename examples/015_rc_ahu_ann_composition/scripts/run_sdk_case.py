from __future__ import annotations

import json
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
SDK_ROOT = REPO_ROOT / "python" / "bcs_sdk"
if SDK_ROOT.exists():
    sys.path.insert(0, str(SDK_ROOT))

from bcs_sdk import RunnerClient


def main() -> None:
    example_root = Path(__file__).resolve().parents[1]
    runner = sys.argv[1] if len(sys.argv) > 1 else "bcs-runner.exe"
    input_payload = json.loads((example_root / "inputs" / "case01.json").read_text(encoding="utf-8"))

    with RunnerClient.start(project=example_root / "project.bcsproj", runner=runner, request_timeout=30) as client:
        result = client.evaluate(input_payload["inputs"], input_payload.get("context") or {})

    outputs = result["outputs"]
    print(
        " ".join(
            [
                f"zone_load_kw={float(outputs['zone_load_kw']):.3f}",
                f"supply_air_temperature_c={float(outputs['supply_air_temperature_c']):.3f}",
                f"total_power_kw={float(outputs['total_power_kw']):.3f}",
            ]
        )
    )


if __name__ == "__main__":
    main()
