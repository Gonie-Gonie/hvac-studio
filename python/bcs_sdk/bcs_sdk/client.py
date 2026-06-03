from __future__ import annotations

import json
import subprocess
import tempfile
import uuid
from pathlib import Path
from typing import Any


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

    def __init__(self, project: str | Path, runner: str | Path = "bcs-runner.exe", persistent: bool = True) -> None:
        self.project = Path(project)
        self.runner = str(runner)
        self.persistent = persistent
        self._process: subprocess.Popen[str] | None = None

    @classmethod
    def start(cls, project: str | Path, runner: str | Path = "bcs-runner.exe") -> "RunnerClient":
        client = cls(project=project, runner=runner, persistent=True)
        client.connect()
        return client

    def connect(self) -> None:
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
    ) -> dict[str, Any]:
        if self.persistent:
            return self._evaluate_serve(inputs, context or {})
        return self.run_once(inputs, context)

    def run_once(
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

    def close(self) -> None:
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
        self.connect()
        process = self._process
        if process is None or process.stdin is None or process.stdout is None:
            raise RunnerError("runner serve process is not available")

        request_id = str(uuid.uuid4())
        request = {"id": request_id, "inputs": inputs, "context": context}
        process.stdin.write(json.dumps(request) + "\n")
        process.stdin.flush()

        line = process.stdout.readline()
        if not line:
            stderr = process.stderr.read() if process.stderr is not None else ""
            raise RunnerError(f"runner serve closed without a response: {stderr.strip()}")
        response = json.loads(line)
        if not response.get("ok"):
            error = response.get("error") or {}
            message = error.get("message") or "runner serve request failed"
            raise RunnerError(message, error=error)
        return response["result"]

    def __enter__(self) -> "RunnerClient":
        self.connect()
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.close()
