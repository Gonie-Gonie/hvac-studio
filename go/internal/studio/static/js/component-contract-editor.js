import {
  parameterDeleteImpact,
  parameterDeleteImpactDetails,
  parameterDeleteImpactSummary,
  stateDeleteImpact,
  stateDeleteImpactDetails,
  stateDeleteImpactSummary,
} from './contract-impact.js';
import { roleLabel } from './contract-labels.js';
import { escapeAttr, escapeHTML } from './dom.js';
import { emptyKVRow, inspectorKVRow, inspectorLabel, inspectorSection, inspectorUnit, rememberDisclosure } from './inspector-ui.js';
import { parameterInputValue } from './format.js';
import { nodeDeleteImpact, nodeDeleteImpactDetails, nodeDeleteImpactSummary } from './node-impact.js';
import { NODE_PRESETS, PARAMETER_ROLES } from './workspace-config.js';

export function componentHasInputNode(component, nodeID) {
  return (component?.nodes?.inputs || []).some((node) => node.id === nodeID);
}

export function componentHasOutputNode(component, nodeID) {
  return (component?.nodes?.outputs || []).some((node) => node.id === nodeID);
}

export function nodeEditor(component, options) {
  const block = inspectorSection("Add Node", {
    collapsible: true, open: false, stateKey: `${component.id}:new-node`,
  });

  const form = document.createElement("div");
  form.className = "connection-form node-form";

  const preset = document.createElement("select");
  preset.id = "newNodePreset";
  preset.setAttribute("aria-label", "Node preset");
  for (const [value, label] of NODE_PRESETS) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    preset.append(option);
  }

  const direction = document.createElement("select");
  direction.id = "newNodeDirection";
  for (const [value, label] of [["input", "Input"], ["output", "Output"]]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    direction.append(option);
  }

  const nodeID = document.createElement("input");
  nodeID.id = "newNodeId";
  nodeID.placeholder = "id";
  nodeID.setAttribute("aria-label", "Node id");

  const nodeName = document.createElement("input");
  nodeName.id = "newNodeName";
  nodeName.placeholder = "name";
  nodeName.setAttribute("aria-label", "Node name");

  const valueType = document.createElement("select");
  valueType.id = "newNodeValueType";
  for (const type of ["float", "int", "bool", "string", "object"]) {
    const option = document.createElement("option");
    option.value = type;
    option.textContent = type;
    valueType.append(option);
  }

  const medium = document.createElement("input");
  medium.id = "newNodeMedium";
  medium.placeholder = "medium";
  medium.value = "signal";
  medium.setAttribute("aria-label", "Node medium");

  const unit = document.createElement("input");
  unit.id = "newNodeUnit";
  unit.placeholder = "unit";
  unit.setAttribute("aria-label", "Node unit");

  const defaultValue = document.createElement("input");
  defaultValue.id = "newNodeDefault";
  defaultValue.placeholder = "default";
  defaultValue.setAttribute("aria-label", "Default value");

  const requiredLabel = document.createElement("label");
  requiredLabel.className = "node-required-toggle node-create-required";
  const required = document.createElement("input");
  required.id = "newNodeRequired";
  required.type = "checkbox";
  required.checked = true;
  required.setAttribute("aria-label", "Required input node");
  requiredLabel.append(required, document.createTextNode("Required"));

  const addNode = () => options.onAddNode(component.id);
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add Node";
  button.addEventListener("click", addNode);

  const syncInputOnlyFields = () => {
    const isInput = direction.value === "input";
    defaultValue.disabled = !isInput;
    required.disabled = !isInput;
  };
  const applyPreset = () => {
    const selected = NODE_PRESETS.find(([value]) => value === preset.value);
    const values = selected?.[2] || {};
    if (!preset.value || !Object.keys(values).length) return;
    direction.value = values.direction || "input";
    nodeID.value = values.id || "";
    nodeName.value = values.name || "";
    valueType.value = values.value_type || "float";
    medium.value = values.medium || "signal";
    unit.value = values.unit || "";
    defaultValue.value = presetDefaultValue(values.default);
    required.checked = values.required !== false;
    syncInputOnlyFields();
  };
  preset.addEventListener("change", applyPreset);
  direction.addEventListener("change", syncInputOnlyFields);
  syncInputOnlyFields();

  for (const input of [nodeID, nodeName, medium, unit, defaultValue]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addNode();
    });
  }
  form.append(preset, direction, nodeID, nodeName, valueType, medium, unit, defaultValue, requiredLabel, button);
  block.append(form);
  return block;
}

