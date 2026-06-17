"""Persistent JSONL stdio worker for user-defined Python components."""

from __future__ import annotations

import argparse
import contextlib
import importlib
import io
import json
import sys
import traceback
from dataclasses import dataclass, field
from typing import Any

from .errors import WorkerError
from .protocol import ensure_project_root, to_jsonable


@dataclass
class ComponentRecord:
    class_path: str
    instance: Any


class CapturedComponentError(Exception):
    def __init__(self, original: BaseException, logs: list[dict[str, Any]]) -> None:
        super().__init__(str(original))
        self.original = original
        self.logs = logs


@dataclass
class ComponentHost:
    components: dict[str, ComponentRecord] = field(default_factory=dict)

    def load_component(self, component_id: str, class_path: str, project_root: str) -> dict[str, Any]:
        if not component_id:
            raise WorkerError("InvalidRequest", "component_id is required")
        if not class_path:
            raise WorkerError("InvalidRequest", f"class is required for component {component_id}")

        resolved_root = ensure_project_root(project_root)
        if resolved_root not in sys.path:
            sys.path.insert(0, resolved_root)

        component_cls = self._import_class(class_path)
        instance = component_cls()
        self.components[component_id] = ComponentRecord(class_path=class_path, instance=instance)
        return {"component_id": component_id}

    def initialize_component(
        self,
        component_id: str,
        params: dict[str, Any] | None,
        context: dict[str, Any] | None,
    ) -> dict[str, Any]:
        instance = self._component(component_id)
        if not hasattr(instance, "initialize"):
            return {}
        state = instance.initialize(params or {}, context or {})
        if state is None:
            return {}
        if not isinstance(state, dict):
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.initialize must return a dict state, got {type(state).__name__}",
            )
        return to_jsonable(state)

    def evaluate_component(
        self,
        component_id: str,
        inputs: dict[str, Any] | None,
        state: dict[str, Any] | None,
        params: dict[str, Any] | None,
        context: dict[str, Any] | None,
    ) -> tuple[dict[str, Any], dict[str, Any]]:
        instance = self._component(component_id)
        if not hasattr(instance, "evaluate"):
            raise WorkerError("InvalidComponent", f"{component_id} does not define evaluate")

        result = instance.evaluate(inputs or {}, state or {}, params or {}, context or {})
        return self._validated_evaluate_result(component_id, "evaluate", result)

    def evaluate_component_batch(
        self,
        component_id: str,
        inputs: dict[str, Any] | None,
        state: dict[str, Any] | None,
        params: dict[str, Any] | None,
        context: dict[str, Any] | None,
    ) -> tuple[dict[str, Any], dict[str, Any]]:
        instance = self._component(component_id)
        method_name = "evaluate_batch"
        if hasattr(instance, method_name):
            method = instance.evaluate_batch
        elif hasattr(instance, "evaluate"):
            method_name = "evaluate"
            method = instance.evaluate
        else:
            raise WorkerError("InvalidComponent", f"{component_id} does not define evaluate_batch")

        result = method(inputs or {}, state or {}, params or {}, context or {})
        return self._validated_evaluate_result(component_id, method_name, result)

    def _validated_evaluate_result(
        self,
        component_id: str,
        method_name: str,
        result: Any,
    ) -> tuple[dict[str, Any], dict[str, Any]]:
        if not isinstance(result, tuple) or len(result) != 2:
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.{method_name} must return (outputs, state)",
            )

        outputs, next_state = result
        if not isinstance(outputs, dict):
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.{method_name} outputs must be a dict, got {type(outputs).__name__}",
            )
        if next_state is None:
            next_state = {}
        if not isinstance(next_state, dict):
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.{method_name} state must be a dict, got {type(next_state).__name__}",
            )

        return to_jsonable(outputs), to_jsonable(next_state)

    def _component(self, component_id: str) -> Any:
        record = self.components.get(component_id)
        if record is None:
            raise WorkerError("UnknownComponent", f"component is not loaded: {component_id}")
        return record.instance

    def _import_class(self, class_path: str) -> type:
        module_name, _, attr_path = class_path.rpartition(".")
        if not module_name or not attr_path:
            raise WorkerError("InvalidClassPath", f"class path must be module.ClassName: {class_path}")
        module = importlib.import_module(module_name)
        obj: Any = module
        for part in attr_path.split("."):
            obj = getattr(obj, part)
        if not callable(obj):
            raise WorkerError("InvalidClassPath", f"class path is not callable: {class_path}")
        return obj


