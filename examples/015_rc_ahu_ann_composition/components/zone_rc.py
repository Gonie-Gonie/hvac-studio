class ZoneLoadRC:
    def initialize(self, params, context):
        return {
            "zone_temperature_c": float(params["initial_zone_temperature_c"]),
        }

    def evaluate(self, inputs, state, params, context):
        outdoor = float(inputs["outdoor_temperature_c"])
        solar_gain = float(inputs["solar_gain_kw"])
        internal_gain = float(inputs["internal_gain_kw"])
        setpoint = float(inputs["zone_setpoint_c"])
        previous_zone = float(state.get("zone_temperature_c", params["initial_zone_temperature_c"]))

        envelope_load = max(outdoor - setpoint, 0.0) * float(params["ua_kw_per_k"])
        zone_load = float(params["base_load_kw"]) + envelope_load + solar_gain + internal_gain

        drift = (outdoor - previous_zone) * float(params["outdoor_coupling"])
        gains = (solar_gain + internal_gain) * float(params["gain_temperature_per_kw"])
        cooling = max(zone_load - float(params["nominal_capacity_kw"]), 0.0) * float(params["cooling_temperature_per_kw"])
        next_zone = previous_zone + drift + gains - cooling

        next_state = dict(state)
        next_state["zone_temperature_c"] = next_zone
        return {
            "zone_load_kw": round(zone_load, 6),
            "zone_temperature_c": round(next_zone, 6),
            "zone_setpoint_c": setpoint,
            "outdoor_temperature_c": outdoor,
        }, next_state