function presetDefaultValue(value) {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}
export function nodeListBlock(title, component, nodes, direction, options) {
  const block = inspectorSection(title, { count: nodes.length });
  block.classList.add("inspector-node-list");
  if (!nodes.length) {
    block.append(emptyKVRow(`No ${String(title || "nodes").toLowerCase()}`));
    return block;
  }
  for (const node of nodes) {
    const details = document.createElement("details");
    details.className = `inspector-node-row inspector-node-${direction}`;
    rememberDisclosure(details, `${component.id}:${direction}:${node.id}`);
    const summary = document.createElement("summary");
    summary.className = "inspector-node-summary";
    summary.title = `${node.id} · Click to ${options.editable ? "edit" : "view"} contract`;
    const label = document.createElement("span");
    label.className = "inspector-node-name";
    label.textContent = node.name || inspectorLabel(node.id);
    const unit = document.createElement("span");
    unit.className = "inspector-node-unit";
    unit.textContent = inspectorUnit(node.unit);
    summary.append(label, unit);
    details.append(summary);
    if (options.editable) {
      details.append(editableNodeRow(component, node, direction, options));
      block.append(details);
      continue;
    }
    const row = document.createElement("div");
    row.className = "inspector-node-contract";
    row.append(
      inspectorKVRow("ID", node.id),
      inspectorKVRow("Medium", inspectorLabel(node.medium || "signal")),
      inspectorKVRow("Type", node.value_type || "float"),
    );
    if (node.unit) row.append(inspectorKVRow("Unit", node.unit));
    if (direction === "input") row.append(inspectorKVRow("Required", node.required !== false ? "Yes" : "No"));
    if (node.default !== undefined) row.append(inspectorKVRow("Default", parameterInputValue(node.default)));
    if (node.description) row.append(inspectorKVRow("Description", node.description));
    details.append(row);
    block.append(details);
  }
  return block;
}

function editableNodeRow(component, node, direction, options) {
  const row = document.createElement("div");
  row.className = "kv node-edit-row";
  row.dataset.nodeComponent = component.id;
  row.dataset.nodeId = node.id;

  const key = document.createElement("span");
  key.className = "kv-key node-id-label";
  key.textContent = node.id;

  const controls = document.createElement("span");
  controls.className = "node-meta-controls";

  const nodeID = document.createElement("input");
  nodeID.className = "inspector-input";
  nodeID.value = node.id;
  nodeID.placeholder = "id";
  nodeID.dataset.nodeField = "id";
  nodeID.setAttribute("aria-label", `${component.id}.${node.id} id`);

  const directionSelect = document.createElement("select");
  directionSelect.className = "inspector-input";
  directionSelect.dataset.nodeField = "direction";
  directionSelect.setAttribute("aria-label", `${component.id}.${node.id} direction`);
  for (const value of ["input", "output"]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    directionSelect.append(option);
  }
  directionSelect.value = direction === "output" ? "output" : "input";

  const name = document.createElement("input");
  name.className = "inspector-input";
  name.value = node.name || node.id;
  name.placeholder = "name";
  name.dataset.nodeField = "name";
  name.setAttribute("aria-label", `${component.id}.${node.id} name`);

  const medium = document.createElement("input");
  medium.className = "inspector-input";
  medium.value = node.medium || "signal";
  medium.placeholder = "medium";
  medium.dataset.nodeField = "medium";
  medium.setAttribute("aria-label", `${component.id}.${node.id} medium`);

  const valueType = document.createElement("select");
  valueType.className = "inspector-input";
  valueType.dataset.nodeField = "value_type";
  valueType.setAttribute("aria-label", `${component.id}.${node.id} value type`);
  for (const type of ["float", "int", "bool", "string", "object"]) {
    const option = document.createElement("option");
    option.value = type;
    option.textContent = type;
    valueType.append(option);
  }
  valueType.value = node.value_type || "float";

  const unit = document.createElement("input");
  unit.className = "inspector-input";
  unit.value = node.unit || "";
  unit.placeholder = "unit";
  unit.dataset.nodeField = "unit";
  unit.setAttribute("aria-label", `${component.id}.${node.id} unit`);

  controls.append(directionSelect, nodeID, name, medium, valueType, unit);

  if (direction === "input") {
    const defaultValue = document.createElement("input");
    defaultValue.className = "inspector-input";
    defaultValue.value = parameterInputValue(node.default);
    defaultValue.placeholder = "default";
    defaultValue.dataset.nodeField = "default";
    defaultValue.setAttribute("aria-label", `${component.id}.${node.id} default`);

    const requiredLabel = document.createElement("label");
    requiredLabel.className = "node-required-toggle";
    const required = document.createElement("input");
    required.type = "checkbox";
    required.checked = node.required !== false;
    required.dataset.nodeField = "required";
    required.setAttribute("aria-label", `${component.id}.${node.id} required`);
    requiredLabel.append(required, document.createTextNode("Required"));
    controls.append(defaultValue, requiredLabel);
  }

  const saveButton = document.createElement("button");
  saveButton.type = "button";
  saveButton.className = "small-action";
  saveButton.textContent = "Save";
  saveButton.addEventListener("click", () => options.onUpdateNode(component.id, node.id, direction, row));

  const impact = nodeDeleteImpact(component, node, options.system, options.connections || []);
  const impactSummary = nodeDeleteImpactSummary(impact);
  if (impactSummary) {
    const impactBadge = document.createElement("span");
    impactBadge.className = "node-impact";
    impactBadge.textContent = impactSummary;
    const impactDetails = nodeDeleteImpactDetails(impact);
    if (impactDetails) impactBadge.title = impactDetails;
    controls.append(impactBadge);
  }

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "small-action";
  deleteButton.textContent = "Delete";
  deleteButton.addEventListener("click", () => options.onDeleteNode(component.id, node.id, impact));

  for (const input of controls.querySelectorAll("input, select")) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") options.onUpdateNode(component.id, node.id, direction, row);
    });
  }
  controls.append(saveButton, deleteButton);
  labelContractFields(controls);
  row.append(key, controls);
  return row;
}

