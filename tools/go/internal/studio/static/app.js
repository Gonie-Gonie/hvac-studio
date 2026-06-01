const state = {
  projects: [],
  currentProjectPath: "",
  detail: null,
  selectedComponentId: "",
  latestResult: null,
  latestRunRecord: null,
  latestExport: null,
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
    const error = new Error(body.message || `Request failed: ${path}`);
    error.body = body;
    throw error;
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
    option.textContent = `${project.source === "workspace" ? "Project" : "Example"} / ${project.relative_path}`;
    select.append(option);
  }
  const feedForward = state.projects.find((p) => p.name === "003_feedforward_system");
  const first = state.projects.find((p) => p.source === "workspace") || feedForward || state.projects[0];
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
  state.latestExport = null;
  state.latestSchema = null;
  state.latestValidation = null;
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
  renderPythonPanel();
  renderExportManifest();
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
    ["Datasets", []],
    ["Parameter Sets", [treeStatic("default", "active")]],
    ["Runs", (state.detail.runs || []).map((item) => runTreeItem(item))],
    ["Scenarios", []],
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

function exportTreeItems() {
  const exports = state.detail?.exports || [];
  if (exports.length) {
    return exports.map((item) => treeStatic(item.profile, item.relative_path));
  }
  return [treeStatic("research_project", "profile"), treeStatic("runtime_package", "profile")];
}

function renderRunInputs() {
  const container = el("runInputs");
  container.innerHTML = "";
  const inputs = currentSystem()?.public_inputs || [];
  const savedInputs = state.detail?.default_run_input?.inputs || {};
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
        ${component.nodes.inputs.map((n) => `<span class="node-pill">${escapeHTML(n.id)}</span>`).join("")}
        ${component.nodes.outputs.map((n) => `<span class="node-pill output">${escapeHTML(n.id)}</span>`).join("")}
      </div>
    `;
    node.addEventListener("click", () => {
      state.selectedComponentId = component.id;
      renderCanvas();
      renderInspector();
      renderProjectTree();
      updateCommandState();
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
  const editable = isWorkspaceProject();
  let count = 0;
  for (const component of components) {
    for (const [name, value] of Object.entries(component.parameters || {})) {
      count++;
      tbody.append(parameterRow(component, name, value, editable));
      calibration.append(row([`${component.id}.${name}`, parameterInputValue(value), "", "", roleFor(name) === "calibration target" ? "yes" : ""]));
      optimization.append(row([`${component.id}.${name}`, "component", "", ""]));
    }
  }
  if (!count) {
    tbody.append(emptyRow(5));
    calibration.append(emptyRow(5));
    optimization.append(emptyRow(4));
  }
}

function parameterRow(component, name, value, editable) {
  const tr = document.createElement("tr");
  for (const cellValue of [`${component.id}.${name}`, "component", unitFor(name)]) {
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

  const roleCell = document.createElement("td");
  roleCell.textContent = roleFor(name);
  tr.append(roleCell);
  return tr;
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

function problemRow(problem) {
  const row = document.createElement("div");
  row.className = "problem-row";
  row.innerHTML = `<span class="status-dot ${problem.severity === "error" ? "error" : ""}"></span><span>${escapeHTML(problem.message)}</span>`;
  if (problem.component_id) {
    row.classList.add("linked");
    row.addEventListener("click", () => selectComponent(problem.component_id));
  }
  return row;
}

function renderLogs() {
  el("logsPanel").textContent = state.logs.join("\n");
}

function renderResults() {
  const value = state.latestRunRecord || state.latestResult;
  el("resultsPanel").textContent = value ? JSON.stringify(value, null, 2) : "";
}

function renderSchema() {
  el("schemaPanel").textContent = state.latestSchema ? JSON.stringify(state.latestSchema, null, 2) : "";
}

function renderPythonPanel() {
  const component = componentById(state.selectedComponentId);
  el("pythonPanel").textContent = component ? `class ${component.class || component.id}\n\n# component contract managed by graph.json` : "";
}

function renderExportManifest() {
  const manifest = state.latestExport || {
    profile: "runtime_package",
    runner: "bin/bcs-runner.exe",
    runtime_python: "runtime/python/python.exe",
    project: state.detail?.project_path,
    default_input: state.detail?.project?.default_input,
  };
  el("exportManifest").textContent = JSON.stringify(manifest, null, 2);
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
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    log(`Validation failed: ${error.message}`);
  }
  renderProblems();
}

