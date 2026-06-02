import { api } from "./api.js";
import { el, escapeAttr, escapeHTML } from "./dom.js";
import {
  coerceInput,
  coerceParameter,
  formatValue,
  parameterInputValue,
  sampleValueFor,
} from "./format.js";
import { renderExportWorkspace } from "./export-workspace.js";
import { renderRunOutputWorkspace } from "./run-output.js";
import { state } from "./state.js";

function log(message) {
  const time = new Date().toLocaleTimeString();
  state.logs.unshift(`[${time}] ${message}`);
  renderLogs();
}

async function loadProjects(preferredProjectPath = "") {
  const body = await api("/api/projects");
  state.projects = body.projects || [];
  const select = el("projectSelect");
  select.innerHTML = "";
  for (const project of state.projects) {
    const option = document.createElement("option");
    option.value = project.project_path;
    option.textContent = `${project.source === "workspace" ? "Project" : "Example"} / ${project.relative_path}`;
    select.append(option);
  }
  const feedForward = state.projects.find((p) => p.name === "003_feedforward_system");
  const preferred = state.projects.find((p) => p.project_path === preferredProjectPath);
  const first = preferred || state.projects.find((p) => p.source === "workspace") || feedForward || state.projects[0];
  if (first) {
    select.value = first.project_path;
    await loadProject(first.project_path);
  }
}

async function loadProject(projectPath) {
  state.currentProjectPath = projectPath;
  state.selectedComponentId = "";
  state.latestResult = null;
  state.latestRunRecord = null;
  state.latestBatchRecord = null;
  state.latestExport = null;
  state.latestSchema = null;
  state.latestValidation = null;
  state.activeRunInput = null;
  state.sourceByComponent = {};
  state.sourceDraftByComponent = {};
  state.sourceCheckByComponent = {};
  state.loadingSource = {};
  state.pendingConnection = null;
  state.selectedConnectionId = "";
  el("saveProjectButton").classList.remove("dirty");

  const body = await api(`/api/project?project_path=${encodeURIComponent(projectPath)}`);
  state.detail = body.project;
  const components = state.detail.graph.components || [];
  if (components.length) {
    state.selectedComponentId = components[0].id;
  }
  renderAll();
  log(`Opened ${state.detail.project.project_name}`);
}

function renderAll() {
  renderProjectTree();
  renderRunInputs();
  renderCanvas();
  renderInspector();
  renderParameters();
  renderProblems();
  renderResults();
  renderSchema();
  renderRunWorkspace();
  renderPythonPanel();
  renderExportWorkspaceView();
  const project = state.detail?.project;
  el("systemTitle").textContent = project?.entry_system || "System";
  el("systemSubtitle").textContent = project ? `${project.project_name} / ${state.detail.graph_path}` : "";
  updateCommandState();
}

function renderProjectTree() {
  const root = el("projectTree");
  root.innerHTML = "";
  if (!state.detail) return;
  const graph = state.detail.graph;
  const sections = [
    ["Systems", graph.systems.map((item) => treeItem(item.id, item.name || item.id, "system"))],
    ["Components", graph.components.map((item) => treeItem(item.id, item.name || item.id, item.kind))],
    ["Python Source", graph.components.map((item) => treeItem(item.id, item.class || "", "class"))],
    ["Runs", (state.detail.runs || []).map((item) => runTreeItem(item))],
    ["Batches", (state.detail.batches || []).map((item) => batchTreeItem(item))],
    ["Scenarios", (state.detail.scenarios || []).map((item) => scenarioTreeItem(item))],
    ["Export Profiles", exportTreeItems()],
  ];
  for (const [title, items] of sections) {
    const section = document.createElement("div");
    section.className = "tree-section";
    section.innerHTML = `<div class="tree-title">${escapeHTML(title)}</div>`;
    if (items.length) {
      for (const item of items) section.append(item);
    } else {
      const empty = document.createElement("div");
      empty.className = "tree-item";
      empty.innerHTML = `<span class="tree-meta">empty</span>`;
      section.append(empty);
    }
    root.append(section);
  }
}

function treeItem(id, label, meta) {
  const row = document.createElement("div");
  row.className = `tree-item ${state.selectedComponentId === id ? "active" : ""}`;
  row.innerHTML = `<span>${escapeHTML(label)}</span><span class="tree-meta">${escapeHTML(meta)}</span>`;
  row.addEventListener("click", () => {
    if (componentById(id)) {
      state.selectedComponentId = id;
      renderCanvas();
      renderInspector();
      renderPythonPanel();
      renderProjectTree();
      updateCommandState();
    }
  });
  return row;
}

function treeStatic(label, meta) {
  const row = document.createElement("div");
  row.className = "tree-item";
  row.innerHTML = `<span>${escapeHTML(label)}</span><span class="tree-meta">${escapeHTML(meta)}</span>`;
  return row;
}

function runTreeItem(run) {
  const row = treeStatic(run.id, run.relative_path);
  row.addEventListener("click", () => loadRunRecord(run.id));
  return row;
}

function batchTreeItem(batch) {
  const row = treeStatic(batch.id, `${batch.ok_count}/${batch.case_count} ok`);
  row.addEventListener("click", () => loadBatchRecord(batch.id));
  return row;
}

function scenarioTreeItem(scenario) {
  const row = treeStatic(scenario.name || scenario.id, scenario.relative_path);
  row.addEventListener("click", () => loadScenario(scenario.id));
  return row;
}

function exportTreeItems() {
  const exports = state.detail?.exports || [];
  if (exports.length) {
    return exports.map((item) => treeStatic(item.profile, item.relative_path));
  }
  return [treeStatic("runtime_package", "ready")];
}

function renderRunInputs() {
  const container = el("runInputs");
  container.innerHTML = "";
  const inputs = currentSystem()?.public_inputs || [];
  const savedInputs = state.activeRunInput?.inputs || state.detail?.default_run_input?.inputs || {};
  for (const input of inputs) {
    const field = document.createElement("div");
    field.className = "input-field";
    const defaultValue = savedInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
    field.innerHTML = `
      <label for="input-${escapeAttr(input.id)}">${escapeHTML(input.id)}</label>
      <input id="input-${escapeAttr(input.id)}" data-input-id="${escapeAttr(input.id)}" value="${escapeAttr(defaultValue)}" />
    `;
    field.querySelector("input").addEventListener("input", markProjectDirty);
    container.append(field);
  }
}

function renderCanvas() {
  const canvas = el("systemCanvas");
  const layer = el("connectionLayer");
  canvas.innerHTML = "";
  layer.innerHTML = "";
  const graph = state.detail?.graph;
  const system = currentSystem();
  if (!graph || !system) return;

  const components = system.components.map(componentById).filter(Boolean);
  const positions = {};
  components.forEach((component, index) => {
    const column = index;
    const x = 48 + column * 220;
    const y = 78 + (index % 2) * 62;
    positions[component.id] = { x, y };

    const node = document.createElement("button");
    node.type = "button";
    node.className = `component-node ${state.selectedComponentId === component.id ? "selected" : ""}`;
    node.style.left = `${x}px`;
    node.style.top = `${y}px`;
    node.dataset.componentId = component.id;
    node.innerHTML = `
      <div class="component-head">
        <span>${escapeHTML(component.name || component.id)}</span>
        <span class="component-kind">${escapeHTML(component.kind)}</span>
      </div>
      <div class="node-list">
        ${component.nodes.inputs.map((n) => canvasNodePill(component.id, n, "input")).join("")}
        ${component.nodes.outputs.map((n) => canvasNodePill(component.id, n, "output")).join("")}
      </div>
    `;
    node.addEventListener("click", () => {
      state.selectedComponentId = component.id;
      state.selectedConnectionId = "";
      renderCanvas();
      renderInspector();
      renderPythonPanel();
      renderProjectTree();
      updateCommandState();
    });
    node.querySelectorAll("[data-node-endpoint]").forEach((endpoint) => {
      endpoint.addEventListener("click", (event) => {
        event.stopPropagation();
        handleCanvasEndpointClick(endpoint.dataset.componentId, endpoint.dataset.nodeId, endpoint.dataset.direction);
      });
    });
    canvas.append(node);
  });

  requestAnimationFrame(() => drawConnections(positions));
}