def handle_request(host: ComponentHost, request: dict[str, Any]) -> dict[str, Any]:
    request_id = str(request.get("id", ""))
    request_type = request.get("type")

    try:
        if request_type == "ping":
            return {"id": request_id, "ok": True, "message": "pong"}
        if request_type == "load_component":
            component_id = str(request.get("component_id", ""))
            payload, logs = captured_component_call(
                component_id=component_id,
                stage="load",
                source=str(request.get("class", "")),
                callback=lambda: host.load_component(
                    component_id=component_id,
                    class_path=str(request.get("class", "")),
                    project_root=str(request.get("project_root", ".")),
                ),
            )
            return {"id": request_id, "ok": True, "logs": logs, **payload}
        if request_type == "initialize_component":
            component_id = str(request.get("component_id", ""))
            state, logs = captured_component_call(
                component_id=component_id,
                stage="initialize",
                source=component_source(host, component_id),
                callback=lambda: host.initialize_component(
                    component_id=component_id,
                    params=request.get("params") or {},
                    context=request.get("context") or {},
                ),
            )
            return {"id": request_id, "ok": True, "state": state, "logs": logs}
        if request_type == "evaluate_component":
            component_id = str(request.get("component_id", ""))
            (outputs, state), logs = captured_component_call(
                component_id=component_id,
                stage="evaluate",
                source=component_source(host, component_id),
                callback=lambda: host.evaluate_component(
                    component_id=component_id,
                    inputs=request.get("inputs") or {},
                    state=request.get("state") or {},
                    params=request.get("params") or {},
                    context=request.get("context") or {},
                ),
            )
            return {"id": request_id, "ok": True, "outputs": outputs, "state": state, "logs": logs}
        if request_type == "evaluate_component_batch":
            component_id = str(request.get("component_id", ""))
            (outputs, state), logs = captured_component_call(
                component_id=component_id,
                stage="evaluate_batch",
                source=component_source(host, component_id),
                callback=lambda: host.evaluate_component_batch(
                    component_id=component_id,
                    inputs=request.get("inputs") or {},
                    state=request.get("state") or {},
                    params=request.get("params") or {},
                    context=request.get("context") or {},
                ),
            )
            return {"id": request_id, "ok": True, "outputs": outputs, "state": state, "logs": logs}
        if request_type == "shutdown":
            return {"id": request_id, "ok": True, "message": "shutdown"}
        raise WorkerError("UnknownRequest", f"unknown request type: {request_type}")
    except CapturedComponentError as exc:
        if isinstance(exc.original, WorkerError):
            error = exc.original.to_dict()
        else:
            error = WorkerError(
                type(exc.original).__name__,
                str(exc.original),
                "".join(traceback.format_exception(type(exc.original), exc.original, exc.original.__traceback__)),
            ).to_dict()
        return {"id": request_id, "ok": False, "error": error, "logs": exc.logs}
    except WorkerError as exc:
        return {"id": request_id, "ok": False, "error": exc.to_dict()}
    except Exception as exc:  # noqa: BLE001 - worker boundary must report all user errors.
        error = WorkerError(type(exc).__name__, str(exc), traceback.format_exc())
        return {"id": request_id, "ok": False, "error": error.to_dict()}


def captured_component_call(
    component_id: str,
    stage: str,
    source: str,
    callback: Any,
) -> tuple[Any, list[dict[str, Any]]]:
    stdout = io.StringIO()
    stderr = io.StringIO()
    try:
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            result = callback()
    except Exception as exc:  # noqa: BLE001 - preserve user exception at worker boundary.
        logs = component_logs(component_id, stage, source, stdout.getvalue(), stderr.getvalue())
        raise CapturedComponentError(exc, logs) from exc
    return result, component_logs(component_id, stage, source, stdout.getvalue(), stderr.getvalue())


def component_source(host: ComponentHost, component_id: str) -> str:
    record = host.components.get(component_id)
    return record.class_path if record is not None else ""


def component_logs(component_id: str, stage: str, source: str, stdout: str, stderr: str) -> list[dict[str, Any]]:
    logs: list[dict[str, Any]] = []
    logs.extend(component_log_entries(component_id, stage, source, "stdout", "info", stdout))
    logs.extend(component_log_entries(component_id, stage, source, "stderr", "error", stderr))
    return logs


def component_log_entries(
    component_id: str,
    stage: str,
    source: str,
    stream: str,
    severity: str,
    text: str,
) -> list[dict[str, Any]]:
    entries: list[dict[str, Any]] = []
    for line in text.splitlines():
        message = line.rstrip()
        if not message:
            continue
        entries.append(
            {
                "component_id": component_id,
                "stage": stage,
                "stream": stream,
                "severity": severity,
                "message": message,
                "source": source,
            }
        )
    return entries


def stdio_loop() -> int:
    host = ComponentHost()
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        request: dict[str, Any] = {}
        try:
            request = json.loads(line)
        except json.JSONDecodeError as exc:
            response = {
                "id": "",
                "ok": False,
                "error": WorkerError("JSONDecodeError", str(exc)).to_dict(),
            }
        else:
            response = handle_request(host, request)

        print(json.dumps(response, separators=(",", ":")), flush=True)
        if request.get("type") == "shutdown":
            break
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="HVAC Studio Python worker")
    parser.add_argument("--stdio", action="store_true", help="run the JSONL stdio protocol")
    args = parser.parse_args()
    if args.stdio:
        return stdio_loop()
    parser.print_help()
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