export function parameterInspectorBlock(component, editable, options) {
  const entries = Object.entries(component.parameters || {});
  const sectionOptions = {
    collapsible: true, open: false, count: entries.length, stateKey: `${component.id}:parameters`,
  };
  if (!editable) {
    const block = inspectorSection("Parameters", sectionOptions);
    for (const [name, value] of entries) {
      const definition = component.parameter_defs?.[name];
      const row = inspectorKVRow(definition?.display_name || inspectorLabel(name), parameterInputValue(value));
      row.title = [name, definition?.unit, definition?.description].filter(Boolean).join(" · ");
      block.append(row);
    }
    if (!entries.length) block.append(emptyKVRow("No parameters"));
    return block;
  }
  const block = inspectorSection("Parameters", sectionOptions);

  if (!entries.length) {
    block.append(emptyKVRow("No parameters"));
  }
  for (const [name, value] of entries) {
    const row = document.createElement("div");
    row.className = "kv connection-row";
    row.innerHTML = `
      <span class="kv-key" title="${escapeAttr(name)}">${escapeHTML(component.parameter_defs?.[name]?.display_name || inspectorLabel(name))}</span>
      <span class="connection-value">
        <input class="inspector-input" value="${escapeAttr(parameterInputValue(value))}" data-parameter-component="${escapeAttr(component.id)}" data-parameter-name="${escapeAttr(name)}" aria-label="${escapeAttr(`${component.id}.${name}`)}" />
      </span>
    `;
    const input = row.querySelector("input");
    input.title = parameterInputValue(value);
    input.addEventListener("input", () => {
      options.onSyncParameterInputs(component.id, name, input.value, input);
      options.onProjectDirty();
    });
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action";
    button.textContent = "Delete";
    const impact = parameterDeleteImpact(component, name);
    button.addEventListener("click", () => options.onDeleteParameter(component.id, name, impact));
    row.querySelector(".connection-value").append(
      impactBadge(parameterDeleteImpactSummary(impact), parameterDeleteImpactDetails(impact)),
      button,
    );
    if (parameterInputValue(value).length > 56 || (value !== null && typeof value === "object")) {
      const editor = document.createElement("details");
      editor.className = "parameter-value-editor";
      const summary = document.createElement("summary");
      summary.textContent = "Expand value";
      const textarea = document.createElement("textarea");
      textarea.className = "inspector-expanded-input";
      textarea.value = input.value;
      textarea.setAttribute("aria-label", `${component.id}.${name} full value`);
      textarea.addEventListener("input", () => {
        input.value = textarea.value;
        input.dispatchEvent(new Event("input", { bubbles: true }));
      });
      editor.addEventListener("toggle", () => {
        if (editor.open) textarea.value = input.value;
      });
      input.addEventListener("input", () => {
        textarea.value = input.value;
        input.title = input.value;
      });
      editor.append(summary, textarea);
      row.append(editor);
    }
    block.append(row);
  }

  const form = document.createElement("div");
  form.className = "connection-form parameter-form";
  const nameInput = document.createElement("input");
  nameInput.placeholder = "name";
  nameInput.setAttribute("aria-label", "Parameter name");
  const valueInput = document.createElement("input");
  valueInput.placeholder = "value";
  valueInput.setAttribute("aria-label", "Parameter value");
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add";
  button.addEventListener("click", () => options.onAddParameter(component.id, nameInput.value, valueInput.value));
  for (const input of [nameInput, valueInput]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") options.onAddParameter(component.id, nameInput.value, valueInput.value);
    });
  }
  form.append(nameInput, valueInput, button);
  block.append(form);
  return block;
}