async function runProject() {
  const inputs = collectRunInputs();
  try {
    const save = currentProject()?.source === "workspace";
    const body = await api("/api/run", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, inputs, context: currentRunContext(), save }),
    });
    state.latestResult = body.result;
    state.latestRunRecord = null;
    if (body.run_record) {
      state.detail.runs = [body.run_record, ...(state.detail.runs || [])];
      log(`Run saved: ${body.run_record.relative_path}`);
      renderProjectTree();
    } else {
      log("Run complete");
    }
    setBottomTab("results");
  } catch (error) {
    log(`Run failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    setBottomTab("problems");
  }
  renderInspector();
  renderProblems();
  renderResults();
}

async function loadRunRecord(runID) {
  try {
    const body = await api(`/api/project/run?project_path=${encodeURIComponent(state.currentProjectPath)}&run_id=${encodeURIComponent(runID)}`);
    state.latestRunRecord = body.run_record;
    state.latestResult = body.run_record.result;
    renderInspector();
    renderResults();
    setBottomTab("results");
    log(`Run opened: ${runID}`);
  } catch (error) {
    log(`Open run failed: ${error.message}`);
    state.latestValidation = { error: error.message };
    renderProblems();
    setBottomTab("problems");
  }
}

async function saveProjectEdits() {
  if (!isWorkspaceProject()) {
    log("Only workspace projects can be edited");
    return;
  }
  const parameters = collectParameterUpdates();
  const inputs = collectRunInputs();
  const context = currentRunContext();
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
    state.detail = body.project;
    el("saveProjectButton").classList.remove("dirty");
    renderAll();
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
  try {
    const body = await api("/api/export", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, profile: "runtime_package" }),
    });
    state.latestExport = body.export;
    state.detail.exports = [body.summary, ...(state.detail.exports || []).filter((item) => item.profile !== body.summary.profile)];
    renderProjectTree();
    renderExportManifest();
    setMode("export");
    log(`Export manifest written: ${body.summary.relative_path}`);
  } catch (error) {
    log(`Export failed: ${error.message}`);
    state.latestValidation = { error: error.message };
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
    await loadProjects();
    el("projectSelect").value = body.project.project_path;
    await loadProject(body.project.project_path);
    log(`Created ${body.project.relative_path}`);
  } catch (error) {
    log(`Create project failed: ${error.message}`);
    state.latestValidation = { error: error.message };
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
  renderProjectTree();
  updateCommandState();
}

function selectedComponentInSystem() {
  const system = currentSystem();
  return Boolean(system && state.selectedComponentId && system.components.includes(state.selectedComponentId));
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

function collectRunInputs() {
  const inputs = {};
  for (const input of document.querySelectorAll("[data-input-id]")) {
    inputs[input.dataset.inputId] = coerceInput(input.value);
  }
  return inputs;
}

function currentRunContext() {
  return state.detail?.default_run_input?.context || { time: 0, dt: 60 };
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

function coerceParameter(value) {
  const trimmed = value.trim();
  if (trimmed === "") return "";
  if (trimmed === "true") return true;
  if (trimmed === "false") return false;
  if (trimmed === "null") return null;
  if (trimmed.startsWith("{") || trimmed.startsWith("[") || trimmed.startsWith('"')) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return trimmed;
    }
  }
  const numeric = Number(trimmed);
  return Number.isNaN(numeric) ? trimmed : numeric;
}

function parameterInputValue(value) {
  if (typeof value === "object" && value !== null) return JSON.stringify(value);
  return String(value ?? "");
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

function updateCommandState() {
  const hasProject = Boolean(state.detail);
  el("validateButton").disabled = !hasProject;
  el("runButton").disabled = !hasProject;
  el("schemaButton").disabled = !hasProject;
  el("exportButton").disabled = !hasProject || !isWorkspaceProject();
  el("saveProjectButton").disabled = !hasProject || !isWorkspaceProject();
  el("addComponentButton").disabled = !hasProject || !isWorkspaceProject();
  el("includeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
}

function markProjectDirty() {
  if (isWorkspaceProject()) {
    el("saveProjectButton").classList.add("dirty");
  }
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
  el("newProjectButton").addEventListener("click", createProject);
  el("addComponentButton").addEventListener("click", createComponent);
  el("includeComponentButton").addEventListener("click", includeSelectedComponent);
  el("saveProjectButton").addEventListener("click", saveProjectEdits);
  el("validateButton").addEventListener("click", validateProject);
  el("runButton").addEventListener("click", runProject);
  el("schemaButton").addEventListener("click", exportSchema);
  el("exportButton").addEventListener("click", exportProject);
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
