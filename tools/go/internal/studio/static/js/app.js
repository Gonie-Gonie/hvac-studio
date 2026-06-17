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

const CANVAS_NODE_WIDTH = 300;
const CANVAS_NODE_HEIGHT = 220;
const CANVAS_NODE_ANCHOR_Y = 92;
const CANVAS_NODE_FIRST_PORT_Y = 84;
const CANVAS_NODE_PORT_GAP = 42;
const CANVAS_COLUMN_GAP = 370;
const CANVAS_ROW_GAP = 250;
const CANVAS_PADDING = 96;
const COMPONENT_CATEGORIES = [
  ["", "Any category"],
  ["physical_component", "Physical Component"],
  ["controller", "Controller"],
  ["data_source", "Data Source"],
  ["data_sink", "Data Sink"],
  ["utility", "Utility"],
  ["solver", "Solver"],
  ["composite_wrapper", "Composite Wrapper"],
];
const EXECUTION_MODES = [
  ["", "Any mode"],
  ["step", "Step"],
  ["vectorized", "Vectorized"],
  ["external_executable", "External Executable"],
  ["initialization_only", "Initialization Only"],
];
const WORKSPACE_HELP = {
  start: "/docs/user/quick-start.md",
  canvas: "/docs/user/build-system.md",
  code: "/docs/user/edit-python-function.md",
  parameters: "/docs/user/parameter-management.md",
  artifacts: "/docs/user/how-it-works.md",
  run: "/docs/user/run-simulation.md",
  export: "/docs/user/export-runtime.md",
};

function log(message) {
  const time = new Date().toLocaleTimeString();
  state.logs.unshift(`[${time}] ${message}`);
  renderLogs();
}

async function loadProjects(preferredProjectPath = "") {
  await loadComponentTemplates();
  const body = await api("/api/projects");
  state.projects = body.projects || [];
  preferredProjectPath = await ensureEditableProject(preferredProjectPath);
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

async function loadComponentTemplates() {
  try {
    const body = await api("/api/component-templates");
    state.componentTemplates = body.templates || [];
  } catch (error) {
    state.componentTemplates = [];
    log(`Component templates unavailable: ${error.message}`);
  }
  renderComponentTemplateSelect();
}

function renderComponentTemplateSelect() {
  renderComponentFilterSelect(el("componentCategorySelect"), COMPONENT_CATEGORIES);
  renderComponentFilterSelect(el("componentExecutionModeSelect"), EXECUTION_MODES);
  const select = el("componentTemplateSelect");
  if (!select) return;
  const previous = select.value;
  const templates = filteredComponentTemplates();
  select.innerHTML = "";
  for (const template of templates) {
    const option = document.createElement("option");
    option.value = template.id;
    const contract = `${template.input_count || 0} in / ${template.output_count || 0} out`;
    const layout = template.source_layout ? ` / ${String(template.source_layout).replace(/_/g, " ")}` : "";
    const mode = template.execution_mode ? ` / ${String(template.execution_mode).replace(/_/g, " ")}` : "";
    option.textContent = `${template.name || template.id} (${contract}${mode}${layout})`;
    select.append(option);
  }
  if (!templates.length) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = "No matching templates";
    select.append(option);
  } else if (templates.some((template) => template.id === previous)) {
    select.value = previous;
  }
  renderComponentTemplateMeta();
}

function renderComponentFilterSelect(select, options) {
  if (!select || select.options.length) return;
  for (const [value, label] of options) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    select.append(option);
  }
}

function filteredComponentTemplates() {
  const category = el("componentCategorySelect")?.value || "";
  const mode = el("componentExecutionModeSelect")?.value || "";
  return (state.componentTemplates || []).filter((template) => {
    if (category && template.category !== category) return false;
    if (mode && template.execution_mode !== mode) return false;
    return true;
  });
}

function selectedComponentTemplate() {
  const templateID = el("componentTemplateSelect")?.value || "";
  return (state.componentTemplates || []).find((template) => template.id === templateID) || null;
}

function renderComponentTemplateMeta() {
  const meta = el("componentTemplateMeta");
  if (!meta) return;
  const template = selectedComponentTemplate();
  if (!template) {
    meta.textContent = "";
    return;
  }
  meta.textContent = [
    template.category || "",
    template.execution_mode || "",
    template.source_layout ? String(template.source_layout).replace(/_/g, " ") : "",
  ].filter(Boolean).join(" / ");
}

async function ensureEditableProject(preferredProjectPath) {
  if (preferredProjectPath || state.projects.some((project) => project.source === "workspace")) {
    return preferredProjectPath;
  }
  try {
    const body = await api("/api/projects", {
      method: "POST",
      body: JSON.stringify({ name: "Starter Workspace", template: "scalar" }),
    });
    state.projects = [body.project, ...state.projects];
    log(`Created editable workspace: ${body.project.relative_path}`);
    return body.project.project_path;
  } catch (error) {
    log(`Editable workspace unavailable: ${error.message}`);
    return preferredProjectPath;
  }
}

async function loadProject(projectPath) {
  if (state.activeRunAbortController) {
    state.activeRunAbortController.abort();
  }
  state.currentProjectPath = projectPath;
  state.selectedComponentId = "";
  state.latestResult = null;
  state.latestSeriesResult = null;
  state.latestRunSource = "";
  state.runComparisonBaseline = null;
  state.latestResultStale = false;
  state.latestRunRecord = null;
  state.latestBatchRecord = null;
  state.latestExport = null;
  state.latestExportSummary = null;
  state.latestSchema = null;
  state.latestValidation = null;
  state.latestDataValidation = null;
  state.latestWorkflowRecord = null;
  state.activeParameterSetPath = "";
  state.activeRunInput = null;
  state.activeRunAbortController = null;
  state.activeRunLabel = "";
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
  setMode(workspaceModeFromHash());
  log(`Opened ${state.detail.project.project_name}`);
}

function renderAll() {
  renderStartWorkspace();
  renderProjectTree();
  renderRunInputs();
  renderCanvas();
  renderInspector();
  renderParameters();
  renderProblems();
  renderResults();
  renderSchema();
  renderArtifactWorkspace();
  renderRunWorkspace();
  renderPythonPanel();
  renderExportWorkspaceView();
  renderSystemHeader();
  updateCommandState();
}

function renderSystemHeader() {
  const project = state.detail?.project;
  el("systemTitle").textContent = project?.entry_system || "System";
  if (!project) {
    el("systemSubtitle").textContent = "";
    return;
  }
  const parts = [`${project.project_name} / ${state.detail.graph_path}`];
  if (latestRuntimeResult()) parts.push(state.latestResultStale ? "last result stale" : "last result current");
  el("systemSubtitle").textContent = parts.join(" / ");
}

function renderProjectTree() {
  const root = el("projectTree");
  root.innerHTML = "";
  if (!state.detail) return;
  const graph = state.detail.graph;
  const system = currentSystem();
  const sections = [
    ["Systems", graph.systems.map((item) => treeItem(item.id, item.name || item.id, "system"))],
    ["Components", graph.components.map((item) => componentTreeItem(item, system))],
    ["Python Source", graph.components.map((item) => sourceTreeItem(item))],
    ["Datasets", datasetTreeItems()],
    ["Validation", validationMappingTreeItems()],
    ["Validation Runs", validationRunTreeItems()],
    ["Parameter Sets", parameterSetTreeItems()],
    ["Calibration Results", calibrationResultTreeItems()],
    ["Optimization Results", optimizationResultTreeItems()],
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
      section.append(emptyTreeItem(emptyTreeMessage(title)));
    }
    root.append(section);
  }
}

function emptyTreeItem(message) {
  const row = document.createElement("div");
  row.className = "tree-item";
  row.innerHTML = `<span class="tree-meta">${escapeHTML(message)}</span>`;
  return row;
}

function emptyTreeMessage(title) {
  const lower = String(title || "items").toLowerCase();
  if (lower === "sources") return "No editable sources";
  if (lower === "export profiles") return "Ready to export";
  return `No ${lower}`;
}

function renderStartWorkspace() {
  renderStartRuntimeRows();
  renderProjectRows(el("startWorkspaceRows"), state.projects.filter((project) => project.source === "workspace"));
  renderProjectRows(el("startExampleRows"), state.projects.filter((project) => project.source === "example"));
  renderProjectTypeRows();
}

function renderStartRuntimeRows() {
  const tbody = el("startRuntimeRows");
  if (!tbody) return;
  const project = state.detail?.project;
  const summary = currentProjectSummary();
  const rows = [
    ["Runtime", el("runtimeStatus")?.textContent || "Runtime ready"],
    ["Current Project", project?.project_name || "No project open"],
    ["Project File", summary?.relative_path || state.detail?.project_path || "No project open"],
    ["Default Input", project?.default_input || "No default input"],
    ["Run Parameters", state.activeParameterSetPath || "Baseline graph parameters"],
    ["Python", project?.environment?.python || "python"],
    ["Environment", project?.environment?.mode || "project"],
  ];
  tbody.innerHTML = rows.map(([name, value]) => `
    <tr>
      <td>${escapeHTML(name)}</td>
      <td>${escapeHTML(value)}</td>
    </tr>
  `).join("");
}

function renderProjectRows(tbody, projects) {
  if (!tbody) return;
  tbody.innerHTML = "";
  if (!projects.length) {
    tbody.append(emptyRow(3, "No projects found"));
    return;
  }
  for (const project of projects.slice(0, 8)) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(project.name || project.relative_path)}</td>
      <td>${escapeHTML(project.relative_path)}</td>
      <td class="action-cell"></td>
    `;
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action table-action";
    button.textContent = "Open";
    button.addEventListener("click", () => {
      el("projectSelect").value = project.project_path;
      loadProject(project.project_path);
      setMode("canvas");
    });
    row.querySelector(".action-cell").append(button);
    tbody.append(row);
  }
}

function renderProjectTypeRows() {
  const tbody = el("startProjectTypeRows");
  if (!tbody) return;
  const types = [
    ["Python Component Project", "Ready"],
  ];
  tbody.innerHTML = types.map(([name, status]) => `
    <tr>
      <td>${escapeHTML(name)}</td>
      <td>${escapeHTML(status)}</td>
    </tr>
  `).join("");
}

function renderArtifactWorkspace() {
  const tbody = el("artifactRows");
  if (!tbody) return;
  const rows = artifactRows();
  tbody.innerHTML = "";
  if (!rows.length) {
    tbody.append(emptyRow(6, "No artifacts yet"));
    return;
  }
  for (const artifact of rows) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${escapeHTML(artifact.type)}</td>
      <td>${escapeHTML(artifact.name)}</td>
      <td class="path-cell">${escapeHTML(artifact.path || "")}</td>
      <td>${escapeHTML(artifact.state || "")}</td>
      <td><span class="policy-pill ${artifact.protected ? "protected" : ""}">${escapeHTML(artifact.policy)}</span></td>
      <td class="action-cell"></td>
    `;
    if (artifact.open) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "small-action table-action";
      button.textContent = artifact.action || "Open";
      button.addEventListener("click", artifact.open);
      tr.querySelector(".action-cell").append(button);
    }
    tbody.append(tr);
  }
}

function artifactRows() {
  if (!state.detail) return [];
  const rows = [];
  const sourcePolicy = { policy: "Source artifact", protected: false };
  const recordPolicy = { policy: "Generated record", protected: true };
  for (const item of state.detail.datasets || []) {
    rows.push({
      type: "Dataset",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.row_count || 0} rows / ${item.column_count || 0} cols`,
      ...sourcePolicy,
      open: () => openArtifactSummary("dataset", item),
    });
  }
  for (const item of state.detail.validation_mappings || []) {
    rows.push({
      type: "Validation Mapping",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.input_count || 0} in / ${item.output_count || 0} out`,
      ...sourcePolicy,
      open: () => openArtifactSummary("validation_mapping", item),
    });
  }
  for (const item of state.detail.parameter_sets || []) {
    rows.push({
      type: "Parameter Set",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.parameter_count || 0} values`,
      ...sourcePolicy,
      open: () => {
        state.activeParameterSetPath = item.relative_path || "";
        openArtifactSummary("parameter_set", item);
        renderProjectTree();
        renderRunInputs();
      },
    });
  }
  for (const item of state.detail.calibration_setups || []) {
    rows.push({
      type: "Calibration Setup",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.algorithm || "grid"} / ${item.parameter_count || 0} params`,
      ...sourcePolicy,
      action: "Run",
      open: () => runCalibrationSetup(item),
    });
  }
  for (const item of state.detail.optimization_setups || []) {
    rows.push({
      type: "Optimization Setup",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.algorithm || "grid"} / ${item.variable_count || 0} vars`,
      ...sourcePolicy,
      action: "Run",
      open: () => runOptimizationSetup(item),
    });
  }
  for (const item of state.detail.scenarios || []) {
    rows.push({
      type: "Scenario",
      name: item.name || item.id,
      path: item.relative_path,
      state: item.created_at_utc || "",
      ...sourcePolicy,
      open: () => loadScenario(item.id),
    });
  }
  for (const item of state.detail.runs || []) {
    rows.push({
      type: "Run Record",
      name: item.id,
      path: item.relative_path,
      state: item.created_at_utc || "",
      ...recordPolicy,
      open: () => loadRunRecord(item.id),
    });
  }
  for (const item of state.detail.batches || []) {
    rows.push({
      type: "Batch Record",
      name: item.id,
      path: item.relative_path,
      state: `${item.ok_count || 0}/${item.case_count || 0} ok`,
      ...recordPolicy,
      open: () => loadBatchRecord(item.id),
    });
  }
  for (const item of state.detail.validation_runs || []) {
    rows.push({
      type: "Validation Record",
      name: item.mapping_name || item.mapping_id || item.id,
      path: item.relative_path,
      state: `${item.row_count || 0} rows`,
      ...recordPolicy,
      open: () => loadWorkflowRecord("validation", item.id),
    });
  }
  for (const item of state.detail.calibration_results || []) {
    rows.push({
      type: "Calibration Record",
      name: item.setup_name || item.setup_id || item.id,
      path: item.relative_path,
      state: `best ${shortNumber(item.best_objective)}`,
      ...recordPolicy,
      open: () => loadWorkflowRecord("calibration", item.id),
    });
  }
  for (const item of state.detail.optimization_results || []) {
    rows.push({
      type: "Optimization Record",
      name: item.setup_name || item.setup_id || item.id,
      path: item.relative_path,
      state: `best ${shortNumber(item.best_objective)}`,
      ...recordPolicy,
      open: () => loadWorkflowRecord("optimization", item.id),
    });
  }
  for (const item of state.detail.exports || []) {
    rows.push({
      type: "Export Manifest",
      name: item.profile,
      path: item.relative_path,
      state: item.created_at_utc || "",
      policy: "Generated export",
      protected: true,
      open: () => loadExportRecord(item.profile),
    });
  }
  return rows;
}

async function openArtifactSummary(kind, item) {
  try {
    if (kind === "dataset" && item.relative_path) {
      const body = await api(`/api/project/dataset?project_path=${encodeURIComponent(state.currentProjectPath)}&path=${encodeURIComponent(item.relative_path)}`);
      state.latestWorkflowRecord = { kind, dataset: body.dataset };
    } else if (kind === "parameter_set" && item.relative_path) {
      const body = await api(`/api/project/parameter-set?project_path=${encodeURIComponent(state.currentProjectPath)}&path=${encodeURIComponent(item.relative_path)}`);
      state.latestWorkflowRecord = { kind, parameter_set: body.parameter_set };
    } else {
      state.latestWorkflowRecord = { kind, artifact: item };
    }
    renderResults();
    renderArtifactWorkspace();
    setBottomTab("results");
    setMode("artifacts");
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Artifact open failed: ${error.message}`);
  }
}

