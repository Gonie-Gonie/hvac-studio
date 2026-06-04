from __future__ import annotations

import os
import threading
from concurrent.futures import Future, ThreadPoolExecutor
from pathlib import Path
from typing import Any, Iterable

from .client import RunnerClient, RunnerError


class RunnerPool:
    """Bounded pool of persistent runner serve sessions.

    Each worker thread owns one `RunnerClient` and sends requests to that
    client's `bcs-runner serve` session serially. This keeps high-volume
    evaluations on the runner path while avoiding concurrent writes to one
    process stdin/stdout pair.
    """

    def __init__(
        self,
        project: str | Path,
        runner: str | Path = "bcs-runner.exe",
        workers: int | None = None,
        request_timeout: float | None = None,
    ) -> None:
        self.project = Path(project)
        self.runner = str(runner)
        self.workers = default_worker_count(workers)
        self.request_timeout = request_timeout
        self._executor = ThreadPoolExecutor(max_workers=self.workers, thread_name_prefix="bcs-sdk-runner")
        self._local = threading.local()
        self._clients: list[RunnerClient] = []
        self._clients_lock = threading.Lock()
        self._closed = False
        self._closed_lock = threading.Lock()

    @classmethod
    def start(
        cls,
        project: str | Path,
        runner: str | Path = "bcs-runner.exe",
        workers: int | None = None,
        request_timeout: float | None = None,
    ) -> "RunnerPool":
        return cls(project=project, runner=runner, workers=workers, request_timeout=request_timeout)

    def submit(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
        parameter_set: str | Path | None = None,
    ) -> Future[dict[str, Any]]:
        self._ensure_open()
        return self._executor.submit(
            self._evaluate_on_worker,
            dict(inputs),
            dict(context or {}),
            parameter_set,
        )

    def submit_case(self, case: dict[str, Any]) -> Future[dict[str, Any]]:
        return self.submit(
            dict(case.get("inputs") or {}),
            dict(case.get("context") or {}),
            parameter_set=case.get("parameter_set"),
        )

    def evaluate(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any] | None = None,
        parameter_set: str | Path | None = None,
    ) -> dict[str, Any]:
        return self.submit(inputs, context, parameter_set=parameter_set).result()

    def evaluate_many(self, cases: Iterable[dict[str, Any]]) -> list[dict[str, Any]]:
        futures = [self.submit_case(case) for case in cases]
        return [future.result() for future in futures]

    def map(self, cases: Iterable[dict[str, Any]]) -> Iterable[dict[str, Any]]:
        futures = [self.submit_case(case) for case in cases]
        for future in futures:
            yield future.result()

    def close(self) -> None:
        with self._closed_lock:
            if self._closed:
                return
            self._closed = True
        self._executor.shutdown(wait=True, cancel_futures=True)
        with self._clients_lock:
            clients = list(self._clients)
            self._clients.clear()
        for client in clients:
            client.close()

    def _evaluate_on_worker(
        self,
        inputs: dict[str, Any],
        context: dict[str, Any],
        parameter_set: str | Path | None,
    ) -> dict[str, Any]:
        client = self._client_for_thread()
        return client.evaluate(inputs, context, parameter_set=parameter_set)

    def _client_for_thread(self) -> RunnerClient:
        client = getattr(self._local, "client", None)
        if client is not None:
            return client
        client = RunnerClient.start(project=self.project, runner=self.runner, request_timeout=self.request_timeout)
        self._local.client = client
        with self._clients_lock:
            self._clients.append(client)
        return client

    def _ensure_open(self) -> None:
        with self._closed_lock:
            if self._closed:
                raise RunnerError("runner pool is closed")

    def __enter__(self) -> "RunnerPool":
        self._ensure_open()
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self.close()


def default_worker_count(workers: int | None) -> int:
    if workers is None:
        return max(1, min(4, os.cpu_count() or 1))
    if workers < 1:
        raise ValueError("workers must be at least 1")
    return workers