function canvasNodePill(componentID, node, direction) {
  const pending = state.pendingConnection;
  const latest = latestCanvasNodeValue(componentID, node.id, direction);
  const selected = direction === "output" && pending?.component === componentID && pending?.node === node.id;
  const targetable = direction === "input" && pending && pending.component !== componentID;
  const classes = [
    "node-pill",
    direction === "output" ? "output" : "",
    latest.hasValue ? "has-value" : "",
    selected ? "pending-source" : "",
    targetable ? "targetable" : "",
  ].filter(Boolean).join(" ");
  const formattedValue = latest.hasValue ? formatValue(latest.value) : "";
  const valueMarkup = latest.hasValue ? `<span class="node-value">${escapeHTML(formattedValue)}</span>` : "";
  const title = latest.hasValue ? `${node.id}: ${formattedValue}` : node.id;
  return `<span class="${classes}" data-node-endpoint="true" data-component-id="${escapeAttr(componentID)}" data-node-id="${escapeAttr(node.id)}" data-direction="${escapeAttr(direction)}" title="${escapeAttr(title)}"><span class="node-label">${escapeHTML(node.id)}</span>${valueMarkup}</span>`;
}

function latestCanvasNodeValue(componentID, nodeID, direction) {
  const componentValues = direction === "output"
    ? state.latestResult?.component_outputs?.[componentID]
    : state.latestResult?.component_inputs?.[componentID];
  if (!componentValues || !Object.prototype.hasOwnProperty.call(componentValues, nodeID)) {
    return { hasValue: false, value: null };
  }
  return { hasValue: true, value: componentValues[nodeID] };
}

function handleCanvasEndpointClick(componentID, nodeID, direction) {
  if (!isWorkspaceProject()) {
    state.selectedComponentId = componentID;
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderProjectTree();
    updateCommandState();
    return;
  }
  state.selectedComponentId = componentID;
  state.selectedConnectionId = "";
  if (direction === "output") {
    state.pendingConnection = { component: componentID, node: nodeID };
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderProjectTree();
    updateCommandState();
    log(`Connection source selected: ${componentID}.${nodeID}`);
    return;
  }
  if (direction === "input" && state.pendingConnection) {
    if (state.pendingConnection.component === componentID) {
      showInlineProblem("Select an input on another component");
      return;
    }
    createConnection(state.pendingConnection.component, state.pendingConnection.node, componentID, nodeID);
    return;
  }
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  updateCommandState();
}

function drawConnections(positions) {
  const layer = el("connectionLayer");
  const graph = state.detail?.graph;
  const system = currentSystem();
  if (!graph || !system) return;
  layer.innerHTML = "";
  const defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
  defs.innerHTML = `<marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#617d98"></path></marker>`;
  layer.append(defs);

  for (const connectionId of system.connections) {
    const connection = graph.connections.find((item) => item.id === connectionId);
    if (!connection) continue;
    const from = positions[connection.from.component];
    const to = positions[connection.to.component];
    if (!from || !to) continue;
    const x1 = from.x + 190;
    const y1 = from.y + 63;
    const x2 = to.x;
    const y2 = to.y + 63;
    const mid = Math.max(40, (x2 - x1) / 2);
    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("class", `connection-line ${state.selectedConnectionId === connection.id ? "selected" : ""}`);
    path.dataset.connectionId = connection.id;
    path.setAttribute("marker-end", "url(#arrow)");
    path.setAttribute("d", `M ${x1} ${y1} C ${x1 + mid} ${y1}, ${x2 - mid} ${y2}, ${x2} ${y2}`);
    path.addEventListener("click", (event) => {
      event.stopPropagation();
      selectConnection(connection.id);
    });
    layer.append(path);
  }
}

function selectConnection(connectionID) {
  const connection = state.detail?.graph?.connections?.find((item) => item.id === connectionID);
  if (!connection) return;
  state.selectedConnectionId = connection.id;
  state.selectedComponentId = connection.to.component;
  state.pendingConnection = null;
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  updateCommandState();
  log(`Connection selected: ${connection.from.component}.${connection.from.node} -> ${connection.to.component}.${connection.to.node}`);
}

function renderInspector() {
  const container = el("inspector");
  container.innerHTML = "";
  const component = componentById(state.selectedComponentId);
  if (!component) {
    container.innerHTML = `<div class="inspector-block"><div class="inspector-title">Selection</div><div class="kv"><span class="kv-key">Item</span><span>Project</span></div></div>`;
    return;
  }
  container.append(inspectorBlock("Component", [
    ["ID", component.id],
    ["Name", component.name || ""],
    ["Kind", component.kind],
    ["Class", component.class || ""],
  ]));
  if (isWorkspaceProject()) container.append(componentEditor(component));
  container.append(nodeListBlock("Inputs", component, component.nodes.inputs || []));
  container.append(nodeListBlock("Outputs", component, component.nodes.outputs || []));
  if (isWorkspaceProject()) container.append(nodeEditor(component));
  container.append(inspectorBlock("Parameters", Object.entries(component.parameters || {}).map(([k, v]) => [k, String(v)])));
  container.append(connectionEditor(component));
  const latestInputs = state.latestResult?.component_inputs?.[component.id];
  const latestOutputs = state.latestResult?.component_outputs?.[component.id];
  if (latestInputs) {
    container.append(inspectorBlock("Last Inputs", Object.entries(latestInputs).map(([k, v]) => [k, formatValue(v)])));
  }
  if (latestOutputs) {
    container.append(inspectorBlock("Last Outputs", Object.entries(latestOutputs).map(([k, v]) => [k, formatValue(v)])));
  }
}

function inspectorBlock(title, rows) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  if (!rows.length) {
    const row = document.createElement("div");
    row.className = "kv";
    row.innerHTML = `<span class="kv-key">empty</span><span></span>`;
    block.append(row);
    return block;
  }
  for (const [key, value] of rows) {
    const row = document.createElement("div");
    row.className = "kv";
    row.innerHTML = `<span class="kv-key">${escapeHTML(key)}</span><span>${escapeHTML(value)}</span>`;
    block.append(row);
  }
  return block;
}

function nodeListBlock(title, component, nodes) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  if (!nodes.length) {
    const row = document.createElement("div");
    row.className = "kv";
    row.innerHTML = `<span class="kv-key">empty</span><span></span>`;
    block.append(row);
    return block;
  }
  for (const node of nodes) {
    const row = document.createElement("div");
    row.className = "kv connection-row";
    row.innerHTML = `
      <span class="kv-key">${escapeHTML(node.id)}</span>
      <span class="connection-value">
        <span>${escapeHTML(`${node.medium || ""} ${node.value_type || ""} ${node.unit || ""}`.trim())}</span>
      </span>
    `;
    if (isWorkspaceProject()) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "small-action";
      button.textContent = "Delete";
      button.addEventListener("click", () => deleteNodeFromInspector(component.id, node.id));
      row.querySelector(".connection-value").append(button);
    }
    block.append(row);
  }
  return block;
}

