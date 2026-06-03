class PIResetController:
    input_nodes = {
        "measured_supply_temp_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
        "target_supply_temp_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
    }

    output_nodes = {
        "chw_setpoint_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
        },
        "control_effort_k": {
            "medium": "signal",
            "value_type": "float",
            "unit": "K",
        },
    }

    parameter_schema = {
        "base_setpoint_c": {"type": "float", "unit": "degC", "required": True},
        "kp": {"type": "float", "required": True},
        "ki": {"type": "float", "required": True},
        "min_setpoint_c": {"type": "float", "unit": "degC", "required": True},
        "max_setpoint_c": {"type": "float", "unit": "degC", "required": True},
    }

    def initialize(self, params, context):
        return {
            "calls": 0,
            "integral_error": 0.0,
            "last_error": 0.0,
        }

    def evaluate(self, inputs, state, params, context):
        dt_minutes = float(context.get("dt", 60.0)) / 60.0
        error = inputs["measured_supply_temp_c"] - inputs["target_supply_temp_c"]
        integral_error = state.get("integral_error", 0.0) + error * dt_minutes
        effort = params["kp"] * error + params["ki"] * integral_error
        raw_setpoint = params["base_setpoint_c"] - effort
        setpoint = max(params["min_setpoint_c"], min(params["max_setpoint_c"], raw_setpoint))

        return {
            "chw_setpoint_c": setpoint,
            "control_effort_k": effort,
        }, {
            "calls": state.get("calls", 0) + 1,
            "integral_error": integral_error,
            "last_error": error,
        }