function datasetTreeItems() {
  return (state.detail?.datasets || []).map((item) => {
    const row = treeStatic(item.name || item.id, item.relative_path || "dataset");
    row.addEventListener("click", () => openArtifactSummary("dataset", item));
    return row;
  });
}

function parameterSetTreeItems() {
  return (state.detail?.parameter_sets || []).map((item) => parameterSetTreeItem(item));
}

function validationMappingTreeItems() {
  return (state.detail?.validation_mappings || []).map((item) => {
    const row = treeStatic(item.name || item.id, `${item.relative_path || "mapping"} / ${item.input_count || 0} in / ${item.output_count || 0} out`);
    row.addEventListener("click", () => openArtifactSummary("validation_mapping", item));
    return row;
  });
}

function validationRunTreeItems() {
  return (state.detail?.validation_runs || []).map((item) => workflowRecordTreeItem("validation", item, item.mapping_name || item.mapping_id || item.id, `${item.row_count || 0} rows`));
}

function calibrationResultTreeItems() {
  return (state.detail?.calibration_results || []).map((item) => workflowRecordTreeItem("calibration", item, item.setup_name || item.setup_id || item.id, `best ${shortNumber(item.best_objective)}`));
}

function optimizationResultTreeItems() {
  return (state.detail?.optimization_results || []).map((item) => workflowRecordTreeItem("optimization", item, item.setup_name || item.setup_id || item.id, `best ${shortNumber(item.best_objective)}`));
}

function workflowRecordTreeItem(kind, item, label, meta) {
  const row = treeStatic(label, meta || item.relative_path || kind);
  row.addEventListener("click", () => loadWorkflowRecord(kind, item.id));
  return row;
}

function parameterSetTreeItem(item) {
  const row = treeStatic(item.name || item.id, item.relative_path || "parameter set");
  if (state.activeParameterSetPath === item.relative_path) row.classList.add("active");
  row.addEventListener("click", () => {
    state.activeParameterSetPath = item.relative_path || "";
    renderProjectTree();
    renderRunInputs();
    renderStartRuntimeRows();
    log(`Parameter set selected: ${state.activeParameterSetPath || "baseline"}`);
  });
  return row;
}

function shortNumber(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return "";
  return Math.round(numeric * 1000) / 1000;
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
      renderRunWorkspace();
      updateCommandState();
    }
  });
  return row;
}

function componentTreeItem(component, system) {
  const inSystem = Boolean(system?.components?.includes(component.id));
  const row = treeItem(component.id, component.name || component.id, inSystem ? component.kind : "unused");
  if (isWorkspaceProject()) {
    if (!inSystem) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "tree-action";
      button.textContent = "Use";
      button.addEventListener("click", (event) => {
        event.stopPropagation();
        includeComponentInSystem(component.id);
      });
      row.append(button);
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "tree-action";
    button.textContent = "Copy";
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      duplicateComponent(component.id);
    });
    row.append(button);
  }
  return row;
}

function sourceTreeItem(component) {
  const row = treeItem(component.id, component.class || component.id, sourceTreeMeta(component));
  const button = document.createElement("button");
  button.type = "button";
  button.className = "tree-action";
  button.textContent = "Open";
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    openComponentCode(component.id);
  });
  row.addEventListener("dblclick", () => openComponentCode(component.id));
  row.append(button);
  return row;
}

function sourceTreeMeta(component) {
  const source = state.sourceByComponent[component.id];
  if (source?.read_only || !isWorkspaceProject()) return "read only";
  if (source && sourceDraft(component.id) !== source.content) return "dirty";
  const check = state.sourceCheckByComponent[component.id];
  const problems = check?.problems || [];
  const issueCount = problems.filter((problem) => problem.severity !== "ok").length;
  if (issueCount) return `${issueCount} issue${issueCount === 1 ? "" : "s"}`;
  if (check?.ok) return "ok";
  return source ? "loaded" : "source";
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
  const meta = batch.parameter_set ? `${batch.ok_count}/${batch.case_count} ok / ${batch.parameter_set}` : `${batch.ok_count}/${batch.case_count} ok`;
  const row = treeStatic(batch.id, meta);
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
    return exports.map((item) => exportTreeItem(item));
  }
  return [exportReadyTreeItem()];
}

function exportReadyTreeItem() {
  const row = treeStatic("runtime_package", "ready");
  row.addEventListener("click", () => {
    state.latestExport = null;
    state.latestExportSummary = null;
    renderExportWorkspaceView();
    setMode("export");
  });
  return row;
}

function exportTreeItem(exportSummary) {
  const row = treeStatic(exportSummary.profile, exportSummary.relative_path);
  row.addEventListener("click", () => loadExportRecord(exportSummary.profile));
  return row;
}

function renderRunInputs() {
  const container = el("runInputs");
  container.innerHTML = "";
  const inputs = currentSystem()?.public_inputs || [];
  const savedInputs = state.activeRunInput?.inputs || state.detail?.default_run_input?.inputs || {};
  container.append(parameterSetField());
  container.append(runTimeoutField());
  for (const input of inputs) {
    const field = document.createElement("div");
    field.className = "input-field";
    const defaultValue = savedInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
    const label = input.name || input.id;
    const meta = runInputMeta(input, label);
    field.innerHTML = `
      <label for="input-${escapeAttr(input.id)}">
        <span class="input-label">${escapeHTML(label)}</span>
        ${meta ? `<span class="input-meta">${escapeHTML(meta)}</span>` : ""}
      </label>
      <input id="input-${escapeAttr(input.id)}" data-input-id="${escapeAttr(input.id)}" value="${escapeAttr(defaultValue)}" />
    `;
    field.querySelector("input").addEventListener("input", markRunInputsEdited);
    const reset = document.createElement("button");
    reset.type = "button";
    reset.className = "input-reset";
    reset.textContent = "Default";
    reset.addEventListener("click", () => resetRunInput(input));
    field.append(reset);
    container.append(field);
  }
  if (isWorkspaceProject()) {
    const activeScenario = activeScenarioBadge();
    if (activeScenario) container.append(activeScenario);
    container.append(scenarioNameField());
  }
}

function runInputMeta(input, label) {
  return [
    input.id && input.id !== label ? input.id : "",
    input.value_type || "",
    input.unit || "",
    input.required === false ? "optional" : "required",
  ].filter(Boolean).join(" / ");
}

function resetRunInput(input) {
  const control = [...document.querySelectorAll("[data-input-id]")].find((item) => item.dataset.inputId === input.id);
  if (!control) return;
  const defaultInputs = state.detail?.default_run_input?.inputs || {};
  const value = defaultInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
  control.value = parameterInputValue(value);
  markRunInputsEdited();
}

function markRunInputsEdited() {
  if (state.activeRunInput) {
    state.activeRunInput = null;
    document.querySelector(".active-scenario")?.remove();
  }
  markProjectDirty();
}

function scenarioNameField() {
  const field = document.createElement("div");
  field.className = "scenario-name-field";
  const input = document.createElement("input");
  input.id = "scenarioNameInput";
  input.placeholder = "Scenario name";
  input.value = state.scenarioDraftName;
  input.setAttribute("aria-label", "Scenario name");
  input.addEventListener("input", () => {
    state.scenarioDraftName = input.value;
  });
  input.addEventListener("keydown", (event) => {
    if (event.key === "Enter") createScenario();
  });
  field.append(input);
  return field;
}

function activeScenarioBadge() {
  if (!state.activeRunInput) return null;
  const field = document.createElement("div");
  field.className = "active-scenario";
  const name = state.activeRunInput.name || state.activeRunInput.id || "scenario";
  field.innerHTML = `<span>${escapeHTML(`Scenario: ${name}`)}</span>`;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "input-reset";
  button.textContent = "Clear";
  button.addEventListener("click", () => {
    state.activeRunInput = null;
    markRunResultStale();
    renderRunInputs();
    renderSystemHeader();
  });
  field.append(button);
  return field;
}

function defaultScenarioName() {
  const stamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, "");
  return `Scenario ${stamp}`;
}

function defaultProjectName(prefix = "Project") {
  const stamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, "");
  return `${prefix} ${stamp}`;
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
    const { x, y } = canvasPositionFor(component.id, index);
    positions[component.id] = { x, y };

    const node = document.createElement("button");
    node.type = "button";
    node.className = `component-node ${state.selectedComponentId === component.id ? "selected" : ""}`;
    node.style.left = `${x}px`;
    node.style.top = `${y}px`;
    node.dataset.componentId = component.id;
    node.innerHTML = `
      <div class="component-head">
        <span class="component-title">${escapeHTML(component.name || component.id)}</span>
        <span class="component-kind">${escapeHTML(component.kind)}</span>
      </div>
      <div class="node-list">
        <div class="node-column">
          <span class="node-column-title">Inputs</span>
          ${component.nodes.inputs.map((n) => canvasNodePill(component.id, n, "input")).join("")}
        </div>
        <div class="node-column">
          <span class="node-column-title">Outputs</span>
          ${component.nodes.outputs.map((n) => canvasNodePill(component.id, n, "output")).join("")}
        </div>
      </div>
      ${canvasParameterSummary(component)}
    `;
    node.addEventListener("click", () => {
      state.selectedComponentId = component.id;
      state.selectedConnectionId = "";
      renderCanvas();
      renderInspector();
      renderPythonPanel();
      renderProjectTree();
      renderRunWorkspace();
      updateCommandState();
    });
    node.querySelectorAll("[data-node-endpoint]").forEach((endpoint) => {
      endpoint.addEventListener("click", (event) => {
        event.stopPropagation();
        handleCanvasEndpointClick(endpoint.dataset.componentId, endpoint.dataset.nodeId, endpoint.dataset.direction);
      });
    });
    node.querySelector(".component-head")?.addEventListener("pointerdown", (event) => {
      startCanvasNodeDrag(event, node, component.id, positions);
    });
    canvas.append(node);
  });

  resizeCanvasSurface(canvas, layer, positions);
  requestAnimationFrame(() => drawConnections(positions));
}

function canvasPositionFor(componentID, index) {
  const saved = state.detail?.layout?.components?.[componentID];
  if (saved && Number.isFinite(saved.x) && Number.isFinite(saved.y)) {
    return { x: saved.x, y: saved.y };
  }
  return {
    x: 48 + index * CANVAS_COLUMN_GAP,
    y: 78 + (index % 2) * 62,
  };
}

function resizeCanvasSurface(canvas, layer, positions) {
  const values = Object.values(positions);
  const maxX = Math.max(0, ...values.map((position) => position.x));
  const maxY = Math.max(0, ...values.map((position) => position.y));
  const width = Math.max(1240, maxX + CANVAS_NODE_WIDTH + CANVAS_PADDING);
  const height = Math.max(430, maxY + CANVAS_NODE_HEIGHT + CANVAS_PADDING);
  canvas.style.minWidth = `${width}px`;
  canvas.style.minHeight = `${height}px`;
  layer.style.width = `${width}px`;
  layer.style.height = `${height}px`;
  layer.setAttribute("width", String(width));
  layer.setAttribute("height", String(height));
}

function startCanvasNodeDrag(event, node, componentID, positions) {
  if (!isWorkspaceProject() || event.button !== 0) return;
  event.preventDefault();
  state.selectedComponentId = componentID;
  state.selectedConnectionId = "";
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  renderRunWorkspace();
  updateCommandState();
  const startX = event.clientX;
  const startY = event.clientY;
  const startLeft = Number.parseFloat(node.style.left) || 0;
  const startTop = Number.parseFloat(node.style.top) || 0;
  let last = { x: startLeft, y: startTop };
  node.classList.add("dragging");
  node.setPointerCapture?.(event.pointerId);

  const onMove = (moveEvent) => {
    last = {
      x: Math.max(16, startLeft + moveEvent.clientX - startX),
      y: Math.max(16, startTop + moveEvent.clientY - startY),
    };
    node.style.left = `${last.x}px`;
    node.style.top = `${last.y}px`;
    positions[componentID] = last;
    resizeCanvasSurface(el("systemCanvas"), el("connectionLayer"), positions);
    drawConnections(positions);
  };

  const onUp = () => {
    node.classList.remove("dragging");
    node.removeEventListener("pointermove", onMove);
    node.removeEventListener("pointerup", onUp);
    node.removeEventListener("pointercancel", onUp);
    saveCanvasLayout(componentID, last.x, last.y);
  };

  node.addEventListener("pointermove", onMove);
  node.addEventListener("pointerup", onUp);
  node.addEventListener("pointercancel", onUp);
}

async function saveCanvasLayout(componentID, x, y) {
  if (!isWorkspaceProject()) return;
  const components = { ...(state.detail?.layout?.components || {}) };
  components[componentID] = { x: Math.round(x), y: Math.round(y) };
  await saveCanvasLayoutPositions(components, componentID);
}