export function parameterDefinitionBlock(component, options) {
  const definitions = component.parameter_defs || {};
  const names = [...new Set([...Object.keys(component.parameters || {}), ...Object.keys(definitions)])].sort();
  const block = inspectorSection("Parameter Definitions", {
    collapsible: true, open: false, count: names.length, stateKey: `${component.id}:parameter-definitions`,
  });
  if (!names.length) {
    block.append(emptyKVRow("No parameter definitions"));
  }
  for (const name of names) {
    const row = parameterDefinitionRow(component, name, definitions[name] || {}, options);
    block.append(contractDisclosure(row, definitions[name]?.display_name || inspectorLabel(name), `${component.id}:parameter-definition:${name}`));
  }
  return block;
}

function parameterDefinitionRow(component, name, definition, options) {
  const row = document.createElement("div");
  row.className = "kv contract-edit-row";
  row.dataset.parameterDefinition = name;

  const key = document.createElement("span");
  key.className = "kv-key node-id-label";
  key.textContent = name;

  const controls = document.createElement("span");
  controls.className = "contract-meta-controls parameter-definition-controls";

  const displayName = contractInput("display", definition.display_name || "");
  const current = contractInput("value", parameterInputValue(component.parameters?.[name] ?? definition.current ?? definition.default ?? ""));
  const defaultValue = contractInput("default", parameterInputValue(definition.default));
  const unit = contractInput("unit", definition.unit || "");
  const group = contractInput("group", definition.group || "");
  const description = contractInput("description", definition.description || "");
  const min = contractInput("min", parameterInputValue(definition.bounds?.min));
  const max = contractInput("max", parameterInputValue(definition.bounds?.max));
  const role = document.createElement("select");
  role.className = "inspector-input";
  role.dataset.contractField = "role";
  role.setAttribute("aria-label", `${component.id}.${name} role`);
  for (const value of PARAMETER_ROLES) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = roleLabel(value);
    role.append(option);
  }
  role.value = definition.role || "fixed";

  const visibleLabel = document.createElement("label");
  visibleLabel.className = "node-required-toggle contract-toggle";
  const visible = document.createElement("input");
  visible.type = "checkbox";
  visible.checked = definition.visible !== false;
  visible.dataset.contractField = "visible";
  visible.setAttribute("aria-label", `${component.id}.${name} visible`);
  visibleLabel.append(visible, document.createTextNode("Visible"));

  const saveButton = document.createElement("button");
  saveButton.type = "button";
  saveButton.className = "small-action";
  saveButton.textContent = "Save";
  saveButton.addEventListener("click", () => options.onSaveParameterDefinition(component.id, name, row));

  const clearButton = document.createElement("button");
  clearButton.type = "button";
  clearButton.className = "small-action";
  clearButton.textContent = "Clear Meta";
  clearButton.addEventListener("click", () => options.onDeleteParameterDefinition(component.id, name));

  for (const input of [displayName, current, defaultValue, unit, group, description, min, max, role, visible]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") options.onSaveParameterDefinition(component.id, name, row);
    });
  }
  controls.append(displayName, current, defaultValue, unit, role, min, max, group, description, visibleLabel, saveButton, clearButton);
  labelContractFields(controls);
  row.append(key, controls);
  return row;
}