function componentEditor(component) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Component Settings</div>`;

  const form = document.createElement("div");
  form.className = "connection-form";
  const name = document.createElement("input");
  name.id = "componentNameInput";
  name.value = component.name || component.id;
  name.setAttribute("aria-label", "Component name");

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Rename";
  button.addEventListener("click", () => updateComponentFromInspector(component.id));
  const duplicateButton = document.createElement("button");
  duplicateButton.type = "button";
  duplicateButton.textContent = "Duplicate";
  duplicateButton.addEventListener("click", () => duplicateComponentFromInspector(component.id));
  name.addEventListener("keydown", (event) => {
    if (event.key === "Enter") updateComponentFromInspector(component.id);
  });

  form.append(name, button, duplicateButton);
  block.append(form);
  return block;
}

function nodeEditor(component) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Node</div>`;

  const form = document.createElement("div");
  form.className = "connection-form node-form";

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

  const valueType = document.createElement("select");
  valueType.id = "newNodeValueType";
  for (const type of ["float", "int", "bool", "string"]) {
    const option = document.createElement("option");
    option.value = type;
    option.textContent = type;
    valueType.append(option);
  }

  const defaultValue = document.createElement("input");
  defaultValue.id = "newNodeDefault";
  defaultValue.placeholder = "default";
  defaultValue.setAttribute("aria-label", "Default value");

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add Node";
  button.addEventListener("click", () => addNodeFromInspector(component.id));

  for (const input of [nodeID, defaultValue]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addNodeFromInspector(component.id);
    });
  }
  form.append(direction, nodeID, valueType, defaultValue, button);
  block.append(form);
  return block;
}

