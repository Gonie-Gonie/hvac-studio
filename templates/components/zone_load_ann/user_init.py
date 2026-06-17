def initialize(params, context):
    return {
        "zone_temperature_c": float(params.get("initial_zone_temperature_c", 24.0)),
    }
