from __future__ import annotations

import textwrap
import tempfile
import unittest
from pathlib import Path

from bcs_worker.worker import ComponentHost, handle_request


class ComponentHostTests(unittest.TestCase):
    def test_load_initialize_and_evaluate_component(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
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

    def test_handle_request_captures_component_logs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
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
                    },
                    {
                        "component_id": "talker",
                        "stage": "evaluate",
                        "stream": "stderr",
                        "severity": "error",
                        "message": "warning from stderr",
                    },
                ],
            )


if __name__ == "__main__":
    unittest.main()