async function saveCanvasLayoutPositions(components, label) {
  if (!isWorkspaceProject()) return;
  state.detail.layout = { components };
  try {
    const body = await api("/api/project/layout", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, components }),
    });
    state.detail = body.project;
    renderCanvas();
    log(`Canvas layout saved: ${label}`);
  } catch (error) {
    log(`Canvas layout save failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function autoLayoutCanvas() {
  if (!isWorkspaceProject()) return;
  const system = currentSystem();
  if (!system) return;
  const positions = autoLayoutPositions(system);
  const components = { ...(state.detail?.layout?.components || {}), ...positions };
  await saveCanvasLayoutPositions(components, "auto layout");
}

function autoLayoutPositions(system) {
  const ids = (system.components || []).filter((id) => componentById(id));
  const idSet = new Set(ids);
  const order = new Map(ids.map((id, index) => [id, index]));
  const levels = Object.fromEntries(ids.map((id) => [id, 0]));
  const connections = (state.detail?.graph?.connections || []).filter((connection) => (
    idSet.has(connection.from.component) && idSet.has(connection.to.component)
  ));

  for (let pass = 0; pass < ids.length; pass += 1) {
    let changed = false;
    for (const connection of connections) {
      const nextLevel = (levels[connection.from.component] || 0) + 1;
      if (nextLevel > (levels[connection.to.component] || 0)) {
        levels[connection.to.component] = nextLevel;
        changed = true;
      }
    }
    if (!changed) break;
  }

  const groups = new Map();
  for (const id of ids) {
    const level = levels[id] || 0;
    if (!groups.has(level)) groups.set(level, []);
    groups.get(level).push(id);
  }

  const positions = {};
  for (const [level, group] of [...groups.entries()].sort(([a], [b]) => a - b)) {
    group.sort((a, b) => (order.get(a) || 0) - (order.get(b) || 0));
    group.forEach((id, row) => {
      positions[id] = { x: 48 + level * CANVAS_COLUMN_GAP, y: 64 + row * CANVAS_ROW_GAP };
    });
  }
  return positions;
}

function canvasNodePill(componentID, node, direction) {
  const pending = state.pendingConnection;
  const latest = latestCanvasNodeValue(componentID, node.id, direction);
  const stale = latest.hasValue && state.latestResultStale;
  const selected = direction === "output" && pending?.component === componentID && pending?.node === node.id;
  const targetable = direction === "input" && pending && pending.component !== componentID;
  const classes = [
    "node-pill",
    direction === "output" ? "output" : "",
    latest.hasValue ? "has-value" : "",
    stale ? "stale" : "",
    selected ? "pending-source" : "",
    targetable ? "targetable" : "",
  ].filter(Boolean).join(" ");
  const formattedValue = latest.hasValue ? formatValue(latest.value) : "";
  const valueMarkup = latest.hasValue ? `<span class="node-value">${escapeHTML(formattedValue)}</span>` : "";
  const displayName = node.name || node.id;
  const meta = canvasNodeMeta(node, displayName);
  const metaMarkup = meta ? `<span class="node-meta">${escapeHTML(meta)}</span>` : "";
  const mediumMarkup = node.medium ? `<span class="node-medium">${escapeHTML(node.medium)}</span>` : "";
  const titleParts = [
    displayName,
    node.medium ? `medium: ${node.medium}` : "",
    meta,
    latest.hasValue ? `${state.latestResultStale ? "stale " : ""}value: ${formattedValue}` : "",
  ].filter(Boolean);
  return `<span class="${classes}" data-node-endpoint="true" data-component-id="${escapeAttr(componentID)}" data-node-id="${escapeAttr(node.id)}" data-direction="${escapeAttr(direction)}" title="${escapeAttr(titleParts.join(" / "))}"><span class="node-label">${escapeHTML(displayName)}</span>${mediumMarkup}${metaMarkup}${valueMarkup}</span>`;
}

function canvasNodeMeta(node, displayName) {
  return [
    node.id && node.id !== displayName ? node.id : "",
    node.value_type || "",
    node.unit || "",
  ].filter(Boolean).join(" / ");
}

function canvasParameterSummary(component) {
  const entries = Object.entries(component.parameters || {});
  if (!entries.length) return "";
  const visible = entries.slice(0, 4);
  const extra = entries.length - visible.length;
  const pills = visible.map(([name, value]) => {
    const formatted = parameterInputValue(value);
    return `
      <span class="canvas-param" title="${escapeAttr(`${name}: ${formatted}`)}">
        <span class="canvas-param-key">${escapeHTML(name)}</span>
        <span class="canvas-param-value">${escapeHTML(formatted)}</span>
      </span>
    `;
  }).join("");
  const extraPill = extra > 0 ? `<span class="canvas-param extra">+${extra}</span>` : "";
  return `<div class="canvas-params"><span class="canvas-param-title">Params</span>${pills}${extraPill}</div>`;
}

function latestCanvasNodeValue(componentID, nodeID, direction) {
  const result = latestRuntimeResult();
  const componentValues = direction === "output"
    ? result?.component_outputs?.[componentID]
    : result?.component_inputs?.[componentID];
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
  defs.innerHTML = `
    <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#617d98"></path></marker>
    <marker id="arrow-selected" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#1864ab"></path></marker>
    <marker id="arrow-warning" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#b7791f"></path></marker>
    <marker id="arrow-danger" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#b42318"></path></marker>
  `;
  layer.append(defs);
  const fanOffsets = canvasConnectionFanOffsets(system, graph);

  system.connections.forEach((connectionId, index) => {
    const connection = graph.connections.find((item) => item.id === connectionId);
    if (!connection) return;
    const from = positions[connection.from.component];
    const to = positions[connection.to.component];
    if (!from || !to) return;
    const fromComponent = componentById(connection.from.component);
    const toComponent = componentById(connection.to.component);
    const x1 = from.x + CANVAS_NODE_WIDTH;
    const y1 = from.y + canvasNodeAnchorY(fromComponent, connection.from.node, "output");
    const x2 = to.x;
    const y2 = to.y + canvasNodeAnchorY(toComponent, connection.to.node, "input");
    const mediumState = connectionMediumState(connection);
    const fanOffset = fanOffsets.get(connection.id) || 0;
    const route = canvasConnectionRoute(x1, y1, x2, y2, fanOffset, index);
    const annotation = connectionAnnotation(connection, mediumState, route);
    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("class", connectionClassList(connection, mediumState, route).join(" "));
    path.dataset.connectionId = connection.id;
    path.setAttribute("marker-end", `url(#${connectionMarkerID(connection, mediumState)})`);
    path.setAttribute("d", route.path);
    path.append(svgTitle(annotation.title));
    path.addEventListener("click", (event) => {
      event.stopPropagation();
      selectConnection(connection.id);
    });
    layer.append(path);
    drawConnectionLabel(layer, connection, annotation, route, mediumState);
  });
}

function canvasNodeAnchorY(component, nodeID, direction) {
  const nodes = direction === "output" ? component?.nodes?.outputs || [] : component?.nodes?.inputs || [];
  const index = nodes.findIndex((node) => node.id === nodeID);
  if (index < 0) return CANVAS_NODE_ANCHOR_Y;
  return CANVAS_NODE_FIRST_PORT_Y + index * CANVAS_NODE_PORT_GAP;
}

function canvasConnectionFanOffsets(system, graph) {
  const groups = new Map();
  for (const connectionId of system.connections || []) {
    const connection = graph.connections.find((item) => item.id === connectionId);
    if (!connection) continue;
    const key = `${connection.from.component}->${connection.to.component}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(connection.id);
  }
  const offsets = new Map();
  for (const ids of groups.values()) {
    const center = (ids.length - 1) / 2;
    ids.forEach((id, index) => offsets.set(id, (index - center) * 18));
  }
  return offsets;
}

function canvasConnectionRoute(x1, y1, x2, y2, fanOffset, index) {
  const backtracking = x2 <= x1 + 24;
  const longPath = x2 - x1 > CANVAS_COLUMN_GAP * 1.4;
  if (backtracking) {
    const lift = 76 + Math.abs(fanOffset) + (index % 4) * 18;
    const control = Math.max(90, Math.abs(x2 - x1) * 0.45);
    return {
      path: `M ${x1} ${y1} C ${x1 + control} ${y1 - lift}, ${x2 - control} ${y2 - lift}, ${x2} ${y2}`,
      labelX: (x1 + x2) / 2,
      labelY: Math.max(24, Math.min(y1, y2) - lift + 12),
      backtracking,
      longPath,
      fanOffset,
    };
  }
  const mid = Math.max(60, (x2 - x1) / 2);
  return {
    path: `M ${x1} ${y1} C ${x1 + mid} ${y1 + fanOffset}, ${x2 - mid} ${y2 + fanOffset}, ${x2} ${y2}`,
    labelX: (x1 + x2) / 2,
    labelY: Math.max(18, (y1 + y2) / 2 + fanOffset - 14),
    backtracking,
    longPath,
    fanOffset,
  };
}

function connectionClassList(connection, mediumState, route) {
  return [
    "connection-line",
    state.selectedConnectionId === connection.id ? "selected" : "",
    mediumState.status === "warning" ? "medium-warning" : "",
    mediumState.status === "override" ? "medium-override" : "",
    mediumState.status === "error" ? "medium-mismatch" : "",
    route.backtracking ? "backtracking" : "",
    route.longPath ? "long-path" : "",
    route.fanOffset ? "connection-fan" : "",
  ].filter(Boolean);
}

function connectionMarkerID(connection, mediumState) {
  if (state.selectedConnectionId === connection.id) return "arrow-selected";
  if (mediumState.status === "error") return "arrow-danger";
  if (mediumState.status === "warning" || mediumState.status === "override") return "arrow-warning";
  return "arrow";
}

function connectionMediumState(connection) {
  const sourceNode = canvasEndpointNode(connection.from, "output");
  const targetNode = canvasEndpointNode(connection.to, "input");
  const sourceMedium = sourceNode?.medium || "";
  const targetMedium = targetNode?.medium || "";
  const normalizedSource = normalizedCanvasMedium(sourceMedium);
  const normalizedTarget = normalizedCanvasMedium(targetMedium);
  let status = "ok";
  if (!canvasMediumCompatible(sourceMedium, targetMedium)) {
    if (connection.allow_medium_mismatch) {
      status = "override";
    } else if (normalizedSource === "signal" && normalizedTarget && normalizedTarget !== "signal") {
      status = "warning";
    } else {
      status = "error";
    }
  }
  const label = sourceMedium && targetMedium && normalizedSource !== normalizedTarget
    ? `${sourceMedium}->${targetMedium}`
    : sourceMedium || targetMedium || "";
  return { sourceNode, targetNode, sourceMedium, targetMedium, label, status };
}

function canvasEndpointNode(endpoint, direction) {
  const component = componentById(endpoint.component);
  const nodes = direction === "output" ? component?.nodes?.outputs || [] : component?.nodes?.inputs || [];
  return nodes.find((node) => node.id === endpoint.node) || null;
}

function canvasMediumCompatible(source, target) {
  const normalizedSource = normalizedCanvasMedium(source);
  const normalizedTarget = normalizedCanvasMedium(target);
  if (!normalizedSource || !normalizedTarget) return true;
  if (normalizedSource === "generic" || normalizedTarget === "generic") return true;
  return normalizedSource === normalizedTarget;
}

function normalizedCanvasMedium(value) {
  return String(value || "").trim().toLowerCase();
}

function connectionAnnotation(connection, mediumState, route) {
  const latest = latestConnectionValue(connection);
  const sourceName = mediumState.sourceNode?.name || connection.from.node;
  const targetName = mediumState.targetNode?.name || connection.to.node;
  const status = connectionStatusLabel(connection, mediumState, route);
  const latestValue = latest.hasValue ? formatValue(latest.value) : "";
  const secondary = [
    mediumState.label,
    latestValue ? `value ${latestValue}` : "",
    status,
  ].filter(Boolean).join(" / ");
  const title = [
    connection.id,
    `${connection.from.component}.${connection.from.node} -> ${connection.to.component}.${connection.to.node}`,
    mediumState.label ? `medium ${mediumState.label}` : "",
    latestValue ? `${state.latestResultStale ? "stale " : ""}value ${latestValue}` : "",
    status,
    connection.medium_override_reason || "",
  ].filter(Boolean).join(" / ");
  return {
    primary: shortCanvasText(`${sourceName} -> ${targetName}`, 32),
    secondary: shortCanvasText(secondary, 42),
    title,
  };
}

function connectionStatusLabel(connection, mediumState, route) {
  if (mediumState.status === "error") return "medium mismatch";
  if (mediumState.status === "override") return connection.medium_override_reason ? "override" : "medium override";
  if (mediumState.status === "warning") return "signal warning";
  if (route.backtracking) return "backtracking";
  if (route.longPath) return "long path";
  return "";
}

function shortCanvasText(value, maxLength) {
  const text = String(value || "");
  if (text.length <= maxLength) return text;
  return `${text.slice(0, Math.max(0, maxLength - 3))}...`;
}

function drawConnectionLabel(layer, connection, annotation, route, mediumState) {
  const group = document.createElementNS("http://www.w3.org/2000/svg", "g");
  group.setAttribute("class", connectionLabelClassList(connection, mediumState, route).join(" "));
  group.dataset.connectionId = connection.id;
  const lines = [annotation.primary, annotation.secondary].filter(Boolean);
  const maxLength = Math.max(12, ...lines.map((line) => line.length));
  const width = Math.min(230, Math.max(92, maxLength * 6.4 + 18));
  const height = lines.length > 1 ? 36 : 24;
  const x = Math.max(width / 2 + 8, route.labelX);
  const y = Math.max(height / 2 + 8, route.labelY);
  const rect = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  rect.setAttribute("class", "connection-label-bg");
  rect.setAttribute("x", String(x - width / 2));
  rect.setAttribute("y", String(y - height / 2));
  rect.setAttribute("width", String(width));
  rect.setAttribute("height", String(height));
  rect.setAttribute("rx", "5");
  const primary = document.createElementNS("http://www.w3.org/2000/svg", "text");
  primary.setAttribute("class", "connection-label-text");
  primary.setAttribute("x", String(x));
  primary.setAttribute("y", String(lines.length > 1 ? y - 3 : y + 4));
  primary.setAttribute("text-anchor", "middle");
  primary.textContent = annotation.primary;
  group.append(svgTitle(annotation.title), rect, primary);
  if (annotation.secondary) {
    const secondary = document.createElementNS("http://www.w3.org/2000/svg", "text");
    secondary.setAttribute("class", "connection-label-meta");
    secondary.setAttribute("x", String(x));
    secondary.setAttribute("y", String(y + 11));
    secondary.setAttribute("text-anchor", "middle");
    secondary.textContent = annotation.secondary;
    group.append(secondary);
  }
  group.addEventListener("click", (event) => {
    event.stopPropagation();
    selectConnection(connection.id);
  });
  layer.append(group);
}

function connectionLabelClassList(connection, mediumState, route) {
  return [
    "connection-label",
    state.selectedConnectionId === connection.id ? "selected" : "",
    mediumState.status === "warning" ? "medium-warning" : "",
    mediumState.status === "override" ? "medium-override" : "",
    mediumState.status === "error" ? "medium-mismatch" : "",
    route.backtracking ? "backtracking" : "",
    route.longPath ? "long-path" : "",
  ].filter(Boolean);
}

function svgTitle(text) {
  const title = document.createElementNS("http://www.w3.org/2000/svg", "title");
  title.textContent = text;
  return title;
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
  renderRunWorkspace();
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
    ["Mode", component.execution_mode || "step"],
    ["Source", component.source?.layout || "single_file_class"],
    ["Class", component.class || ""],
  ]));
  if (component.ml_metadata) container.append(mlMetadataBlock(component));
  if (isWorkspaceProject()) container.append(componentEditor(component));
  container.append(nodeListBlock("Inputs", component, component.nodes.inputs || [], "input"));
  container.append(nodeListBlock("Outputs", component, component.nodes.outputs || [], "output"));
  if (isWorkspaceProject()) container.append(nodeEditor(component));
  container.append(parameterInspectorBlock(component));
  if (isWorkspaceProject()) {
    container.append(parameterDefinitionBlock(component));
    container.append(stateDefinitionBlock(component));
  }
  container.append(connectionEditor(component));
  const result = latestRuntimeResult();
  const latestInputs = result?.component_inputs?.[component.id];
  const latestOutputs = result?.component_outputs?.[component.id];
  if (latestInputs) {
    container.append(inspectorBlock(runValueTitle("Last Inputs"), Object.entries(latestInputs).map(([k, v]) => [k, formatValue(v)])));
  }
  if (latestOutputs) {
    container.append(inspectorBlock(runValueTitle("Last Outputs"), Object.entries(latestOutputs).map(([k, v]) => [k, formatValue(v)])));
  }
}

function runValueTitle(title) {
  return state.latestResultStale ? `${title} (stale)` : title;
}

function inspectorBlock(title, rows) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  if (!rows.length) {
    block.append(emptyKVRow("No values"));
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

function nodeListBlock(title, component, nodes, direction) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  if (!nodes.length) {
    block.append(emptyKVRow(`No ${String(title || "nodes").toLowerCase()}`));
    return block;
  }
  for (const node of nodes) {
    if (isWorkspaceProject()) {
      block.append(editableNodeRow(component, node, direction));
      continue;
    }
    const row = document.createElement("div");
    row.className = "kv connection-row";
    row.innerHTML = `
      <span class="kv-key">${escapeHTML(node.id)}</span>
      <span class="connection-value">
        <span>${escapeHTML(`${node.medium || ""} ${node.value_type || ""} ${node.unit || ""}`.trim())}</span>
      </span>
    `;
    block.append(row);
  }
  return block;
}

function editableNodeRow(component, node, direction) {
  const row = document.createElement("div");
  row.className = "kv node-edit-row";
  row.dataset.nodeComponent = component.id;
  row.dataset.nodeId = node.id;

  const key = document.createElement("span");
  key.className = "kv-key node-id-label";
  key.textContent = node.id;

  const controls = document.createElement("span");
  controls.className = "node-meta-controls";

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

  controls.append(name, medium, valueType, unit);

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
  saveButton.addEventListener("click", () => updateNodeFromInspector(component.id, node.id, direction, row));

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "small-action";
  deleteButton.textContent = "Delete";
  deleteButton.addEventListener("click", () => deleteNodeFromInspector(component.id, node.id));

  for (const input of controls.querySelectorAll("input, select")) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") updateNodeFromInspector(component.id, node.id, direction, row);
    });
  }
  controls.append(saveButton, deleteButton);
  row.append(key, controls);
  return row;
}

function parameterInspectorBlock(component) {
  const editable = isWorkspaceProject();
  if (!editable) {
    return inspectorBlock("Parameters", Object.entries(component.parameters || {}).map(([k, v]) => [k, parameterInputValue(v)]));
  }
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Parameters</div>`;

  const entries = Object.entries(component.parameters || {});
  if (!entries.length) {
    block.append(emptyKVRow("No parameters"));
  }
  for (const [name, value] of entries) {
    const row = document.createElement("div");
    row.className = "kv connection-row";
    row.innerHTML = `
      <span class="kv-key">${escapeHTML(name)}</span>
      <span class="connection-value">
        <input class="inspector-input" value="${escapeAttr(parameterInputValue(value))}" data-parameter-component="${escapeAttr(component.id)}" data-parameter-name="${escapeAttr(name)}" aria-label="${escapeAttr(`${component.id}.${name}`)}" />
      </span>
    `;
    const input = row.querySelector("input");
    input.addEventListener("input", () => {
      syncParameterInputs(component.id, name, input.value, input);
      markProjectDirty();
    });
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action";
    button.textContent = "Delete";
    button.addEventListener("click", () => deleteParameterFromManager(component.id, name));
    row.querySelector(".connection-value").append(button);
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
  button.addEventListener("click", () => addParameter(component.id, nameInput.value, valueInput.value));
  for (const input of [nameInput, valueInput]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addParameter(component.id, nameInput.value, valueInput.value);
    });
  }
  form.append(nameInput, valueInput, button);
  block.append(form);
  return block;
}

