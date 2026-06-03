from .helpers import sample_time_seconds


def step(inputs, state, params, context):
    setpoint = float(inputs["setpoint"])
    measurement = float(inputs["measurement"])
    error = setpoint - measurement
    dt = sample_time_seconds(context)
    integral = float(state.get("integral", 0.0)) + error * dt
    command = float(params.get("kp", 1.0)) * error + float(params.get("ki", 0.1)) * integral
    next_state = {"integral": integral}
    return {"command": command, "integral": integral}, next_state
