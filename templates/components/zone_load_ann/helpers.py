def predict_zone_load(features, params):
    outdoor = features["outdoor_temperature_c"]
    solar = features["solar_gain_kw"]
    internal = features["internal_gain_kw"]
    setpoint = features["zone_setpoint_c"]
    envelope = max(outdoor - setpoint, 0.0) * float(params.get("ua_kw_per_k", 0.45))
    raw = float(params.get("base_load_kw", 18.0)) + envelope + solar + internal
    return raw + float(params.get("ann_bias_kw", 0.0))


def predict_zone_temperature(features, previous_zone, zone_load, params):
    outdoor = features["outdoor_temperature_c"]
    solar = features["solar_gain_kw"]
    internal = features["internal_gain_kw"]
    drift = (outdoor - previous_zone) * float(params.get("outdoor_coupling", 0.05))
    gains = (solar + internal) * float(params.get("gain_temperature_per_kw", 0.02))
    cooling = max(zone_load - float(params.get("nominal_capacity_kw", 20.0)), 0.0) * float(params.get("cooling_temperature_per_kw", 0.03))
    return previous_zone + drift + gains - cooling + float(params.get("ann_temperature_bias_c", 0.0))