function parameterDefinitionBlock(component) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Parameter Definitions</div>`;
  const definitions = component.parameter_defs || {};
  const names = [...new Set([...Object.keys(component.parameters || {}), ...Object.keys(definitions)])].sort();
  if (!names.length) {
    block.append(emptyKVRow("No parameter definitions"));
  }
  for (const name of names) {
    block.append(parameterDefinitionRow(component, name, definitions[name] || {}));
  }
  return block;
}

function parameterDefinitionRow(component, name, definition) {
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
  for (const value of ["fixed", "scenario_input", "calibration_target", "optimization_variable", "derived"]) {
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
  saveButton.addEventListener("click", () => saveParameterDefinition(component.id, name, row));

  const clearButton = document.createElement("button");
  clearButton.type = "button";
  clearButton.className = "small-action";
  clearButton.textContent = "Clear Meta";
  clearButton.addEventListener("click", () => deleteParameterDefinition(component.id, name));

  for (const input of [displayName, current, defaultValue, unit, group, description, min, max, role, visible]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") saveParameterDefinition(component.id, name, row);
    });
  }
  controls.append(displayName, current, defaultValue, unit, role, min, max, group, description, visibleLabel, saveButton, clearButton);
  row.append(key, controls);
  return row;
}

function stateDefinitionBlock(component) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">State Definitions</div>`;
  const entries = Object.entries(component.state_defs || {}).sort(([left], [right]) => left.localeCompare(right));
  if (!entries.length) {
    block.append(emptyKVRow("No state definitions"));
  }
  for (const [name, definition] of entries) {
    block.append(stateDefinitionRow(component, name, definition || {}));
  }

  const form = document.createElement("div");
  form.className = "connection-form state-form";
  const name = document.createElement("input");
  name.placeholder = "state name";
  name.setAttribute("aria-label", "State name");
  const initial = document.createElement("input");
  initial.placeholder = "initial";
  initial.setAttribute("aria-label", "State initial value");
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add State";
  button.addEventListener("click", () => addStateDefinition(component.id, name.value, initial.value));
  for (const input of [name, initial]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addStateDefinition(component.id, name.value, initial.value);
    });
  }
  form.append(name, initial, button);
  block.append(form);
  return block;
}

function stateDefinitionRow(component, name, definition) {
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
  saveButton.addEventListener("click", () => saveStateDefinition(component.id, name, row));

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "small-action";
  deleteButton.textContent = "Delete";
  deleteButton.addEventListener("click", () => deleteStateDefinition(component.id, name));

  for (const input of [displayName, initial, unit, description]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") saveStateDefinition(component.id, name, row);
    });
  }
  controls.append(displayName, initial, unit, description, saveButton, deleteButton);
  row.append(key, controls);
  return row;
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
  duplicateButton.addEventListener("click", () => duplicateComponent(component.id));
  const codeButton = document.createElement("button");
  codeButton.type = "button";
  codeButton.textContent = "Code";
  codeButton.addEventListener("click", () => openComponentCode(component.id));
  name.addEventListener("keydown", (event) => {
    if (event.key === "Enter") updateComponentFromInspector(component.id);
  });

  form.append(name, button, duplicateButton, codeButton);
  block.append(form);
  return block;
}

function openComponentCode(componentID) {
  if (!componentById(componentID)) return;
  state.selectedComponentId = componentID;
  state.selectedConnectionId = "";
  setMode("code");
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  updateCommandState();
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

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Add Node";
  button.addEventListener("click", () => addNodeFromInspector(component.id));

  const syncInputOnlyFields = () => {
    const isInput = direction.value === "input";
    defaultValue.disabled = !isInput;
    required.disabled = !isInput;
  };
  direction.addEventListener("change", syncInputOnlyFields);
  syncInputOnlyFields();

  for (const input of [nodeID, nodeName, medium, unit, defaultValue]) {
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") addNodeFromInspector(component.id);
    });
  }
  form.append(direction, nodeID, nodeName, valueType, medium, unit, defaultValue, requiredLabel, button);
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
      const latest = latestConnectionValue(connectionRow.connection);
      const flowValue = latest.hasValue
        ? `<span class="connection-flow ${state.latestResultStale ? "stale" : ""}">${escapeHTML(formatValue(latest.value))}</span>`
        : "";
      const rowEl = document.createElement("div");
      rowEl.className = `kv connection-row ${connectionRow.id === state.selectedConnectionId ? "selected" : ""}`;
      rowEl.innerHTML = `
        <span class="kv-key">${escapeHTML(connectionRow.key)}</span>
        <span class="connection-value">
          <span>${escapeHTML(connectionRow.value)}</span>
          ${flowValue}
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
      block.append(emptyKVRow("No connections"));
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
      rows.push({ id: connection.id, key: `input ${connection.to.node}`, value: `${connection.from.component}.${connection.from.node}`, connection });
    }
    if (connection.from.component === component.id) {
      rows.push({ id: connection.id, key: `output ${connection.from.node}`, value: `${connection.to.component}.${connection.to.node}`, connection });
    }
  }
  return rows;
}

function latestConnectionValue(connection) {
  if (!connection) return { hasValue: false, value: null };
  const result = latestRuntimeResult();
  const traced = (result?.connection_values || []).find((item) => item.id === connection.id);
  if (traced && Object.prototype.hasOwnProperty.call(traced, "value")) {
    return { hasValue: true, value: traced.value };
  }
  const outputs = result?.component_outputs?.[connection.from.component] || {};
  if (Object.prototype.hasOwnProperty.call(outputs, connection.from.node)) {
    return { hasValue: true, value: outputs[connection.from.node] };
  }
  const inputs = result?.component_inputs?.[connection.to.component] || {};
  if (Object.prototype.hasOwnProperty.call(inputs, connection.to.node)) {
    return { hasValue: true, value: inputs[connection.to.node] };
  }
  return { hasValue: false, value: null };
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
    tbody.append(emptyRow(4, "No parameters"));
  }
}

function mlMetadataBlock(component) {
  const metadata = component.ml_metadata || {};
  const rows = [
    ["Model Format", metadata.model_format || ""],
    ["Model File", metadata.model_file || ""],
    ["Feature Schema", metadata.feature_schema_file || ""],
    ["Target Schema", metadata.target_schema_file || ""],
    ["Validation Report", metadata.validation_report_file || ""],
    ["Required Packages", (metadata.required_packages || []).join(", ")],
    ["Time Resolution", metadata.valid_time_resolution || ""],
  ].filter(([, value]) => value);
  return inspectorBlock("ML Metadata", rows);
}

function renderParameterAddForm(container, components, editable) {
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
      syncParameterInputs(component.id, name, input.value, input);
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

function emptyKVRow(message) {
  const row = document.createElement("div");
  row.className = "kv";
  row.innerHTML = `<span class="kv-key">${escapeHTML(message)}</span><span></span>`;
  return row;
}

function emptyRow(cols, message = "No rows") {
  const tr = document.createElement("tr");
  tr.innerHTML = `<td colspan="${cols}" class="empty-cell">${escapeHTML(message)}</td>`;
  return tr;
}

function renderProblems() {
  const panel = el("problemsPanel");
  panel.innerHTML = "";
  const problems = state.latestValidation?.problems || [];
  if (problems.length) {
    for (const problem of problems) panel.append(problemRow(problem));
    refreshSourceProblemMarkers();
    return;
  }
  if (state.latestValidation?.error) {
    panel.append(problemRow({ severity: "error", message: state.latestValidation.error }));
    refreshSourceProblemMarkers();
    return;
  }
  panel.append(problemRow({ severity: "ok", message: "No problems" }));
  refreshSourceProblemMarkers();
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
  const value = state.latestWorkflowRecord || state.latestDataValidation || state.latestSeriesResult || state.latestBatchRecord || state.latestRunRecord || state.latestResult;
  const panel = el("resultsPanel");
  panel.innerHTML = "";
  if (!value) return;
  const view = structuredResultView(value);
  if (view) panel.append(view);
  panel.append(rawJSONBlock(value));
}

function structuredResultView(value) {
  const wrapper = document.createElement("div");
  wrapper.className = "result-structured";
  if (value.kind === "dataset" && value.dataset) {
    wrapper.append(resultHeader("Dataset Preview", value.dataset.summary?.relative_path || "", `${value.dataset.summary?.row_count || 0} rows`, "/docs/user/data-validation.md"));
    wrapper.append(datasetResultSection(value.dataset));
    return wrapper;
  }
  if (value.kind === "parameter_set" && value.parameter_set) {
    wrapper.append(resultHeader("Parameter Set", value.parameter_set.summary?.relative_path || "", `${value.parameter_set.summary?.parameter_count || 0} values`, "/docs/user/parameter-management.md"));
    wrapper.append(parameterSetResultSection(value.parameter_set));
    return wrapper;
  }
  if (value.kind && value.artifact) {
    wrapper.append(resultHeader(value.kind.replace(/_/g, " "), value.artifact.relative_path || value.artifact.path || "", value.artifact.state || ""));
    wrapper.append(resultTable("Summary", objectRows(value.artifact)));
    return wrapper;
  }

  const validation = value.result?.metrics ? value.result : value.metrics ? value : null;
  const series = value.series && value.outputs && value.step_count !== undefined ? value : null;
  if (series) {
    wrapper.append(resultHeader("Time Series", `${series.step_count || 0} steps`, series.parameter_set || "baseline", "/docs/user/run-simulation.md"));
    wrapper.append(resultTable("Public Output Series", seriesOutputRows(series), ["Output", "Values"]));
    wrapper.append(resultTable("Final States", Object.keys(series.final_states || {}).map((component) => [component, formatValue(series.final_states[component])]), ["Component", "State"]));
    return wrapper;
  }

  if (validation) {
    wrapper.append(resultHeader("Validation Result", validation.mapping_name || validation.mapping_id || "", `${validation.row_count || 0} rows`, "/docs/user/data-validation.md"));
    wrapper.append(validationResultSection(validation));
    return wrapper;
  }

  const calibration = value.result?.candidates && value.result?.saved_parameter_set !== undefined ? value.result : value.candidates && value.saved_parameter_set !== undefined ? value : null;
  if (calibration) {
    wrapper.append(resultHeader("Calibration Result", calibration.setup_name || calibration.setup_id || "", `best ${shortNumber(calibration.best_objective)}`, "/docs/user/calibration.md"));
    wrapper.append(candidateResultSection(calibration, "Saved parameter set", calibration.saved_parameter_set));
    return wrapper;
  }

  const optimization = value.result?.candidates && value.result?.saved_scenario !== undefined ? value.result : value.candidates && value.saved_scenario !== undefined ? value : null;
  if (optimization) {
    wrapper.append(resultHeader("Optimization Result", optimization.setup_name || optimization.setup_id || "", `best ${shortNumber(optimization.best_objective)}`, "/docs/user/optimization.md"));
    wrapper.append(candidateResultSection(optimization, "Saved scenario", optimization.saved_scenario));
    return wrapper;
  }

  if (value.cases) {
    wrapper.append(resultHeader("Batch Result", value.id || "", `${(value.cases || []).filter((item) => item.ok).length}/${(value.cases || []).length} ok`, "/docs/user/run-simulation.md"));
    wrapper.append(resultTable("Cases", (value.cases || []).map((item) => [
      item.scenario_name || item.scenario_id || "",
      item.ok ? "ok" : "failed",
      item.ok ? resultPublicOutputSummary(item.result?.outputs || {}) : item.error || "",
    ]), ["Scenario", "Status", "Output / Error"]));
    return wrapper;
  }

  const run = value.result?.outputs ? value.result : value.outputs ? value : null;
  if (run) {
    wrapper.append(resultHeader("Run Result", value.id || "current run", `${Object.keys(run.outputs || {}).length} outputs`, "/docs/user/run-simulation.md"));
    wrapper.append(resultTable("Public Outputs", Object.entries(run.outputs || {}).map(([name, output]) => [name, formatValue(output)]), ["Output", "Value"]));
    return wrapper;
  }
  return null;
}

function resultHeader(title, subtitle, status, helpPath = "") {
  const header = document.createElement("div");
  header.className = "result-header";
  header.innerHTML = `
    <div>
      <div class="result-title">${escapeHTML(title)}</div>
      <div class="result-subtitle">${escapeHTML(subtitle || "")}</div>
    </div>
    <div class="result-header-actions">
      <div class="result-status">${escapeHTML(status || "")}</div>
      ${helpPath ? `<a class="help-button result-help-button" href="${escapeAttr(helpPath)}" target="_blank" rel="noopener" title="Open related help" aria-label="Open related help">?</a>` : ""}
    </div>
  `;
  return header;
}

function datasetResultSection(dataset) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Dataset", [
    ["Path", dataset.summary?.relative_path || ""],
    ["Shape", `${dataset.summary?.row_count || 0} rows / ${dataset.summary?.column_count || 0} columns`],
    ["Format", dataset.summary?.format || ""],
  ]));
  section.append(resultTable("Public IO Mapping", [
    ...suggestionRows("input", dataset.suggested_inputs || []),
    ...suggestionRows("output", dataset.suggested_outputs || []),
  ], ["Direction", "Public ID", "Column", "Unit"]));
  section.append(previewRowsSection(dataset));
  if (isWorkspaceProject()) {
    const actions = document.createElement("div");
    actions.className = "result-actions";
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action";
    button.textContent = "Create Mapping";
    button.addEventListener("click", () => createValidationMappingFromDataset(dataset));
    actions.append(button);
    section.append(actions);
  }
  return section;
}

function suggestionRows(direction, suggestions) {
  return suggestions.map((item) => [
    direction,
    item.public_id || "",
    item.column || "unmatched",
    [item.value_type || "", item.unit || "", item.required ? "required" : "optional"].filter(Boolean).join(" / "),
  ]);
}

function previewRowsSection(dataset) {
  const columns = dataset.columns || [];
  const rows = (dataset.preview_rows || []).map((row) => columns.map((column) => row[column] ?? ""));
  return resultTable("Preview Rows", rows, columns);
}

async function createValidationMappingFromDataset(dataset) {
  try {
    const body = await api("/api/project/validation-mapping", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        dataset_path: dataset.summary?.relative_path || "",
        missing_value_policy: "fail_fast",
      }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "validation_mapping", artifact: body.summary, mapping: body.mapping };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    log(`Validation mapping created: ${body.summary?.relative_path || body.summary?.id}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Validation mapping failed: ${error.message}`);
  }
}

function parameterSetResultSection(detail) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Summary", [
    ["Path", detail.summary?.relative_path || ""],
    ["Components", String(detail.summary?.component_count || 0)],
    ["Values", String(detail.summary?.parameter_count || 0)],
    ["Created", detail.summary?.created_at_utc || ""],
  ]));
  section.append(resultTable("Differences", (detail.differences || []).map((item) => [
    item.component,
    item.parameter,
    item.exists ? formatValue(item.baseline) : "new",
    formatValue(item.value),
  ]), ["Component", "Parameter", "Current", "Set Value"]));
  const actions = document.createElement("div");
  actions.className = "result-actions";
  const activate = document.createElement("button");
  activate.type = "button";
  activate.className = "small-action";
  activate.textContent = state.activeParameterSetPath === detail.summary?.relative_path ? "Active" : "Use for Runs";
  activate.disabled = state.activeParameterSetPath === detail.summary?.relative_path;
  activate.addEventListener("click", () => {
    state.activeParameterSetPath = detail.summary?.relative_path || "";
    renderRunInputs();
    renderProjectTree();
    renderStartRuntimeRows();
    renderResults();
    log(`Active parameter set: ${state.activeParameterSetPath}`);
  });
  actions.append(activate);
  if (isWorkspaceProject()) {
    const apply = document.createElement("button");
    apply.type = "button";
    apply.className = "small-action";
    apply.textContent = "Apply to Graph";
    apply.addEventListener("click", () => applyParameterSetToGraph(detail.summary?.relative_path || ""));
    actions.append(apply);
  }
  section.append(actions);
  return section;
}

async function applyParameterSetToGraph(path) {
  try {
    const body = await api("/api/project/parameter-set/apply", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, path }),
    });
    state.detail = body.project;
    state.activeParameterSetPath = path;
    renderAll();
    log(`Parameter set applied: ${path}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Parameter set apply failed: ${error.message}`);
  }
}

function validationResultSection(result) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Summary", [
    ["Dataset", result.dataset || ""],
    ["Mapping", result.mapping || result.mapping_id || ""],
    ["Parameter set", result.parameter_set || ""],
    ["Missing values", result.missing_value_policy || "fail_fast"],
  ]));
  section.append(metricBars(result.metrics || {}));
  section.append(highErrorRows(result.metrics || {}));
  return section;
}

function metricBars(metrics) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Metrics</div>`;
  const rows = document.createElement("div");
  rows.className = "metric-bars";
  const entries = Object.entries(metrics);
  if (!entries.length) {
    rows.innerHTML = `<div class="empty-cell">No metrics</div>`;
  } else {
    const maxRMSE = Math.max(...entries.map(([, item]) => Number(item.rmse) || 0), 1);
    for (const [name, item] of entries) {
      const width = Math.max(3, ((Number(item.rmse) || 0) / maxRMSE) * 100);
      const row = document.createElement("div");
      row.className = "metric-row";
      row.innerHTML = `
        <div class="metric-name">${escapeHTML(name)}</div>
        <div class="metric-track"><div class="metric-fill" style="width: ${width}%"></div></div>
        <div class="metric-values">RMSE ${escapeHTML(shortNumber(item.rmse))} / MAE ${escapeHTML(shortNumber(item.mae))} / R2 ${escapeHTML(shortNumber(item.r2))}</div>
      `;
      rows.append(row);
    }
  }
  block.append(rows);
  return block;
}

