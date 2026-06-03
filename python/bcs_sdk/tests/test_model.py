from __future__ import annotations

import json
import tempfile
import unittest
from io import StringIO
from pathlib import Path
from unittest.mock import patch

from bcs_sdk import RunnerClient
from bcs_sdk.model import load_project


class ModelTests(unittest.TestCase):
    def test_load_project(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "project.bcsproj"
            path.write_text(json.dumps({"project_name": "case"}), encoding="utf-8")
            self.assertEqual(load_project(path)["project_name"], "case")


class RunnerClientTests(unittest.TestCase):
    def test_evaluate_uses_serve_session(self) -> None:
        process = FakeProcess([
            {"id": "case-1", "ok": True, "result": {"outputs": {"result": 10}}},
            {"id": "shutdown", "ok": True, "message": "shutdown"},
        ])

        with patch("subprocess.Popen", return_value=process) as popen:
            client = RunnerClient.start(project="project.bcsproj", runner="bcs-runner")
            result = client.evaluate({"value": 4}, {"time": 0})
            client.close()

        popen.assert_called_once()
        self.assertEqual(result["outputs"]["result"], 10)
        self.assertIn('"inputs": {"value": 4}', process.stdin.getvalue())
        self.assertIn('"type": "shutdown"', process.stdin.getvalue())


class FakeProcess:
    def __init__(self, responses: list[dict[str, object]]) -> None:
        self.stdin = StringIO()
        self.stdout = StringIO("".join(json.dumps(response) + "\n" for response in responses))
        self.stderr = StringIO()
        self.returncode: int | None = None

    def poll(self) -> int | None:
        return self.returncode

    def wait(self, timeout: float | None = None) -> int:
        self.returncode = 0
        return 0

    def kill(self) -> None:
        self.returncode = -9


if __name__ == "__main__":
    unittest.main()
