const state = {
  projects: [],
  currentProjectPath: "",
  detail: null,
  selectedComponentId: "",
  latestResult: null,
  latestSchema: null,
  latestValidation: null,
  logs: [],
};

const el = (id) => document.getElementById(id);

function log(message) {
  const time = new Date().toLocaleTimeString();
  state.logs.unshift(`[${time}] ${message}`);
  renderLogs();
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const body = await response.json();
  if (!response.ok || body.ok === false) {
    throw new Error(body.message || `Request failed: ${path}`);
  }
  return body;
}

async function loadProjects() {
  const body = await api("/api/projects");
  state.projects = body.projects || [];
  const select = el("projectSelect");
  select.innerHTML = "";
  for (const project of state.projects) {
    const option = document.createElement("option");
    option.value = project.project_path;
    option.textContent = project.relative_path;
    select.append(option);
  }
  const feedForward = state.projects.find((p) => p.name === "003_feedforward_system");
  const first = feedForward || state.projects[0];
  if (first) {
    select.value = first.project_path;
    await loadProject(first.project_path);
  }
}

async function loadProject(projectPath) {
  state.currentProjectPath = projectPath;
  state.selectedComponentId = "";
  state.latestResult = null;
  state.latestSchema = null;
  state.latestValidation = null;

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
  renderPythonPanel();
  renderExportManifest();
  const project = state.detail?.project;
  el("systemTitle").textContent = project?.entry_system || "System";
  el("systemSubtitle").textContent = project ? `${project.project_name} / ${state.detail.graph_path}` : "";
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
    ["Datasets", []],
    ["Parameter Sets", [treeStatic("default", "active")]],
    ["Runs", []],
    ["Scenarios", []],
    ["Export Profiles", [treeStatic("research_project", "profile"), treeStatic("runtime_package", "profile")]],
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
      renderProjectTree();
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

function renderRunInputs() {
  const container = el("runInputs");
  container.innerHTML = "";
  const inputs = currentSystem()?.public_inputs || [];
  for (const input of inputs) {
    const field = document.createElement("div");
    field.className = "input-field";
    const defaultValue = input.default ?? sampleValueFor(input.id);
    field.innerHTML = `
      <label for="input-${escapeAttr(input.id)}">${escapeHTML(input.id)}</label>
      <input id="input-${escapeAttr(input.id)}" data-input-id="${escapeAttr(input.id)}" value="${escapeAttr(defaultValue)}" />
    `;
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
        ${component.nodes.inputs.map((n) => `<span class="node-pill">${escapeHTML(n.id)}</span>`).join("")}
        ${component.nodes.outputs.map((n) => `<span class="node-pill output">${escapeHTML(n.id)}</span>`).join("")}
      </div>
    `;
    node.addEventListener("click", () => {
      state.selectedComponentId = component.id;
      renderCanvas();
      renderInspector();
      renderProjectTree();
    });
    canvas.append(node);
  });

  requestAnimationFrame(() => drawConnections(positions));
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
    path.setAttribute("class", "connection-line");
    path.setAttribute("marker-end", "url(#arrow)");
    path.setAttribute("d", `M ${x1} ${y1} C ${x1 + mid} ${y1}, ${x2 - mid} ${y2}, ${x2} ${y2}`);
    layer.append(path);
  }
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
  container.append(inspectorBlock("Inputs", component.nodes.inputs.map((n) => [n.id, `${n.medium || ""} ${n.value_type || ""} ${n.unit || ""}`.trim()])));
  container.append(inspectorBlock("Outputs", component.nodes.outputs.map((n) => [n.id, `${n.medium || ""} ${n.value_type || ""} ${n.unit || ""}`.trim()])));
  container.append(inspectorBlock("Parameters", Object.entries(component.parameters || {}).map(([k, v]) => [k, String(v)])));
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

function renderParameters() {
  const tbody = el("parameterRows");
  const calibration = el("calibrationRows");
  const optimization = el("optimizationRows");
  tbody.innerHTML = "";
  calibration.innerHTML = "";
  optimization.innerHTML = "";
  const components = state.detail?.graph?.components || [];
  let count = 0;
  for (const component of components) {
    for (const [name, value] of Object.entries(component.parameters || {})) {
      count++;
      tbody.append(row([`${component.id}.${name}`, "component", unitFor(name), String(value), roleFor(name)]));
      calibration.append(row([`${component.id}.${name}`, String(value), "", "", roleFor(name) === "calibration target" ? "yes" : ""]));
      optimization.append(row([`${component.id}.${name}`, "component", "", ""]));
    }
  }
  if (!count) {
    tbody.append(emptyRow(5));
    calibration.append(emptyRow(5));
    optimization.append(emptyRow(4));
  }
}

function row(values) {
  const tr = document.createElement("tr");
  tr.innerHTML = values.map((value) => `<td>${escapeHTML(value)}</td>`).join("");
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
  if (state.latestValidation?.error) {
    panel.append(problemRow("error", state.latestValidation.error));
    return;
  }
  panel.append(problemRow("ok", "No problems"));
}

function problemRow(kind, message) {
  const row = document.createElement("div");
  row.className = "problem-row";
  row.innerHTML = `<span class="status-dot ${kind === "error" ? "error" : ""}"></span><span>${escapeHTML(message)}</span>`;
  return row;
}

function renderLogs() {
  el("logsPanel").textContent = state.logs.join("\n");
}

function renderResults() {
  el("resultsPanel").textContent = state.latestResult ? JSON.stringify(state.latestResult, null, 2) : "";
}

function renderSchema() {
  el("schemaPanel").textContent = state.latestSchema ? JSON.stringify(state.latestSchema, null, 2) : "";
}

function renderPythonPanel() {
  const component = componentById(state.selectedComponentId);
  el("pythonPanel").textContent = component ? `class ${component.class || component.id}\n\n# component contract managed by graph.json` : "";
}

function renderExportManifest() {
  el("exportManifest").textContent = JSON.stringify({
    profile: "runtime_package",
    runner: "bin/bcs-runner.exe",
    project: state.detail?.project_path,
  }, null, 2);
}

async function validateProject() {
  try {
    const body = await api("/api/validate", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath }),
    });
    state.latestValidation = body.validation;
    log("Validation ok");
  } catch (error) {
    state.latestValidation = { error: error.message };
    log(`Validation failed: ${error.message}`);
  }
  renderProblems();
}