function highErrorRows(metrics) {
  const rows = [];
  for (const [metric, item] of Object.entries(metrics)) {
    for (const high of item.high_error_rows || []) {
      rows.push([
        metric,
        String(high.row_index),
        high.time ?? "",
        shortNumber(high.observed),
        shortNumber(high.simulated),
        shortNumber(high.error),
      ]);
    }
  }
  return resultTable("High Error Rows", rows, ["Metric", "Row", "Time", "Observed", "Simulated", "Error"]);
}

function candidateResultSection(result, savedLabel, savedPath) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Summary", [
    ["Setup", result.setup_name || result.setup_id || ""],
    ["Objective", shortNumber(result.objective)],
    ["Best objective", shortNumber(result.best_objective)],
    [savedLabel, savedPath || ""],
  ]));
  section.append(resultTable("Candidates", (result.candidates || []).slice(0, 12).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    "candidate",
    parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
  ]), ["#", "Objective", "Status", "Values"]));
  return section;
}

function parameterCandidateSummary(values) {
  const entries = [];
  for (const [component, parameters] of Object.entries(values || {})) {
    if (parameters && typeof parameters === "object" && !Array.isArray(parameters)) {
      for (const [name, value] of Object.entries(parameters)) {
        entries.push(`${component}.${name}=${formatValue(value)}`);
      }
    } else {
      entries.push(`${component}=${formatValue(parameters)}`);
    }
  }
  return entries.slice(0, 5).join(", ");
}

function resultPublicOutputSummary(outputs) {
  const entries = Object.entries(outputs || {});
  if (!entries.length) return "";
  return entries.map(([name, value]) => `${name}: ${formatValue(value)}`).join(", ");
}

function seriesOutputRows(series) {
  return Object.entries(series.outputs || {}).map(([name, values]) => {
    const list = Array.isArray(values) ? values : [];
    const last = list.length ? list[list.length - 1] : "";
    return [name, `${list.length} values / last ${formatValue(last)}`];
  });
}

