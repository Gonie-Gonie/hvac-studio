import json
import sys


def main():
    request = json.load(sys.stdin)
    inputs = request.get("inputs", {})
    params = request.get("params", {})
    state = request.get("state", {})
    context = request.get("context", {})

    value = float(inputs["request"])
    gain = float(params.get("gain", 1.0))
    calls = int(state.get("calls", 0)) + 1
    print(f"external gain stderr call {calls}", file=sys.stderr)

    response = {
        "ok": True,
        "outputs": {
            "response": value * gain,
        },
        "state": {
            "calls": calls,
        },
        "logs": [
            {
                "severity": "info",
                "message": f"external gain evaluated call {calls}",
                "source": "components/external_gain/external_gain.py",
                "time": context.get("time"),
            }
        ],
    }
    print(json.dumps(response, separators=(",", ":")))


if __name__ == "__main__":
    main()