function connectionEditor(targetComponent) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Connections</div>`;
  const existingRows = connectionRowsFor(targetComponent);
  const canEditConnections = isWorkspaceProject() && selectedComponentInSystem();
  if (existingRows.length) {
    for (const connectionRow of existingRows) {
      const rowEl = document.createElement("div");
      rowEl.className = `kv connection-row ${connectionRow.id === state.selectedConnectionId ? "selected" : ""}`;
      rowEl.innerHTML = `
        <span class="kv-key">${escapeHTML(connectionRow.key)}</span>
        <span class="connection-value">
          <span>${escapeHTML(connectionRow.value)}</span>
        </span>
      `;
      rowEl.addEventListener("click", () => selectConnection(connectionRow.id));
      if (canEditConnections) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "small-action";
        button.textContent = "Remove";
        button.addEventListener("click", (event) => {
          event.stopPropagation();
          deleteConnectionFromInspector(connectionRow.id);
        });
        rowEl.querySelector(".connection-value").append(button);
      }
      block.append(rowEl);
    }
  }

  if (!canEditConnections) {
    if (!existingRows.length) {
      const row = document.createElement("div");
      row.className = "kv";
      row.innerHTML = `<span class="kv-key">empty</span><span></span>`;
      block.append(row);
    }
    return block;
  }

  const sourceOptions = systemOutputEndpoints(targetComponent.id);
  const targetOptions = targetComponent.nodes.inputs || [];
  if (!sourceOptions.length || !targetOptions.length) return block;

  const form = document.createElement("div");
  form.className = "connection-form";
  const sourceSelect = document.createElement("select");
  sourceSelect.dataset.connectionSource = "true";
  for (const endpoint of sourceOptions) {
    const option = document.createElement("option");
    option.value = `${endpoint.component}.${endpoint.node}`;
    option.textContent = `${endpoint.component}.${endpoint.node}`;
    sourceSelect.append(option);
  }
  const targetSelect = document.createElement("select");
  targetSelect.dataset.connectionTarget = "true";
  for (const node of targetOptions) {
    const option = document.createElement("option");
    option.value = node.id;
    option.textContent = `${targetComponent.id}.${node.id}`;
    targetSelect.append(option);
  }
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Connect";
  button.addEventListener("click", () => createConnectionFromInspector(sourceSelect.value, targetComponent.id, targetSelect.value));
  form.append(sourceSelect, targetSelect, button);
  block.append(form);
  return block;
}

function connectionRowsFor(component) {
  const graph = state.detail?.graph;
  const system = currentSystem();
  if (!graph || !system) return [];
  const rows = [];
  for (const connectionId of system.connections || []) {
    const connection = graph.connections.find((item) => item.id === connectionId);
    if (!connection) continue;
    if (connection.to.component === component.id) {
      rows.push({ id: connection.id, key: `input ${connection.to.node}`, value: `${connection.from.component}.${connection.from.node}` });
    }
    if (connection.from.component === component.id) {
      rows.push({ id: connection.id, key: `output ${connection.from.node}`, value: `${connection.to.component}.${connection.to.node}` });
    }
  }
  return rows;
}

function systemOutputEndpoints(excludeComponentId) {
  const system = currentSystem();
  if (!system) return [];
  return system.components
    .map(componentById)
    .filter((component) => component && component.id !== excludeComponentId)
    .flatMap((component) => (component.nodes.outputs || []).map((node) => ({ component: component.id, node: node.id })));
}

function renderParameters() {
  const tbody = el("parameterRows");
  const addForm = el("parameterAddForm");
  tbody.innerHTML = "";
  addForm.innerHTML = "";
  const components = state.detail?.graph?.components || [];
  const editable = isWorkspaceProject();
  renderParameterAddForm(addForm, components, editable);
  let count = 0;
  for (const component of components) {
    for (const [name, value] of Object.entries(component.parameters || {})) {
      count++;
      tbody.append(parameterRow(component, name, value, editable));
    }
  }
  if (!count) {
    tbody.append(emptyRow(4));
  }
}

function renderParameterAddForm(container, components, editable) {
  if (!editable || !components.length) return;
  const select = document.createElement("select");
  select.id = "newParameterComponent";
  select.setAttribute("aria-label", "Component");
  for (const component of components) {
    const option = document.createElement("option");
    option.value = component.id;
    option.textContent = component.id;
    select.append(option);
  }
  if (state.selectedComponentId && components.some((component) => component.id === state.selectedComponentId)) {
    select.value = state.selectedComponentId;
  }

  const name = document.createElement("input");
  name.id = "newParameterName";
  name.placeholder = "name";
  name.setAttribute("aria-label", "Parameter name");

  const value = document.createElement("input");
  value.id = "newParameterValue";
  value.placeholder = "value";
  value.setAttribute("aria-label", "Parameter value");

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add";
  button.addEventListener("click", addParameterFromManager);

  for (const input of [name, value]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addParameterFromManager();
    });
  }
  container.append(select, name, value, button);
}

function parameterRow(component, name, value, editable) {
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
      markProjectDirty();
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
    button.addEventListener("click", () => deleteParameterFromManager(component.id, name));
    actionCell.append(button);
  }
  tr.append(actionCell);
  return tr;
}

function emptyRow(cols) {
  const tr = document.createElement("tr");
  tr.innerHTML = `<td colspan="${cols}" class="empty-cell">empty</td>`;
  return tr;
}

function renderProblems() {
  const panel = el("problemsPanel");
  panel.innerHTML = "";
  const problems = state.latestValidation?.problems || [];
  if (problems.length) {
    for (const problem of problems) panel.append(problemRow(problem));
    return;
  }
  if (state.latestValidation?.error) {
    panel.append(problemRow({ severity: "error", message: state.latestValidation.error }));
    return;
  }
  panel.append(problemRow({ severity: "ok", message: "No problems" }));
}

function setProblems(problems = []) {
  state.latestValidation = { problems };
}

function problemRow(problem) {
  const row = document.createElement("div");
  row.className = "problem-row";
  const location = problem.line ? `:${problem.line}${problem.column ? `:${problem.column}` : ""}` : "";
  row.innerHTML = `<span class="status-dot ${problem.severity === "error" ? "error" : ""}"></span><span>${escapeHTML(problem.message)}${escapeHTML(location)}</span>`;
  if (problem.component_id) {
    row.classList.add("linked");
    row.addEventListener("click", () => openProblem(problem));
  }
  return row;
}

function openProblem(problem) {
  if (!problem.component_id || !componentById(problem.component_id)) return;
  state.selectedComponentId = problem.component_id;
  if (problem.line) {
    state.pendingSourceFocus = {
      component_id: problem.component_id,
      line: problem.line,
      column: problem.column || 1,
    };
    setMode("code");
  } else {
    setMode("canvas");
    state.pendingSourceFocus = null;
  }
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  updateCommandState();
}

function renderLogs() {
  el("logsPanel").textContent = state.logs.join("\n");
}

function renderResults() {
  const value = state.latestBatchRecord || state.latestRunRecord || state.latestResult;
  el("resultsPanel").textContent = value ? JSON.stringify(value, null, 2) : "";
}

function renderRunWorkspace() {
  renderRunOutputWorkspace(state, el("runSummaryRows"), el("runOutputRows"), el("runOutputChart"));
}

function renderSchema() {
  el("schemaPanel").textContent = state.latestSchema ? JSON.stringify(state.latestSchema, null, 2) : "";
}

function renderPythonPanel() {
  const component = componentById(state.selectedComponentId);
  renderSourceComponentSelect(component?.id || "");
  renderSourceContract(component);
  setSourceEditors("", true);
  updateSourceChrome(component, null, "");
  if (!component) {
    updateLineNumbers("");
    return;
  }
  const source = state.sourceByComponent[component.id];
  if (!source) {
    updateSourceChrome(component, null, "");
    updateLineNumbers("");
    loadComponentSource(component.id);
    return;
  }
  const draft = sourceDraft(component.id);
  setSourceEditors(draft, source.read_only || !isWorkspaceProject());
  updateSourceChrome(component, source, draft);
  updateLineNumbers(draft);
  renderSourceCheck(component.id);
  focusPendingSourceLine(component.id);
}

function sourceEditors() {
  return [el("sourceEditor"), el("pythonPanel")].filter(Boolean);
}

function sourceDraft(componentID) {
  const source = state.sourceByComponent[componentID];
  if (!source) return "";
  return Object.prototype.hasOwnProperty.call(state.sourceDraftByComponent, componentID)
    ? state.sourceDraftByComponent[componentID]
    : source.content;
}

function setSourceEditors(value, readOnly) {
  for (const editor of sourceEditors()) {
    if (document.activeElement !== editor || editor.value !== value) {
      editor.value = value;
    }
    editor.readOnly = readOnly;
  }
}

function renderSourceComponentSelect(selectedID) {
  const select = el("sourceComponentSelect");
  if (!select) return;
  const components = state.detail?.graph?.components || [];
  select.innerHTML = "";
  for (const component of components) {
    const option = document.createElement("option");
    option.value = component.id;
    option.textContent = component.id;
    select.append(option);
  }
  select.disabled = !components.length;
  if (selectedID) select.value = selectedID;
}

function updateSourceChrome(component, source, draft) {
  const path = el("sourcePath");
  const status = el("sourceStatus");
  const editable = Boolean(component && source && !source.read_only && isWorkspaceProject());
  const dirty = Boolean(component && source && draft !== source.content);
  if (path) path.textContent = source?.relative_path || component?.class || "";
  if (status) {
    status.className = "source-status";
    if (!component) {
      status.textContent = "";
    } else if (!source) {
      status.textContent = "loading";
    } else if (source.read_only || !isWorkspaceProject()) {
      status.textContent = "read only";
    } else if (dirty) {
      status.textContent = "modified";
    } else {
      status.textContent = "saved";
      status.classList.add("ok");
    }
  }
  for (const id of ["saveSourceButton", "revertSourceButton", "checkSourceButton", "insertSnippetButton", "sourceSnippetSelect"]) {
    const control = el(id);
    if (control) control.disabled = !component || !source || (id !== "checkSourceButton" && !editable);
  }
}

function renderSourceContract(component) {
  const container = el("sourceContract");
  if (!container) return;
  container.innerHTML = "";
  if (!component) return;
  container.append(contractBlock("Component", [
    [component.id, component.class || ""],
    [component.kind || "", component.name || ""],
  ]));
  container.append(contractBlock("Inputs", (component.nodes.inputs || []).map((node) => [node.id, nodeTypeLabel(node)])));
  container.append(contractBlock("Outputs", (component.nodes.outputs || []).map((node) => [node.id, nodeTypeLabel(node)])));
  container.append(contractBlock("Parameters", Object.entries(component.parameters || {}).map(([name, value]) => [name, parameterInputValue(value)])));
  container.append(sourceIssueBlock(component.id));
}

function contractBlock(title, rows) {
  const block = document.createElement("div");
  block.className = "contract-block";
  block.innerHTML = `<div class="contract-title">${escapeHTML(title)}</div>`;
  if (!rows.length) {
    const empty = document.createElement("div");
    empty.className = "contract-row";
    empty.innerHTML = `<span>empty</span><span class="contract-meta"></span>`;
    block.append(empty);
    return block;
  }
  for (const [name, meta] of rows) {
    const rowEl = document.createElement("div");
    rowEl.className = "contract-row";
    rowEl.innerHTML = `<span>${escapeHTML(name)}</span><span class="contract-meta">${escapeHTML(meta)}</span>`;
    block.append(rowEl);
  }
  return block;
}

function nodeTypeLabel(node) {
  return `${node.value_type || ""} ${node.unit || ""}`.trim() || node.medium || "";
}

function renderSourceCheck(componentID) {
  const status = el("sourceStatus");
  const check = state.sourceCheckByComponent[componentID];
  if (!status || !check) return;
  status.className = "source-status";
  const problems = check.problems || [];
  const errorCount = problems.filter((problem) => problem.severity === "error").length;
  if (!problems.length) {
    status.textContent = "checked";
    status.classList.add("ok");
  } else if (errorCount) {
    status.textContent = `${errorCount} error${errorCount === 1 ? "" : "s"}`;
    status.classList.add("error");
  } else {
    status.textContent = `${problems.length} warning${problems.length === 1 ? "" : "s"}`;
  }
}

function sourceIssueBlock(componentID) {
  const block = document.createElement("div");
  block.className = "contract-block source-issues";
  block.innerHTML = `<div class="contract-title">Source Check</div>`;
  const check = state.sourceCheckByComponent[componentID];
  if (!check) {
    block.append(sourceIssueRow({ severity: "info", message: "No source check yet" }));
    return block;
  }
  const problems = check.problems || [];
  if (!problems.length) {
    block.append(sourceIssueRow({ severity: "ok", message: "No source issues" }));
    return block;
  }
  for (const problem of problems) block.append(sourceIssueRow(problem));
  return block;
}

function sourceIssueRow(problem) {
  const row = document.createElement("div");
  const line = problem.line ? `:${problem.line}${problem.column ? `:${problem.column}` : ""}` : "";
  row.className = `contract-row source-issue ${problem.severity === "error" ? "error" : ""}`;
  row.innerHTML = `
    <span>${escapeHTML(problem.message)}${escapeHTML(line)}</span>
    <span class="contract-meta">${escapeHTML(problem.severity || "")}</span>
  `;
  if (problem.line) {
    row.classList.add("linked");
    row.addEventListener("click", () => focusSourceIssue(problem));
  }
  return row;
}

function focusSourceIssue(problem) {
  const componentID = problem.component_id || state.selectedComponentId;
  if (!componentID || !problem.line) return;
  state.pendingSourceFocus = {
    component_id: componentID,
    line: problem.line,
    column: problem.column || 1,
  };
  focusPendingSourceLine(componentID);
}

function updateLineNumbers(value) {
  const gutter = el("sourceLineNumbers");
  if (!gutter) return;
  const lines = Math.max(1, (value.match(/\n/g) || []).length + 1);
  gutter.textContent = Array.from({ length: lines }, (_, index) => String(index + 1)).join("\n");
}

function focusPendingSourceLine(componentID) {
  const pending = state.pendingSourceFocus;
  if (!pending || pending.component_id !== componentID) return;
  const editor = el("sourceEditor");
  if (!editor) return;
  const line = Math.max(1, Number(pending.line) || 1);
  const column = Math.max(1, Number(pending.column) || 1);
  const position = sourceOffsetForLineColumn(editor.value, line, column);
  editor.focus();
  editor.setSelectionRange(position, position);
  const lineCount = Math.max(1, (editor.value.match(/\n/g) || []).length + 1);
  editor.scrollTop = ((line - 1) / lineCount) * editor.scrollHeight;
  state.pendingSourceFocus = null;
}

function sourceOffsetForLineColumn(value, line, column) {
  let offset = 0;
  for (let currentLine = 1; currentLine < line; currentLine++) {
    const next = value.indexOf("\n", offset);
    if (next < 0) return value.length;
    offset = next + 1;
  }
  return Math.min(value.length, offset + column - 1);
}

function renderExportWorkspaceView() {
  renderExportWorkspace(state, el("exportSummaryRows"), el("exportFileRows"), el("exportManifest"));
}

async function validateProject() {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/validate", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath }),
    });
    state.latestValidation = body.validation;
    log("Validation ok");
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    log(`Validation failed: ${error.message}`);
  }
  renderProblems();
  setBottomTab("problems");
}

async function runProject() {
  if (!(await saveModelEditsBeforeExecution())) return;
  const inputs = collectRunInputs();
  try {
    const save = currentProject()?.source === "workspace";
    const body = await api("/api/run", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, inputs, context: currentRunContext(), save }),
    });
    state.latestResult = body.result;
    state.latestRunRecord = null;
    state.latestBatchRecord = null;
    setProblems();
    if (body.run_record) {
      state.detail.runs = [body.run_record, ...(state.detail.runs || [])];
      log(`Run saved: ${body.run_record.relative_path}`);
      renderProjectTree();
    } else {
      log("Run complete");
    }
    setMode("run");
    setBottomTab("results");
  } catch (error) {
    log(`Run failed: ${error.message}`);
    state.latestResult = null;
    state.latestRunRecord = null;
    state.latestBatchRecord = null;
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    setBottomTab("problems");
  }
  renderCanvas();
  renderInspector();
  renderProblems();
  renderResults();
  renderRunWorkspace();
}

async function runBatch() {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/batch", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath }),
    });
    state.latestBatchRecord = body.batch;
    state.latestRunRecord = null;
    state.latestResult = null;
    const batchProblems = collectBatchProblems(body.batch);
    state.latestValidation = { problems: batchProblems };
    state.detail.batches = [body.summary, ...(state.detail.batches || [])];
    renderProjectTree();
    renderCanvas();
    renderResults();
    renderRunWorkspace();
    renderProblems();
    setMode("run");
    setBottomTab(batchProblems.length ? "problems" : "results");
    log(`Batch saved: ${body.summary.relative_path}`);
  } catch (error) {
    log(`Batch failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function collectBatchProblems(record) {
  const problems = [];
  for (const item of record?.cases || []) {
    if (item.ok) continue;
    const caseName = item.scenario_name || item.scenario_id || "batch case";
    const caseProblems = item.problems || [];
    if (caseProblems.length) {
      for (const problem of caseProblems) {
        problems.push({ ...problem, message: `${caseName}: ${problem.message}` });
      }
    } else if (item.error) {
      problems.push({ severity: "error", message: `${caseName}: ${item.error}` });
    }
  }
  return problems;
}

