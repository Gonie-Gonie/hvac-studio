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


def evaluate_model(loaded, inputs, bias):
    model = loaded["model"]
    feature_schema = loaded["feature_schema"]
    target_schema = loaded["target_schema"]
    features = extract_features(inputs, feature_schema)
    prepared = preprocess_features(features, model)
    raw_outputs = run_inference(model, prepared, bias)
    return postprocess_outputs(raw_outputs, target_schema)


def extract_features(inputs, feature_schema):
    raw_features = inputs["features"]
    if not isinstance(raw_features, dict):
        raise TypeError("ML input 'features' must be an object")
    ordered = {}
    for feature in feature_schema["features"]:
        if feature not in raw_features:
            raise KeyError(f"missing ML feature: {feature}")
        ordered[feature] = float(raw_features[feature])
    return ordered


def preprocess_features(features, model):
    return features


def run_inference(model, features, bias):
    outputs = {}
    for name, spec in model["outputs"].items():
        value = float(spec.get("intercept", 0.0)) + bias
        coefficients = spec.get("coefficients", {})
        for feature, feature_value in features.items():
            value += float(coefficients.get(feature, 0.0)) * float(feature_value)
        outputs[name] = round(value, 6)
    return outputs


def postprocess_outputs(outputs, target_schema):
    targets = target_schema.get("targets") or list(outputs.keys())
    ordered = {}
    for target in targets:
        if isinstance(target, dict):
            target_id = target.get("id") or target.get("name")
        else:
            target_id = target
        if target_id not in outputs:
            raise KeyError(f"missing ML output: {target_id}")
        ordered[target_id] = outputs[target_id]
    return ordered
