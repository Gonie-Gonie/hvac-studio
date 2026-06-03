def as_sequence(value):
    if isinstance(value, (list, tuple)):
        return value
    return [value]
