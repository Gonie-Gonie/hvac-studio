# Unit Conversion Component

This example shows explicit connection-level unit conversion. The source
component emits `power_w` in W, the target component accepts `power_kw` in kW,
and the connection declares a linear conversion factor of `0.001`.

The runtime applies the conversion before evaluating the target component. The
project graph still records the source and target node units explicitly.