async function saveModelEditsBeforeExecution() {
  if (!isWorkspaceProject()) return true;
  const parameters = collectParameterUpdates();
  const sourceUpdates = collectSourceUpdates();
  const sourceProblems = [];
  try {
    if (Object.keys(parameters).length) {
      const body = await api("/api/project/parameters", {
        method: "POST",
        body: JSON.stringify({ project_path: state.currentProjectPath, parameters }),
      });
      state.detail = body.project;
    }
    for (const sourceUpdate of sourceUpdates) {
      const sourceBody = await api("/api/project/source", {
        method: "POST",
        body: JSON.stringify({ project_path: state.currentProjectPath, component_id: sourceUpdate.component_id, content: sourceUpdate.content }),
      });
      sourceProblems.push(...applySourceSaveResponse(sourceUpdate.component_id, sourceBody));
    }
    if (sourceProblems.some((problem) => problem.severity === "error")) {
      state.latestValidation = { problems: sourceProblems };
      renderProblems();
      setBottomTab("problems");
      log("Source validation failed before execution");
      return false;
    }
    if (sourceProblems.length) state.latestValidation = { problems: sourceProblems };
    return true;
  } catch (error) {
    log(`Save before execution failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    return false;
  }
}

async function loadRunRecord(runID) {
  try {
    const body = await api(`/api/project/run?project_path=${encodeURIComponent(state.currentProjectPath)}&run_id=${encodeURIComponent(runID)}`);
    state.latestRunRecord = body.run_record;
    state.latestBatchRecord = null;
    state.latestResult = body.run_record.result;
    setProblems();
    renderCanvas();
    renderInspector();
    renderResults();
    renderRunWorkspace();
    setMode("run");
    setBottomTab("results");
    log(`Run opened: ${runID}`);
  } catch (error) {
    log(`Open run failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function loadBatchRecord(batchID) {
  try {
    const body = await api(`/api/project/batch?project_path=${encodeURIComponent(state.currentProjectPath)}&batch_id=${encodeURIComponent(batchID)}`);
    state.latestBatchRecord = body.batch_record;
    state.latestRunRecord = null;
    state.latestResult = null;
    const batchProblems = collectBatchProblems(body.batch_record);
    state.latestValidation = { problems: batchProblems };
    renderCanvas();
    renderResults();
    renderRunWorkspace();
    renderProblems();
    setMode("run");
    setBottomTab(batchProblems.length ? "problems" : "results");
    log(`Batch opened: ${batchID}`);
  } catch (error) {
    log(`Open batch failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function loadScenario(scenarioID) {
  try {
    const body = await api(`/api/project/scenario?project_path=${encodeURIComponent(state.currentProjectPath)}&scenario_id=${encodeURIComponent(scenarioID)}`);
    state.activeRunInput = body.scenario;
    renderRunInputs();
    log(`Scenario loaded: ${body.scenario.name || scenarioID}`);
  } catch (error) {
    log(`Open scenario failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function loadComponentSource(componentID) {
  if (!componentID || state.loadingSource[componentID]) return;
  state.loadingSource[componentID] = true;
  try {
    const body = await api(`/api/project/source?project_path=${encodeURIComponent(state.currentProjectPath)}&component_id=${encodeURIComponent(componentID)}`);
    state.sourceByComponent[componentID] = body.source;
    if (!Object.prototype.hasOwnProperty.call(state.sourceDraftByComponent, componentID)) {
      state.sourceDraftByComponent[componentID] = body.source.content;
    }
    if (state.selectedComponentId === componentID) {
      renderPythonPanel();
    }
  } catch (error) {
    log(`Source load failed: ${error.message}`);
  } finally {
    state.loadingSource[componentID] = false;
  }
}

async function saveCurrentSource() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  if (!component || !source || source.read_only || !isWorkspaceProject()) return;
  const content = sourceDraft(component.id);
  try {
    const body = await api("/api/project/source", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id, content }),
    });
    const sourceProblems = applySourceSaveResponse(component.id, body);
    state.latestValidation = { problems: sourceProblems };
    renderPythonPanel();
    renderProblems();
    if (sourceProblems.length) setBottomTab("problems");
    log(`Source saved: ${component.id}`);
  } catch (error) {
    log(`Source save failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function applySourceSaveResponse(componentID, body) {
  state.sourceByComponent[componentID] = body.source;
  state.sourceDraftByComponent[componentID] = body.source.content;
  if (!body.check) return [];
  state.sourceCheckByComponent[componentID] = body.check;
  return body.check.problems || [];
}

async function checkCurrentSource() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  if (!component || !source) return;
  try {
    const body = await api("/api/project/source/check", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id, content: sourceDraft(component.id) }),
    });
    state.sourceCheckByComponent[component.id] = body.check;
    state.latestValidation = { problems: body.check.problems || [] };
    renderSourceCheck(component.id);
    renderSourceContract(component);
    renderProblems();
    if (!body.check.ok) setBottomTab("problems");
    log(`Source checked: ${component.id}`);
  } catch (error) {
    log(`Source check failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function revertCurrentSource() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  if (!component || !source || source.read_only || !isWorkspaceProject()) return;
  state.sourceDraftByComponent[component.id] = source.content;
  renderPythonPanel();
  log(`Source reverted: ${component.id}`);
}

function insertSourceSnippet() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  const editor = el("sourceEditor") || el("pythonPanel");
  if (!component || !source || source.read_only || !isWorkspaceProject() || !editor || editor.readOnly) return;
  const snippet = sourceSnippet(el("sourceSnippetSelect")?.value || "evaluate", component);
  const start = editor.selectionStart ?? editor.value.length;
  const end = editor.selectionEnd ?? editor.value.length;
  editor.value = `${editor.value.slice(0, start)}${snippet}${editor.value.slice(end)}`;
  editor.selectionStart = editor.selectionEnd = start + snippet.length;
  updateSourceDraftFromEditor(editor);
  editor.focus();
}

function sourceSnippet(kind, component) {
  const inputBindings = pythonInputBindings(component);
  const firstInput = inputBindings[0]?.id || "value";
  const firstOutput = (component.nodes.outputs || [])[0]?.id || "result";
  const firstParam = Object.keys(component.parameters || {})[0] || "gain";
  switch (kind) {
    case "initialize":
      return `\n    def initialize(self, params, context):\n        return {}\n`;
    case "output":
      return `${pythonStringLiteral(firstOutput)}: value`;
    case "input":
      return `inputs.get(${pythonStringLiteral(firstInput)}, 0.0)`;
    case "parameter":
      return `params.get(${pythonStringLiteral(firstParam)}, 1.0)`;
    default:
      return evaluateSnippet(component, inputBindings);
  }
}

function evaluateSnippet(component, inputBindings) {
  const bindings = inputBindings.length ? inputBindings : [{ id: "value", varName: "value" }];
  const inputLines = bindings.map((item) => `        ${item.varName} = float(inputs.get(${pythonStringLiteral(item.id)}, 0.0))`).join("\n");
  const primaryValue = bindings[0].varName;
  const outputs = component.nodes.outputs || [];
  const outputLines = (outputs.length ? outputs : [{ id: "result" }])
    .map((node) => `            ${pythonStringLiteral(node.id)}: ${primaryValue},`)
    .join("\n");
  return `\n    def evaluate(self, inputs, state, params, context):\n${inputLines}\n        return {\n${outputLines}\n        }, state\n`;
}

function pythonInputBindings(component) {
  const used = new Set();
  return (component.nodes.inputs || []).map((node, index) => {
    const fallback = `input_${index + 1}`;
    const base = pythonIdentifier(node.id) || fallback;
    let candidate = base;
    let suffix = 2;
    while (used.has(candidate)) {
      candidate = `${base}_${suffix}`;
      suffix += 1;
    }
    used.add(candidate);
    return { id: node.id || fallback, varName: candidate };
  });
}

function pythonIdentifier(value) {
  const identifier = String(value || "")
    .replace(/[^A-Za-z0-9_]/g, "_")
    .replace(/^([0-9])/, "_$1")
    .replace(/^_+$/, "");
  return identifier || "";
}

function pythonStringLiteral(value) {
  return JSON.stringify(String(value || ""));
}

async function saveProjectEdits() {
  if (!isWorkspaceProject()) {
    log("Only workspace projects can be edited");
    return;
  }
  const parameters = collectParameterUpdates();
  const inputs = collectRunInputs();
  const context = currentRunContext();
  const sourceUpdates = collectSourceUpdates();
  const sourceProblems = [];
  try {
    let body = await api("/api/project/input", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, inputs, context }),
    });
    if (Object.keys(parameters).length) {
      body = await api("/api/project/parameters", {
        method: "POST",
        body: JSON.stringify({ project_path: state.currentProjectPath, parameters }),
      });
    }
    for (const sourceUpdate of sourceUpdates) {
      const sourceBody = await api("/api/project/source", {
        method: "POST",
        body: JSON.stringify({ project_path: state.currentProjectPath, component_id: sourceUpdate.component_id, content: sourceUpdate.content }),
      });
      sourceProblems.push(...applySourceSaveResponse(sourceUpdate.component_id, sourceBody));
    }
    state.detail = body.project;
    setProblems(sourceProblems);
    el("saveProjectButton").classList.remove("dirty");
    renderAll();
    if (sourceProblems.some((problem) => problem.severity === "error")) setBottomTab("problems");
    log("Project saved");
  } catch (error) {
    log(`Save failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    renderProblems();
    setBottomTab("problems");
  }
}

async function exportSchema() {
  try {
    const body = await api("/api/schema", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath }),
    });
    state.latestSchema = body.schema;
    log("Schema exported");
    setBottomTab("schema");
  } catch (error) {
    log(`Schema failed: ${error.message}`);
  }
  renderSchema();
}

async function exportProject() {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/export", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, profile: "runtime_package" }),
    });
    state.latestExport = body.export;
    state.detail.exports = [body.summary, ...(state.detail.exports || []).filter((item) => item.profile !== body.summary.profile)];
    setProblems();
    renderProjectTree();
    renderExportWorkspaceView();
    setMode("export");
    log(`Export manifest written: ${body.summary.relative_path}`);
  } catch (error) {
    log(`Export failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function createScenario() {
  if (!isWorkspaceProject()) {
    log("Only workspace projects can be edited");
    return;
  }
  const name = window.prompt("Scenario name", "Current Inputs");
  if (!name || !name.trim()) return;
  try {
    const body = await api("/api/project/scenarios", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name: name.trim(), inputs: collectRunInputs(), context: currentRunContext() }),
    });
    state.detail.scenarios = [body.summary, ...(state.detail.scenarios || [])];
    renderProjectTree();
    log(`Scenario saved: ${body.summary.relative_path}`);
  } catch (error) {
    log(`Scenario failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function createProject() {
  const name = window.prompt("Project name", "New Python Component Project");
  if (!name || !name.trim()) return;
  try {
    const body = await api("/api/projects", {
      method: "POST",
      body: JSON.stringify({ name: name.trim(), template: "scalar" }),
    });
    await loadProjects(body.project.project_path);
    log(`Created ${body.project.relative_path}`);
  } catch (error) {
    log(`Create project failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    renderProblems();
    setBottomTab("problems");
  }
}

async function copyProject() {
  const project = currentProject();
  if (!project) return;
  const sourceName = state.detail?.project?.project_name || project.name || "Project";
  const defaultName = `${sourceName} Copy`;
  const name = window.prompt("Copy project as", defaultName);
  if (!name || !name.trim()) return;
  try {
    const body = await api("/api/projects/copy", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name: name.trim() }),
    });
    await loadProjects(body.project.project_path);
    log(`Copied project: ${body.project.relative_path}`);
  } catch (error) {
    log(`Copy project failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function createComponent() {
  if (!isWorkspaceProject()) {
    log("Only workspace projects can be edited");
    return;
  }
  const name = window.prompt("Component name", "New Scalar Component");
  if (!name || !name.trim()) return;
  try {
    const body = await api("/api/project/components", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name: name.trim(), template: "scalar" }),
    });
    state.detail = body.project;
    state.selectedComponentId = body.component.id;
    renderAll();
    log(`Component created: ${body.component.id}`);
  } catch (error) {
    log(`Create component failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    renderProblems();
    setBottomTab("problems");
  }
}

