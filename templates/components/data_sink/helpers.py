def latest_values(context):
    values = context.get("sink_values")
    if isinstance(values, list):
        return values
    return []
