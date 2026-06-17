FEATURE_ORDER = [
    "outdoor_temperature_c",
    "return_air_temperature_c",
    "chw_setpoint_c",
    "fan_speed_fraction",
]


def ordered_features(inputs):
    return {name: float(inputs[name]) for name in FEATURE_ORDER}
