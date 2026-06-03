from .helpers import clamp


def step(inputs, state, params, context):
    setpoint = float(inputs["setpoint"])
    measurement = float(inputs["measurement"])
    bias = float(params.get("bias", 0.0))
    gain = float(params.get("gain", 1.0))
    command = bias + gain * (setpoint - measurement)
    return {"command": clamp(command, 0.0, 1.0)}, state
