from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROJECT = ROOT / "examples" / "001_scalar_component" / "project.bcsproj"
REQUESTS = Path(__file__).with_name("serve-requests.jsonl")


def main() -> int:
    runner_command = sys.argv[1:] or ["bcs-runner.exe"]
    process = subprocess.Popen(
        [*runner_command, "serve", "--project", str(PROJECT)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )
    try:
        assert process.stdin is not None
        assert process.stdout is not None
        with REQUESTS.open("r", encoding="utf-8") as handle:
            for line in handle:
                if not line.strip():
                    continue
                process.stdin.write(line)
                process.stdin.flush()
                response_line = process.stdout.readline()
                if not response_line:
                    stderr = process.stderr.read() if process.stderr is not None else ""
                    raise RuntimeError(f"serve closed without a response: {stderr.strip()}")
                response = json.loads(response_line)
                print(json.dumps(response, sort_keys=True))
        return process.wait(timeout=5)
    finally:
        if process.poll() is None:
            process.kill()


if __name__ == "__main__":
    raise SystemExit(main())
