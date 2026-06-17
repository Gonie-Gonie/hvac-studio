# Unit Conversion Component

This example shows explicit connection-level unit conversion. The source
component emits `power_w` in W, the target component accepts `power_kw` in kW,
and the connection declares a linear conversion factor of `0.001`.

The runtime applies the conversion before evaluating the target component. The
project graph still records the source and target node units explicitly, and
run traces keep both the source value and the converted target value.

In Studio, select the connection to edit the same conversion from the Inspector.
The preset list includes W to kW and the preview shows the converted sample
before saving.