async function updateComponentFromInspector(componentID) {
  if (!componentID || !isWorkspaceProject()) return;
  const name = (el("componentNameInput")?.value || "").trim();
  if (!name) {
    showInlineProblem("Component name is required");
    return;
  }
  try {
    const body = await api("/api/project/components/update", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: componentID, name }),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    renderAll();
    log(`Component renamed: ${componentID}`);
  } catch (error) {
    log(`Rename component failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function duplicateComponentFromInspector(componentID) {
  const component = componentById(componentID);
  if (!component || !isWorkspaceProject()) return;
  const name = `${component.name || component.id} Copy`;
  try {
    const body = await api("/api/project/components/duplicate", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, source_component_id: componentID, name }),
    });
    state.detail = body.project;
    state.selectedComponentId = body.component.id;
    renderAll();
    log(`Component duplicated: ${componentID} -> ${body.component.id}`);
  } catch (error) {
    log(`Duplicate component failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function includeSelectedComponent() {
  const component = componentById(state.selectedComponentId);
  if (!component || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/system/components", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id }),
    });
    state.detail = body.project;
    renderAll();
    log(`Component added to system: ${component.id}`);
  } catch (error) {
    log(`Add to system failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    renderProblems();
    setBottomTab("problems");
  }
}

async function removeSelectedComponentFromSystem() {
  const component = componentById(state.selectedComponentId);
  if (!component || !isWorkspaceProject() || !selectedComponentInSystem()) return;
  if (!window.confirm(`Remove ${component.id} from the runnable system? The component source will remain in the project.`)) return;
  try {
    const body = await api("/api/project/system/components/remove", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id }),
    });
    state.detail = body.project;
    renderAll();
    log(`Component removed from system: ${component.id}`);
  } catch (error) {
    log(`Remove from system failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteSelectedComponent() {
  const component = componentById(state.selectedComponentId);
  if (!component || !isWorkspaceProject() || selectedComponentInSystem()) return;
  if (!window.confirm(`Delete component ${component.id} and its source file?`)) return;
  try {
    const body = await api("/api/project/components/delete", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id }),
    });
    delete state.sourceByComponent[component.id];
    delete state.sourceDraftByComponent[component.id];
    delete state.sourceCheckByComponent[component.id];
    state.detail = body.project;
    state.selectedComponentId = null;
    renderAll();
    log(`Component deleted: ${component.id}`);
  } catch (error) {
    log(`Delete component failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function createConnectionFromInspector(sourceValue, toComponent, toNode) {
  const [fromComponent, fromNode] = sourceValue.split(".");
  if (!fromComponent || !fromNode || !toComponent || !toNode) return;
  await createConnection(fromComponent, fromNode, toComponent, toNode);
}

async function createConnection(fromComponent, fromNode, toComponent, toNode) {
  if (!fromComponent || !fromNode || !toComponent || !toNode || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/connections", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        from_component: fromComponent,
        from_node: fromNode,
        to_component: toComponent,
        to_node: toNode,
      }),
    });
    state.detail = body.project;
    state.pendingConnection = null;
    state.selectedConnectionId = body.connection?.id || "";
    renderAll();
    log(`Connected ${fromComponent}.${fromNode} -> ${toComponent}.${toNode}`);
  } catch (error) {
    state.pendingConnection = null;
    state.selectedConnectionId = "";
    log(`Connection failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    renderCanvas();
    setBottomTab("problems");
  }
}

async function addNodeFromInspector(componentID) {
  if (!componentID || !isWorkspaceProject()) return;
  const component = componentById(componentID);
  const direction = el("newNodeDirection")?.value || "input";
  const nodeID = (el("newNodeId")?.value || "").trim();
  const valueType = el("newNodeValueType")?.value || "float";
  const rawDefault = el("newNodeDefault")?.value || "";
  if (!component || !nodeID) {
    showInlineProblem("Select a component and node id");
    return;
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(nodeID)) {
    showInlineProblem("Node id must start with a letter or underscore and contain only letters, numbers, and underscores");
    return;
  }
  const existingNodes = [...(component.nodes.inputs || []), ...(component.nodes.outputs || [])];
  if (existingNodes.some((node) => node.id === nodeID)) {
    showInlineProblem(`Node already exists: ${componentID}.${nodeID}`);
    return;
  }

  const payload = {
    project_path: state.currentProjectPath,
    component_id: componentID,
    direction,
    id: nodeID,
    name: nodeID,
    medium: "signal",
    value_type: valueType,
  };
  if (direction === "input" && rawDefault.trim() !== "") {
    payload.default = coerceParameter(rawDefault);
  }

  try {
    const body = await api("/api/project/nodes", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    renderAll();
    log(`Node added: ${componentID}.${nodeID}`);
  } catch (error) {
    log(`Add node failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteNodeFromInspector(componentID, nodeID) {
  if (!componentID || !nodeID || !isWorkspaceProject()) return;
  if (!window.confirm(`Delete node ${componentID}.${nodeID}? Related connections and public IO mappings will be updated.`)) return;
  try {
    const body = await api("/api/project/nodes/delete", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: componentID, node_id: nodeID }),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    renderAll();
    log(`Node deleted: ${componentID}.${nodeID}`);
  } catch (error) {
    log(`Delete node failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function addParameterFromManager() {
  if (!isWorkspaceProject()) return;
  const componentID = el("newParameterComponent")?.value || "";
  const name = (el("newParameterName")?.value || "").trim();
  const value = el("newParameterValue")?.value || "";
  const component = componentById(componentID);
  if (!component || !name) {
    showInlineProblem("Select a component and parameter name");
    return;
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name)) {
    showInlineProblem("Parameter name must start with a letter or underscore and contain only letters, numbers, and underscores");
    return;
  }
  if (Object.prototype.hasOwnProperty.call(component.parameters || {}, name)) {
    showInlineProblem(`Parameter already exists: ${componentID}.${name}`);
    return;
  }
  try {
    const body = await api("/api/project/parameters", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, parameters: { [componentID]: { [name]: coerceParameter(value) } } }),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    renderAll();
    log(`Parameter added: ${componentID}.${name}`);
  } catch (error) {
    log(`Add parameter failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteParameterFromManager(componentID, name) {
  if (!componentID || !name || !isWorkspaceProject()) return;
  if (!window.confirm(`Delete parameter ${componentID}.${name}?`)) return;
  try {
    const pending = parameterUpdatesExcluding(componentID, name);
    if (Object.keys(pending).length) {
      const updated = await api("/api/project/parameters", {
        method: "POST",
        body: JSON.stringify({ project_path: state.currentProjectPath, parameters: pending }),
      });
      state.detail = updated.project;
    }
    const body = await api("/api/project/parameters/delete", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: componentID, name }),
    });
    state.detail = body.project;
    renderAll();
    log(`Parameter deleted: ${componentID}.${name}`);
  } catch (error) {
    log(`Delete parameter failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteConnectionFromInspector(connectionId) {
  if (!connectionId || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/connections/delete", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, connection_id: connectionId }),
    });
    state.detail = body.project;
    if (state.selectedConnectionId === connectionId) state.selectedConnectionId = "";
    renderAll();
    log(`Connection removed: ${connectionId}`);
  } catch (error) {
    log(`Remove connection failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function showInlineProblem(message) {
  log(message);
  state.latestValidation = { error: message, problems: [] };
  renderProblems();
  setBottomTab("problems");
}

function currentSystem() {
  const detail = state.detail;
  if (!detail) return null;
  return detail.graph.systems.find((system) => system.id === detail.project.entry_system) || detail.graph.systems[0];
}

function currentProject() {
  return state.projects.find((project) => project.project_path === state.currentProjectPath);
}

function isWorkspaceProject() {
  return currentProject()?.source === "workspace";
}

function componentById(id) {
  return (state.detail?.graph?.components || []).find((component) => component.id === id);
}

function selectComponent(id) {
  if (!componentById(id)) return;
  state.selectedComponentId = id;
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  updateCommandState();
}

function selectedComponentInSystem() {
  const system = currentSystem();
  return Boolean(system && state.selectedComponentId && system.components.includes(state.selectedComponentId));
}

function collectRunInputs() {
  const inputs = {};
  for (const input of document.querySelectorAll("[data-input-id]")) {
    inputs[input.dataset.inputId] = coerceInput(input.value);
  }
  return inputs;
}

function currentRunContext() {
  return state.activeRunInput?.context || state.detail?.default_run_input?.context || { time: 0, dt: 60 };
}

function currentSourceUpdate() {
  const component = componentById(state.selectedComponentId);
  if (!component || !isWorkspaceProject()) return null;
  const source = state.sourceByComponent[component.id];
  const draft = sourceDraft(component.id);
  if (!source || source.read_only || draft === source.content) return null;
  return { component_id: component.id, content: draft };
}

function collectSourceUpdates() {
  if (!isWorkspaceProject()) return [];
  const updates = [];
  for (const [componentID, source] of Object.entries(state.sourceByComponent)) {
    const draft = sourceDraft(componentID);
    if (!source.read_only && draft !== source.content) {
      updates.push({ component_id: componentID, content: draft });
    }
  }
  return updates;
}

function collectParameterUpdates() {
  const updates = {};
  for (const input of document.querySelectorAll("[data-parameter-component]")) {
    const componentID = input.dataset.parameterComponent;
    const name = input.dataset.parameterName;
    updates[componentID] ||= {};
    updates[componentID][name] = coerceParameter(input.value);
  }
  return updates;
}

function parameterUpdatesExcluding(componentID, name) {
  const updates = collectParameterUpdates();
  if (updates[componentID]) {
    delete updates[componentID][name];
    if (!Object.keys(updates[componentID]).length) delete updates[componentID];
  }
  return updates;
}

function setMode(mode) {
  document.querySelectorAll(".mode-button").forEach((button) => {
    button.classList.toggle("active", button.dataset.mode === mode);
  });
  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("active", view.id === `${mode}View`);
  });
}

function setBottomTab(name) {
  document.querySelectorAll(".bottom-tab").forEach((button) => {
    button.classList.toggle("active", button.dataset.bottom === name);
  });
  document.querySelectorAll(".bottom-view").forEach((view) => {
    view.classList.toggle("active", view.id === `${name}Panel`);
  });
}

function updateCommandState() {
  const hasProject = Boolean(state.detail);
  el("validateButton").disabled = !hasProject;
  el("runButton").disabled = !hasProject;
  el("scenarioButton").disabled = !hasProject || !isWorkspaceProject();
  el("batchButton").disabled = !hasProject || !isWorkspaceProject();
  el("schemaButton").disabled = !hasProject;
  el("exportButton").disabled = !hasProject || !isWorkspaceProject();
  el("saveProjectButton").disabled = !hasProject || !isWorkspaceProject();
  el("copyProjectButton").disabled = !hasProject;
  el("addComponentButton").disabled = !hasProject || !isWorkspaceProject();
  el("includeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
  el("removeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || !selectedComponentInSystem();
  el("deleteComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
}

function markProjectDirty() {
  if (isWorkspaceProject()) {
    el("saveProjectButton").classList.add("dirty");
  }
}

function updateSourceDraftFromEditor(editor) {
  const component = componentById(state.selectedComponentId);
  if (!component) return;
  state.sourceDraftByComponent[component.id] = editor.value;
  delete state.sourceCheckByComponent[component.id];
  for (const other of sourceEditors()) {
    if (other !== editor && document.activeElement !== other) {
      other.value = editor.value;
    }
  }
  updateLineNumbers(editor.value);
  updateSourceChrome(component, state.sourceByComponent[component.id], editor.value);
  markProjectDirty();
}

function handleSourceEditorInput(event) {
  updateSourceDraftFromEditor(event.target);
}

function handleSourceEditorKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
    event.preventDefault();
    saveCurrentSource();
    return;
  }
  if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
    event.preventDefault();
    checkCurrentSource();
    return;
  }
  if (event.key === "Tab") {
    event.preventDefault();
    handleSourceIndent(event.target, event.shiftKey);
  }
}

function handleSourceIndent(editor, outdent) {
  const start = editor.selectionStart ?? 0;
  const end = editor.selectionEnd ?? start;
  if (!outdent && start === end) {
    editor.value = `${editor.value.slice(0, start)}    ${editor.value.slice(end)}`;
    editor.selectionStart = editor.selectionEnd = start + 4;
    updateSourceDraftFromEditor(editor);
    return;
  }

  const value = editor.value;
  const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  const adjustedEnd = end > start && value[end - 1] === "\n" ? end - 1 : end;
  const nextLineBreak = value.indexOf("\n", adjustedEnd);
  const lineEnd = nextLineBreak < 0 ? value.length : nextLineBreak;
  const selected = value.slice(lineStart, lineEnd);
  const lines = selected.split("\n");
  const transformed = lines.map((line) => (outdent ? outdentLine(line) : `    ${line}`));
  const replacement = transformed.join("\n");
  const delta = replacement.length - selected.length;
  const firstLineDelta = transformed[0].length - lines[0].length;

  editor.value = `${value.slice(0, lineStart)}${replacement}${value.slice(lineEnd)}`;
  editor.selectionStart = Math.max(lineStart, start + firstLineDelta);
  editor.selectionEnd = Math.max(editor.selectionStart, end + delta);
  updateSourceDraftFromEditor(editor);
}

function outdentLine(line) {
  if (line.startsWith("    ")) return line.slice(4);
  if (line.startsWith("\t")) return line.slice(1);
  const leadingSpaces = line.match(/^ +/)?.[0]?.length || 0;
  return line.slice(Math.min(4, leadingSpaces));
}

function syncSourceGutterScroll(event) {
  const gutter = el("sourceLineNumbers");
  if (gutter) gutter.scrollTop = event.target.scrollTop;
}

function bindEvents() {
  el("projectSelect").addEventListener("change", (event) => loadProject(event.target.value));
  el("newProjectButton").addEventListener("click", createProject);
  el("copyProjectButton").addEventListener("click", copyProject);
  el("addComponentButton").addEventListener("click", createComponent);
  el("includeComponentButton").addEventListener("click", includeSelectedComponent);
  el("removeComponentButton").addEventListener("click", removeSelectedComponentFromSystem);
  el("deleteComponentButton").addEventListener("click", deleteSelectedComponent);
  el("saveProjectButton").addEventListener("click", saveProjectEdits);
  el("validateButton").addEventListener("click", validateProject);
  el("runButton").addEventListener("click", runProject);
  el("scenarioButton").addEventListener("click", createScenario);
  el("batchButton").addEventListener("click", runBatch);
  el("schemaButton").addEventListener("click", exportSchema);
  el("exportButton").addEventListener("click", exportProject);
  el("sourceComponentSelect").addEventListener("change", (event) => selectComponent(event.target.value));
  el("saveSourceButton").addEventListener("click", saveCurrentSource);
  el("checkSourceButton").addEventListener("click", checkCurrentSource);
  el("revertSourceButton").addEventListener("click", revertCurrentSource);
  el("insertSnippetButton").addEventListener("click", insertSourceSnippet);
  for (const editor of sourceEditors()) {
    editor.addEventListener("input", handleSourceEditorInput);
    editor.addEventListener("keydown", handleSourceEditorKeydown);
  }
  el("sourceEditor").addEventListener("scroll", syncSourceGutterScroll);
  document.querySelectorAll(".mode-button").forEach((button) => {
    button.addEventListener("click", () => setMode(button.dataset.mode));
  });
  document.querySelectorAll(".bottom-tab").forEach((button) => {
    button.addEventListener("click", () => setBottomTab(button.dataset.bottom));
  });
}

bindEvents();
loadProjects().catch((error) => {
  el("runtimeStatus").textContent = "Runtime error";
  state.latestValidation = { error: error.message };
  renderProblems();
  log(error.message);
});