export function stateDefinitionBlock(component, options) {
  const entries = Object.entries(component.state_defs || {}).sort(([left], [right]) => left.localeCompare(right));
  const block = inspectorSection("State Definitions", {
    collapsible: true, open: false, count: entries.length, stateKey: `${component.id}:state-definitions`,
  });
  if (!entries.length) {
    block.append(emptyKVRow("No state definitions"));
  }
  for (const [name, definition] of entries) {
    const row = stateDefinitionRow(component, name, definition || {}, options);
    block.append(contractDisclosure(row, definition?.display_name || inspectorLabel(name), `${component.id}:state-definition:${name}`));
  }

  const form = document.createElement("div");
  form.className = "connection-form state-form";
  const name = document.createElement("input");
  name.id = "newStateName";
  name.placeholder = "state name";
  name.setAttribute("aria-label", "State name");
  const initial = document.createElement("input");
  initial.id = "newStateInitial";
  initial.placeholder = "initial";
  initial.setAttribute("aria-label", "State initial value");
  const unit = document.createElement("input");
  unit.id = "newStateUnit";
  unit.placeholder = "unit";
  unit.setAttribute("aria-label", "State unit");
  const description = document.createElement("input");
  description.id = "newStateDescription";
  description.placeholder = "description";
  description.setAttribute("aria-label", "State description");
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add State";
  button.addEventListener("click", () => options.onAddState(component.id, name.value, initial.value, {
    unit: unit.value,
    description: description.value,
  }));
  for (const input of [name, initial, unit, description]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") options.onAddState(component.id, name.value, initial.value, {
        unit: unit.value,
        description: description.value,
      });
    });
  }
  form.append(name, initial, unit, description, button);
  block.append(form);
  return block;
}

function stateDefinitionRow(component, name, definition, options) {
  const row = document.createElement("div");
  row.className = "kv contract-edit-row";
  row.dataset.stateDefinition = name;

  const key = document.createElement("span");
  key.className = "kv-key node-id-label";
  key.textContent = name;

  const controls = document.createElement("span");
  controls.className = "contract-meta-controls state-definition-controls";

  const displayName = contractInput("display", definition.display_name || "");
  const initial = contractInput("initial", parameterInputValue(definition.initial));
  const unit = contractInput("unit", definition.unit || "");
  const description = contractInput("description", definition.description || "");

  const saveButton = document.createElement("button");
  saveButton.type = "button";
  saveButton.className = "small-action";
  saveButton.textContent = "Save";
  saveButton.addEventListener("click", () => options.onSaveStateDefinition(component.id, name, row));

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "small-action";
  deleteButton.textContent = "Delete";
  const impact = stateDeleteImpact(component, name);
  deleteButton.addEventListener("click", () => options.onDeleteStateDefinition(component.id, name, impact));

  for (const input of [displayName, initial, unit, description]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") options.onSaveStateDefinition(component.id, name, row);
    });
  }
  controls.append(
    displayName,
    initial,
    unit,
    description,
    impactBadge(stateDeleteImpactSummary(impact), stateDeleteImpactDetails(impact)),
    saveButton,
    deleteButton,
  );
  labelContractFields(controls);
  row.append(key, controls);
  return row;
}

export function stateInspectorBlock(component) {
  const entries = Object.entries(component.state_defs || {});
  if (!entries.length) return null;
  const block = inspectorSection("Initial State", {
    collapsible: true, open: false, count: entries.length, stateKey: `${component.id}:initial-state`,
  });
  for (const [name, definition] of entries) {
    const row = inspectorKVRow(definition?.display_name || inspectorLabel(name), parameterInputValue(definition?.initial));
    row.title = [name, definition?.unit, definition?.description].filter(Boolean).join(" · ");
    block.append(row);
  }
  return block;
}

function contractDisclosure(row, name, stateKey) {
  const details = document.createElement("details");
  details.className = "inspector-contract-disclosure";
  rememberDisclosure(details, stateKey);
  const summary = document.createElement("summary");
  summary.textContent = name;
  details.append(summary, row);
  return details;
}

function labelContractFields(controls) {
  for (const input of Array.from(controls.children)) {
    if (!input.matches("input:not([type=checkbox]), select")) continue;
    const field = input.dataset.nodeField || input.dataset.contractField || input.placeholder;
    const label = document.createElement("label");
    label.className = `inspector-contract-field inspector-field-${field}`;
    const caption = document.createElement("span");
    caption.textContent = inspectorLabel(({ id: "ID", display: "Display name", value_type: "Value type" })[field] || field);
    controls.insertBefore(label, input);
    label.append(caption, input);
  }
}

function contractInput(placeholder, value) {
  const input = document.createElement("input");
  input.className = "inspector-input";
  input.placeholder = placeholder;
  input.value = value ?? "";
  input.dataset.contractField = placeholder;
  input.setAttribute("aria-label", placeholder);
  return input;
}

function impactBadge(summary, details) {
  if (!summary) return document.createDocumentFragment();
  const badge = document.createElement("span");
  badge.className = "contract-impact";
  badge.textContent = summary;
  if (details) badge.title = details;
  return badge;
}
