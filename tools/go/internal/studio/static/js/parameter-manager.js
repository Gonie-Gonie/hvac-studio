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
    for (const [name, value] of Object.entries(component.parameters || {})) {
      count++;
      tbody.append(parameterRow(component, name, value, editable, actions));
    }
  }
  if (!count) {
    tbody.append(emptyRow(4, "No parameters"));
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

  const role = document.createElement("select");
  role.id = "newParameterRole";
  role.setAttribute("aria-label", "Parameter role");
  for (const value of PARAMETER_ROLES) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = roleLabel(value);
    role.append(option);
  }

  const min = document.createElement("input");
  min.id = "newParameterMin";
  min.placeholder = "min";
  min.setAttribute("aria-label", "Parameter minimum bound");

  const max = document.createElement("input");
  max.id = "newParameterMax";
  max.placeholder = "max";
  max.setAttribute("aria-label", "Parameter maximum bound");

  const add = () => actions.onAddParameter(select.value, name.value, value.value, {
    role: role.value || "fixed",
    min: min.value || "",
    max: max.value || "",
  });

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add";
  button.addEventListener("click", add);

  for (const input of [name, value, role, min, max]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") add();
    });
  }
  container.append(select, name, value, role, min, max, button);
}

function parameterRow(component, name, value, editable, actions) {
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
  tr.append(valueCell);

  const actionCell = document.createElement("td");
  actionCell.className = "action-cell";
  if (editable) {
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

function componentOptionLabel(component) {
  return component?.name && component.name !== component.id ? `${component.name} (${component.id})` : component?.id || "";
}

function emptyRow(cols, message = "No rows") {
  const tr = document.createElement("tr");
  tr.innerHTML = `<td colspan="${cols}" class="empty-cell">${escapeHTML(message)}</td>`;
  return tr;
}