function resultTable(title, rows, headers = ["Item", "Value"]) {
  const block = document.createElement("div");
  block.className = "result-block";
  const normalizedRows = rows || [];
  block.innerHTML = `
    <div class="result-block-title">${escapeHTML(title)}</div>
    <table class="result-table">
      <thead><tr>${headers.map((header) => `<th>${escapeHTML(header)}</th>`).join("")}</tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!normalizedRows.length) {
    tbody.innerHTML = `<tr><td colspan="${headers.length}" class="empty-cell">No ${escapeHTML(String(title || "rows").toLowerCase())}</td></tr>`;
    return block;
  }
  for (const row of normalizedRows) {
    const cells = Array.isArray(row) ? row : [row.name || row[0] || "", row.value || row[1] || ""];
    const tr = document.createElement("tr");
    tr.innerHTML = headers.map((_, index) => `<td>${escapeHTML(cells[index] ?? "")}</td>`).join("");
    tbody.append(tr);
  }
  return block;
}

function objectRows(value) {
  return Object.entries(value || {}).filter(([, item]) => typeof item !== "object").map(([key, item]) => [key, item]);
}

function rawJSONBlock(value) {
  const details = document.createElement("details");
  details.className = "result-raw";
  const summary = document.createElement("summary");
  summary.textContent = "Raw JSON";
  const pre = document.createElement("pre");
  pre.className = "code-pane result-json";
  pre.textContent = JSON.stringify(value, null, 2);
  details.append(summary, pre);
  return details;
}

function latestRuntimeResult() {
  if (state.latestSeriesResult) return seriesLastResult(state.latestSeriesResult);
  if (state.latestBatchRecord) {
    const found = (state.latestBatchRecord.cases || []).find((item) => item.ok && item.result);
    return found?.result || null;
  }
  if (state.latestRunRecord?.result) return state.latestRunRecord.result;
  return state.latestResult;
}

function latestRuntimeComparisonContext() {
  if (state.latestSeriesResult) {
    return { result: seriesLastResult(state.latestSeriesResult), source: seriesSourceLabel(state.latestSeriesResult) };
  }
  if (state.latestBatchRecord) {
    const found = (state.latestBatchRecord.cases || []).find((item) => item.ok && item.result);
    if (found?.result) {
      return { result: found.result, source: batchRunSourceLabel(state.latestBatchRecord, found) };
    }
  }
  if (state.latestRunRecord?.result) {
    return { result: state.latestRunRecord.result, source: runRecordSourceLabel(state.latestRunRecord) };
  }
  if (state.latestResult) {
    return { result: state.latestResult, source: state.latestRunSource || currentRunSourceLabel() };
  }
  return null;
}

function seriesLastResult(series) {
  const points = series?.series || [];
  const point = points[points.length - 1];
  if (!point) return null;
  return {
    ok: true,
    parameter_set: series.parameter_set || "",
    outputs: point.outputs || {},
    component_inputs: point.component_inputs || {},
    component_outputs: point.component_outputs || {},
    node_values: point.node_values || [],
    connection_values: point.connection_values || [],
    states: point.states || {},
    context: point.context || {},
    execution_order: series.execution_order || point.execution_order || [],
    component_timings: point.component_timings || [],
    component_logs: point.component_logs || [],
    duration_ms: point.duration_ms,
  };
}

function seriesSourceLabel(series) {
  const parts = ["series preview"];
  if (series?.step_count) parts.push(`${series.step_count} steps`);
  parts.push(series?.parameter_set ? `parameter set ${series.parameter_set}` : "baseline");
  return parts.join(" / ");
}

function currentRunSourceLabel() {
  const parts = ["current run"];
  if (state.activeRunInput) {
    parts.push(`scenario ${state.activeRunInput.name || state.activeRunInput.id || "active"}`);
  }
  parts.push(state.activeParameterSetPath ? `parameter set ${state.activeParameterSetPath}` : "baseline");
  return parts.join(" / ");
}

function runRecordSourceLabel(record) {
  const parts = [record.id || "saved run"];
  parts.push(record.parameter_set ? `parameter set ${record.parameter_set}` : "baseline");
  return parts.join(" / ");
}

function batchRunSourceLabel(record, firstCase) {
  const parts = [record.id || "batch"];
  const caseName = firstCase?.scenario_name || firstCase?.scenario_id || "";
  if (caseName) parts.push(`case ${caseName}`);
  parts.push(record.parameter_set ? `parameter set ${record.parameter_set}` : "baseline");
  return parts.join(" / ");
}

function renderRunWorkspace() {
  renderRunOutputWorkspace(
    state,
    el("runSummaryRows"),
    el("runOutputRows"),
    el("runComparisonRows"),
    el("runOutputChart"),
    el("componentRunRows"),
    el("batchCaseRows"),
    el("executionTraceRows"),
    el("componentLogRows"),
    el("connectionTraceRows"),
    el("nodeTraceRows"),
  );
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
    hideSourceCompletionPanel();
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
  updateSourceHighlight(value);
}

function renderSourceComponentSelect(selectedID) {
  const select = el("sourceComponentSelect");
  if (!select) return;
  const components = state.detail?.graph?.components || [];
  select.innerHTML = "";
  for (const component of components) {
    const option = document.createElement("option");
    option.value = component.id;
    option.textContent = componentOptionLabel(component);
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
  renderSourceEditorMeta(component && source ? draft || "" : null);
  for (const id of ["saveSourceButton", "saveRunSourceButton", "revertSourceButton", "checkSourceButton", "insertSnippetButton", "formatSourceButton", "sourceSnippetSelect"]) {
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
  container.append(contractBlock("Runtime Contract", sourceContractRows(component)));
  container.append(sourceReferenceBlock("Inputs", (component.nodes.inputs || []).map((node) => ({
    name: node.id,
    meta: nodeTypeLabel(node),
    snippet: `inputs.get(${pythonStringLiteral(node.id)}, 0.0)`,
  })), component));
  container.append(sourceReferenceBlock("Outputs", (component.nodes.outputs || []).map((node) => ({
    name: node.id,
    meta: nodeTypeLabel(node),
    snippet: `${pythonStringLiteral(node.id)}: value`,
  })), component));
  container.append(sourceReferenceBlock("Parameters", Object.entries(component.parameters || {}).map(([name, value]) => ({
    name,
    meta: parameterInputValue(value),
    snippet: `params.get(${pythonStringLiteral(name)}, ${pythonLiteral(value)})`,
  })), component));
  container.append(sourceReferenceBlock("Completions", sourceCompletionItems(component), component));
  const runtimeBlock = sourceRuntimeBlock(component);
  if (runtimeBlock) container.append(runtimeBlock);
  container.append(sourceIssueBlock(component.id));
}

function sourceContractRows(component) {
  if (component.source?.layout === "generated_wrapper") {
    return [
      ["Editable", component.source.step || "user_step.py"],
      ["Function", "step(inputs, state, params, context)"],
    ];
  }
  return [
    ["Editable", component.source?.step || component.class || ""],
    ["Evaluate", "evaluate(self, inputs, state, params, context)"],
    ["Initialize", "initialize(self, params, context)"],
  ];
}

function sourceRuntimeBlock(component) {
  const result = latestRuntimeResult();
  const latestInputs = result?.component_inputs?.[component.id] || {};
  const latestOutputs = result?.component_outputs?.[component.id] || {};
  const rows = [
    ...Object.entries(latestInputs).map(([name, value]) => [`in ${name}`, formatValue(value)]),
    ...Object.entries(latestOutputs).map(([name, value]) => [`out ${name}`, formatValue(value)]),
  ];
  if (!rows.length) return null;
  return contractBlock(runValueTitle("Last Run"), rows);
}

function contractBlock(title, rows) {
  const block = document.createElement("div");
  block.className = "contract-block";
  block.innerHTML = `<div class="contract-title">${escapeHTML(title)}</div>`;
  if (!rows.length) {
    block.append(emptyContractRow(`No ${String(title || "entries").toLowerCase()}`));
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

function sourceReferenceBlock(title, rows, component) {
  const block = document.createElement("div");
  block.className = "contract-block";
  block.innerHTML = `<div class="contract-title">${escapeHTML(title)}</div>`;
  if (!rows.length) {
    block.append(emptyContractRow(`No ${String(title || "references").toLowerCase()}`));
    return block;
  }
  const editable = canEditSource(component);
  for (const item of rows) {
    const rowEl = document.createElement("div");
    rowEl.className = "contract-row";
    rowEl.title = [item.name, item.meta].filter(Boolean).join(" / ");
    rowEl.innerHTML = `<span>${escapeHTML(item.name)}</span><span class="contract-meta">${escapeHTML(item.meta || "")}</span>`;
    if (editable) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "contract-insert";
      button.textContent = "Insert";
      button.title = `Insert ${item.name}`;
      button.addEventListener("click", () => insertSourceText(item.snippet));
      rowEl.append(button);
    }
    block.append(rowEl);
  }
  return block;
}

function emptyContractRow(message) {
  const row = document.createElement("div");
  row.className = "contract-row";
  row.innerHTML = `<span>${escapeHTML(message)}</span><span class="contract-meta"></span>`;
  return row;
}

function canEditSource(component) {
  const source = component ? state.sourceByComponent[component.id] : null;
  return Boolean(component && source && !source.read_only && isWorkspaceProject());
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
  const quickFix = sourceQuickFixForProblem(problem, componentById(problem.component_id || state.selectedComponentId));
  if (quickFix) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "contract-insert source-quick-fix";
    button.textContent = "Fix";
    button.title = quickFix.title;
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      insertSourceText(quickFix.snippet);
    });
    row.append(button);
  }
  if (problem.line) {
    row.classList.add("linked");
    row.addEventListener("click", () => focusSourceIssue(problem));
  }
  return row;
}

function sourceQuickFixForProblem(problem, component) {
  if (!component || !canEditSource(component)) return null;
  const message = String(problem.message || "");
  let match = message.match(/^required input node is not referenced in source: (.+)$/);
  if (match) {
    const nodeID = match[1];
    const variable = pythonIdentifier(nodeID) || "value";
    return {
      title: `Insert input read for ${nodeID}`,
      snippet: `${variable} = inputs.get(${pythonStringLiteral(nodeID)}, 0.0)`,
    };
  }
  match = message.match(/^output node is not obviously returned by source: (.+)$/);
  if (match) {
    const nodeID = match[1];
    return {
      title: `Insert output entry for ${nodeID}`,
      snippet: `${pythonStringLiteral(nodeID)}: value`,
    };
  }
  if (message === "evaluate method is missing") {
    return {
      title: "Insert evaluate method scaffold",
      snippet: evaluateSnippet(component, pythonInputBindings(component)),
    };
  }
  if (message === "step function is missing") {
    return {
      title: "Insert step function scaffold",
      snippet: stepSnippet(component, pythonInputBindings(component)),
    };
  }
  return null;
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
  const markers = sourceLineProblemMap(state.selectedComponentId);
  gutter.innerHTML = Array.from({ length: lines }, (_, index) => {
    const line = index + 1;
    const marker = markers.get(line);
    const classes = ["source-line-number"];
    if (marker) {
      classes.push("has-marker", marker.severity === "error" ? "error" : "warning");
    }
    const title = marker ? marker.messages.join(" / ") : `Line ${line}`;
    return `<span class="${classes.join(" ")}" title="${escapeAttr(title)}">${line}</span>`;
  }).join("");
}

function refreshSourceProblemMarkers() {
  const editor = el("sourceEditor");
  if (!editor) return;
  updateLineNumbers(editor.value || "");
}

function sourceLineProblemMap(componentID) {
  const markers = new Map();
  if (!componentID) return markers;
  for (const problem of sourceMarkerProblems(componentID)) {
    const line = Number(problem.line) || 0;
    if (line <= 0) continue;
    const existing = markers.get(line) || { severity: "info", messages: [] };
    const severity = strongestProblemSeverity(existing.severity, problem.severity);
    existing.severity = severity;
    existing.messages.push(problem.message || problem.severity || "source issue");
    markers.set(line, existing);
  }
  return markers;
}

function sourceMarkerProblems(componentID) {
  const problems = [];
  const check = state.sourceCheckByComponent[componentID];
  if (check?.problems) problems.push(...check.problems);
  const source = state.sourceByComponent[componentID];
  const draft = sourceDraft(componentID);
  const dirty = source && draft !== source.content;
  if (!dirty) {
    for (const problem of state.latestValidation?.problems || []) {
      if (problem.component_id === componentID) problems.push(problem);
    }
  }
  const seen = new Set();
  return problems.filter((problem) => {
    const key = [problem.severity, problem.message, problem.line, problem.column].join("\x00");
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function strongestProblemSeverity(current, next) {
  const rank = { error: 3, warning: 2, info: 1, ok: 0 };
  return (rank[next] || 0) > (rank[current] || 0) ? next : current;
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
  const comparisonBaseline = latestRuntimeComparisonContext();
  const runSource = currentRunSourceLabel();
  const controller = beginRuntimeRequest("Run");
  if (!controller) return;
  try {
    const save = currentProject()?.source === "workspace";
    const body = await api("/api/run", {
      method: "POST",
      signal: controller.signal,
      body: JSON.stringify({ project_path: state.currentProjectPath, inputs, context: currentRunContext(), parameter_set_path: state.activeParameterSetPath, timeout_ms: state.runTimeoutMS, save }),
    });
    state.latestResult = body.result;
    state.latestSeriesResult = null;
    state.latestRunSource = runSource;
    state.runComparisonBaseline = comparisonBaseline;
    state.latestResultStale = false;
    state.latestRunRecord = null;
    state.latestBatchRecord = null;
    state.latestDataValidation = null;
    state.latestWorkflowRecord = null;
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
    if (isAbortError(error)) {
      log("Run canceled");
      state.latestValidation = { error: "Run canceled", problems: [] };
      setBottomTab("problems");
    } else {
      log(`Run failed: ${error.message}`);
      state.latestResult = null;
      state.latestSeriesResult = null;
      state.latestRunSource = "";
      state.latestResultStale = false;
      state.latestRunRecord = null;
      state.latestBatchRecord = null;
      state.latestDataValidation = null;
      state.latestWorkflowRecord = null;
      state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
      setBottomTab("problems");
    }
  } finally {
    finishRuntimeRequest(controller);
  }
  renderSystemHeader();
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProblems();
  renderResults();
  renderRunWorkspace();
}

async function runSeries() {
  if (!(await saveModelEditsBeforeExecution())) return;
  const seriesInput = buildSeriesInput();
  const comparisonBaseline = latestRuntimeComparisonContext();
  const controller = beginRuntimeRequest("Series");
  if (!controller) return;
  try {
    const body = await api("/api/run-series", {
      method: "POST",
      signal: controller.signal,
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        schema_version: "0.1.0",
        context: seriesInput.context,
        steps: seriesInput.steps,
        parameter_set_path: state.activeParameterSetPath,
        timeout_ms: state.runTimeoutMS,
      }),
    });
    state.latestSeriesResult = body.result;
    state.latestResult = null;
    state.latestRunSource = "";
    state.runComparisonBaseline = comparisonBaseline;
    state.latestResultStale = false;
    state.latestRunRecord = null;
    state.latestBatchRecord = null;
    state.latestDataValidation = null;
    state.latestWorkflowRecord = null;
    setProblems();
    renderSystemHeader();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderResults();
    renderRunWorkspace();
    renderProblems();
    setMode("run");
    setBottomTab("results");
    log(`Series complete: ${body.result.step_count || 0} steps`);
  } catch (error) {
    if (isAbortError(error)) {
      log("Series canceled");
      state.latestValidation = { error: "Series canceled", problems: [] };
    } else {
      log(`Series failed: ${error.message}`);
      state.latestSeriesResult = null;
      state.latestResult = null;
      state.latestRunSource = "";
      state.latestResultStale = false;
      state.latestRunRecord = null;
      state.latestBatchRecord = null;
      state.latestDataValidation = null;
      state.latestWorkflowRecord = null;
      state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    }
    renderProblems();
    renderResults();
    renderRunWorkspace();
    setBottomTab("problems");
  } finally {
    finishRuntimeRequest(controller);
  }
}

async function runBatch() {
  if (!(await saveModelEditsBeforeExecution())) return;
  const comparisonBaseline = latestRuntimeComparisonContext();
  const controller = beginRuntimeRequest("Batch");
  if (!controller) return;
  try {
    const body = await api("/api/batch", {
      method: "POST",
      signal: controller.signal,
      body: JSON.stringify({ project_path: state.currentProjectPath, parameter_set_path: state.activeParameterSetPath, timeout_ms: state.runTimeoutMS }),
    });
    state.latestBatchRecord = body.batch;
    state.latestSeriesResult = null;
    state.runComparisonBaseline = comparisonBaseline;
    state.latestRunRecord = null;
    state.latestResult = null;
    state.latestRunSource = "";
    state.latestResultStale = false;
    state.latestDataValidation = null;
    state.latestWorkflowRecord = null;
    const batchProblems = collectBatchProblems(body.batch);
    state.latestValidation = { problems: batchProblems };
    state.detail.batches = [body.summary, ...(state.detail.batches || [])];
    renderProjectTree();
    renderSystemHeader();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderResults();
    renderRunWorkspace();
    renderProblems();
    setMode("run");
    setBottomTab(batchProblems.length ? "problems" : "results");
    log(`Batch saved: ${body.summary.relative_path}`);
  } catch (error) {
    if (isAbortError(error)) {
      log("Batch canceled");
      state.latestValidation = { error: "Batch canceled", problems: [] };
    } else {
      log(`Batch failed: ${error.message}`);
      state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    }
    renderProblems();
    setBottomTab("problems");
  } finally {
    finishRuntimeRequest(controller);
  }
}

function beginRuntimeRequest(label) {
  if (state.activeRunAbortController) {
    log(`${state.activeRunLabel || "Run"} is already running`);
    return null;
  }
  const controller = new AbortController();
  state.activeRunAbortController = controller;
  state.activeRunLabel = label;
  updateCommandState();
  return controller;
}

function finishRuntimeRequest(controller) {
  if (state.activeRunAbortController !== controller) return;
  state.activeRunAbortController = null;
  state.activeRunLabel = "";
  updateCommandState();
}

function cancelActiveRun() {
  if (!state.activeRunAbortController) return;
  const label = state.activeRunLabel || "Run";
  state.activeRunAbortController.abort();
  log(`${label} cancel requested`);
  updateCommandState();
}

function isAbortError(error) {
  return error?.name === "AbortError";
}

async function runDataValidation() {
  if (!(await saveModelEditsBeforeExecution())) return;
  const mapping = (state.detail?.validation_mappings || [])[0];
  if (!mapping) {
    state.latestValidation = { error: "No validation mapping is available for this project" };
    renderProblems();
    setBottomTab("problems");
    log("Data validation unavailable: no mapping");
    return;
  }
  try {
    const body = await api("/api/validation/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: mapping.relative_path,
        parameter_set_path: state.activeParameterSetPath,
        high_error_rows: 3,
        save: isWorkspaceProject(),
      }),
    });
    state.latestDataValidation = body.validation_result;
    state.latestSeriesResult = null;
    state.latestWorkflowRecord = null;
    if (body.validation_record) {
      state.detail.validation_runs = [body.validation_record, ...(state.detail.validation_runs || [])];
      await refreshCurrentProjectDetail();
    }
    setProblems();
    renderResults();
    renderProblems();
    setBottomTab("results");
    log(`Data validation complete: ${mapping.name || mapping.id}`);
  } catch (error) {
    state.latestDataValidation = null;
    state.latestSeriesResult = null;
    state.latestWorkflowRecord = null;
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderResults();
    renderProblems();
    setBottomTab("problems");
    log(`Data validation failed: ${error.message}`);
  }
}

async function runCalibrationSetup(setup) {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/calibration/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        setup_path: setup.relative_path,
        save: isWorkspaceProject(),
      }),
    });
    state.latestWorkflowRecord = body.calibration_result;
    state.latestSeriesResult = null;
    state.latestDataValidation = null;
    setProblems();
    await refreshCurrentProjectDetail();
    renderResults();
    renderProblems();
    setBottomTab("results");
    setMode("artifacts");
    log(`Calibration complete: ${setup.name || setup.id}`);
  } catch (error) {
    state.latestWorkflowRecord = null;
    state.latestSeriesResult = null;
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderResults();
    renderProblems();
    setBottomTab("problems");
    log(`Calibration failed: ${error.message}`);
  }
}

async function runOptimizationSetup(setup) {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/optimization/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        setup_path: setup.relative_path,
        save: isWorkspaceProject(),
      }),
    });
    state.latestWorkflowRecord = body.optimization_result;
    state.latestSeriesResult = null;
    state.latestDataValidation = null;
    setProblems();
    await refreshCurrentProjectDetail();
    renderResults();
    renderProblems();
    setBottomTab("results");
    setMode("artifacts");
    log(`Optimization complete: ${setup.name || setup.id}`);
  } catch (error) {
    state.latestWorkflowRecord = null;
    state.latestSeriesResult = null;
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderResults();
    renderProblems();
    setBottomTab("problems");
    log(`Optimization failed: ${error.message}`);
  }
}

async function refreshCurrentProjectDetail() {
  if (!state.currentProjectPath) return;
  const body = await api(`/api/project?project_path=${encodeURIComponent(state.currentProjectPath)}`);
  state.detail = body.project;
  renderProjectTree();
  renderArtifactWorkspace();
  renderRunInputs();
  updateCommandState();
}

function parameterSetField() {
  const field = document.createElement("div");
  field.className = "input-field parameter-set-field";
  const sets = state.detail?.parameter_sets || [];
  field.innerHTML = `
    <label for="runParameterSetSelect">
      <span class="input-label">Parameter Set</span>
      <span class="input-meta">${escapeHTML(state.activeParameterSetPath || "baseline")}</span>
    </label>
    <select id="runParameterSetSelect" class="run-select"></select>
  `;
  const select = field.querySelector("select");
  select.append(new Option("Baseline", ""));
  for (const item of sets) {
    select.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  select.value = state.activeParameterSetPath || "";
  select.addEventListener("change", () => {
    state.activeParameterSetPath = select.value;
    renderProjectTree();
    renderRunInputs();
    log(`Parameter set selected: ${state.activeParameterSetPath || "baseline"}`);
  });
  return field;
}

function runTimeoutField() {
  const field = document.createElement("div");
  field.className = "input-field timeout-field";
  const seconds = Math.max(1, Math.round((state.runTimeoutMS || 30000) / 1000));
  field.innerHTML = `
    <label for="runTimeoutInput">
      <span class="input-label">Timeout</span>
      <span class="input-meta">seconds per request</span>
    </label>
    <input id="runTimeoutInput" type="number" min="1" max="1800" step="1" value="${escapeAttr(seconds)}" />
  `;
  field.querySelector("input").addEventListener("input", (event) => {
    const value = Math.max(1, Math.min(1800, Number(event.target.value) || 30));
    state.runTimeoutMS = value * 1000;
  });
  return field;
}

function buildSeriesInput() {
  const inputs = collectRunInputs();
  const baseContext = { ...(currentRunContext() || {}) };
  const dt = Number.isFinite(Number(baseContext.dt)) ? Number(baseContext.dt) : 60;
  const start = Number.isFinite(Number(baseContext.time)) ? Number(baseContext.time) : 0;
  const context = { ...baseContext, dt };
  const steps = [0, 1, 2].map((offset) => ({
    id: `step-${offset + 1}`,
    inputs: { ...inputs },
    context: { time: start + offset * dt, dt },
  }));
  return { context, steps };
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
  const comparisonBaseline = latestRuntimeComparisonContext();
  try {
    const body = await api(`/api/project/run?project_path=${encodeURIComponent(state.currentProjectPath)}&run_id=${encodeURIComponent(runID)}`);
    state.latestRunRecord = body.run_record;
    state.runComparisonBaseline = comparisonBaseline;
    state.latestBatchRecord = null;
    state.latestSeriesResult = null;
    state.latestDataValidation = null;
    state.latestWorkflowRecord = null;
    state.activeParameterSetPath = body.run_record.parameter_set || "";
    state.latestResult = body.run_record.result;
    state.latestRunSource = "";
    state.latestResultStale = false;
    setProblems();
    renderSystemHeader();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
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
  const comparisonBaseline = latestRuntimeComparisonContext();
  try {
    const body = await api(`/api/project/batch?project_path=${encodeURIComponent(state.currentProjectPath)}&batch_id=${encodeURIComponent(batchID)}`);
    state.latestBatchRecord = body.batch_record;
    state.latestSeriesResult = null;
    state.runComparisonBaseline = comparisonBaseline;
    state.latestRunRecord = null;
    state.latestDataValidation = null;
    state.latestWorkflowRecord = null;
    state.activeParameterSetPath = body.batch_record.parameter_set || "";
    state.latestResult = null;
    state.latestRunSource = "";
    state.latestResultStale = false;
    const batchProblems = collectBatchProblems(body.batch_record);
    state.latestValidation = { problems: batchProblems };
    renderSystemHeader();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderRunInputs();
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

async function loadWorkflowRecord(kind, recordID) {
  const endpoints = {
    validation: "validation-record",
    calibration: "calibration-record",
    optimization: "optimization-record",
  };
  const keys = {
    validation: "validation_record",
    calibration: "calibration_record",
    optimization: "optimization_record",
  };
  const endpoint = endpoints[kind];
  const responseKey = keys[kind];
  if (!endpoint || !responseKey) return;
  try {
    const body = await api(`/api/project/${endpoint}?project_path=${encodeURIComponent(state.currentProjectPath)}&record_id=${encodeURIComponent(recordID)}`);
    state.latestWorkflowRecord = body[responseKey];
    state.latestDataValidation = null;
    state.latestBatchRecord = null;
    state.latestSeriesResult = null;
    state.latestRunRecord = null;
    state.latestResult = null;
    state.latestRunSource = "";
    state.latestResultStale = false;
    setProblems();
    renderSystemHeader();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderRunInputs();
    renderResults();
    renderRunWorkspace();
    renderProblems();
    setBottomTab("results");
    log(`${kind} record opened: ${recordID}`);
  } catch (error) {
    log(`Open ${kind} record failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function loadScenario(scenarioID) {
  try {
    const body = await api(`/api/project/scenario?project_path=${encodeURIComponent(state.currentProjectPath)}&scenario_id=${encodeURIComponent(scenarioID)}`);
    state.activeRunInput = body.scenario;
    markRunResultStale(false);
    renderRunInputs();
    renderCanvas();
    renderInspector();
    renderPythonPanel();
    renderSystemHeader();
    setMode("canvas");
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
    renderProjectTree();
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
    renderProjectTree();
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
    renderProjectTree();
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
  delete state.sourceCheckByComponent[component.id];
  renderPythonPanel();
  renderProjectTree();
  log(`Source reverted: ${component.id}`);
}

function insertSourceSnippet() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  if (!component || !source || source.read_only || !isWorkspaceProject()) return;
  const snippet = sourceSnippet(el("sourceSnippetSelect")?.value || "evaluate", component);
  insertSourceText(snippet);
}

function formatCurrentSource() {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  const editor = el("sourceEditor") || el("pythonPanel");
  if (!component || !source || source.read_only || !isWorkspaceProject() || !editor || editor.readOnly) return;
  const formatted = formatPythonSource(sourceDraft(component.id));
  if (formatted === editor.value) {
    log(`Source already formatted: ${component.id}`);
    return;
  }
  const previousStart = editor.selectionStart ?? formatted.length;
  editor.value = formatted;
  const position = Math.min(previousStart, formatted.length);
  editor.selectionStart = editor.selectionEnd = position;
  updateSourceDraftFromEditor(editor);
  hideSourceCompletionPanel();
  editor.focus();
  log(`Source formatted: ${component.id}`);
}

function formatPythonSource(value) {
  const normalized = String(value || "").replace(/\r\n?/g, "\n");
  const lines = normalized.split("\n").map((line) => {
    const withoutTrailing = line.replace(/[ \t]+$/g, "");
    return withoutTrailing.replace(/^\t+/, (tabs) => "    ".repeat(tabs.length));
  });
  while (lines.length > 1 && lines[lines.length - 1] === "") {
    lines.pop();
  }
  return `${lines.join("\n")}\n`;
}

function insertSourceText(snippet) {
  const component = componentById(state.selectedComponentId);
  const source = component ? state.sourceByComponent[component.id] : null;
  const editor = el("sourceEditor") || el("pythonPanel");
  if (!component || !source || source.read_only || !isWorkspaceProject() || !editor || editor.readOnly) return;
  const start = editor.selectionStart ?? editor.value.length;
  const end = editor.selectionEnd ?? editor.value.length;
  editor.value = `${editor.value.slice(0, start)}${snippet}${editor.value.slice(end)}`;
  editor.selectionStart = editor.selectionEnd = start + snippet.length;
  updateSourceDraftFromEditor(editor);
  hideSourceCompletionPanel();
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
      return component.source?.layout === "generated_wrapper"
        ? stepSnippet(component, inputBindings)
        : evaluateSnippet(component, inputBindings);
  }
}

