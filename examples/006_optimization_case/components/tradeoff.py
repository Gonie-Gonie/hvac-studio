class SetpointTradeoff:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        load_power_kw = inputs["building_load_kw"] / params["cop"]
        setpoint_credit_kw = (inputs["chw_setpoint_c"] - params["reference_setpoint_c"]) * params["power_credit_kw_per_k"]
        chiller_power_kw = load_power_kw - setpoint_credit_kw
        comfort_delta = max(0.0, inputs["chw_setpoint_c"] - params["comfort_limit_c"])
        comfort_penalty_kw = comfort_delta * comfort_delta * params["comfort_penalty_kw_per_k2"]

        return {
            "chiller_power_kw": chiller_power_kw,
            "comfort_penalty_kw": comfort_penalty_kw,
            "objective_kw": chiller_power_kw + comfort_penalty_kw,
        }, state
