from __future__ import annotations

import json
import subprocess
import tempfile
from pathlib import Path
from typing import Any


class RunnerClient:
    """Thin wrapper around bcs-runner.

    The SDK is intentionally not a simulation engine. It prepares JSON input,
    calls the runner, and returns the runner output for research workflows.
    """

    def __init__(self, project: str | Path, runner: str | Path = "bcs-runner.exe") -> None:
        self.project = Path(project)
        self.runner = str(runner)

    @classmethod
    def start(cls, project: str | Path, runner: str | Path = "bcs-runner.exe") -> "RunnerClient":
        return cls(project=project, runner=runner)

    def evaluate(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        payload = {"inputs": inputs, "context": context or {}}
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            input_path = tmp_path / "input.json"
            output_path = tmp_path / "output.json"
            input_path.write_text(json.dumps(payload), encoding="utf-8")

            subprocess.run(
                [
                    self.runner,
                    "run",
                    "--project",
                    str(self.project),
                    "--input",
                    str(input_path),
                    "--output",
                    str(output_path),
                ],
                check=True,
            )
            return json.loads(output_path.read_text(encoding="utf-8"))

