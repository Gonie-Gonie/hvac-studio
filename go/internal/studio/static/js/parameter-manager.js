import { roleLabel } from "./contract-labels.js";
import { escapeHTML } from "./dom.js";
import { parameterInputValue } from "./format.js";
import { PARAMETER_ROLES } from "./workspace-config.js";

export function renderParameterManager(context, elements, actions) {
  const tbody = elements.rows;
  const addForm = elements.addForm;
  tbody.innerHTML = "";
  addForm.innerHTML = "";
  const components = context.components || [];
  const editable = Boolean(context.editable);
  renderParameterAddForm(addForm, components, context.selectedComponentId, editable, actions);
  let count = 0;
  for (const component of components) {
    for (const name of componentParameterNames(component)) {
      count++;
      const definition = component.parameter_defs?.[name] || {};
      const value = component.parameters?.[name] ?? definition.current ?? definition.default ?? "";
      tbody.append(parameterRow(component, name, value, definition, editable, actions));
    }
  }
  if (!count) {
    tbody.append(emptyRow(5, "No parameters"));
  }
}

function renderParameterAddForm(container, components, selectedComponentId, editable, actions) {
  if (!editable || !components.length) return;
  const select = document.createElement("select");
  select.id = "newParameterComponent";
  select.setAttribute("aria-label", "Component");
  for (const component of components) {
    const option = document.createElement("option");
    option.value = component.id;
    option.textContent = componentOptionLabel(component);
    select.append(option);
  }
  if (selectedComponentId && components.some((component) => component.id === selectedComponentId)) {
    select.value = selectedComponentId;
  }

  const name = document.createElement("input");
  name.id = "newParameterName";
  name.placeholder = "name";
  name.setAttribute("aria-label", "Parameter name");

  const value = document.createElement("input");
  value.id = "newParameterValue";
  value.placeholder = "value";
  value.setAttribute("aria-label", "Parameter value");

  const display = document.createElement("input");
  display.id = "newParameterDisplayName";
  display.placeholder = "display";
  display.setAttribute("aria-label", "Parameter display name");

  const role = document.createElement("select");
  role.id = "newParameterRole";
  role.setAttribute("aria-label", "Parameter role");
  for (const value of PARAMETER_ROLES) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = roleLabel(value);
    role.append(option);
  }

  const unit = document.createElement("input");
  unit.id = "newParameterUnit";
  unit.placeholder = "unit";
  unit.setAttribute("aria-label", "Parameter unit");

  const min = document.createElement("input");
  min.id = "newParameterMin";
  min.placeholder = "min";
  min.setAttribute("aria-label", "Parameter minimum bound");

  const max = document.createElement("input");
  max.id = "newParameterMax";
  max.placeholder = "max";
  max.setAttribute("aria-label", "Parameter maximum bound");

  const group = document.createElement("input");
  group.id = "newParameterGroup";
  group.placeholder = "group";
  group.setAttribute("aria-label", "Parameter group");

  const description = document.createElement("input");
  description.id = "newParameterDescription";
  description.placeholder = "description";
  description.setAttribute("aria-label", "Parameter description");

  const add = () => actions.onAddParameter(select.value, name.value, value.value, {
    display: display.value || "",
    role: role.value || "fixed",
    unit: unit.value || "",
    min: min.value || "",
    max: max.value || "",
    group: group.value || "",
    description: description.value || "",
  });

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add";
  button.addEventListener("click", add);

  for (const input of [name, value, display, role, unit, min, max, group, description]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") add();
    });
  }
  container.append(select, name, value, display, role, unit, min, max, group, description, button);
}

function parameterRow(component, name, value, definition, editable, actions) {
  const tr = document.createElement("tr");
  for (const cellValue of [component.id, name]) {
    const td = document.createElement("td");
    td.textContent = cellValue;
    tr.append(td);
  }

  const valueCell = document.createElement("td");
  if (editable) {
    const input = document.createElement("input");
    input.className = "table-input";
    input.value = parameterInputValue(value);
    input.dataset.parameterComponent = component.id;
    input.dataset.parameterName = name;
    input.addEventListener("input", () => {
      actions.onSyncParameterInputs(component.id, name, input.value, input);
      actions.onProjectDirty();
    });
    valueCell.append(input);
  } else {
    valueCell.textContent = parameterInputValue(value);
  }
  tr.append(valueCell, parameterMetadataCell(name, definition));

  const actionCell = document.createElement("td");
  actionCell.className = "action-cell";
  if (editable && Object.prototype.hasOwnProperty.call(component.parameters || {}, name)) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action table-action";
    button.textContent = "Delete";
    button.addEventListener("click", () => actions.onDeleteParameter(component.id, name));
    actionCell.append(button);
  }
  tr.append(actionCell);
  return tr;
}

function parameterMetadataCell(name, definition) {
  const td = document.createElement("td");
  td.className = "parameter-meta-cell";
  const items = parameterMetadataItems(name, definition);
  if (!items.length) {
    td.textContent = "";
    return td;
  }
  const stack = document.createElement("div");
  stack.className = "parameter-meta-stack";
  for (const item of items) {
    const span = document.createElement("span");
    span.className = "parameter-meta-pill";
    span.textContent = item;
    stack.append(span);
  }
  td.append(stack);
  return td;
}

function parameterMetadataItems(name, definition = {}) {
  const items = [];
  if (!definition || !Object.keys(definition).length) return items;
  if (definition.display_name && definition.display_name !== name) items.push(`Display: ${definition.display_name}`);
  items.push(roleLabel(definition.role || "fixed"));
  if (definition.unit) items.push(`Unit: ${definition.unit}`);
  if (definition.default !== undefined) items.push(`Default: ${parameterInputValue(definition.default)}`);
  const bounds = definition.bounds || {};
  const min = bounds.min !== undefined ? parameterInputValue(bounds.min) : "";
  const max = bounds.max !== undefined ? parameterInputValue(bounds.max) : "";
  if (min || max) items.push(`Bounds: ${min || "..."} to ${max || "..."}`);
  if (definition.group) items.push(`Group: ${definition.group}`);
  if (definition.description) items.push(definition.description);
  if (definition.visible === false) items.push("Hidden");
  return items;
}

function componentParameterNames(component) {
  return [...new Set([
    ...Object.keys(component.parameters || {}),
    ...Object.keys(component.parameter_defs || {}),
  ])].sort();
}

function componentOptionLabel(component) {
  return component?.name && component.name !== component.id ? `${component.name} (${component.id})` : component?.id || "";
}

function emptyRow(cols, message = "No rows") {
  const tr = document.createElement("tr");
  tr.innerHTML = `<td colspan="${cols}" class="empty-cell">${escapeHTML(message)}</td>`;
  return tr;
}