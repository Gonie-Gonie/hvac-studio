from __future__ import annotations

import contextlib
import shutil
import textwrap
import unittest
import uuid
from pathlib import Path

from bcs_worker.worker import ComponentHost, handle_request


@contextlib.contextmanager
def temporary_project_dir():
    temp_root = Path(__file__).resolve().parents[3] / ".tmp" / "python-worker-tests"
    temp_root.mkdir(parents=True, exist_ok=True)
    root = temp_root / f"case-{uuid.uuid4().hex}"
    root.mkdir()
    try:
        yield str(root)
    finally:
        shutil.rmtree(root, ignore_errors=True)


class ComponentHostTests(unittest.TestCase):
    def test_load_initialize_and_evaluate_component(self) -> None:
        with temporary_project_dir() as tmp:
            root = Path(tmp)
            components = root / "components"
            components.mkdir()
            (components / "__init__.py").write_text("", encoding="utf-8")
            (components / "gain.py").write_text(
                textwrap.dedent(
                    """
                    class Gain:
                        def initialize(self, params, context):
                            return {"calls": 0}

                        def evaluate(self, inputs, state, params, context):
                            calls = state.get("calls", 0) + 1
                            return {"result": inputs["value"] * params["gain"]}, {"calls": calls}
                    """
                ),
                encoding="utf-8",
            )

            host = ComponentHost()
            host.load_component("gain", "components.gain.Gain", str(root))
            state = host.initialize_component("gain", {"gain": 2}, {})
            outputs, next_state = host.evaluate_component("gain", {"value": 3}, state, {"gain": 2}, {})

            self.assertEqual(outputs, {"result": 6})
            self.assertEqual(next_state, {"calls": 1})

    def test_evaluate_component_batch_prefers_batch_method(self) -> None:
        with temporary_project_dir() as tmp:
            root = Path(tmp)
            components = root / "vector_components"
            components.mkdir()
            (components / "__init__.py").write_text("", encoding="utf-8")
            (components / "vector_gain.py").write_text(
                textwrap.dedent(
                    """
                    class VectorGain:
                        def evaluate(self, inputs, state, params, context):
                            return {"results": ["step"]}, state

                        def evaluate_batch(self, inputs, state, params, context):
                            gain = params.get("gain", 1)
                            return {"results": [value * gain for value in inputs["values"]]}, {"mode": "batch"}
                    """
                ),
                encoding="utf-8",
            )

            host = ComponentHost()
            host.load_component("vector_gain", "vector_components.vector_gain.VectorGain", str(root))
            outputs, next_state = host.evaluate_component_batch(
                "vector_gain",
                {"values": [1, 2, 3]},
                {},
                {"gain": 2},
                {},
            )

            self.assertEqual(outputs, {"results": [2, 4, 6]})
            self.assertEqual(next_state, {"mode": "batch"})

    def test_handle_request_captures_component_logs(self) -> None:
        with temporary_project_dir() as tmp:
            root = Path(tmp)
            components = root / "noisy_components"
            components.mkdir()
            (components / "__init__.py").write_text("", encoding="utf-8")
            (components / "talker.py").write_text(
                textwrap.dedent(
                    """
                    import sys

                    class Talker:
                        def evaluate(self, inputs, state, params, context):
                            print("hello from stdout")
                            print("warning from stderr", file=sys.stderr)
                            return {"result": inputs["value"]}, state
                    """
                ),
                encoding="utf-8",
            )

            host = ComponentHost()
            load_response = handle_request(
                host,
                {
                    "id": "load",
                    "type": "load_component",
                    "component_id": "talker",
                    "class": "noisy_components.talker.Talker",
                    "project_root": str(root),
                },
            )
            self.assertTrue(load_response["ok"])

            response = handle_request(
                host,
                {
                    "id": "eval",
                    "type": "evaluate_component",
                    "component_id": "talker",
                    "inputs": {"value": 4},
                    "state": {},
                    "params": {},
                    "context": {},
                },
            )

            self.assertTrue(response["ok"])
            self.assertEqual(response["outputs"], {"result": 4})
            self.assertEqual(
                response["logs"],
                [
                    {
                        "component_id": "talker",
                        "stage": "evaluate",
                        "stream": "stdout",
                        "severity": "info",
                        "message": "hello from stdout",
                        "source": "noisy_components.talker.Talker",
                    },
                    {
                        "component_id": "talker",
                        "stage": "evaluate",
                        "stream": "stderr",
                        "severity": "error",
                        "message": "warning from stderr",
                        "source": "noisy_components.talker.Talker",
                    },
                ],
            )

    def test_handle_request_captures_batch_logs(self) -> None:
        with temporary_project_dir() as tmp:
            root = Path(tmp)
            components = root / "batch_components"
            components.mkdir()
            (components / "__init__.py").write_text("", encoding="utf-8")
            (components / "talker.py").write_text(
                textwrap.dedent(
                    """
                    class BatchTalker:
                        def evaluate_batch(self, inputs, state, params, context):
                            print("hello from batch")
                            return {"results": inputs["values"]}, state
                    """
                ),
                encoding="utf-8",
            )

            host = ComponentHost()
            load_response = handle_request(
                host,
                {
                    "id": "load",
                    "type": "load_component",
                    "component_id": "batch_talker",
                    "class": "batch_components.talker.BatchTalker",
                    "project_root": str(root),
                },
            )
            self.assertTrue(load_response["ok"])

            response = handle_request(
                host,
                {
                    "id": "batch",
                    "type": "evaluate_component_batch",
                    "component_id": "batch_talker",
                    "inputs": {"values": [1, 2]},
                    "state": {},
                    "params": {},
                    "context": {},
                },
            )

            self.assertTrue(response["ok"])
            self.assertEqual(response["outputs"], {"results": [1, 2]})
            self.assertEqual(
                response["logs"],
                [
                    {
                        "component_id": "batch_talker",
                        "stage": "evaluate_batch",
                        "stream": "stdout",
                        "severity": "info",
                        "message": "hello from batch",
                        "source": "batch_components.talker.BatchTalker",
                    },
                ],
            )


if __name__ == "__main__":
    unittest.main()
