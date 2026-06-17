FEATURE_ORDER = [
    "outdoor_temperature_c",
    "return_air_temperature_c",
    "chw_setpoint_c",
    "fan_speed_fraction",
]


def ordered_features(inputs, feature_config=None):
    feature_config = feature_config or {}
    features = {}
    for name in FEATURE_ORDER:
        spec = feature_config.get(name, {})
        source = spec.get("source", name)
        if source not in inputs:
            raise KeyError(f"missing feature input: {source}")
        value = float(inputs[source])
        value = value * float(spec.get("scale", 1.0)) + float(spec.get("offset", 0.0))
        if spec.get("min") is not None:
            value = max(value, float(spec["min"]))
        if spec.get("max") is not None:
            value = min(value, float(spec["max"]))
        features[name] = value
    return features
