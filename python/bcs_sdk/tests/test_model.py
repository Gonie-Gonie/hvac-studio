from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from bcs_sdk.model import load_project


class ModelTests(unittest.TestCase):
    def test_load_project(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "project.bcsproj"
            path.write_text(json.dumps({"project_name": "case"}), encoding="utf-8")
            self.assertEqual(load_project(path)["project_name"], "case")


if __name__ == "__main__":
    unittest.main()

