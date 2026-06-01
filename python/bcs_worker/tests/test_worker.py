from __future__ import annotations

import textwrap
import tempfile
import unittest
from pathlib import Path

from bcs_worker.worker import ComponentHost


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


if __name__ == "__main__":
    unittest.main()

