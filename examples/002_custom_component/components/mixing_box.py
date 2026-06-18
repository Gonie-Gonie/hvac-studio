class MixingBox:
    input_nodes = {
        "outdoor_air_temp_c": {
            "medium": "air",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
        "return_air_temp_c": {
            "medium": "air",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
        "outdoor_air_fraction": {
            "medium": "signal",
            "value_type": "float",
            "unit": "fraction",
            "required": True,
        },
        "supply_airflow_kg_s": {
            "medium": "air",
            "value_type": "float",
            "unit": "kg/s",
            "required": True,
        },
        "outdoor_air_co2_ppm": {
            "medium": "air",
            "value_type": "float",
            "unit": "ppm",
            "required": True,
        },
        "return_air_co2_ppm": {
            "medium": "air",
            "value_type": "float",
            "unit": "ppm",
            "required": True,
        },
    }

    output_nodes = {
        "mixed_air_temp_c": {
            "medium": "air",
            "value_type": "float",
            "unit": "degC",
        },
        "outdoor_airflow_kg_s": {
            "medium": "air",
            "value_type": "float",
            "unit": "kg/s",
        },
        "return_airflow_kg_s": {
            "medium": "air",
            "value_type": "float",
            "unit": "kg/s",
        },
        "mixed_air_co2_ppm": {
            "medium": "air",
            "value_type": "float",
            "unit": "ppm",
        },
        "effective_outdoor_air_fraction": {
            "medium": "signal",
            "value_type": "float",
            "unit": "fraction",
        },
    }

    parameter_schema = {
        "minimum_outdoor_air_fraction": {
            "type": "float",
            "required": True,
        },
        "maximum_outdoor_air_fraction": {
            "type": "float",
            "required": True,
        },
    }

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        requested_fraction = float(inputs["outdoor_air_fraction"])
        minimum_fraction = float(params["minimum_outdoor_air_fraction"])
        maximum_fraction = float(params["maximum_outdoor_air_fraction"])
        outdoor_fraction = min(max(requested_fraction, minimum_fraction), maximum_fraction)

        supply_airflow = float(inputs["supply_airflow_kg_s"])
        outdoor_airflow = supply_airflow * outdoor_fraction
        return_airflow = supply_airflow - outdoor_airflow

        return_fraction = 1.0 - outdoor_fraction
        mixed_air_temp = (
            float(inputs["outdoor_air_temp_c"]) * outdoor_fraction
            + float(inputs["return_air_temp_c"]) * return_fraction
        )
        mixed_air_co2 = (
            float(inputs["outdoor_air_co2_ppm"]) * outdoor_fraction
            + float(inputs["return_air_co2_ppm"]) * return_fraction
        )

        return {
            "mixed_air_temp_c": mixed_air_temp,
            "outdoor_airflow_kg_s": outdoor_airflow,
            "return_airflow_kg_s": return_airflow,
            "mixed_air_co2_ppm": mixed_air_co2,
            "effective_outdoor_air_fraction": outdoor_fraction,
        }, state