async function runProject() {
  const inputs = {};
  for (const input of document.querySelectorAll("[data-input-id]")) {
    inputs[input.dataset.inputId] = coerceInput(input.value);
  }
  try {
    const body = await api("/api/run", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, inputs, context: { time: 0, dt: 60 } }),
    });
    state.latestResult = body.result;
    log("Run complete");
    setBottomTab("results");
  } catch (error) {
    log(`Run failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    setBottomTab("problems");
  }
  renderInspector();
  renderProblems();
  renderResults();
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

function currentSystem() {
  const detail = state.detail;
  if (!detail) return null;
  return detail.graph.systems.find((system) => system.id === detail.project.entry_system) || detail.graph.systems[0];
}

function componentById(id) {
  return (state.detail?.graph?.components || []).find((component) => component.id === id);
}

function sampleValueFor(id) {
  const samples = {
    value: 4,
    building_load_kw: 500,
    base_chw_setpoint_c: 7,
  };
  return samples[id] ?? "";
}

function coerceInput(value) {
  const trimmed = value.trim();
  if (trimmed === "") return "";
  const numeric = Number(trimmed);
  return Number.isNaN(numeric) ? trimmed : numeric;
}

function unitFor(name) {
  if (name.includes("power") || name.includes("capacity") || name.includes("load")) return "kW";
  if (name.includes("setpoint") || name.includes("_c")) return "degC";
  return "";
}

function roleFor(name) {
  if (name.includes("cop") || name.includes("factor")) return "calibration target";
  if (name.includes("setpoint")) return "scenario input";
  return "fixed";
}

function formatValue(value) {
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
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

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;",
  })[char]);
}

function escapeAttr(value) {
  return escapeHTML(value);
}

function bindEvents() {
  el("projectSelect").addEventListener("change", (event) => loadProject(event.target.value));
  el("validateButton").addEventListener("click", validateProject);
  el("runButton").addEventListener("click", runProject);
  el("schemaButton").addEventListener("click", exportSchema);
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

