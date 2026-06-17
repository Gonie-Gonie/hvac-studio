# RC AHU ANN Composition Example

This example combines a small RC zone model, supervisory reset logic, an AHU ANN surrogate, a chiller-like equipment model, a variable-speed pump, and an output aggregator.

It is intended to exercise the full practical workflow:

- public inputs drive a composed system without hand-editing runner internals;
- the ANN component declares local model assets through `ml_metadata`;
- validation maps measured columns onto public outputs;
- calibration tunes a numeric equipment parameter;
- optimization searches chilled-water setpoint and pump speed.

Run the default case:

```powershell
go run ./cmd/bcs-runner run --project ../../examples/015_rc_ahu_ann_composition/project.bcsproj --input ../../examples/015_rc_ahu_ann_composition/inputs/case01.json
```

Run the workflow checks:

```powershell
go run ./cmd/bcs-runner validate-data --project ../../examples/015_rc_ahu_ann_composition/project.bcsproj --mapping validation/mappings/rc_ahu_validation.json
go run ./cmd/bcs-runner calibrate --project ../../examples/015_rc_ahu_ann_composition/project.bcsproj --setup calibration/setups/chiller_cop_grid.json
go run ./cmd/bcs-runner optimize --project ../../examples/015_rc_ahu_ann_composition/project.bcsproj --setup optimization/setups/chw_pump_grid.json
```

Run the SDK example after placing `bcs-runner.exe` on `PATH`, or pass the runner
path as the first argument:

```powershell
python ../../examples/015_rc_ahu_ann_composition/scripts/run_sdk_case.py bcs-runner.exe
```

Studio walkthrough:

1. Copy this example into a workspace project.
2. Open the Canvas and run `case01`.
3. Select `RC Zone Load`.
4. In the Project panel, choose `Zone Load ANN Surrogate`.
5. Confirm the Inspector `Replacement Preview` shows no missing inputs or
   outputs.
6. Keep `Copy same-name parameters` enabled and select `Replace And Validate`.
7. Run `case01` again and compare before/after public outputs in Run Output.

The replacement path is intentionally contract-compatible: `ZoneLoadANN`
preserves the `ZoneLoadRC` input and output node IDs so public IO and existing
connections are rewired automatically.
