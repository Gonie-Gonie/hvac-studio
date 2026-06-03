from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from io import StringIO
from pathlib import Path
from unittest.mock import patch

from bcs_sdk import RunnerClient, RunnerError
from bcs_sdk.model import load_export_manifest, load_parameter_set, load_project, load_scenario


class ModelTests(unittest.TestCase):
    def test_load_project(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "project.bcsproj"
            path.write_text(json.dumps({"project_name": "case"}), encoding="utf-8")
            self.assertEqual(load_project(path)["project_name"], "case")

    def test_load_project_artifacts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = root / "project.bcsproj"
            project.write_text(json.dumps({"project_name": "case"}), encoding="utf-8")
            (root / "parameter_sets").mkdir()
            (root / "scenarios").mkdir()
            (root / "exports" / "runtime_package").mkdir(parents=True)
            (root / "parameter_sets" / "baseline.json").write_text(json.dumps({"id": "baseline"}), encoding="utf-8")
            (root / "scenarios" / "case01.json").write_text(json.dumps({"id": "case01"}), encoding="utf-8")
            (root / "exports" / "runtime_package" / "manifest.json").write_text(
                json.dumps({"profile": "runtime_package", "commands": ["run-default.ps1"]}),
                encoding="utf-8",
            )

            self.assertEqual(load_parameter_set(project, "parameter_sets/baseline.json")["id"], "baseline")
            self.assertEqual(load_scenario(project, "scenarios/case01.json")["id"], "case01")
            self.assertEqual(load_export_manifest(project)["commands"], ["run-default.ps1"])


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

    def test_evaluate_preserves_structured_error(self) -> None:
        process = FakeProcess([
            {
                "id": "case-1",
                "ok": False,
                "error": {
                    "schema": "hvac-studio.error.v1",
                    "code": 3,
                    "kind": "input",
                    "message": "missing required public input: value",
                },
            },
            {"id": "shutdown", "ok": True, "message": "shutdown"},
        ])

        with patch("subprocess.Popen", return_value=process):
            client = RunnerClient.start(project="project.bcsproj", runner="bcs-runner")
            with self.assertRaises(RunnerError) as raised:
                client.evaluate({}, {"time": 0})
            client.close()

        self.assertEqual(raised.exception.schema, "hvac-studio.error.v1")
        self.assertEqual(raised.exception.kind, "input")
        self.assertEqual(raised.exception.code, 3)
        self.assertIn("missing required public input", str(raised.exception))

    def test_run_once_uses_runner_json_mode_and_parameter_set(self) -> None:
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=json.dumps({"outputs": {"result": 12}}),
            stderr="",
        )

        with patch("subprocess.run", return_value=completed) as run:
            client = RunnerClient(project="project.bcsproj", runner="bcs-runner", persistent=False)
            result = client.run_once({"value": 4}, parameter_set="parameter_sets/high.json")

        self.assertEqual(result["outputs"]["result"], 12)
        args = run.call_args.args[0]
        self.assertEqual(args[:3], ["bcs-runner", "--error-format", "json"])
        self.assertIn("run", args)
        self.assertIn("--parameter-set", args)
        self.assertIn("parameter_sets/high.json", args)

    def test_workflow_helpers_build_cli_json_commands(self) -> None:
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=json.dumps({"ok": True, "saved_record": "record.json"}),
            stderr="",
        )

        with patch("subprocess.run", return_value=completed) as run:
            client = RunnerClient(project="project.bcsproj", runner="bcs-runner", persistent=False)
            validation = client.run_validation(
                "validation/mappings/case.json",
                parameter_set="parameter_sets/high.json",
                high_error_rows=2,
                save_record=True,
            )
            calibration = client.run_calibration(
                "calibration/setups/case.json",
                save_parameter_set="parameter_sets/calibrated.json",
                save_record=True,
            )
            optimization = client.run_optimization(
                "optimization/setups/case.json",
                save_scenario="scenarios/best.json",
                save_record=True,
            )

        self.assertTrue(validation["ok"])
        self.assertTrue(calibration["ok"])
        self.assertTrue(optimization["ok"])
        calls = [" ".join(call.args[0]) for call in run.call_args_list]
        self.assertIn("validate-data --project project.bcsproj --mapping validation/mappings/case.json", calls[0])
        self.assertIn("--parameter-set parameter_sets/high.json", calls[0])
        self.assertIn("--save-record", calls[0])
        self.assertIn("calibrate --project project.bcsproj --setup calibration/setups/case.json", calls[1])
        self.assertIn("--save-parameter-set parameter_sets/calibrated.json", calls[1])
        self.assertIn("optimize --project project.bcsproj --setup optimization/setups/case.json", calls[2])
        self.assertIn("--save-scenario scenarios/best.json", calls[2])

    def test_subprocess_errors_preserve_structured_payload(self) -> None:
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=3,
            stdout="",
            stderr=json.dumps({
                "ok": False,
                "error": {
                    "schema": "hvac-studio.error.v1",
                    "code": 3,
                    "kind": "input",
                    "message": "mapping is required",
                },
            }),
        )

        with patch("subprocess.run", return_value=completed):
            client = RunnerClient(project="project.bcsproj", runner="bcs-runner", persistent=False)
            with self.assertRaises(RunnerError) as raised:
                client.run_validation("")

        self.assertEqual(raised.exception.schema, "hvac-studio.error.v1")
        self.assertEqual(raised.exception.kind, "input")
        self.assertEqual(raised.exception.code, 3)


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
