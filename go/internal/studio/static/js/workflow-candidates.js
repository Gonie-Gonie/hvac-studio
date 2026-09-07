import { finiteNumber } from "./format.js";
import { defaultCalibrationGridStep } from "./setup-editor-ui.js";

export function calibrationParameterCandidates(graph) {
  const candidates = [];
  for (const component of graph?.components || []) {
    const definitions = component.parameter_defs || {};
    const names = [...new Set([...Object.keys(component.parameters || {}), ...Object.keys(definitions)])].sort();
    for (const name of names) {
      const definition = definitions[name] || {};
      const bounds = definition.bounds || {};
      const min = finiteNumber(bounds.min);
      const max = finiteNumber(bounds.max);
      const hasBounds = min !== null && max !== null && max >= min;
      const current = component.parameters?.[name] ?? definition.current ?? definition.default ?? "";
      const role = definition.role || "fixed";
      const selected = role === "calibration_target" && hasBounds && finiteNumber(current) !== null;
      candidates.push({
        component: component.id,
        componentName: component.name || component.id,
        name,
        role,
        unit: definition.unit || "",
        current,
        min,
        max,
        step: hasBounds ? defaultCalibrationGridStep(min, max) : "",
        hasBounds,
        selected,
      });
    }
  }
  return candidates;
}

export function optimizationPublicOutputs(system) {
  return (system?.public_outputs || []).filter((output) => isNumericValueType(output.value_type || output.valueType || "float"));
}

export function optimizationDecisionCandidates(graph, system, baseInputs = {}) {
  const candidates = [];
  const runInputs = baseInputs || {};
  for (const input of system?.public_inputs || []) {
    const value = finiteNumber(runInputs[input.id]);
    if (value === null) continue;
    const [min, max] = defaultDecisionBounds(value);
    const name = input.id || "";
    candidates.push({
      kind: "public_input",
      label: name,
      name,
      component: "",
      role: "public_input",
      unit: input.unit || "",
      current: value,
      min,
      max,
      step: defaultCalibrationGridStep(min, max),
      selected: /setpoint|speed|fraction|load/i.test(name),
    });
  }
  const systemComponentIDs = new Set(system?.components || []);
  const systemID = system?.id || "system";
  for (const component of graph?.components || []) {
    const definitions = component.parameter_defs || {};
    for (const name of Object.keys(definitions).sort()) {
      const definition = definitions[name] || {};
      const current = finiteNumber(component.parameters?.[name] ?? definition.current ?? definition.default);
      const min = finiteNumber(definition.bounds?.min);
      const max = finiteNumber(definition.bounds?.max);
      if (current === null || min === null || max === null || max < min) continue;
      const candidate = {
        kind: "component_parameter",
        label: `${component.id}.${name}`,
        name,
        component: component.id,
        role: definition.role || "fixed",
        unit: definition.unit || "",
        current,
        min,
        max,
        step: defaultCalibrationGridStep(min, max),
        selected: definition.role === "optimization_variable",
      };
      candidates.push(candidate);
      if (systemComponentIDs.has(component.id)) {
        candidates.push({
          ...candidate,
          kind: "system_parameter",
          label: `${systemID}.${component.id}.${name}`,
          selected: false,
        });
      }
    }
  }
  return candidates;
}

export function defaultDecisionBounds(value) {
  const numeric = Number(value);
  const delta = Math.max(Math.abs(numeric) * 0.2, 1);
  return [numeric - delta, numeric + delta];
}

function isNumericValueType(valueType) {
  return ["float", "int", "integer", "number"].includes(String(valueType || "").toLowerCase());
}
