import json
from pathlib import Path


def load_model_assets():
    root = Path(__file__).resolve().parent
    with (root / "model.json").open("r", encoding="utf-8") as handle:
        model = json.load(handle)
    with (root / "feature_schema.json").open("r", encoding="utf-8") as handle:
        feature_schema = json.load(handle)
    with (root / "target_schema.json").open("r", encoding="utf-8") as handle:
        target_schema = json.load(handle)
    return {
        "model": model,
        "feature_schema": feature_schema,
        "target_schema": target_schema,
    }


def evaluate_model(loaded, features, bias):
    model = loaded["model"]
    feature_order = loaded["feature_schema"]["features"]
    outputs = {}
    for name, spec in model["outputs"].items():
        value = float(spec.get("intercept", 0.0)) + bias
        coefficients = spec.get("coefficients", {})
        for feature in feature_order:
            value += float(coefficients.get(feature, 0.0)) * float(features[feature])
        outputs[name] = round(value, 6)
    return outputs
