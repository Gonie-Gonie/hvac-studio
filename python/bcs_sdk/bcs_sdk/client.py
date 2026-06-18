from __future__ import annotations

import asyncio
import json
import os
import shutil
import subprocess
import threading
import uuid
from contextlib import contextmanager
from concurrent.futures import ThreadPoolExecutor, TimeoutError as FutureTimeoutError
from pathlib import Path
from typing import Any, Iterable, Iterator


class RunnerError(RuntimeError):
    def __init__(self, message: str, error: dict[str, Any] | None = None) -> None:
        super().__init__(message)
        self.error = error or {}
        self.kind = self.error.get("kind")
        self.code = self.error.get("code")
        self.schema = self.error.get("schema")


class RunnerClient:
    """Thin wrapper around bcs-runner.

    The SDK is intentionally not a simulation engine. It prepares JSON input,
    calls the runner, and returns the runner output for research workflows.
    """

    def __init__(
        self,
        project: str | Path,
        runner: str | Path = "bcs-runner.exe",
        persistent: bool = True,
        request_timeout: float | None = None,
    ) -> None:
        self.project = Path(project)
        self.runner = str(runner)
        self.persistent = persistent
        self.request_timeout = request_timeout
        self._process: subprocess.Popen[str] | None = None
        self._serve_lock = threading.RLock()

    @classmethod
    def start(
        cls,
        project: str | Path,
        runner: str | Path = "bcs-runner.exe",
        request_timeout: float | None = None,
    ) -> "RunnerClient":
        client = cls(project=project, runner=runner, persistent=True, request_timeout=request_timeout)
        client.connect()
        return client

    def connect(self) -> None:
        with self._serve_lock:
            if self._process is not None and self._process.poll() is None:
                return
            self._process = subprocess.Popen(
                [
                    self.runner,
                    "serve",
                    "--project",
                    str(self.project),
                ],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=1,
            )

    def evaluate(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
        parameter_set: str | Path | None = None,
    ) -> dict[str, Any]:
        if self.persistent and parameter_set is None:
            return self._evaluate_serve(inputs, context or {})
        return self.run_once(inputs, context, parameter_set=parameter_set)

    async def evaluate_async(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
        parameter_set: str | Path | None = None,
    ) -> dict[str, Any]:
        """Evaluate from an asyncio workflow without reimplementing the runner."""

        return await asyncio.to_thread(self.evaluate, inputs, context, parameter_set)

    def run_once(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
        parameter_set: str | Path | None = None,
        output: str | Path | None = None,
    ) -> dict[str, Any]:
        payload = {"inputs": inputs, "context": context or {}}
        with sdk_temporary_directory() as tmp:
            tmp_path = Path(tmp)
            input_path = tmp_path / "input.json"
            input_path.write_text(json.dumps(payload), encoding="utf-8")

            args = [
                "run",
                "--project",
                str(self.project),
                "--input",
                str(input_path),
            ]
            if parameter_set is not None:
                args.extend(["--parameter-set", str(parameter_set)])
            if output is not None:
                args.extend(["--output", str(output)])
                self._run_runner(args, expect_json=False)
                return json.loads(Path(output).read_text(encoding="utf-8"))
            return self._run_runner(args)

    def validate_project(self) -> str:
        """Run the runner's project contract validation and return stdout."""

        return self._run_runner(["validate", "--project", str(self.project)], expect_json=False)

    def export_schema(self, output: str | Path | None = None) -> dict[str, Any]:
        args = ["schema", "--project", str(self.project)]
        if output is not None:
            args.extend(["--output", str(output)])
            self._run_runner(args, expect_json=False)
            return json.loads(Path(output).read_text(encoding="utf-8"))
        return self._run_runner(args)

    def run_validation(
        self,
        mapping: str | Path,
        parameter_set: str | Path | None = None,
        high_error_rows: int = 3,
        save_record: bool = False,
        output: str | Path | None = None,
    ) -> dict[str, Any]:
        args = [
            "validate-data",
            "--project",
            str(self.project),
            "--mapping",
            str(mapping),
            "--high-error-rows",
            str(high_error_rows),
        ]
        if parameter_set is not None:
            args.extend(["--parameter-set", str(parameter_set)])
        if save_record:
            args.append("--save-record")
        return self._run_workflow_json(args, output)

    def run_calibration(
        self,
        setup: str | Path,
        save_parameter_set: str | Path | None = None,
        save_record: bool = False,
        output: str | Path | None = None,
    ) -> dict[str, Any]:
        args = ["calibrate", "--project", str(self.project), "--setup", str(setup)]
        if save_parameter_set is not None:
            args.extend(["--save-parameter-set", str(save_parameter_set)])
        if save_record:
            args.append("--save-record")
        return self._run_workflow_json(args, output)

    def run_optimization(
        self,
        setup: str | Path,
        save_scenario: str | Path | None = None,
        save_parameter_set: str | Path | None = None,
        save_record: bool = False,
        output: str | Path | None = None,
    ) -> dict[str, Any]:
        args = ["optimize", "--project", str(self.project), "--setup", str(setup)]
        if save_scenario is not None:
            args.extend(["--save-scenario", str(save_scenario)])
        if save_parameter_set is not None:
            args.extend(["--save-parameter-set", str(save_parameter_set)])
        if save_record:
            args.append("--save-record")
        return self._run_workflow_json(args, output)

    def run_batch(
        self,
        scenarios: Iterable[dict[str, Any]],
        parameter_set: str | Path | None = None,
    ) -> dict[str, Any]:
        cases: list[dict[str, Any]] = []
        for index, scenario in enumerate(scenarios):
            name = str(scenario.get("name") or scenario.get("id") or f"case-{index + 1}")
            try:
                result = self.evaluate(
                    dict(scenario.get("inputs") or {}),
                    dict(scenario.get("context") or {}),
                    parameter_set=parameter_set,
                )
                cases.append({"scenario_name": name, "ok": True, "result": result})
            except RunnerError as exc:
                cases.append({"scenario_name": name, "ok": False, "error": str(exc), "structured_error": exc.error})
        return {
            "ok": all(case["ok"] for case in cases),
            "parameter_set": str(parameter_set or ""),
            "case_count": len(cases),
            "ok_count": sum(1 for case in cases if case["ok"]),
            "cases": cases,
        }

    def close(self) -> None:
        with self._serve_lock:
            process = self._process
            if process is None:
                return
            try:
                if process.poll() is None and process.stdin is not None:
                    request = {"id": "shutdown", "type": "shutdown"}
                    process.stdin.write(json.dumps(request) + "\n")
                    process.stdin.flush()
                    if process.stdout is not None:
                        process.stdout.readline()
                    process.wait(timeout=5)
            finally:
                if process.poll() is None:
                    process.kill()
                self._process = None

    def _evaluate_serve(self, inputs: dict[str, Any], context: dict[str, Any]) -> dict[str, Any]:
        with self._serve_lock:
            self.connect()
            process = self._process
            if process is None or process.stdin is None or process.stdout is None:
                raise RunnerError("runner serve process is not available")

            request_id = str(uuid.uuid4())
            request = {"id": request_id, "inputs": inputs, "context": context}
            process.stdin.write(json.dumps(request) + "\n")
            process.stdin.flush()

            line = self._read_serve_line(process)
            if not line:
                stderr = process.stderr.read() if process.stderr is not None else ""
                raise RunnerError(f"runner serve closed without a response: {stderr.strip()}")
            response = json.loads(line)
            if not response.get("ok"):
                error = response.get("error") or {}
                message = error.get("message") or "runner serve request failed"
                raise RunnerError(message, error=error)
            return response["result"]

    def _read_serve_line(self, process: subprocess.Popen[str]) -> str:
        if process.stdout is None:
            raise RunnerError("runner serve stdout is not available")
        if self.request_timeout is None:
            return process.stdout.readline()
        with ThreadPoolExecutor(max_workers=1) as executor:
            future = executor.submit(process.stdout.readline)
            try:
                return future.result(timeout=self.request_timeout)
            except FutureTimeoutError as exc:
                if process.poll() is None:
                    process.kill()
                self._process = None
                raise RunnerError(f"runner serve request timed out after {self.request_timeout:g} seconds") from exc

    def _run_workflow_json(self, args: list[str], output: str | Path | None) -> dict[str, Any]:
        if output is not None:
            args = [*args, "--output", str(output)]
            self._run_runner(args, expect_json=False)
            return json.loads(Path(output).read_text(encoding="utf-8"))
        return self._run_runner(args)

    def _run_runner(self, args: list[str], expect_json: bool = True) -> Any:
        try:
            completed = subprocess.run(
                [self.runner, "--error-format", "json", *args],
                check=False,
                capture_output=True,
                text=True,
                timeout=self.request_timeout,
            )
        except subprocess.TimeoutExpired as exc:
            timeout = self.request_timeout if self.request_timeout is not None else exc.timeout
            message = f"runner command timed out after {timeout:g} seconds"
            raise RunnerError(
                message,
                error={"schema": "hvac-studio.error.v1", "kind": "timeout", "message": message},
            ) from exc
        if completed.returncode != 0:
            raise self._runner_error(completed)
        if not expect_json:
            return completed.stdout
        output = completed.stdout.strip()
        if not output:
            return {}
        return json.loads(output)

    def _runner_error(self, completed: subprocess.CompletedProcess[str]) -> RunnerError:
        stderr = completed.stderr.strip()
        payload: dict[str, Any] = {}
        if stderr:
            for line in reversed(stderr.splitlines()):
                try:
                    decoded = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(decoded, dict):
                    payload = decoded.get("error") or decoded
                    break
        message = payload.get("message") or stderr or f"runner exited with code {completed.returncode}"
        return RunnerError(message, error=payload)

    def __enter__(self) -> "RunnerClient":
        self.connect()
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.close()


@contextmanager
def sdk_temporary_directory() -> Iterator[str]:
    root = os.environ.get("HVAC_STUDIO_TMP")
    if root:
        tmp_root = Path(root) / "bcs-sdk"
    elif os.environ.get("HVAC_STUDIO_REPO_ROOT"):
        tmp_root = Path(os.environ["HVAC_STUDIO_REPO_ROOT"]) / ".tmp" / "bcs-sdk"
    else:
        tmp_root = Path.cwd() / ".tmp" / "bcs-sdk"

    tmp_root.mkdir(parents=True, exist_ok=True)
    tmp_dir = tmp_root / f"tmp-{uuid.uuid4().hex}"
    tmp_dir.mkdir(parents=True, exist_ok=False)
    try:
        yield str(tmp_dir)
    finally:
        shutil.rmtree(tmp_dir, ignore_errors=True)
        try:
            tmp_root.rmdir()
        except OSError:
            pass
