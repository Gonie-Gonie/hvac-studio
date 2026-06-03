# 003 Feed-Forward System

This example verifies acyclic component-node execution across multiple user-defined Python components.

```text
building_load_kw public input
  -> LoadModel.adjusted_load_kw
  -> Controller.chw_setpoint_c
  -> Chiller
  -> Aggregator.total_power_kw public output
```

The same adjusted load also feeds the chiller directly. The example is intentionally scalar and small; its job is to test graph compilation, connection propagation, public IO mapping, and result inspection.

Runner output includes `component_inputs`, `component_outputs`, `node_values`, and `connection_values` so the GUI and CLI consumers can inspect how each node and connection was evaluated.

Expected public outputs for `inputs/case01.json`:

```json
{
  "total_power_kw": 122.0,
  "chiller_power_kw": 110.0,
  "chw_supply_temp_c": 7.15
}
```
