from .helpers import predict_zone_load, predict_zone_temperature


def step(inputs, state, params, context):
    features = {
        "outdoor_temperature_c": float(inputs["outdoor_temperature_c"]),
        "solar_gain_kw": float(inputs["solar_gain_kw"]),
        "internal_gain_kw": float(inputs["internal_gain_kw"]),
        "zone_setpoint_c": float(inputs["zone_setpoint_c"]),
    }
    previous_zone = float(state.get("zone_temperature_c", params.get("initial_zone_temperature_c", 24.0)))
    zone_load = predict_zone_load(features, params)
    next_zone = predict_zone_temperature(features, previous_zone, zone_load, params)
    next_state = dict(state)
    next_state["zone_temperature_c"] = next_zone
    return {
        "zone_load_kw": round(zone_load, 6),
        "zone_temperature_c": round(next_zone, 6),
        "zone_setpoint_c": features["zone_setpoint_c"],
        "outdoor_temperature_c": features["outdoor_temperature_c"],
    }, next_state
