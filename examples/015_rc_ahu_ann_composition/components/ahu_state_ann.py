import json
from pathlib import Path


class AHUStateANN:
    def initialize(self, params, context):
        root = Path(__file__).resolve().parents[1] / "assets" / "ahu_state_ann"
        with (root / "model.json").open("r", encoding="utf-8") as handle:
            model = json.load(handle)
        with (root / "feature_schema.json").open("r", encoding="utf-8") as handle:
            feature_schema = json.load(handle)
        return {
            "model": model,
            "feature_order": feature_schema["features"],
        }

    def evaluate(self, inputs, state, params, context):
        features = inputs["features"]
        model = state["model"]
        outputs = {}
        for output_id, spec in model["outputs"].items():
            value = float(spec.get("intercept", 0.0)) + float(params.get("output_bias", 0.0))
            coefficients = spec.get("coefficients", {})
            for feature_id in state["feature_order"]:
                value += float(coefficients.get(feature_id, 0.0)) * float(features[feature_id])
            outputs[output_id] = round(value, 6)
        return outputs, state
