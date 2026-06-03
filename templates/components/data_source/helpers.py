def context_value(context, key, default):
    if key in context:
        return context[key]
    return default
