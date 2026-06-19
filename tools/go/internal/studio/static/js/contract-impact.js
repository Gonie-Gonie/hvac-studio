import { parameterInputValue, formatValue } from "./format.js";
import { roleLabel } from "./source-authoring.js";

export function parameterDeleteImpact(component, name) {
  const componentID = component?.id || "";
  if (!componentID || !name) return emptyParameterImpact();
  const definitions = component.parameter_defs || {};
  const parameters = component.parameters || {};
  const hasValue = Object.prototype.hasOwnProperty.call(parameters, name);
  const definition = definitions[name] || {};
  const hasDefinition = Object.prototype.hasOwnProperty.call(definitions, name);
  const role = definition.role || "";

  return {
    component_id: componentID,
    name,
    baseline_values: hasValue ? [`${componentID}.${name} = ${parameterInputValue(parameters[name])}`] : [],
    metadata: hasDefinition ? parameterDefinitionDetails(definition) : [],
    source_completions: hasValue || hasDefinition ? [`params.get("${name}", ...)`] : [],
    candidate_roles: ["calibration_target", "optimization_variable"].includes(role) ? [roleLabel(role)] : [],
  };
}

export function parameterDeleteImpactSummary(impact) {
  const removals = [
    countLabel(impact.baseline_values, "baseline value"),
    countLabel(impact.metadata, "metadata field"),
    countLabel(impact.source_completions, "source completion"),
    countLabel(impact.candidate_roles, "workflow candidate role"),
  ].filter(Boolean);
  return removals.length ? `Removes ${removals.join(", ")}` : "";
}

export function parameterDeleteImpactDetails(impact) {
  return [
    detailLine("Baseline values", impact.baseline_values),
    detailLine("Metadata", impact.metadata),
    detailLine("Source completions", impact.source_completions),
    detailLine("Workflow roles", impact.candidate_roles),
  ].filter(Boolean).join("\n");
}

export function parameterDeleteImpactConfirmText(impact) {
  const summary = parameterDeleteImpactSummary(impact) || "No parameter metadata will be removed.";
  const details = parameterDeleteImpactDetails(impact);
  return details ? `${summary}\n${details}` : summary;
}

export function stateDeleteImpact(component, name) {
  const componentID = component?.id || "";
  if (!componentID || !name) return emptyStateImpact();
  const definitions = component.state_defs || {};
  const definition = definitions[name] || {};
  const hasDefinition = Object.prototype.hasOwnProperty.call(definitions, name);

  return {
    component_id: componentID,
    name,
    definitions: hasDefinition ? stateDefinitionDetails(definition) : [],
    initial_values: hasDefinition && definition.initial !== undefined ? [`${componentID}.${name} = ${formatValue(definition.initial)}`] : [],
    source_completions: hasDefinition ? [`state.get("${name}", ...)`] : [],
  };
}

export function stateDeleteImpactSummary(impact) {
  const removals = [
    countLabel(impact.definitions, "definition field"),
    countLabel(impact.initial_values, "initial value"),
    countLabel(impact.source_completions, "source completion"),
  ].filter(Boolean);
  return removals.length ? `Removes ${removals.join(", ")}` : "";
}

export function stateDeleteImpactDetails(impact) {
  return [
    detailLine("Definitions", impact.definitions),
    detailLine("Initial values", impact.initial_values),
    detailLine("Source completions", impact.source_completions),
  ].filter(Boolean).join("\n");
}

export function stateDeleteImpactConfirmText(impact) {
  const summary = stateDeleteImpactSummary(impact) || "No state metadata will be removed.";
  const details = stateDeleteImpactDetails(impact);
  return details ? `${summary}\n${details}` : summary;
}

function parameterDefinitionDetails(definition) {
  return [
    definition.display_name ? `display ${definition.display_name}` : "",
    definition.default !== undefined ? `default ${parameterInputValue(definition.default)}` : "",
    definition.unit ? `unit ${definition.unit}` : "",
    definition.role ? `role ${roleLabel(definition.role)}` : "",
    definition.group ? `group ${definition.group}` : "",
    definition.description ? "description" : "",
    definition.visible === false ? "hidden" : "",
    definition.bounds ? `bounds ${boundsLabel(definition.bounds)}` : "",
  ].filter(Boolean);
}

function stateDefinitionDetails(definition) {
  return [
    definition.display_name ? `display ${definition.display_name}` : "",
    definition.unit ? `unit ${definition.unit}` : "",
    definition.description ? "description" : "",
  ].filter(Boolean);
}

function boundsLabel(bounds) {
  const min = bounds.min !== undefined ? parameterInputValue(bounds.min) : "";
  const max = bounds.max !== undefined ? parameterInputValue(bounds.max) : "";
  return [min, max].filter(Boolean).join("..") || "set";
}

function emptyParameterImpact() {
  return {
    component_id: "",
    name: "",
    baseline_values: [],
    metadata: [],
    source_completions: [],
    candidate_roles: [],
  };
}

function emptyStateImpact() {
  return {
    component_id: "",
    name: "",
    definitions: [],
    initial_values: [],
    source_completions: [],
  };
}

function countLabel(values, singular) {
  const count = (values || []).length;
  if (!count) return "";
  return `${count} ${singular}${count === 1 ? "" : "s"}`;
}

function detailLine(label, values) {
  return values?.length ? `${label}: ${values.join(", ")}` : "";
}
