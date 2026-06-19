import { coerceParameter } from "./format.js";
import { PARAMETER_ROLES } from "./workspace-config.js";

const IDENTIFIER_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function displayNameFromIdentifier(value) {
  return String(value || "")
    .split(/[_\-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function newNodePayload(projectPath, component, values = {}) {
  const componentID = component?.id || "";
  const nodeID = (values.id || "").trim();
  if (!component || !nodeID) {
    return { error: "Select a component and node id" };
  }
  if (!IDENTIFIER_PATTERN.test(nodeID)) {
    return { error: "Node id must start with a letter or underscore and contain only letters, numbers, and underscores" };
  }
  const existingNodes = [...(component.nodes?.inputs || []), ...(component.nodes?.outputs || [])];
  if (existingNodes.some((node) => node.id === nodeID)) {
    return { error: `Node already exists: ${componentID}.${nodeID}` };
  }
  const direction = (values.direction || "input").trim();
  if (direction !== "input" && direction !== "output") {
    return { error: "Node direction must be input or output" };
  }
  const rawDefault = values.default || "";
  const payload = {
    project_path: projectPath,
    component_id: componentID,
    direction,
    id: nodeID,
    name: (values.name || "").trim() || nodeID,
    preset: values.preset || "",
    medium: (values.medium || "").trim() || "signal",
    value_type: values.value_type || "float",
    unit: (values.unit || "").trim(),
  };
  if (direction === "input" && rawDefault.trim() !== "") {
    payload.default = coerceParameter(rawDefault);
  }
  if (direction === "input") {
    payload.required = Boolean(values.required);
  }
  return { payload, nodeID };
}

export function nodeUpdatePayload(projectPath, component, nodeID, direction, fields = {}) {
  const componentID = component?.id || "";
  const newID = (fields.id || "").trim();
  const targetDirection = (fields.direction || direction || "input").trim();
  if (!newID) {
    return { error: "Node id is required" };
  }
  if (!IDENTIFIER_PATTERN.test(newID)) {
    return { error: "Node id must start with a letter or underscore and contain only letters, numbers, and underscores" };
  }
  if (targetDirection !== "input" && targetDirection !== "output") {
    return { error: "Node direction must be input or output" };
  }
  if (newID !== nodeID) {
    const existingNodes = [...(component?.nodes?.inputs || []), ...(component?.nodes?.outputs || [])];
    if (existingNodes.some((node) => node.id === newID)) {
      return { error: `Node already exists: ${componentID}.${newID}` };
    }
  }
  const name = (fields.name || "").trim();
  if (!name) {
    return { error: "Node name is required" };
  }
  const payload = {
    project_path: projectPath,
    component_id: componentID,
    node_id: nodeID,
    new_id: newID,
    direction: targetDirection,
    name,
    medium: (fields.medium || "").trim(),
    value_type: fields.value_type || "float",
    unit: (fields.unit || "").trim(),
  };
  if (targetDirection === "input") {
    const rawDefault = fields.default || "";
    payload.required = Boolean(fields.required);
    payload.default = rawDefault.trim() === "" ? null : coerceParameter(rawDefault);
  }
  return { payload, newID, direction: targetDirection, renamed: newID !== nodeID, directionChanged: targetDirection !== direction };
}

export function newParameterDefinition(name, value, options = {}) {
  const role = (options.role || "fixed").trim() || "fixed";
  const min = (options.min || "").trim();
  const max = (options.max || "").trim();
  if (role === "fixed" && min === "" && max === "") return { definition: null };
  if (!PARAMETER_ROLES.includes(role)) {
    return { error: `Parameter role is invalid: ${role}` };
  }
  const boundsResult = parameterBoundsFromInputs(name, min, max);
  if (boundsResult.error) return boundsResult;
  const definition = {
    display_name: displayNameFromIdentifier(name),
    role,
    current: value,
    default: value,
    visible: true,
  };
  if (boundsResult.bounds) definition.bounds = boundsResult.bounds;
  return { definition };
}

export function parameterDefinitionFromFields(label, fields) {
  const value = coerceParameter(fields.value || "");
  const role = fields.role || "fixed";
  if (!PARAMETER_ROLES.includes(role)) {
    return { error: `Parameter role is invalid: ${role}` };
  }
  const definition = {
    display_name: fields.display || "",
    unit: fields.unit || "",
    role,
    group: fields.group || "",
    description: fields.description || "",
    current: value,
    visible: fields.visible !== false,
  };
  if ((fields.default || "").trim() !== "") definition.default = coerceParameter(fields.default);
  const boundsResult = parameterBoundsFromInputs(label, fields.min || "", fields.max || "");
  if (boundsResult.error) return boundsResult;
  if (boundsResult.bounds) definition.bounds = boundsResult.bounds;
  return { definition, value };
}

export function newStateDefinition(name, initial, options = {}) {
  const definition = {
    display_name: displayNameFromIdentifier(name),
    initial: (initial || "").trim() === "" ? 0.0 : coerceParameter(initial),
  };
  if ((options.unit || "").trim() !== "") definition.unit = options.unit.trim();
  if ((options.description || "").trim() !== "") definition.description = options.description.trim();
  return { definition };
}

export function stateDefinitionFromFields(fields) {
  const definition = {
    display_name: fields.display || "",
    unit: fields.unit || "",
    description: fields.description || "",
  };
  if ((fields.initial || "").trim() !== "") definition.initial = coerceParameter(fields.initial);
  return { definition };
}

function parameterBoundsFromInputs(label, minValue, maxValue) {
  const min = (minValue || "").trim();
  const max = (maxValue || "").trim();
  if (min === "" && max === "") return { bounds: null };
  const minNumber = min === "" ? null : Number(min);
  const maxNumber = max === "" ? null : Number(max);
  if (min !== "" && !Number.isFinite(minNumber)) {
    return { error: `Parameter bounds min must be numeric: ${label}` };
  }
  if (max !== "" && !Number.isFinite(maxNumber)) {
    return { error: `Parameter bounds max must be numeric: ${label}` };
  }
  if (minNumber !== null && maxNumber !== null && minNumber > maxNumber) {
    return { error: `Parameter bounds min must be <= max: ${label}` };
  }
  const bounds = {};
  if (min !== "") bounds.min = coerceParameter(min);
  if (max !== "") bounds.max = coerceParameter(max);
  return { bounds };
}
