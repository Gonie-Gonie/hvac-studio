export const CANVAS_NODE_WIDTH = 300;
export const CANVAS_NODE_HEIGHT = 220;
export const CANVAS_NODE_ANCHOR_Y = 92;
export const CANVAS_NODE_FIRST_PORT_Y = 84;
export const CANVAS_NODE_PORT_GAP = 42;
export const CANVAS_COLUMN_GAP = 370;
export const CANVAS_ROW_GAP = 250;
export const CANVAS_PADDING = 96;

export const COMPONENT_CATEGORIES = [
  ["", "Any category"],
  ["physical_component", "Physical Component"],
  ["controller", "Controller"],
  ["data_source", "Data Source"],
  ["data_sink", "Data Sink"],
  ["utility", "Utility"],
  ["solver", "Solver"],
  ["composite_wrapper", "Composite Wrapper"],
];

export const EXECUTION_MODES = [
  ["", "Any mode"],
  ["step", "Step"],
  ["vectorized", "Vectorized"],
  ["external_executable", "External Executable"],
  ["initialization_only", "Initialization Only"],
];

export const NODE_PRESETS = [
  ["", "Custom", {}],
  ["water_inlet", "Water Inlet", { direction: "input", id: "water_inlet", name: "Water Inlet", medium: "water", value_type: "float" }],
  ["water_outlet", "Water Outlet", { direction: "output", id: "water_outlet", name: "Water Outlet", medium: "water", value_type: "float" }],
  ["air_inlet", "Air Inlet", { direction: "input", id: "air_inlet", name: "Air Inlet", medium: "air", value_type: "float" }],
  ["air_outlet", "Air Outlet", { direction: "output", id: "air_outlet", name: "Air Outlet", medium: "air", value_type: "float" }],
  ["control_signal_input", "Control Signal Input", { direction: "input", id: "control_signal", name: "Control Signal", medium: "control", value_type: "float", default: 0 }],
  ["electric_power_output", "Electric Power Output", { direction: "output", id: "electric_power", name: "Electric Power", medium: "electric", value_type: "float", unit: "W" }],
  ["scalar_input", "Scalar Input", { direction: "input", id: "value", name: "Value", medium: "signal", value_type: "float", default: 0 }],
  ["scalar_output", "Scalar Output", { direction: "output", id: "result", name: "Result", medium: "signal", value_type: "float" }],
  ["time_series_input", "Time Series Input", { direction: "input", id: "series", name: "Time Series", medium: "signal", value_type: "object", default: [] }],
];

export const ML_MODEL_FORMATS = ["custom", "pickle", "joblib", "onnx", "torch", "tensorflow"];

export const ML_ASSET_FIELDS = [
  ["model_file", "Model File"],
  ["input_scaler_file", "Input Scaler"],
  ["output_scaler_file", "Output Scaler"],
  ["feature_schema_file", "Feature Schema"],
  ["target_schema_file", "Target Schema"],
  ["training_metadata_file", "Training Metadata"],
  ["validation_report_file", "Validation Report"],
];

export const UNIT_CONVERSION_PRESETS = [
  ["custom", "Custom", null],
  ["w_to_kw", "W to kW", { factor: 0.001, offset: 0, description: "Convert W to kW." }],
  ["kw_to_w", "kW to W", { factor: 1000, offset: 0, description: "Convert kW to W." }],
  ["degc_to_k", "degC to K", { factor: 1, offset: 273.15, description: "Convert degC to K." }],
  ["kgs_to_kgh", "kg/s to kg/h", { factor: 3600, offset: 0, description: "Convert kg/s to kg/h." }],
  ["fraction_to_percent", "fraction to percent", { factor: 100, offset: 0, description: "Convert fraction to percent." }],
];

export const WORKSPACE_HELP = {
  start: "/docs/user/quick-start.md",
  canvas: "/docs/user/build-system.md",
  code: "/docs/user/edit-python-function.md",
  parameters: "/docs/user/parameter-management.md",
  artifacts: "/docs/user/how-it-works.md",
  run: "/docs/user/run-simulation.md",
  export: "/docs/user/export-runtime.md",
};

export const RESULT_HELP = {
  calibration: "/docs/user/calibration.md",
  dataValidation: "/docs/user/data-validation.md",
  optimization: "/docs/user/optimization.md",
  parameterManagement: "/docs/user/parameter-management.md",
  run: "/docs/user/run-simulation.md",
};
