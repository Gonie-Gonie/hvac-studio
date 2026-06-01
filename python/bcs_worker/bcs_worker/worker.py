"""Persistent JSONL stdio worker for user-defined Python components."""

from __future__ import annotations

import argparse
import importlib
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
        if not isinstance(result, tuple) or len(result) != 2:
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.evaluate must return (outputs, state)",
            )

        outputs, next_state = result
        if not isinstance(outputs, dict):
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.evaluate outputs must be a dict, got {type(outputs).__name__}",
            )
        if next_state is None:
            next_state = {}
        if not isinstance(next_state, dict):
            raise WorkerError(
                "InvalidComponentReturn",
                f"{component_id}.evaluate state must be a dict, got {type(next_state).__name__}",
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
            payload = host.load_component(
                component_id=str(request.get("component_id", "")),
                class_path=str(request.get("class", "")),
                project_root=str(request.get("project_root", ".")),
            )
            return {"id": request_id, "ok": True, **payload}
        if request_type == "initialize_component":
            state = host.initialize_component(
                component_id=str(request.get("component_id", "")),
                params=request.get("params") or {},
                context=request.get("context") or {},
            )
            return {"id": request_id, "ok": True, "state": state}
        if request_type == "evaluate_component":
            outputs, state = host.evaluate_component(
                component_id=str(request.get("component_id", "")),
                inputs=request.get("inputs") or {},
                state=request.get("state") or {},
                params=request.get("params") or {},
                context=request.get("context") or {},
            )
            return {"id": request_id, "ok": True, "outputs": outputs, "state": state}
        if request_type == "shutdown":
            return {"id": request_id, "ok": True, "message": "shutdown"}
        raise WorkerError("UnknownRequest", f"unknown request type: {request_type}")
    except WorkerError as exc:
        return {"id": request_id, "ok": False, "error": exc.to_dict()}
    except Exception as exc:  # noqa: BLE001 - worker boundary must report all user errors.
        error = WorkerError(type(exc).__name__, str(exc), traceback.format_exc())
        return {"id": request_id, "ok": False, "error": error.to_dict()}


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