function stepSnippet(component, inputBindings) {
  const bindings = inputBindings.length ? inputBindings : [{ id: "value", varName: "value" }];
  const inputLines = bindings.map((item) => `    ${item.varName} = float(inputs.get(${pythonStringLiteral(item.id)}, 0.0))`).join("\n");
  const primaryValue = bindings[0].varName;
  const outputs = component.nodes.outputs || [];
  const outputLines = (outputs.length ? outputs : [{ id: "result" }])
    .map((node) => `        ${pythonStringLiteral(node.id)}: ${primaryValue},`)
    .join("\n");
  return `\ndef step(inputs, state, params, context):\n${inputLines}\n    return {\n${outputLines}\n    }, state\n`;
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

function sourceCompletionItems(component) {
  if (!component) return [];
  const items = [];
  const inputBindings = pythonInputBindings(component);
  for (const item of inputBindings) {
    const node = (component.nodes.inputs || []).find((candidate) => candidate.id === item.id) || {};
    items.push({
      name: `inputs[${pythonStringLiteral(item.id)}]`,
      meta: nodeTypeLabel(node) || "input",
      snippet: `inputs.get(${pythonStringLiteral(item.id)}, 0.0)`,
    });
    items.push({
      name: item.varName,
      meta: `local from ${item.id}`,
      snippet: item.varName,
    });
  }
  for (const node of component.nodes.outputs || []) {
    items.push({
      name: `${pythonStringLiteral(node.id)}: value`,
      meta: nodeTypeLabel(node) || "output",
      snippet: `${pythonStringLiteral(node.id)}: value`,
    });
  }
  const parameterDefinitions = component.parameter_defs || {};
  const parameterNames = new Set([...Object.keys(component.parameters || {}), ...Object.keys(parameterDefinitions)]);
  for (const name of [...parameterNames].sort()) {
    const definition = parameterDefinitions[name] || {};
    const value = component.parameters?.[name] ?? definition.current ?? definition.default ?? 0.0;
    items.push({
      name: `params[${pythonStringLiteral(name)}]`,
      meta: [definition.unit || "", roleLabel(definition.role || "parameter")].filter(Boolean).join(" / "),
      snippet: `params.get(${pythonStringLiteral(name)}, ${pythonLiteral(value)})`,
    });
  }
  for (const [name, definition] of Object.entries(component.state_defs || {})) {
    items.push({
      name: `state[${pythonStringLiteral(name)}]`,
      meta: [definition.unit || "", "state"].filter(Boolean).join(" / "),
      snippet: `state.get(${pythonStringLiteral(name)}, ${pythonLiteral(definition.initial)})`,
    });
  }
  for (const name of ["time", "dt"]) {
    items.push({
      name: `context[${pythonStringLiteral(name)}]`,
      meta: "context",
      snippet: `context.get(${pythonStringLiteral(name)}, 0.0)`,
    });
  }
  return items;
}

function roleLabel(role) {
  return String(role || "")
    .replace(/_target$/, "")
    .replace(/_/g, " ");
}

function showSourceCompletionPanel() {
  const panel = el("sourceCompletionPanel");
  const component = componentById(state.selectedComponentId);
  if (!panel || !canEditSource(component)) return;
  const items = sourceCompletionItems(component);
  if (!items.length) {
    hideSourceCompletionPanel();
    return;
  }
  panel.hidden = false;
  panel.innerHTML = `<div class="source-completion-title">Completions</div>`;
  for (const item of items.slice(0, 14)) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "source-completion-item";
    button.title = [item.name, item.meta].filter(Boolean).join(" / ");
    button.innerHTML = `
      <span class="source-completion-label">${escapeHTML(item.name)}</span>
      <span class="source-completion-meta">${escapeHTML(item.meta || "")}</span>
    `;
    button.addEventListener("click", () => insertSourceText(item.snippet));
    panel.append(button);
  }
}

function hideSourceCompletionPanel() {
  const panel = el("sourceCompletionPanel");
  if (!panel) return;
  panel.hidden = true;
  panel.innerHTML = "";
}

function renderSourceEditorMeta(value) {
  const meta = el("sourceEditorMeta");
  if (!meta) return;
  if (value === null) {
    meta.textContent = "";
    meta.className = "source-editor-meta";
    return;
  }
  const check = bracketCheck(value || "");
  meta.textContent = check.message;
  meta.className = `source-editor-meta ${check.ok ? "ok" : "error"}`;
}

function bracketCheck(value) {
  const openers = new Map([["(", ")"], ["[", "]"], ["{", "}"]]);
  const closers = new Set([...openers.values()]);
  const stack = [];
  let quote = "";
  let escaped = false;
  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === quote) {
        quote = "";
      }
      continue;
    }
    if (char === "#") {
      const nextLine = value.indexOf("\n", index);
      if (nextLine < 0) break;
      index = nextLine;
      continue;
    }
    if (char === "\"" || char === "'") {
      quote = char;
      continue;
    }
    if (openers.has(char)) {
      stack.push({ char, index });
      continue;
    }
    if (closers.has(char)) {
      const expected = stack.length ? openers.get(stack[stack.length - 1].char) : "";
      if (expected !== char) return { ok: false, message: "bracket mismatch" };
      stack.pop();
    }
  }
  if (stack.length) return { ok: false, message: `open ${stack[stack.length - 1].char}` };
  return { ok: true, message: "brackets ok" };
}

function updateSourceHighlight(value) {
  const highlight = el("sourceHighlight");
  if (!highlight) return;
  highlight.innerHTML = highlightPython(value || "");
}

function highlightPython(value) {
  const keywords = new Set([
    "and", "as", "assert", "break", "class", "continue", "def", "del", "elif", "else", "except",
    "False", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "None",
    "nonlocal", "not", "or", "pass", "raise", "return", "True", "try", "while", "with", "yield",
  ]);
  const builtins = new Set(["abs", "bool", "dict", "enumerate", "float", "int", "len", "list", "max", "min", "range", "round", "str", "sum"]);
  let output = "";
  for (let index = 0; index < value.length;) {
    const char = value[index];
    if (char === "#") {
      const end = value.indexOf("\n", index);
      const next = end < 0 ? value.length : end;
      output += `<span class="tok-comment">${escapeHTML(value.slice(index, next))}</span>`;
      index = next;
      continue;
    }
    if (char === "\"" || char === "'") {
      const start = index;
      const quote = char;
      index += 1;
      let escaped = false;
      while (index < value.length) {
        const nextChar = value[index];
        index += 1;
        if (escaped) {
          escaped = false;
        } else if (nextChar === "\\") {
          escaped = true;
        } else if (nextChar === quote) {
          break;
        }
      }
      output += `<span class="tok-string">${escapeHTML(value.slice(start, index))}</span>`;
      continue;
    }
    if (/[0-9]/.test(char)) {
      const match = value.slice(index).match(/^[0-9]+(?:\.[0-9]+)?/);
      output += `<span class="tok-number">${escapeHTML(match[0])}</span>`;
      index += match[0].length;
      continue;
    }
    if (/[A-Za-z_]/.test(char)) {
      const match = value.slice(index).match(/^[A-Za-z_][A-Za-z0-9_]*/);
      const token = match[0];
      if (keywords.has(token)) {
        output += `<span class="tok-keyword">${escapeHTML(token)}</span>`;
      } else if (builtins.has(token)) {
        output += `<span class="tok-builtin">${escapeHTML(token)}</span>`;
      } else {
        output += escapeHTML(token);
      }
      index += token.length;
      continue;
    }
    output += escapeHTML(char);
    index += 1;
  }
  return output.endsWith("\n") ? `${output} ` : output || " ";
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

function pythonLiteral(value) {
  if (typeof value === "number") return Number.isFinite(value) ? String(value) : "0.0";
  if (typeof value === "boolean") return value ? "True" : "False";
  if (value === null || value === undefined) return "None";
  if (typeof value === "string") return pythonStringLiteral(value);
  return pythonStringLiteral(parameterInputValue(value));
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
      body: JSON.stringify({ project_path: state.currentProjectPath, profile: "runtime_package", include_records: true }),
    });
    state.latestExport = body.export;
    state.latestExportSummary = body.summary;
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

