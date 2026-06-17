class FeatureMapper:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        features = {
            "outdoor_temperature_c": float(inputs["outdoor_temperature_c"]),
            "zone_load_kw": float(inputs["zone_load_kw"]),
            "chw_setpoint_c": float(inputs["chw_setpoint_c"]),
            "pump_speed_fraction": float(inputs["pump_speed_fraction"]),
            "cooling_request_kw": float(inputs["cooling_request_kw"]),
        }
        return {"features": features}, state