async function loadExportRecord(profile) {
  try {
    const body = await api(`/api/project/export?project_path=${encodeURIComponent(state.currentProjectPath)}&profile=${encodeURIComponent(profile)}`);
    state.latestExport = body.export;
    state.latestExportSummary = body.summary;
    state.detail.exports = [body.summary, ...(state.detail.exports || []).filter((item) => item.profile !== body.summary.profile)];
    renderProjectTree();
    renderExportWorkspaceView();
    setMode("export");
    log(`Export opened: ${body.summary.relative_path}`);
  } catch (error) {
    log(`Open export failed: ${error.message}`);
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
  const nameInput = el("scenarioNameInput");
  const name = (state.scenarioDraftName || nameInput?.value || defaultScenarioName()).trim();
  if (!name) return;
  try {
    const body = await api("/api/project/scenarios", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name, inputs: collectRunInputs(), context: currentRunContext() }),
    });
    state.detail.scenarios = [body.summary, ...(state.detail.scenarios || [])];
    state.scenarioDraftName = "";
    if (nameInput) nameInput.value = "";
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
  const nameInput = el("projectNameInput");
  const name = (nameInput?.value || defaultProjectName("Project")).trim();
  if (!name) return;
  const template = el("projectTemplateSelect")?.value || "scalar";
  try {
    const body = await api("/api/projects", {
      method: "POST",
      body: JSON.stringify({ name, template }),
    });
    if (nameInput) nameInput.value = "";
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
  const nameInput = el("projectNameInput");
  const name = (nameInput?.value || defaultName).trim();
  if (!name) return;
  try {
    const body = await api("/api/projects/copy", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name }),
    });
    if (nameInput) nameInput.value = "";
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
  const nameInput = el("newComponentName");
  const name = (nameInput?.value || "").trim();
  if (!name) {
    showInlineProblem("Component name is required");
    nameInput?.focus();
    return;
  }
  const template = el("componentTemplateSelect")?.value || state.componentTemplates[0]?.id || "";
  if (!template) {
    showInlineProblem("Component template is required");
    return;
  }
  const includeInSystem = el("includeComponentOnCreate")?.checked !== false;
  try {
    const body = await api("/api/project/components", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, name, template, include_in_system: includeInSystem }),
    });
    state.detail = body.project;
    state.selectedComponentId = body.component.id;
    if (nameInput) nameInput.value = "";
    renderAll();
    setMode("code");
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

async function duplicateComponent(componentID) {
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
  await includeComponentInSystem(component.id);
}

async function includeComponentInSystem(componentID) {
  const component = componentById(componentID);
  if (!component || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/system/components", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id }),
    });
    state.detail = body.project;
    state.selectedComponentId = component.id;
    markRunResultStale(false);
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
    markRunResultStale(false);
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
    markRunResultStale(false);
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
  const nodeName = (el("newNodeName")?.value || "").trim() || nodeID;
  const valueType = el("newNodeValueType")?.value || "float";
  const medium = (el("newNodeMedium")?.value || "").trim() || "signal";
  const unit = (el("newNodeUnit")?.value || "").trim();
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
    name: nodeName,
    medium,
    value_type: valueType,
    unit,
  };
  if (direction === "input" && rawDefault.trim() !== "") {
    payload.default = coerceParameter(rawDefault);
  }
  if (direction === "input") {
    payload.required = Boolean(el("newNodeRequired")?.checked);
  }

  try {
    const body = await api("/api/project/nodes", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    markRunResultStale(false);
    renderAll();
    log(`Node added: ${componentID}.${nodeID}`);
  } catch (error) {
    log(`Add node failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function updateNodeFromInspector(componentID, nodeID, direction, row) {
  if (!componentID || !nodeID || !isWorkspaceProject()) return;
  const form = row || findNodeEditRow(componentID, nodeID);
  if (!form) return;
  const field = (name) => form.querySelector(`[data-node-field="${name}"]`);
  const name = (field("name")?.value || "").trim();
  const medium = (field("medium")?.value || "").trim();
  const valueType = field("value_type")?.value || "float";
  const unit = (field("unit")?.value || "").trim();
  if (!name) {
    showInlineProblem("Node name is required");
    return;
  }
  const payload = {
    project_path: state.currentProjectPath,
    component_id: componentID,
    node_id: nodeID,
    name,
    medium,
    value_type: valueType,
    unit,
  };
  if (direction === "input") {
    const rawDefault = field("default")?.value || "";
    payload.required = Boolean(field("required")?.checked);
    payload.default = rawDefault.trim() === "" ? null : coerceParameter(rawDefault);
  }
  try {
    const body = await api("/api/project/nodes/update", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    markRunResultStale(false);
    renderAll();
    log(`Node updated: ${componentID}.${nodeID}`);
  } catch (error) {
    log(`Update node failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function findNodeEditRow(componentID, nodeID) {
  for (const row of document.querySelectorAll("[data-node-component][data-node-id]")) {
    if (row.dataset.nodeComponent === componentID && row.dataset.nodeId === nodeID) {
      return row;
    }
  }
  return null;
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
    markRunResultStale(false);
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
  await addParameter(componentID, name, value);
}

async function addParameter(componentID, name, value) {
  name = (name || "").trim();
  value = value || "";
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
    markRunResultStale(false);
    renderAll();
    log(`Parameter added: ${componentID}.${name}`);
  } catch (error) {
    log(`Add parameter failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function saveParameterDefinition(componentID, name, row) {
  if (!componentID || !name || !row || !isWorkspaceProject()) return;
  const fields = contractFields(row);
  const value = coerceParameter(fields.value || "");
  const definition = {
    display_name: fields.display || "",
    unit: fields.unit || "",
    role: fields.role || "fixed",
    group: fields.group || "",
    description: fields.description || "",
    current: value,
    visible: fields.visible !== false,
  };
  if ((fields.default || "").trim() !== "") definition.default = coerceParameter(fields.default);
  const min = (fields.min || "").trim();
  const max = (fields.max || "").trim();
  if (min !== "" || max !== "") {
    definition.bounds = {};
    if (min !== "") definition.bounds.min = coerceParameter(min);
    if (max !== "") definition.bounds.max = coerceParameter(max);
  }
  try {
    const body = await api("/api/project/component-contract", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        component_id: componentID,
        parameters: { [name]: value },
        parameter_defs: { [name]: definition },
      }),
    });
    state.detail = body.project;
    markRunResultStale(false);
    renderAll();
    log(`Parameter definition saved: ${componentID}.${name}`);
  } catch (error) {
    log(`Save parameter definition failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteParameterDefinition(componentID, name) {
  if (!componentID || !name || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/component-contract", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        component_id: componentID,
        delete_parameter_defs: [name],
      }),
    });
    state.detail = body.project;
    renderAll();
    log(`Parameter metadata cleared: ${componentID}.${name}`);
  } catch (error) {
    log(`Clear parameter metadata failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function addStateDefinition(componentID, name, initial) {
  name = (name || "").trim();
  if (!componentID || !name || !isWorkspaceProject()) {
    showInlineProblem("Select a component and state name");
    return;
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name)) {
    showInlineProblem("State name must start with a letter or underscore and contain only letters, numbers, and underscores");
    return;
  }
  await saveStateDefinitionPayload(componentID, name, {
    display_name: displayNameFromIdentifier(name),
    initial: (initial || "").trim() === "" ? 0.0 : coerceParameter(initial),
  }, `State definition added: ${componentID}.${name}`);
}

async function saveStateDefinition(componentID, name, row) {
  if (!componentID || !name || !row || !isWorkspaceProject()) return;
  const fields = contractFields(row);
  const definition = {
    display_name: fields.display || "",
    unit: fields.unit || "",
    description: fields.description || "",
  };
  if ((fields.initial || "").trim() !== "") definition.initial = coerceParameter(fields.initial);
  await saveStateDefinitionPayload(componentID, name, definition, `State definition saved: ${componentID}.${name}`);
}

async function saveStateDefinitionPayload(componentID, name, definition, successMessage) {
  try {
    const body = await api("/api/project/component-contract", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        component_id: componentID,
        state_defs: { [name]: definition },
      }),
    });
    state.detail = body.project;
    markRunResultStale(false);
    renderAll();
    log(successMessage);
  } catch (error) {
    log(`Save state definition failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

async function deleteStateDefinition(componentID, name) {
  if (!componentID || !name || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/component-contract", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        component_id: componentID,
        delete_state_defs: [name],
      }),
    });
    state.detail = body.project;
    markRunResultStale(false);
    renderAll();
    log(`State definition deleted: ${componentID}.${name}`);
  } catch (error) {
    log(`Delete state definition failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function contractFields(row) {
  const fields = {};
  for (const input of row.querySelectorAll("[data-contract-field]")) {
    if (input.type === "checkbox") {
      fields[input.dataset.contractField] = input.checked;
    } else {
      fields[input.dataset.contractField] = input.value;
    }
  }
  return fields;
}

function displayNameFromIdentifier(value) {
  return String(value || "")
    .split(/[_\-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function syncParameterInputs(componentID, name, value, source) {
  for (const input of document.querySelectorAll("[data-parameter-component]")) {
    if (input === source) continue;
    if (input.dataset.parameterComponent === componentID && input.dataset.parameterName === name) {
      input.value = value;
    }
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
    markRunResultStale(false);
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
    markRunResultStale(false);
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

function componentOptionLabel(component) {
  return component?.name && component.name !== component.id ? `${component.name} (${component.id})` : component?.id || "";
}

function selectComponent(id) {
  if (!componentById(id)) return;
  hideSourceCompletionPanel();
  state.selectedComponentId = id;
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderProjectTree();
  renderRunWorkspace();
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
  if (!WORKSPACE_HELP[mode]) mode = "canvas";
  document.querySelectorAll(".mode-button").forEach((button) => {
    button.classList.toggle("active", button.dataset.mode === mode);
  });
  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("active", view.id === `${mode}View`);
  });
  updateWorkspaceHelp(mode);
  if (window.location.hash !== `#${mode}`) {
    window.history.replaceState(null, "", `#${mode}`);
  }
}

function updateWorkspaceHelp(mode) {
  const link = el("workspaceHelpLink");
  if (!link) return;
  const href = WORKSPACE_HELP[mode] || "/docs/user/index.md";
  link.href = href;
  link.title = `Open ${displayNameFromIdentifier(mode || "workspace")} help`;
}

function workspaceModeFromHash() {
  const mode = window.location.hash.replace(/^#/, "");
  return WORKSPACE_HELP[mode] ? mode : "canvas";
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
  const runtimeBusy = Boolean(state.activeRunAbortController);
  el("validateButton").disabled = !hasProject;
  el("dataValidateButton").disabled = !hasProject || !(state.detail?.validation_mappings || []).length;
  el("runButton").disabled = !hasProject || runtimeBusy;
  el("seriesButton").disabled = !hasProject || runtimeBusy;
  el("scenarioButton").disabled = !hasProject || !isWorkspaceProject();
  el("batchButton").disabled = !hasProject || !isWorkspaceProject() || runtimeBusy;
  el("cancelRunButton").disabled = !runtimeBusy;
  el("schemaButton").disabled = !hasProject;
  el("serveButton").disabled = true;
  el("exportButton").disabled = !hasProject || !isWorkspaceProject();
  el("saveProjectButton").disabled = !hasProject || !isWorkspaceProject();
  el("copyProjectButton").disabled = !hasProject;
  el("addComponentButton").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("newComponentName").disabled = !hasProject || !isWorkspaceProject();
  el("componentCategorySelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("componentExecutionModeSelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("componentTemplateSelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("includeComponentOnCreate").disabled = !hasProject || !isWorkspaceProject();
  el("autoLayoutButton").disabled = !hasProject || !isWorkspaceProject();
  el("includeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
  el("removeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || !selectedComponentInSystem();
  el("deleteComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
}

function markProjectDirty() {
  markRunResultStale();
  if (isWorkspaceProject()) {
    el("saveProjectButton").classList.add("dirty");
  }
}

function markRunResultStale(render = true) {
  if (!latestRuntimeResult() || state.latestResultStale) return;
  state.latestResultStale = true;
  if (!render) return;
  renderSystemHeader();
  renderCanvas();
  renderInspector();
  renderPythonPanel();
  renderRunWorkspace();
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
  updateSourceHighlight(editor.value);
  updateSourceChrome(component, state.sourceByComponent[component.id], editor.value);
  renderProjectTree();
  markProjectDirty();
}

function handleSourceEditorInput(event) {
  updateSourceDraftFromEditor(event.target);
}

function handleSourceEditorKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.code === "Space") {
    event.preventDefault();
    showSourceCompletionPanel();
    return;
  }
  if (event.key === "Escape") {
    hideSourceCompletionPanel();
    return;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    handleSourceNewline(event.target);
    return;
  }
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
  if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key.toLowerCase() === "f") {
    event.preventDefault();
    formatCurrentSource();
    return;
  }
  if (event.key === "Tab") {
    event.preventDefault();
    handleSourceIndent(event.target, event.shiftKey);
  }
}

function handleSourceNewline(editor) {
  const start = editor.selectionStart ?? 0;
  const end = editor.selectionEnd ?? start;
  const value = editor.value;
  const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  const currentLine = value.slice(lineStart, start);
  const indent = currentLine.match(/^\s*/)?.[0] || "";
  const extra = currentLine.trimEnd().endsWith(":") ? "    " : "";
  const insert = `\n${indent}${extra}`;
  editor.value = `${value.slice(0, start)}${insert}${value.slice(end)}`;
  editor.selectionStart = editor.selectionEnd = start + insert.length;
  updateSourceDraftFromEditor(editor);
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
  const highlight = el("sourceHighlight");
  if (highlight) {
    highlight.scrollTop = event.target.scrollTop;
    highlight.scrollLeft = event.target.scrollLeft;
  }
}

function bindEvents() {
  el("projectSelect").addEventListener("change", (event) => loadProject(event.target.value));
  el("newProjectButton").addEventListener("click", createProject);
  el("copyProjectButton").addEventListener("click", copyProject);
  el("addComponentButton").addEventListener("click", createComponent);
  el("autoLayoutButton").addEventListener("click", autoLayoutCanvas);
  el("newComponentName").addEventListener("keydown", (event) => {
    if (event.key === "Enter") createComponent();
  });
  el("componentCategorySelect").addEventListener("change", renderComponentTemplateSelect);
  el("componentExecutionModeSelect").addEventListener("change", renderComponentTemplateSelect);
  el("componentTemplateSelect").addEventListener("change", renderComponentTemplateMeta);
  el("includeComponentButton").addEventListener("click", includeSelectedComponent);
  el("removeComponentButton").addEventListener("click", removeSelectedComponentFromSystem);
  el("deleteComponentButton").addEventListener("click", deleteSelectedComponent);
  el("saveProjectButton").addEventListener("click", saveProjectEdits);
  el("validateButton").addEventListener("click", validateProject);
  el("dataValidateButton").addEventListener("click", runDataValidation);
  el("runButton").addEventListener("click", runProject);
  el("seriesButton").addEventListener("click", runSeries);
  el("scenarioButton").addEventListener("click", createScenario);
  el("batchButton").addEventListener("click", runBatch);
  el("cancelRunButton").addEventListener("click", cancelActiveRun);
  el("schemaButton").addEventListener("click", exportSchema);
  el("exportButton").addEventListener("click", exportProject);
  el("sourceComponentSelect").addEventListener("change", (event) => selectComponent(event.target.value));
  el("saveSourceButton").addEventListener("click", saveCurrentSource);
  el("saveRunSourceButton").addEventListener("click", runProject);
  el("checkSourceButton").addEventListener("click", checkCurrentSource);
  el("formatSourceButton").addEventListener("click", formatCurrentSource);
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
  window.addEventListener("hashchange", () => setMode(workspaceModeFromHash()));
}

bindEvents();
loadProjects().catch((error) => {
  el("runtimeStatus").textContent = "Runtime error";
  state.latestValidation = { error: error.message };
  renderProblems();
  log(error.message);
});
