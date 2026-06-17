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
const ML_MODEL_FORMATS = ["custom", "pickle", "joblib", "onnx", "torch", "tensorflow"];
const ML_ASSET_FIELDS = [
  ["model_file", "Model File"],
  ["input_scaler_file", "Input Scaler"],
  ["output_scaler_file", "Output Scaler"],
  ["feature_schema_file", "Feature Schema"],
  ["target_schema_file", "Target Schema"],
  ["training_metadata_file", "Training Metadata"],
  ["validation_report_file", "Validation Report"],
];
const UNIT_CONVERSION_PRESETS = [
  ["custom", "Custom", null],
  ["w_to_kw", "W to kW", { factor: 0.001, offset: 0, description: "Convert W to kW." }],
  ["kw_to_w", "kW to W", { factor: 1000, offset: 0, description: "Convert kW to W." }],
  ["degc_to_k", "degC to K", { factor: 1, offset: 273.15, description: "Convert degC to K." }],
  ["kgs_to_kgh", "kg/s to kg/h", { factor: 3600, offset: 0, description: "Convert kg/s to kg/h." }],
  ["fraction_to_percent", "fraction to percent", { factor: 100, offset: 0, description: "Convert fraction to percent." }],
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
  state.validationComparisonBaseline = null;
  state.latestWorkflowRecord = null;
  state.activeParameterSetPath = "";
  state.activeRunInput = null;
  state.activeSeriesInputPath = "";
  state.activeRunAbortController = null;
  state.activeRunLabel = "";
  state.lastRuntimeAction = "";
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
    ["Series Input", activeSeriesInputSummary()?.relative_path || "Current fields preview"],
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
    } else if (kind === "validation_mapping" && item.relative_path) {
      const body = await api(`/api/project/validation-mapping?project_path=${encodeURIComponent(state.currentProjectPath)}&path=${encodeURIComponent(item.relative_path)}`);
      state.latestWorkflowRecord = { kind, artifact: item, mapping: body.mapping };
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
  normalizeSeriesInputSelection();
  container.append(parameterSetField());
  container.append(runTimeoutField());
  container.append(seriesInputField());
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
  const unitState = connectionUnitState(connection);
  return [
    "connection-line",
    state.selectedConnectionId === connection.id ? "selected" : "",
    mediumState.status === "warning" ? "medium-warning" : "",
    mediumState.status === "override" ? "medium-override" : "",
    mediumState.status === "error" ? "medium-mismatch" : "",
    unitState.status === "warning" ? "unit-warning" : "",
    unitState.status === "converted" ? "unit-converted" : "",
    route.backtracking ? "backtracking" : "",
    route.longPath ? "long-path" : "",
    route.fanOffset ? "connection-fan" : "",
  ].filter(Boolean);
}

function connectionMarkerID(connection, mediumState) {
  const unitState = connectionUnitState(connection);
  if (state.selectedConnectionId === connection.id) return "arrow-selected";
  if (mediumState.status === "error") return "arrow-danger";
  if (mediumState.status === "warning" || mediumState.status === "override" || unitState.status === "warning") return "arrow-warning";
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
  const unitState = connectionUnitState(connection);
  const sourceName = mediumState.sourceNode?.name || connection.from.node;
  const targetName = mediumState.targetNode?.name || connection.to.node;
  const status = connectionStatusLabel(connection, mediumState, route, unitState);
  const latestValue = latest.hasValue ? formatValue(latest.value) : "";
  const secondary = [
    mediumState.label,
    unitState.label,
    unitState.valueTypeLabel,
    latestValue ? `value ${latestValue}` : "",
    status,
  ].filter(Boolean).join(" / ");
  const title = [
    connection.id,
    `${connection.from.component}.${connection.from.node} -> ${connection.to.component}.${connection.to.node}`,
    mediumState.label ? `medium ${mediumState.label}` : "",
    unitState.label ? `unit ${unitState.label}` : "",
    unitState.valueTypeLabel ? `value_type ${unitState.valueTypeLabel}` : "",
    unitState.conversionLabel,
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

function connectionUnitState(connection) {
  const sourceNode = canvasEndpointNode(connection.from, "output");
  const targetNode = canvasEndpointNode(connection.to, "input");
  const sourceUnit = sourceNode?.unit || "";
  const targetUnit = targetNode?.unit || "";
  const sourceValueType = sourceNode?.value_type || "";
  const targetValueType = targetNode?.value_type || "";
  const unitMismatch = normalizedUnitLabel(sourceUnit) && normalizedUnitLabel(targetUnit) && normalizedUnitLabel(sourceUnit) !== normalizedUnitLabel(targetUnit);
  const hasConversion = Boolean(connection.unit_conversion);
  const label = sourceUnit || targetUnit
    ? (unitMismatch ? `${sourceUnit || "?"}->${targetUnit || "?"}` : sourceUnit || targetUnit)
    : "";
  const valueTypeLabel = sourceValueType || targetValueType
    ? (sourceValueType && targetValueType && sourceValueType !== targetValueType ? `${sourceValueType}->${targetValueType}` : sourceValueType || targetValueType)
    : "";
  let status = "ok";
  if (hasConversion) status = "converted";
  else if (unitMismatch) status = "warning";
  return {
    sourceNode,
    targetNode,
    sourceUnit,
    targetUnit,
    label,
    valueTypeLabel,
    status,
    conversionLabel: hasConversion ? connectionUnitConversionSummary(connection) : "",
  };
}

function normalizedUnitLabel(value) {
  return String(value || "").trim().toLowerCase();
}

function connectionStatusLabel(connection, mediumState, route, unitState = connectionUnitState(connection)) {
  if (mediumState.status === "error") return "medium mismatch";
  if (mediumState.status === "override") return connection.medium_override_reason ? "override" : "medium override";
  if (mediumState.status === "warning") return "signal warning";
  if (unitState.status === "converted") return "converted";
  if (unitState.status === "warning") return "unit mismatch";
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
  const unitState = connectionUnitState(connection);
  return [
    "connection-label",
    state.selectedConnectionId === connection.id ? "selected" : "",
    mediumState.status === "warning" ? "medium-warning" : "",
    mediumState.status === "override" ? "medium-override" : "",
    mediumState.status === "error" ? "medium-mismatch" : "",
    unitState.status === "warning" ? "unit-warning" : "",
    unitState.status === "converted" ? "unit-converted" : "",
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
  const mlValidationReport = mlValidationReportBlock(component);
  if (mlValidationReport) container.append(mlValidationReport);
  if (component.ml_metadata && isWorkspaceProject()) container.append(mlAssetEditorBlock(component));
  const featureMappingSuggestion = featureMappingSuggestionBlock(component);
  if (featureMappingSuggestion) container.append(featureMappingSuggestion);
  if (isWorkspaceProject()) {
    container.append(componentEditor(component));
    container.append(replacementPreviewBlock(component));
  }
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
  const featurePreview = featurePreviewValue(latestOutputs, latestInputs);
  if (featurePreview) {
    container.append(featurePreviewBlock(featurePreview.title, featurePreview.features));
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

function featurePreviewValue(latestOutputs, latestInputs) {
  if (isPlainObject(latestOutputs?.features)) {
    return { title: runValueTitle("Feature Preview"), features: latestOutputs.features };
  }
  if (isPlainObject(latestInputs?.features)) {
    return { title: runValueTitle("Received Features"), features: latestInputs.features };
  }
  return null;
}

function featurePreviewBlock(title, features) {
  const block = document.createElement("div");
  block.className = "inspector-block feature-preview-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  const table = document.createElement("table");
  table.className = "feature-preview-table";
  table.innerHTML = "<thead><tr><th>Feature</th><th>Value</th></tr></thead>";
  const tbody = document.createElement("tbody");
  const rows = Object.entries(features || {});
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="2" class="empty-cell">No features</td></tr>`;
  } else {
    for (const [name, value] of rows) {
      const row = document.createElement("tr");
      row.innerHTML = `<td>${escapeHTML(name)}</td><td>${escapeHTML(formatValue(value))}</td>`;
      tbody.append(row);
    }
  }
  table.append(tbody);
  block.append(table);
  return block;
}

function isPlainObject(value) {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function componentHasInputNode(component, nodeID) {
  return (component?.nodes?.inputs || []).some((node) => node.id === nodeID);
}

function componentHasOutputNode(component, nodeID) {
  return (component?.nodes?.outputs || []).some((node) => node.id === nodeID);
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

function replacementPreviewBlock(component) {
  const block = document.createElement("div");
  block.className = "inspector-block replacement-preview-block";
  block.innerHTML = `<div class="inspector-title">Replacement Preview</div>`;
  const template = selectedComponentTemplate();
  if (!template) {
    block.append(emptyKVRow("No replacement template selected"));
    return block;
  }
  const preview = replacementPreview(component, template);
  block.append(inspectorKVRow("Template", `${template.name || template.id} (${template.id})`));
  block.append(inspectorKVRow("Contract", `${preview.diff.matchedInputs.length}/${preview.diff.originalInputs.length} inputs, ${preview.diff.matchedOutputs.length}/${preview.diff.originalOutputs.length} outputs, ${preview.diff.matchedParameters.length}/${preview.diff.originalParameters.length} parameters`));
  block.append(inspectorKVRow("Status", preview.problems.length ? `${preview.problems.length} broken mapping${preview.problems.length === 1 ? "" : "s"}` : "Compatible"));
  block.append(replacementMappingTable("Node Mapping", preview.nodeMappings));
  block.append(replacementMappingTable("Parameter Mapping", preview.parameterMappings));
  block.append(replacementDiffSummary(preview.diff));

  const form = document.createElement("div");
  form.className = "connection-form replacement-form";
  const mapLabel = document.createElement("label");
  mapLabel.className = "node-required-toggle contract-toggle";
  const mapParameters = document.createElement("input");
  mapParameters.id = "replacementMapParameters";
  mapParameters.type = "checkbox";
  mapParameters.checked = state.replacementMapParameters !== false;
  mapParameters.setAttribute("aria-label", "Copy same-name parameters");
  mapParameters.addEventListener("change", () => {
    state.replacementMapParameters = mapParameters.checked;
    renderInspector();
  });
  mapLabel.append(mapParameters, document.createTextNode("Copy same-name parameters"));
  const replaceButton = document.createElement("button");
  replaceButton.type = "button";
  replaceButton.textContent = "Replace And Validate";
  replaceButton.disabled = Boolean(preview.problems.length);
  replaceButton.addEventListener("click", replaceSelectedComponent);
  form.append(mapLabel, replaceButton);
  block.append(form);
  return block;
}

function inspectorKVRow(key, value) {
  const row = document.createElement("div");
  row.className = "kv";
  row.innerHTML = `<span class="kv-key">${escapeHTML(key)}</span><span>${escapeHTML(value)}</span>`;
  return row;
}

function replacementMappingTable(title, mappings) {
  const wrap = document.createElement("div");
  wrap.className = "replacement-table-wrap";
  const heading = document.createElement("div");
  heading.className = "replacement-subtitle";
  heading.textContent = title;
  const table = document.createElement("table");
  table.className = "feature-preview-table replacement-preview-table";
  table.innerHTML = "<thead><tr><th>Scope</th><th>From</th><th>To</th><th>Status</th></tr></thead>";
  const tbody = document.createElement("tbody");
  if (!mappings.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No entry-system references</td></tr>`;
  } else {
    for (const mapping of mappings) {
      const row = document.createElement("tr");
      row.className = mapping.status === "missing" ? "replacement-missing" : "";
      row.innerHTML = `
        <td>${escapeHTML(String(mapping.scope || "").replace(/_/g, " "))}</td>
        <td>${escapeHTML(mapping.from || "")}</td>
        <td>${escapeHTML(mapping.to || "")}</td>
        <td>${escapeHTML(mapping.status || "")}</td>
      `;
      if (mapping.detail) row.title = mapping.detail;
      tbody.append(row);
    }
  }
  table.append(tbody);
  wrap.append(heading, table);
  return wrap;
}

function replacementDiffSummary(diff) {
  const wrap = document.createElement("div");
  wrap.className = "replacement-diff";
  const rows = [
    ["Input diff", replacementDiffText(diff.matchedInputs, diff.missingInputs, diff.addedInputs)],
    ["Output diff", replacementDiffText(diff.matchedOutputs, diff.missingOutputs, diff.addedOutputs)],
    ["Parameter diff", replacementDiffText(diff.matchedParameters, diff.missingParameters, diff.addedParameters)],
  ];
  for (const [key, value] of rows) wrap.append(inspectorKVRow(key, value));
  return wrap;
}

function replacementDiffText(matched, missing, added) {
  return [
    `${matched.length} matched`,
    missing.length ? `missing ${missing.join(", ")}` : "",
    added.length ? `new ${added.join(", ")}` : "",
  ].filter(Boolean).join(" / ");
}

function replacementPreview(component, template) {
  const diff = replacementContractDiff(component, template);
  const nodeMappings = replacementNodeMappings(component, template);
  const parameterMappings = replacementParameterMappings(component, template, state.replacementMapParameters !== false);
  const problems = nodeMappings
    .filter((mapping) => mapping.status === "missing")
    .map((mapping) => ({
      severity: "error",
      component_id: component.id,
      node_id: mapping.node_id || "",
      message: `replacement missing ${mapping.direction || "node"} for ${String(mapping.scope || "").replace(/_/g, " ")} ${mapping.id}: ${mapping.node_id || mapping.from}`,
    }));
  return { diff, nodeMappings, parameterMappings, problems };
}

function replacementNodeMappings(component, template) {
  const system = currentSystem();
  if (!system) return [];
  const templateInputs = new Set(contractNodeIDs(template.inputs || []));
  const templateOutputs = new Set(contractNodeIDs(template.outputs || []));
  const mappings = [];
  for (const input of system.public_inputs || []) {
    if (input.component !== component.id) continue;
    const found = templateInputs.has(input.node);
    mappings.push(replacementMapping("public_input", input.id, component.id, template.id, input.node, "input", found, found ? "public input preserved" : "replacement input node is missing"));
  }
  for (const output of system.public_outputs || []) {
    if (output.component !== component.id) continue;
    const found = templateOutputs.has(output.node);
    mappings.push(replacementMapping("public_output", output.id, component.id, template.id, output.node, "output", found, found ? "public output preserved" : "replacement output node is missing"));
  }
  for (const connectionID of system.connections || []) {
    const connection = (state.detail?.graph?.connections || []).find((item) => item.id === connectionID);
    if (!connection) continue;
    if (connection.from?.component === component.id) {
      const nodeID = connection.from.node;
      const found = templateOutputs.has(nodeID);
      mappings.push(replacementMapping("connection_output", connection.id, component.id, template.id, nodeID, "output", found, found ? "connection source preserved" : "replacement output node is missing"));
    }
    if (connection.to?.component === component.id) {
      const nodeID = connection.to.node;
      const found = templateInputs.has(nodeID);
      mappings.push(replacementMapping("connection_input", connection.id, component.id, template.id, nodeID, "input", found, found ? "connection target preserved" : "replacement input node is missing"));
    }
  }
  return mappings;
}

function replacementMapping(scope, id, sourceComponent, replacementTemplate, nodeID, direction, found, detail) {
  return {
    scope,
    id,
    node_id: nodeID,
    direction,
    from: `${sourceComponent}.${nodeID}`,
    to: `${replacementTemplate}.${nodeID}`,
    status: found ? "preserved" : "missing",
    detail,
  };
}

function replacementParameterMappings(component, template, mapParameters) {
  return contractParameterIDs(template).map((name) => {
    const found = contractParameterIDs(component).includes(name);
    return {
      scope: "parameter",
      id: name,
      from: `${component.id}.${name}`,
      to: `${template.id}.${name}`,
      status: mapParameters ? (found ? "copied" : "missing") : "skipped",
      detail: mapParameters ? (found ? "same-name parameter value copied" : "source parameter is not present") : "parameter mapping disabled",
    };
  });
}

function replacementContractDiff(component, template) {
  const originalInputs = contractNodeIDs(component.nodes?.inputs || []);
  const replacementInputs = contractNodeIDs(template.inputs || []);
  const originalOutputs = contractNodeIDs(component.nodes?.outputs || []);
  const replacementOutputs = contractNodeIDs(template.outputs || []);
  const originalParameters = contractParameterIDs(component);
  const replacementParameters = contractParameterIDs(template);
  return {
    originalInputs,
    replacementInputs,
    matchedInputs: intersectLists(originalInputs, replacementInputs),
    missingInputs: differenceLists(originalInputs, replacementInputs),
    addedInputs: differenceLists(replacementInputs, originalInputs),
    originalOutputs,
    replacementOutputs,
    matchedOutputs: intersectLists(originalOutputs, replacementOutputs),
    missingOutputs: differenceLists(originalOutputs, replacementOutputs),
    addedOutputs: differenceLists(replacementOutputs, originalOutputs),
    originalParameters,
    replacementParameters,
    matchedParameters: intersectLists(originalParameters, replacementParameters),
    missingParameters: differenceLists(originalParameters, replacementParameters),
    addedParameters: differenceLists(replacementParameters, originalParameters),
  };
}

function contractNodeIDs(nodes) {
  return [...new Set((nodes || []).map((node) => node.id).filter(Boolean))].sort();
}

function contractParameterIDs(contract) {
  return [...new Set([
    ...Object.keys(contract.parameters || {}),
    ...Object.keys(contract.parameter_defs || {}),
  ].filter(Boolean))].sort();
}

function intersectLists(left, right) {
  const rightSet = new Set(right);
  return left.filter((item) => rightSet.has(item));
}

function differenceLists(left, right) {
  const rightSet = new Set(right);
  return left.filter((item) => !rightSet.has(item));
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
      const unitState = connectionUnitState(connectionRow.connection);
      const mediumValue = connectionMediumBadge(connectionRow.connection);
      const conversionValue = connectionRow.connection.unit_conversion
        ? `<span class="connection-flow converted">${escapeHTML(connectionUnitConversionSummary(connectionRow.connection))}</span>`
        : (unitState.status === "warning" ? `<span class="connection-flow warning">unit mismatch</span>` : "");
      const rowEl = document.createElement("div");
      rowEl.className = `kv connection-row ${connectionRow.id === state.selectedConnectionId ? "selected" : ""}`;
      rowEl.innerHTML = `
        <span class="kv-key">${escapeHTML(connectionRow.key)}</span>
        <span class="connection-value">
          <span>${escapeHTML(connectionRow.value)}</span>
          ${mediumValue}
          ${conversionValue}
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

  const selectedConnection = selectedConnectionForInspector(targetComponent.id);
  if (selectedConnection && canEditConnections) {
    block.append(connectionUnitConversionEditor(selectedConnection));
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

function connectionMediumBadge(connection) {
  const mediumState = connectionMediumState(connection);
  if (!mediumState.label && mediumState.status === "ok") return "";
  const classes = ["connection-flow", "medium-state"];
  let label = mediumState.label || "medium";
  if (mediumState.status === "error") {
    classes.push("error");
    label = `medium mismatch ${label}`;
  } else if (mediumState.status === "override") {
    classes.push("warning");
    label = `override ${label}`;
  } else if (mediumState.status === "warning") {
    classes.push("warning");
    label = `signal warning ${label}`;
  }
  const title = [
    mediumState.sourceMedium ? `source ${mediumState.sourceMedium}` : "",
    mediumState.targetMedium ? `target ${mediumState.targetMedium}` : "",
    connection.medium_override_reason || "",
  ].filter(Boolean).join(" / ");
  return `<span class="${classes.join(" ")}"${title ? ` title="${escapeAttr(title)}"` : ""}>${escapeHTML(label)}</span>`;
}

function selectedConnectionForInspector(componentID) {
  if (!state.selectedConnectionId) return null;
  const connection = state.detail?.graph?.connections?.find((item) => item.id === state.selectedConnectionId);
  if (!connection) return null;
  if (connection.from.component !== componentID && connection.to.component !== componentID) return null;
  return connection;
}

function connectionUnitConversionEditor(connection) {
  const wrapper = document.createElement("div");
  wrapper.className = "connection-conversion-editor";
  const unitState = connectionUnitState(connection);
  const conversion = connection.unit_conversion || null;
  const presetID = connectionPresetID(connection, conversion);

  const header = document.createElement("div");
  header.className = "connection-conversion-header";
  header.innerHTML = `
    <span>Unit Conversion</span>
    <span>${escapeHTML(unitState.label || "same unit")}</span>
  `;

  const form = document.createElement("div");
  form.className = "connection-conversion-form";

  const preset = document.createElement("select");
  preset.id = "connectionUnitConversionPreset";
  for (const [id, label] of UNIT_CONVERSION_PRESETS) {
    const option = document.createElement("option");
    option.value = id;
    option.textContent = label;
    preset.append(option);
  }
  preset.value = presetID;

  const factor = document.createElement("input");
  factor.id = "connectionUnitConversionFactor";
  factor.type = "number";
  factor.step = "any";
  factor.value = String(unitConversionInitialNumber(conversion, presetID, "factor", 1));
  factor.placeholder = "Factor";

  const offset = document.createElement("input");
  offset.id = "connectionUnitConversionOffset";
  offset.type = "number";
  offset.step = "any";
  offset.value = String(unitConversionInitialNumber(conversion, presetID, "offset", 0));
  offset.placeholder = "Offset";

  const sample = document.createElement("input");
  sample.id = "connectionUnitConversionSample";
  sample.type = "number";
  sample.step = "any";
  sample.value = "1";
  sample.placeholder = "Sample";

  const description = document.createElement("input");
  description.id = "connectionUnitConversionDescription";
  description.value = conversion?.description || presetDefinition(presetID)?.description || "";
  description.placeholder = "Description";

  const preview = document.createElement("div");
  preview.id = "connectionUnitConversionPreview";
  preview.className = "connection-conversion-preview";

  const save = document.createElement("button");
  save.type = "button";
  save.id = "saveConnectionUnitConversionButton";
  save.textContent = "Save Conversion";
  save.addEventListener("click", () => {
    const parsedFactor = finiteNumberOrDefault(factor.value, 1);
    const parsedOffset = finiteNumberOrDefault(offset.value, 0);
    if (!Number.isFinite(parsedFactor) || parsedFactor === 0) {
      showInlineProblem("Conversion factor must be a non-zero number");
      return;
    }
    if (!Number.isFinite(parsedOffset)) {
      showInlineProblem("Conversion offset must be numeric");
      return;
    }
    updateConnectionUnitConversion(connection.id, {
      mode: "linear",
      factor: parsedFactor,
      offset: parsedOffset,
      description: description.value.trim(),
    });
  });

  const clear = document.createElement("button");
  clear.type = "button";
  clear.id = "clearConnectionUnitConversionButton";
  clear.className = "ghost";
  clear.textContent = "Clear";
  clear.addEventListener("click", () => updateConnectionUnitConversion(connection.id, null));

  const updatePreview = () => {
    const parsedFactor = finiteNumberOrDefault(factor.value, 1);
    const parsedOffset = finiteNumberOrDefault(offset.value, 0);
    const sampleValue = finiteNumberOrDefault(sample.value, 1);
    if (!Number.isFinite(parsedFactor) || !Number.isFinite(parsedOffset) || !Number.isFinite(sampleValue) || parsedFactor === 0) {
      preview.textContent = "Invalid conversion";
      preview.className = "connection-conversion-preview invalid";
      return;
    }
    const converted = sampleValue * parsedFactor + parsedOffset;
    const units = [unitState.sourceUnit, unitState.targetUnit].filter(Boolean).join(" to ");
    preview.textContent = `${formatValue(sampleValue)}${unitState.sourceUnit ? ` ${unitState.sourceUnit}` : ""} = ${formatValue(converted)}${unitState.targetUnit ? ` ${unitState.targetUnit}` : ""}${units ? ` / ${units}` : ""}`;
    preview.className = "connection-conversion-preview";
  };

  preset.addEventListener("change", () => {
    const definition = presetDefinition(preset.value);
    if (!definition) {
      updatePreview();
      return;
    }
    factor.value = String(definition.factor);
    offset.value = String(definition.offset);
    description.value = definition.description || "";
    updatePreview();
  });
  [factor, offset, sample, description].forEach((input) => input.addEventListener("input", updatePreview));
  form.append(preset, factor, offset, sample, description, preview, save, clear);
  wrapper.append(header, form);
  updatePreview();
  return wrapper;
}

function presetDefinition(presetID) {
  return UNIT_CONVERSION_PRESETS.find(([id]) => id === presetID)?.[2] || null;
}

function connectionPresetID(connection, conversion) {
  if (conversion) {
    const factor = Number(conversion.factor ?? 1);
    const offset = Number(conversion.offset ?? 0);
    const match = UNIT_CONVERSION_PRESETS.find(([, , definition]) => (
      definition && approximatelyEqual(definition.factor, factor) && approximatelyEqual(definition.offset, offset)
    ));
    return match?.[0] || "custom";
  }
  const unitState = connectionUnitState(connection);
  const sourceUnit = normalizedUnitLabel(unitState.sourceUnit);
  const targetUnit = normalizedUnitLabel(unitState.targetUnit);
  if (sourceUnit === "w" && targetUnit === "kw") return "w_to_kw";
  if (sourceUnit === "kw" && targetUnit === "w") return "kw_to_w";
  if (sourceUnit === "degc" && targetUnit === "k") return "degc_to_k";
  if (sourceUnit === "kg/s" && targetUnit === "kg/h") return "kgs_to_kgh";
  if (sourceUnit === "fraction" && targetUnit === "percent") return "fraction_to_percent";
  return "custom";
}

function unitConversionInitialNumber(conversion, presetID, key, fallback) {
  if (conversion && conversion[key] !== undefined && conversion[key] !== null) return Number(conversion[key]);
  const preset = presetDefinition(presetID);
  if (preset && preset[key] !== undefined) return preset[key];
  return fallback;
}

function finiteNumberOrDefault(value, fallback) {
  const text = String(value ?? "").trim();
  if (text === "") return fallback;
  const parsed = Number(text);
  return Number.isFinite(parsed) ? parsed : Number.NaN;
}

function approximatelyEqual(a, b) {
  return Math.abs(Number(a) - Number(b)) < 1e-12;
}

function connectionUnitConversionSummary(connection) {
  const conversion = connection.unit_conversion;
  if (!conversion) return "";
  const unitState = connectionUnitState(connection);
  const factor = Number(conversion.factor ?? 1);
  const offset = Number(conversion.offset ?? 0);
  const offsetLabel = offset === 0 ? "" : (offset > 0 ? ` + ${formatValue(offset)}` : ` - ${formatValue(Math.abs(offset))}`);
  return [
    unitState.label ? `${unitState.label}` : "converted",
    `x ${formatValue(factor)}${offsetLabel}`,
  ].filter(Boolean).join(" ");
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

function mlValidationReportBlock(component) {
  const report = state.detail?.ml_validation_reports?.[component.id];
  if (!report) return null;
  const rows = [
    ["Dataset", report.dataset || ""],
    ["Report", report.report_path || ""],
    ["Feature Schema", report.feature_schema_version || ""],
    ["Model SHA256", report.model_asset_checksum || ""],
    ["Training Period", report.training_period || ""],
    ["Validation Period", report.validation_period || ""],
    ["Time Resolution", report.time_resolution || ""],
  ].filter(([, value]) => value);
  const block = inspectorBlock("ML Validation", rows);
  const metricRows = [];
  for (const [target, metrics] of Object.entries(report.metrics || {})) {
    for (const [metric, value] of Object.entries(metrics || {})) {
      metricRows.push([target, metric, formatValue(value)]);
    }
  }
  if (metricRows.length) {
    const table = document.createElement("table");
    table.className = "feature-preview-table";
    table.innerHTML = "<thead><tr><th>Target</th><th>Metric</th><th>Value</th></tr></thead>";
    const tbody = document.createElement("tbody");
    for (const rowValues of metricRows) {
      const row = document.createElement("tr");
      row.innerHTML = rowValues.map((value) => `<td>${escapeHTML(value)}</td>`).join("");
      tbody.append(row);
    }
    table.append(tbody);
    block.append(table);
  }
  return block;
}

function mlAssetEditorBlock(component) {
  const metadata = component.ml_metadata || {};
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">ML Assets</div>`;

  const form = document.createElement("div");
  form.className = "connection-form ml-asset-form";

  const format = document.createElement("select");
  format.dataset.mlMetadataField = "model_format";
  format.setAttribute("aria-label", "Model format");
  for (const value of ML_MODEL_FORMATS) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    format.append(option);
  }
  format.value = metadata.model_format || "custom";

  const packages = document.createElement("input");
  packages.dataset.mlMetadataField = "required_packages";
  packages.placeholder = "required packages";
  packages.value = (metadata.required_packages || []).join(", ");
  packages.setAttribute("aria-label", "Required packages");

  const resolution = document.createElement("input");
  resolution.dataset.mlMetadataField = "valid_time_resolution";
  resolution.placeholder = "time resolution";
  resolution.value = metadata.valid_time_resolution || "";
  resolution.setAttribute("aria-label", "Valid time resolution");

  form.append(format, packages, resolution);

  for (const [field, label] of ML_ASSET_FIELDS) {
    const row = document.createElement("div");
    row.className = "ml-asset-row";
    row.dataset.mlAssetField = field;
    const caption = document.createElement("span");
    caption.textContent = label;
    const file = document.createElement("input");
    file.type = "file";
    file.setAttribute("aria-label", label);
    row.append(caption, file);
    form.append(row);
  }

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Save ML Assets";
  button.addEventListener("click", () => updateMLAssetsFromInspector(component.id, block));
  const schemaButton = document.createElement("button");
  schemaButton.type = "button";
  schemaButton.textContent = "Apply Schema Nodes";
  schemaButton.addEventListener("click", () => applyMLSchemaNodes(component.id));
  form.append(button, schemaButton);
  block.append(form);
  return block;
}

function featureMappingSuggestionBlock(targetComponent) {
  const suggestions = featureMappingSuggestions(targetComponent);
  if (!suggestions.length) return null;
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Feature Mapping Suggestion</div>`;
  const form = document.createElement("div");
  form.className = "connection-form";
  const select = document.createElement("select");
  select.setAttribute("aria-label", "Feature mapper source");
  for (const suggestion of suggestions) {
    const option = document.createElement("option");
    option.value = `${suggestion.component}.${suggestion.node}`;
    option.textContent = `${suggestion.component}.${suggestion.node}`;
    select.append(option);
  }
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Connect Feature Mapper";
  button.addEventListener("click", () => createConnectionFromInspector(select.value, targetComponent.id, "features"));
  form.append(select, button);
  block.append(form);
  return block;
}

function featureMappingSuggestions(targetComponent) {
  if (!targetComponent?.ml_metadata || !isWorkspaceProject() || !componentHasInputNode(targetComponent, "features")) return [];
  const system = currentSystem();
  if (!system || !(system.components || []).includes(targetComponent.id)) return [];
  const graphConnections = state.detail?.graph?.connections || [];
  const systemConnections = (system.connections || []).map((id) => graphConnections.find((connection) => connection.id === id)).filter(Boolean);
  if (systemConnections.some((connection) => connection.to.component === targetComponent.id && connection.to.node === "features")) return [];
  return (system.components || [])
    .map(componentById)
    .filter((component) => component && component.id !== targetComponent.id && componentHasOutputNode(component, "features"))
    .sort((left, right) => featureMapperRank(left) - featureMapperRank(right))
    .map((component) => ({ component: component.id, node: "features" }));
}

function featureMapperRank(component) {
  const id = component?.id || "";
  const name = component?.name || "";
  return id.includes("feature_mapper") || name.toLowerCase().includes("feature mapper") ? 0 : 1;
}

async function updateMLAssetsFromInspector(componentID, block) {
  if (!componentID || !isWorkspaceProject()) return;
  const assets = [];
  for (const row of block.querySelectorAll("[data-ml-asset-field]")) {
    const file = row.querySelector("input[type='file']")?.files?.[0];
    if (!file) continue;
    assets.push({
      field: row.dataset.mlAssetField,
      file_name: file.name,
      content_base64: await fileToBase64(file),
    });
  }
  const packagesValue = block.querySelector("[data-ml-metadata-field='required_packages']")?.value || "";
  try {
    const body = await api("/api/project/components/ml-assets", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        component_id: componentID,
        model_format: block.querySelector("[data-ml-metadata-field='model_format']")?.value || "custom",
        required_packages: splitRequiredPackages(packagesValue),
        valid_time_resolution: block.querySelector("[data-ml-metadata-field='valid_time_resolution']")?.value || "",
        assets,
      }),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    markRunResultStale(false);
    renderAll();
    log(`ML assets updated: ${componentID} files=${(body.imported_files || []).length}`);
  } catch (error) {
    log(`Update ML assets failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
  }
}

function splitRequiredPackages(value) {
  return String(value || "")
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => {
      const value = String(reader.result || "");
      const comma = value.indexOf(",");
      resolve(comma >= 0 ? value.slice(comma + 1) : value);
    });
    reader.addEventListener("error", () => reject(reader.error || new Error("File read failed")));
    reader.readAsDataURL(file);
  });
}

async function applyMLSchemaNodes(componentID) {
  if (!componentID || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/components/ml-schema-nodes", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: componentID }),
    });
    state.detail = body.project;
    state.selectedComponentId = componentID;
    markRunResultStale(false);
    renderAll();
    const summary = body.summary || {};
    log(`ML schema nodes applied: ${componentID} inputs=${(summary.added_inputs || []).length} outputs=${(summary.added_outputs || []).length}`);
  } catch (error) {
    log(`Apply ML schema nodes failed: ${error.message}`);
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
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
  const panel = el("logsPanel");
  if (!panel) return;
  panel.innerHTML = "";
  const controls = document.createElement("div");
  controls.className = "log-controls";

  const severity = document.createElement("select");
  severity.id = "logSeverityFilter";
  for (const [value, label] of [["all", "All"], ["app", "Studio"], ["info", "Info"], ["warning", "Warning"], ["error", "Error"]]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    severity.append(option);
  }
  severity.value = state.logSeverityFilter || "all";

  const search = document.createElement("input");
  search.id = "logTextFilter";
  search.type = "search";
  search.placeholder = "Filter logs";
  search.value = state.logTextFilter || "";

  const exportButton = document.createElement("button");
  exportButton.type = "button";
  exportButton.id = "exportLogBundleButton";
  exportButton.textContent = "Export Logs";
  exportButton.addEventListener("click", downloadLogBundle);

  const rows = document.createElement("div");
  rows.className = "log-rows";
  const updateRows = () => renderLogRows(rows);
  severity.addEventListener("change", () => {
    state.logSeverityFilter = severity.value;
    updateRows();
  });
  search.addEventListener("input", () => {
    state.logTextFilter = search.value;
    updateRows();
  });
  controls.append(severity, search, exportButton);
  panel.append(controls, rows);
  updateRows();
}

function renderLogRows(container) {
  const rows = filteredLogRows();
  container.innerHTML = "";
  if (!rows.length) {
    const empty = document.createElement("div");
    empty.className = "log-empty";
    empty.textContent = "No logs match the current filter";
    container.append(empty);
    return;
  }
  for (const item of rows) {
    const row = document.createElement("div");
    row.className = `log-row ${logSeverityClassName(item.severity)}`;
    const time = item.time !== undefined && item.time !== null && item.time !== "" ? `time ${formatValue(item.time)}` : "";
    row.innerHTML = `
      <span class="log-row-meta">${escapeHTML([item.source, item.component, item.stage, item.stream, time, item.location].filter(Boolean).join(" / "))}</span>
      <span class="log-row-severity">${escapeHTML(item.severity || "info")}</span>
      <span class="log-row-message">${escapeHTML(item.message || "")}</span>
    `;
    container.append(row);
  }
}

function filteredLogRows() {
  const severity = state.logSeverityFilter || "all";
  const needle = String(state.logTextFilter || "").trim().toLowerCase();
  return combinedLogRows().filter((item) => {
    if (severity !== "all" && item.severity !== severity && item.source !== severity) return false;
    if (!needle) return true;
    return [item.source, item.component, item.stage, item.stream, item.severity, item.time, item.location, item.message]
      .filter(Boolean)
      .join(" ")
      .toLowerCase()
      .includes(needle);
  });
}

function combinedLogRows() {
  const appLogs = (state.logs || []).map((message) => ({
    source: "app",
    severity: "app",
    message,
  }));
  const runtimeLogs = (latestRuntimeResult()?.component_logs || []).map((entry) => ({
    source: "runtime",
    component: entry.component || "",
    stage: entry.stage || "",
    stream: entry.stream || "",
    severity: String(entry.severity || "info").toLowerCase(),
    time: entry.time ?? "",
    location: logSourceLocation(entry),
    message: entry.message || "",
  }));
  return [...runtimeLogs, ...appLogs];
}

function logSourceLocation(entry) {
  const source = entry?.source || "";
  const line = entry?.line ? `:${entry.line}${entry.column ? `:${entry.column}` : ""}` : "";
  return `${source}${line}`;
}

function logSeverityClassName(severity) {
  const normalized = String(severity || "info").toLowerCase();
  if (normalized === "error" || normalized === "warning" || normalized === "info" || normalized === "app") return normalized;
  return "info";
}

function downloadLogBundle() {
  const project = state.detail?.project?.project_name || "hvac-studio";
  const bundle = {
    project: state.detail?.project?.project_name || "",
    project_path: state.currentProjectPath || "",
    active_component: state.selectedComponentId || "",
    latest_result_source: state.latestRunSource || "",
    filters: {
      severity: state.logSeverityFilter || "all",
      text: state.logTextFilter || "",
    },
    logs: combinedLogRows(),
  };
  downloadTextFile(`${safeFileName(project)}-logs.json`, `${JSON.stringify(bundle, null, 2)}\n`, "application/json;charset=utf-8");
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
  if (value.kind === "high_error_inspection") {
    wrapper.append(resultHeader("High Error Inspection", value.metric || "", `row ${value.row_index ?? ""}`, "/docs/user/data-validation.md"));
    wrapper.append(highErrorInspectionSection(value));
    return wrapper;
  }
  if (value.kind === "calibration_validation_comparison") {
    wrapper.append(resultHeader("Calibration Validation", value.calibration_result?.setup_name || value.calibration_result?.setup_id || "", "before / after", "/docs/user/calibration.md"));
    wrapper.append(calibrationValidationComparisonSection(value));
    return wrapper;
  }
  if (value.kind === "validation_mapping" && value.artifact) {
    wrapper.append(resultHeader("Validation Mapping", value.artifact.relative_path || value.artifact.path || "", `${value.artifact.input_count || 0} in / ${value.artifact.output_count || 0} out`, "/docs/user/data-validation.md"));
    wrapper.append(validationMappingArtifactSection(value.artifact, value.mapping));
    return wrapper;
  }
  if (value.kind === "calibration_setup_editor") {
    wrapper.append(resultHeader("Calibration Setup", value.mapping_summary?.relative_path || "", `${(value.candidates || []).length} candidates`, "/docs/user/calibration.md"));
    wrapper.append(calibrationSetupEditorSection(value));
    return wrapper;
  }
  if (value.kind === "optimization_setup_editor") {
    wrapper.append(resultHeader("Optimization Setup", currentSystem()?.id || "", `${(value.candidates || []).length} variables`, "/docs/user/optimization.md"));
    wrapper.append(optimizationSetupEditorSection(value));
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
    wrapper.append(seriesResultSection(series));
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
    ["SHA256", dataset.summary?.sha256 || ""],
  ]));
  section.append(resultTable("Column Profiles", (dataset.column_profiles || []).map((item) => [
    item.column || "",
    item.value_type || "",
    String(item.missing_count || 0),
    (item.samples || []).join(", "),
  ]), ["Column", "Type", "Missing", "Samples"]));
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
        missing_value_policy: el("validationMissingPolicySelect").value || "error",
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

function validationMappingArtifactSection(summary, mapping = null) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Summary", [
    ["Name", summary.name || summary.id || ""],
    ["Path", summary.relative_path || ""],
    ["Dataset", summary.dataset || mapping?.dataset || ""],
    ["Dataset SHA256", summary.dataset_checksum || mapping?.dataset_checksum || ""],
    ["Inputs", String(summary.input_count || Object.keys(mapping?.input_columns || {}).length || 0)],
    ["Observed Outputs", String(summary.output_count || Object.keys(mapping?.observed_output_columns || {}).length || 0)],
    ["Missing Value Policy", summary.missing_value_policy || mapping?.missing_value_policy || ""],
  ]));
  if (mapping?.input_columns) {
    section.append(resultTable("Input Columns", Object.entries(mapping.input_columns).map(([publicID, column]) => [publicID, column]), ["Public Input", "Dataset Column"]));
  }
  if (mapping?.observed_output_columns) {
    section.append(resultTable("Observed Output Columns", Object.entries(mapping.observed_output_columns).map(([publicID, column]) => [publicID, column]), ["Public Output", "Dataset Column"]));
  }
  if (isWorkspaceProject()) {
    const actions = document.createElement("div");
    actions.className = "result-actions mapping-actions";
    const nameInput = document.createElement("input");
    nameInput.type = "text";
    nameInput.className = "mapping-name-input";
    nameInput.value = summary.name || summary.id || "";
    nameInput.placeholder = "Mapping name";
    const save = document.createElement("button");
    save.type = "button";
    save.className = "small-action";
    save.textContent = "Save Name";
    save.addEventListener("click", () => renameValidationMapping(summary, nameInput.value));
    const copy = document.createElement("button");
    copy.type = "button";
    copy.className = "small-action";
    copy.textContent = "Copy";
    copy.addEventListener("click", () => copyValidationMapping(summary));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "small-action danger-action";
    remove.textContent = "Delete";
    remove.addEventListener("click", () => deleteValidationMapping(summary));
    actions.append(nameInput, save, copy, remove);
    section.append(actions);
  }
  return section;
}

async function renameValidationMapping(summary, name) {
  try {
    const body = await api("/api/project/validation-mapping/update", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: summary.relative_path || summary.path || "",
        name,
      }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "validation_mapping", artifact: body.summary, mapping: body.mapping };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    log(`Validation mapping renamed: ${body.summary?.relative_path || body.summary?.id}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Validation mapping rename failed: ${error.message}`);
  }
}

async function copyValidationMapping(summary) {
  try {
    const body = await api("/api/project/validation-mapping/copy", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: summary.relative_path || summary.path || "",
      }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "validation_mapping", artifact: body.summary, mapping: body.mapping };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    log(`Validation mapping copied: ${body.summary?.relative_path || body.summary?.id}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Validation mapping copy failed: ${error.message}`);
  }
}

async function deleteValidationMapping(summary) {
  const path = summary.relative_path || summary.path || "";
  if (!path || !window.confirm(`Delete validation mapping ${path}?`)) return;
  try {
    const body = await api("/api/project/validation-mapping/delete", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: path,
      }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "validation_mapping_deleted", artifact: { relative_path: body.mapping_path || path, state: "deleted" } };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    log(`Validation mapping deleted: ${body.mapping_path || path}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Validation mapping delete failed: ${error.message}`);
  }
}

async function importDataset() {
  if (!isWorkspaceProject()) return;
  const sourcePath = el("datasetSourcePathInput").value.trim();
  if (!sourcePath) {
    state.latestValidation = { error: "CSV source path is required" };
    renderProblems();
    setBottomTab("problems");
    log("Dataset import unavailable: missing CSV path");
    return;
  }
  try {
    const body = await api("/api/project/datasets/import", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        source_path: sourcePath,
        id: el("datasetIDInput").value.trim(),
        delimiter: el("datasetDelimiterSelect").value,
        encoding: "utf-8",
      }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "dataset", dataset: body.dataset };
    el("datasetSourcePathInput").value = "";
    el("datasetIDInput").value = "";
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    setMode("artifacts");
    setBottomTab("results");
    log(`Dataset imported: ${body.summary?.relative_path || body.dataset?.summary?.relative_path || ""}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Dataset import failed: ${error.message}`);
  }
}

async function openCalibrationSetupEditor(mappingPath = "") {
  const mappings = state.detail?.validation_mappings || [];
  const mapping = mappings.find((item) => item.relative_path === mappingPath) || mappings[0];
  if (!mapping) {
    state.latestValidation = { error: "No validation mapping is available for this project" };
    renderProblems();
    setBottomTab("problems");
    log("Calibration setup unavailable: no mapping");
    return;
  }
  try {
    const body = await api(`/api/project/validation-mapping?project_path=${encodeURIComponent(state.currentProjectPath)}&path=${encodeURIComponent(mapping.relative_path)}`);
    state.latestWorkflowRecord = {
      kind: "calibration_setup_editor",
      mapping_summary: mapping,
      mapping: body.mapping,
      candidates: calibrationParameterCandidates(),
    };
    renderResults();
    setMode("artifacts");
    setBottomTab("results");
    log(`Calibration setup editor opened: ${mapping.relative_path}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Calibration setup editor failed: ${error.message}`);
  }
}

async function createCalibrationSetup(payload) {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/project/calibration-setup", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, ...payload }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "calibration_setup", artifact: body.summary, setup: body.setup };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    setMode("artifacts");
    setBottomTab("results");
    log(`Calibration setup created: ${body.summary?.relative_path || body.summary?.id}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Calibration setup failed: ${error.message}`);
  }
}

function calibrationParameterCandidates() {
  const candidates = [];
  for (const component of state.detail?.graph?.components || []) {
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

function defaultCalibrationGridStep(min, max) {
  const step = (Number(max) - Number(min)) / 4;
  if (!Number.isFinite(step) || step <= 0) return "1";
  return String(Math.round(step * 1e9) / 1e9);
}

function calibrationSetupEditorSection(context) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(calibrationSetupControls(context));
  section.append(calibrationObjectiveOutputEditor(context.mapping));
  section.append(calibrationCandidateEditor(context.candidates || []));
  const actions = document.createElement("div");
  actions.className = "result-actions calibration-setup-actions";
  const runCount = document.createElement("span");
  runCount.className = "input-meta calibration-run-count";
  const create = document.createElement("button");
  create.type = "button";
  create.className = "small-action";
  create.textContent = "Create Setup";
  create.addEventListener("click", () => {
    const payload = collectCalibrationSetupEditorPayload(section, context);
    if (payload) createCalibrationSetup(payload);
  });
  actions.append(runCount, create);
  section.append(actions);
  section.querySelectorAll("[data-calibration-filter], [data-cal-param-check], [data-cal-param-field], [data-cal-output-check], [data-cal-output-weight]").forEach((control) => {
    control.addEventListener("input", () => updateCalibrationEditorState(section));
    control.addEventListener("change", () => updateCalibrationEditorState(section));
  });
  updateCalibrationEditorState(section);
  return section;
}

function calibrationSetupControls(context) {
  const block = document.createElement("div");
  block.className = "result-block calibration-editor-block";
  block.innerHTML = `<div class="result-block-title">Setup</div>`;
  const controls = document.createElement("div");
  controls.className = "calibration-editor-grid";
  const mappingSelect = document.createElement("select");
  mappingSelect.dataset.calibrationMapping = "true";
  for (const item of state.detail?.validation_mappings || []) {
    mappingSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  mappingSelect.value = context.mapping_summary?.relative_path || "";
  mappingSelect.addEventListener("change", () => openCalibrationSetupEditor(mappingSelect.value));
  const baseSelect = document.createElement("select");
  baseSelect.dataset.calibrationBase = "true";
  baseSelect.append(new Option("Baseline", ""));
  for (const item of state.detail?.parameter_sets || []) {
    baseSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  baseSelect.value = state.activeParameterSetPath || "";
  const algorithmSelect = document.createElement("select");
  algorithmSelect.dataset.calibrationAlgorithm = "true";
  algorithmSelect.append(new Option("Grid Search", "grid"));
  algorithmSelect.append(new Option("Least Squares", "least_squares"));
  controls.append(
    labeledEditorControl("Mapping", mappingSelect),
    labeledEditorControl("Base Parameter Set", baseSelect),
    labeledEditorControl("Algorithm", algorithmSelect),
    labeledEditorInput("Setup ID", "text", "auto", "calibration-setup-id"),
    labeledEditorInput("Setup Name", "text", "auto", "calibration-setup-name"),
  );
  block.append(controls);
  return block;
}

function calibrationObjectiveOutputEditor(mapping) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `
    <div class="result-block-title">Target Outputs</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Output</th><th>Dataset Column</th><th>Weight</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  const outputs = Object.entries(mapping?.observed_output_columns || {});
  if (!outputs.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No observed outputs</td></tr>`;
    return block;
  }
  for (const [output, column] of outputs) {
    const row = document.createElement("tr");
    row.dataset.calOutput = output;
    row.innerHTML = `
      <td><input type="checkbox" data-cal-output-check checked aria-label="Use ${escapeAttr(output)}" /></td>
      <td>${escapeHTML(output)}</td>
      <td>${escapeHTML(column)}</td>
      <td><input type="number" data-cal-output-weight value="1" min="0" step="0.1" aria-label="${escapeAttr(output)} weight" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function calibrationCandidateEditor(candidates) {
  const block = document.createElement("div");
  block.className = "result-block calibration-candidate-block";
  block.innerHTML = `
    <div class="result-block-title">Candidate Parameters</div>
    <div class="calibration-filters">
      <select data-calibration-filter="role" aria-label="Role filter">
        <option value="">All roles</option>
        <option value="calibration_target" selected>calibration_target</option>
      </select>
      <select data-calibration-filter="component" aria-label="Component filter"></select>
      <select data-calibration-filter="unit" aria-label="Unit filter"></select>
      <label class="compact-toggle"><input type="checkbox" data-calibration-filter="bounds" checked /> Bounds present</label>
    </div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Component</th><th>Parameter</th><th>Role</th><th>Unit</th><th>Current</th><th>Min</th><th>Max</th><th>Step</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const componentFilter = block.querySelector('[data-calibration-filter="component"]');
  componentFilter.append(new Option("All components", ""));
  [...new Set(candidates.map((item) => item.component))].sort().forEach((component) => componentFilter.append(new Option(component, component)));
  const unitFilter = block.querySelector('[data-calibration-filter="unit"]');
  unitFilter.append(new Option("All units", ""));
  [...new Set(candidates.map((item) => item.unit).filter(Boolean))].sort().forEach((unit) => unitFilter.append(new Option(unit, unit)));
  const tbody = block.querySelector("tbody");
  if (!candidates.length) {
    tbody.innerHTML = `<tr><td colspan="9" class="empty-cell">No parameters</td></tr>`;
    return block;
  }
  for (const candidate of candidates) {
    const row = document.createElement("tr");
    row.dataset.calParam = `${candidate.component}.${candidate.name}`;
    row.dataset.paramComponent = candidate.component;
    row.dataset.paramName = candidate.name;
    row.dataset.role = candidate.role;
    row.dataset.component = candidate.component;
    row.dataset.unit = candidate.unit;
    row.dataset.hasBounds = candidate.hasBounds ? "true" : "false";
    row.innerHTML = `
      <td><input type="checkbox" data-cal-param-check ${candidate.selected ? "checked" : ""} aria-label="Use ${escapeAttr(candidate.component)}.${escapeAttr(candidate.name)}" /></td>
      <td>${escapeHTML(candidate.componentName)}</td>
      <td>${escapeHTML(candidate.name)}</td>
      <td>${escapeHTML(candidate.role)}</td>
      <td>${escapeHTML(candidate.unit)}</td>
      <td>${escapeHTML(parameterInputValue(candidate.current))}</td>
      <td><input type="number" data-cal-param-field="min" value="${escapeAttr(candidate.min ?? "")}" step="any" /></td>
      <td><input type="number" data-cal-param-field="max" value="${escapeAttr(candidate.max ?? "")}" step="any" /></td>
      <td><input type="number" data-cal-param-field="step" value="${escapeAttr(candidate.step ?? "")}" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function labeledEditorControl(label, control) {
  const field = document.createElement("label");
  field.className = "editor-control";
  field.append(textSpan("input-label", label), control);
  return field;
}

function labeledEditorInput(label, type, placeholder, dataName) {
  const input = document.createElement("input");
  input.type = type;
  input.placeholder = placeholder;
  input.setAttribute(`data-${dataName}`, "true");
  return labeledEditorControl(label, input);
}

function textSpan(className, text) {
  const span = document.createElement("span");
  span.className = className;
  span.textContent = text;
  return span;
}

function updateCalibrationEditorState(section) {
  const roleFilter = section.querySelector('[data-calibration-filter="role"]')?.value || "";
  const componentFilter = section.querySelector('[data-calibration-filter="component"]')?.value || "";
  const unitFilter = section.querySelector('[data-calibration-filter="unit"]')?.value || "";
  const boundsOnly = section.querySelector('[data-calibration-filter="bounds"]')?.checked || false;
  for (const row of section.querySelectorAll("[data-cal-param]")) {
    const visible = (!roleFilter || row.dataset.role === roleFilter) &&
      (!componentFilter || row.dataset.component === componentFilter) &&
      (!unitFilter || row.dataset.unit === unitFilter) &&
      (!boundsOnly || row.dataset.hasBounds === "true");
    row.hidden = !visible;
  }
  const selectedRows = [...section.querySelectorAll("[data-cal-param]")].filter((row) => row.querySelector("[data-cal-param-check]")?.checked);
  const expectedRuns = selectedRows.length ? selectedRows.reduce((product, row) => product * calibrationGridPointCount(row), 1) : 0;
  const runCount = section.querySelector(".calibration-run-count");
  if (runCount) {
    runCount.textContent = `Selected ${selectedRows.length} / Expected Runs ${formatExpectedRunCount(expectedRuns)}`;
  }
}

function calibrationGridPointCount(row) {
  const min = Number(row.querySelector('[data-cal-param-field="min"]')?.value);
  const max = Number(row.querySelector('[data-cal-param-field="max"]')?.value);
  const step = Number(row.querySelector('[data-cal-param-field="step"]')?.value);
  if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) return 0;
  return Math.max(1, Math.floor(((max - min) / step) + 1.000000001));
}

function formatExpectedRunCount(value) {
  if (!Number.isFinite(value)) return "too many";
  if (value > 999999) return `${shortNumber(value)}+`;
  return String(value);
}

function collectCalibrationSetupEditorPayload(section, context) {
  const mappingPath = section.querySelector("[data-calibration-mapping]")?.value || context.mapping_summary?.relative_path || "";
  const outputs = {};
  for (const row of section.querySelectorAll("[data-cal-output]")) {
    if (!row.querySelector("[data-cal-output-check]")?.checked) continue;
    const weight = Number(row.querySelector("[data-cal-output-weight]")?.value);
    if (!Number.isFinite(weight) || weight < 0) {
      showInlineProblem(`Invalid calibration output weight: ${row.dataset.calOutput}`);
      return null;
    }
    outputs[row.dataset.calOutput] = weight;
  }
  if (!Object.keys(outputs).length) {
    showInlineProblem("Select at least one calibration target output");
    return null;
  }
  const parameters = [];
  for (const row of section.querySelectorAll("[data-cal-param]")) {
    if (!row.querySelector("[data-cal-param-check]")?.checked) continue;
    const min = Number(row.querySelector('[data-cal-param-field="min"]')?.value);
    const max = Number(row.querySelector('[data-cal-param-field="max"]')?.value);
    const step = Number(row.querySelector('[data-cal-param-field="step"]')?.value);
    if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) {
      showInlineProblem(`Invalid calibration bounds: ${row.dataset.paramComponent}.${row.dataset.paramName}`);
      return null;
    }
    parameters.push({
      component: row.dataset.paramComponent,
      name: row.dataset.paramName,
      min,
      max,
      step,
    });
  }
  if (!parameters.length) {
    showInlineProblem("Select at least one calibration parameter");
    return null;
  }
  return {
    mapping_path: mappingPath,
    id: section.querySelector("[data-calibration-setup-id]")?.value.trim() || "",
    name: section.querySelector("[data-calibration-setup-name]")?.value.trim() || "",
    algorithm: section.querySelector("[data-calibration-algorithm]")?.value || "grid",
    base_parameter_set: section.querySelector("[data-calibration-base]")?.value || "",
    objective_outputs: outputs,
    parameters,
  };
}

function openOptimizationSetupEditor() {
  state.latestWorkflowRecord = {
    kind: "optimization_setup_editor",
    candidates: optimizationDecisionCandidates(),
    outputs: optimizationPublicOutputs(),
  };
  renderResults();
  setMode("artifacts");
  setBottomTab("results");
  log("Optimization setup editor opened");
}

async function createOptimizationSetup(payload) {
  if (!(await saveModelEditsBeforeExecution())) return;
  try {
    const body = await api("/api/project/optimization-setup", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, ...payload }),
    });
    state.detail = body.project;
    state.latestWorkflowRecord = { kind: "optimization_setup", artifact: body.summary, setup: body.setup };
    renderProjectTree();
    renderArtifactWorkspace();
    renderResults();
    setMode("artifacts");
    setBottomTab("results");
    log(`Optimization setup created: ${body.summary?.relative_path || body.summary?.id}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Optimization setup failed: ${error.message}`);
  }
}

function optimizationPublicOutputs() {
  return (currentSystem()?.public_outputs || []).filter((output) => isNumericValueType(output.value_type || output.valueType || "float"));
}

function optimizationDecisionCandidates() {
  const candidates = [];
  const runInputs = collectRunInputs();
  for (const input of currentSystem()?.public_inputs || []) {
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
  for (const component of state.detail?.graph?.components || []) {
    const definitions = component.parameter_defs || {};
    for (const name of Object.keys(definitions).sort()) {
      const definition = definitions[name] || {};
      const current = finiteNumber(component.parameters?.[name] ?? definition.current ?? definition.default);
      const min = finiteNumber(definition.bounds?.min);
      const max = finiteNumber(definition.bounds?.max);
      if (current === null || min === null || max === null || max < min) continue;
      candidates.push({
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
      });
    }
  }
  return candidates;
}

function defaultDecisionBounds(value) {
  const numeric = Number(value);
  const delta = Math.max(Math.abs(numeric) * 0.2, 1);
  return [numeric - delta, numeric + delta];
}

function isNumericValueType(valueType) {
  return ["float", "int", "integer", "number"].includes(String(valueType || "").toLowerCase());
}

function optimizationSetupEditorSection(context) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(optimizationSetupControls(context));
  section.append(optimizationDecisionEditor(context.candidates || []));
  section.append(optimizationConstraintEditor(context.outputs || []));
  const actions = document.createElement("div");
  actions.className = "result-actions calibration-setup-actions";
  const runCount = document.createElement("span");
  runCount.className = "input-meta optimization-run-count";
  const create = document.createElement("button");
  create.type = "button";
  create.className = "small-action";
  create.textContent = "Create Setup";
  create.addEventListener("click", () => {
    const payload = collectOptimizationSetupEditorPayload(section);
    if (payload) createOptimizationSetup(payload);
  });
  actions.append(runCount, create);
  section.append(actions);
  section.querySelectorAll("[data-opt-var-check], [data-opt-var-field], [data-opt-constraint-check], [data-opt-constraint-field]").forEach((control) => {
    control.addEventListener("input", () => updateOptimizationEditorState(section));
    control.addEventListener("change", () => updateOptimizationEditorState(section));
  });
  updateOptimizationEditorState(section);
  return section;
}

function optimizationSetupControls(context) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Setup</div>`;
  const controls = document.createElement("div");
  controls.className = "calibration-editor-grid";
  const objectiveSelect = document.createElement("select");
  objectiveSelect.dataset.optimizationObjective = "true";
  for (const output of context.outputs || []) {
    objectiveSelect.append(new Option(output.name || output.id, output.id || ""));
  }
  const senseSelect = document.createElement("select");
  senseSelect.dataset.optimizationSense = "true";
  senseSelect.append(new Option("Minimize", "min"), new Option("Maximize", "max"));
  const baseSelect = document.createElement("select");
  baseSelect.dataset.optimizationBase = "true";
  baseSelect.append(new Option("Baseline", ""));
  for (const item of state.detail?.parameter_sets || []) {
    baseSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  baseSelect.value = state.activeParameterSetPath || "";
  const algorithmSelect = document.createElement("select");
  algorithmSelect.dataset.optimizationAlgorithm = "true";
  algorithmSelect.append(new Option("Grid Search", "grid"));
  algorithmSelect.append(new Option("Differential Evolution", "differential_evolution"));
  controls.append(
    labeledEditorControl("Objective", objectiveSelect),
    labeledEditorControl("Sense", senseSelect),
    labeledEditorControl("Base Parameter Set", baseSelect),
    labeledEditorControl("Algorithm", algorithmSelect),
    labeledEditorInput("Setup ID", "text", "auto", "optimization-setup-id"),
    labeledEditorInput("Setup Name", "text", "auto", "optimization-setup-name"),
  );
  block.append(controls);
  return block;
}

function optimizationDecisionEditor(candidates) {
  const block = document.createElement("div");
  block.className = "result-block calibration-candidate-block";
  block.innerHTML = `
    <div class="result-block-title">Decision Variables</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Kind</th><th>Target</th><th>Role</th><th>Unit</th><th>Current</th><th>Min</th><th>Max</th><th>Step</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!candidates.length) {
    tbody.innerHTML = `<tr><td colspan="9" class="empty-cell">No decision variables</td></tr>`;
    return block;
  }
  for (const candidate of candidates) {
    const row = document.createElement("tr");
    row.dataset.optVar = candidate.label;
    row.dataset.kind = candidate.kind;
    row.dataset.component = candidate.component;
    row.dataset.name = candidate.name;
    row.innerHTML = `
      <td><input type="checkbox" data-opt-var-check ${candidate.selected ? "checked" : ""} aria-label="Use ${escapeAttr(candidate.label)}" /></td>
      <td>${escapeHTML(candidate.kind)}</td>
      <td>${escapeHTML(candidate.label)}</td>
      <td>${escapeHTML(candidate.role)}</td>
      <td>${escapeHTML(candidate.unit)}</td>
      <td>${escapeHTML(parameterInputValue(candidate.current))}</td>
      <td><input type="number" data-opt-var-field="min" value="${escapeAttr(candidate.min)}" step="any" /></td>
      <td><input type="number" data-opt-var-field="max" value="${escapeAttr(candidate.max)}" step="any" /></td>
      <td><input type="number" data-opt-var-field="step" value="${escapeAttr(candidate.step)}" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function optimizationConstraintEditor(outputs) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `
    <div class="result-block-title">Constraints</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Output</th><th>Operator</th><th>Value</th><th>Tolerance</th><th>Penalty</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!outputs.length) {
    tbody.innerHTML = `<tr><td colspan="6" class="empty-cell">No numeric outputs</td></tr>`;
    return block;
  }
  for (const output of outputs) {
    const row = document.createElement("tr");
    row.dataset.optConstraint = output.id || "";
    row.innerHTML = `
      <td><input type="checkbox" data-opt-constraint-check aria-label="Constrain ${escapeAttr(output.id)}" /></td>
      <td>${escapeHTML(output.name || output.id)}</td>
      <td><select data-opt-constraint-field="operator"><option value="<=">&lt;=</option><option value=">=">&gt;=</option><option value="==">==</option></select></td>
      <td><input type="number" data-opt-constraint-field="value" value="0" step="any" /></td>
      <td><input type="number" data-opt-constraint-field="tolerance" value="0" min="0" step="any" /></td>
      <td><input type="number" data-opt-constraint-field="penalty" value="1000" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function updateOptimizationEditorState(section) {
  const selectedRows = [...section.querySelectorAll("[data-opt-var]")].filter((row) => row.querySelector("[data-opt-var-check]")?.checked);
  const expectedRuns = selectedRows.length ? selectedRows.reduce((product, row) => product * optimizationGridPointCount(row), 1) : 0;
  const runCount = section.querySelector(".optimization-run-count");
  if (runCount) {
    const constraintCount = [...section.querySelectorAll("[data-opt-constraint]")].filter((row) => row.querySelector("[data-opt-constraint-check]")?.checked).length;
    runCount.textContent = `Selected ${selectedRows.length} / Constraints ${constraintCount} / Estimated Runs ${formatExpectedRunCount(expectedRuns)}`;
  }
}

function optimizationGridPointCount(row) {
  const min = Number(row.querySelector('[data-opt-var-field="min"]')?.value);
  const max = Number(row.querySelector('[data-opt-var-field="max"]')?.value);
  const step = Number(row.querySelector('[data-opt-var-field="step"]')?.value);
  if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) return 0;
  return Math.max(1, Math.floor(((max - min) / step) + 1.000000001));
}

function collectOptimizationSetupEditorPayload(section) {
  const objectiveOutput = section.querySelector("[data-optimization-objective]")?.value || "";
  if (!objectiveOutput) {
    showInlineProblem("Select an optimization objective output");
    return null;
  }
  const variables = [];
  for (const row of section.querySelectorAll("[data-opt-var]")) {
    if (!row.querySelector("[data-opt-var-check]")?.checked) continue;
    const min = Number(row.querySelector('[data-opt-var-field="min"]')?.value);
    const max = Number(row.querySelector('[data-opt-var-field="max"]')?.value);
    const step = Number(row.querySelector('[data-opt-var-field="step"]')?.value);
    if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) {
      showInlineProblem(`Invalid optimization bounds: ${row.dataset.optVar}`);
      return null;
    }
    const variable = {
      kind: row.dataset.kind,
      name: row.dataset.name,
      min,
      max,
      step,
    };
    if (row.dataset.kind === "component_parameter") {
      variable.component = row.dataset.component;
    }
    variables.push(variable);
  }
  if (!variables.length) {
    showInlineProblem("Select at least one optimization decision variable");
    return null;
  }
  const constraints = [];
  for (const row of section.querySelectorAll("[data-opt-constraint]")) {
    if (!row.querySelector("[data-opt-constraint-check]")?.checked) continue;
    const value = Number(row.querySelector('[data-opt-constraint-field="value"]')?.value);
    const tolerance = Number(row.querySelector('[data-opt-constraint-field="tolerance"]')?.value);
    const penalty = Number(row.querySelector('[data-opt-constraint-field="penalty"]')?.value);
    if (!Number.isFinite(value) || !Number.isFinite(tolerance) || !Number.isFinite(penalty) || tolerance < 0 || penalty < 0) {
      showInlineProblem(`Invalid optimization constraint: ${row.dataset.optConstraint}`);
      return null;
    }
    constraints.push({
      output: row.dataset.optConstraint,
      operator: row.querySelector('[data-opt-constraint-field="operator"]')?.value || "<=",
      value,
      tolerance,
      penalty,
    });
  }
  return {
    id: section.querySelector("[data-optimization-setup-id]")?.value.trim() || "",
    name: section.querySelector("[data-optimization-setup-name]")?.value.trim() || "",
    algorithm: section.querySelector("[data-optimization-algorithm]")?.value || "grid",
    base_parameter_set: section.querySelector("[data-optimization-base]")?.value || "",
    base_inputs: collectRunInputs(),
    context: currentRunContext(),
    objective: {
      output: objectiveOutput,
      sense: section.querySelector("[data-optimization-sense]")?.value || "min",
    },
    decision_variables: variables,
    constraints,
  };
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
    ["Missing values", result.missing_value_policy || "error"],
    ["Rows evaluated", String(result.row_count || 0)],
    ["Rows in dataset", String(result.input_row_count || result.row_count || 0)],
    ["Rows skipped", String(result.skipped_row_count || 0)],
    ["Values filled", String(result.filled_value_count || 0)],
  ]));
  section.append(metricBars(result.metrics || {}));
  const comparisonRows = validationComparisonRows(result, state.validationComparisonBaseline);
  if (comparisonRows.length) {
    section.append(resultTable("Parameter Set Comparison", comparisonRows, ["Metric", "Baseline", "Current", "RMSE Delta", "MAE Delta", "R2 Delta"]));
  }
  section.append(validationComparisonControls(result));
  section.append(validationPlotSection(result));
  section.append(highErrorRows(result));
  return section;
}

function validationComparisonControls(result) {
  const actions = document.createElement("div");
  actions.className = "result-actions validation-compare-actions";
  const select = document.createElement("select");
  select.className = "validation-compare-select";
  const currentPath = result.parameter_set || "";
  const choices = [{ label: "Baseline", value: "" }, ...(state.detail?.parameter_sets || []).map((item) => ({
    label: item.name || item.id || item.relative_path,
    value: item.relative_path || "",
  }))].filter((item) => item.value !== currentPath);
  if (!choices.length) {
    select.append(new Option("No comparison sets", currentPath));
  } else {
    for (const item of choices) {
      select.append(new Option(item.label, item.value));
    }
  }
  const button = document.createElement("button");
  button.type = "button";
  button.className = "small-action";
  button.textContent = "Compare Parameter Set";
  button.disabled = !choices.length || !validationResultMappingPath(result);
  button.addEventListener("click", () => compareValidationParameterSet(result, select.value));
  actions.append(select, button);
  return actions;
}

function validationResultMappingPath(result) {
  if (result.mapping) return result.mapping;
  const mappingID = result.mapping_id || "";
  const found = (state.detail?.validation_mappings || []).find((item) => item.id === mappingID || item.name === result.mapping_name);
  return found?.relative_path || "";
}

async function compareValidationParameterSet(baselineResult, parameterSetPath) {
  if (!(await saveModelEditsBeforeExecution())) return;
  const mappingPath = validationResultMappingPath(baselineResult);
  if (!mappingPath) {
    state.latestValidation = { error: "Validation comparison requires a saved mapping path" };
    renderProblems();
    setBottomTab("problems");
    log("Validation comparison unavailable: no mapping path");
    return;
  }
  try {
    const body = await api("/api/validation/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: mappingPath,
        parameter_set_path: parameterSetPath,
        high_error_rows: 3,
        save: isWorkspaceProject(),
      }),
    });
    state.validationComparisonBaseline = baselineResult;
    state.latestDataValidation = body.validation_result;
    state.latestWorkflowRecord = null;
    if (body.validation_record) {
      state.detail.validation_runs = [body.validation_record, ...(state.detail.validation_runs || [])];
      await refreshCurrentProjectDetail();
    }
    setProblems();
    renderResults();
    renderProblems();
    setBottomTab("results");
    log(`Validation compared with parameter set: ${parameterSetPath || "baseline"}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Validation comparison failed: ${error.message}`);
  }
}

function validationComparisonRows(current, baseline) {
  if (!sameValidationComparisonScope(current, baseline)) return [];
  const currentLabel = current.parameter_set || "baseline";
  const baselineLabel = baseline.parameter_set || "baseline";
  return Object.entries(current.metrics || {}).filter(([name]) => baseline.metrics?.[name]).map(([name, metric]) => {
    const previous = baseline.metrics[name] || {};
    return [
      name,
      baselineLabel,
      currentLabel,
      metricDelta(metric.rmse, previous.rmse),
      metricDelta(metric.mae, previous.mae),
      metricDelta(metric.r2, previous.r2),
    ];
  });
}

function sameValidationComparisonScope(current, baseline) {
  if (!current || !baseline) return false;
  const currentMapping = current.mapping || current.mapping_id || "";
  const baselineMapping = baseline.mapping || baseline.mapping_id || "";
  if (currentMapping && baselineMapping && currentMapping !== baselineMapping) return false;
  if ((current.dataset || "") !== (baseline.dataset || "")) return false;
  return Object.keys(current.metrics || {}).some((name) => baseline.metrics?.[name]);
}

function metricDelta(current, baseline) {
  const currentValue = Number(current);
  const baselineValue = Number(baseline);
  if (!Number.isFinite(currentValue) || !Number.isFinite(baselineValue)) return "";
  const delta = currentValue - baselineValue;
  return delta > 0 ? `+${shortNumber(delta)}` : String(shortNumber(delta));
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

function validationPlotSection(result) {
  const block = document.createElement("div");
  block.className = "result-block validation-plots-block";
  block.innerHTML = `<div class="result-block-title">Validation Plots</div>`;
  const content = document.createElement("div");
  content.className = "validation-plots";
  const outputs = validationOutputSeries(result);
  if (!outputs.length) {
    content.innerHTML = `<div class="empty-cell">No validation plot data</div>`;
    block.append(content);
    return block;
  }
  for (const output of outputs) {
    const group = document.createElement("div");
    group.className = "validation-output-plots";
    const title = document.createElement("div");
    title.className = "validation-output-title";
    title.textContent = `${output.name} (${output.points.length} rows)`;
    const grid = document.createElement("div");
    grid.className = "validation-plot-grid";
    grid.append(
      plotPanel("Measured vs Simulated", measuredSeriesPlot(output)),
      plotPanel("Scatter", scatterPlot(output)),
      plotPanel("Residuals", residualPlot(output)),
      plotPanel("Residual Histogram", residualHistogram(output)),
    );
    group.append(title, grid);
    content.append(group);
  }
  block.append(content);
  return block;
}

function validationOutputSeries(result) {
  const outputs = new Set(Object.keys(result.metrics || {}));
  for (const row of result.rows || []) {
    Object.keys(row.observed || {}).forEach((name) => outputs.add(name));
    Object.keys(row.simulated || {}).forEach((name) => outputs.add(name));
  }
  return [...outputs].sort().map((name) => {
    const points = (result.rows || []).map((row, index) => {
      if (!row || row.skipped) return null;
      const observed = finiteNumber(row.observed?.[name]);
      const simulated = finiteNumber(row.simulated?.[name]);
      if (observed === null || simulated === null) return null;
      const error = finiteNumber(row.errors?.[name]) ?? simulated - observed;
      return {
        rowIndex: row.row_index ?? index,
        label: row.time ?? row.row_index ?? index + 1,
        observed,
        simulated,
        error,
      };
    }).filter(Boolean);
    return { name, points };
  }).filter((output) => output.points.length);
}

function plotPanel(title, svg) {
  const panel = document.createElement("div");
  panel.className = "validation-plot-panel";
  const label = document.createElement("div");
  label.className = "validation-plot-title";
  label.textContent = title;
  panel.append(label, svg);
  return panel;
}

function measuredSeriesPlot(output) {
  const svg = validationSVG(`${output.name} measured vs simulated`);
  const bounds = plotBounds();
  const yExtent = numericExtent(output.points.flatMap((point) => [point.observed, point.simulated]));
  appendPlotFrame(svg, bounds);
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.observed, "validation-series-observed");
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.simulated, "validation-series-simulated");
  appendPlotLegend(svg, [["Measured", "validation-series-observed"], ["Simulated", "validation-series-simulated"]]);
  return svg;
}

function scatterPlot(output) {
  const svg = validationSVG(`${output.name} measured simulated scatter`);
  const bounds = plotBounds();
  const extent = numericExtent(output.points.flatMap((point) => [point.observed, point.simulated]));
  appendPlotFrame(svg, bounds);
  svg.append(svgNode("line", {
    class: "validation-reference",
    x1: scaleValue(extent.min, extent, bounds.left, bounds.right),
    y1: scaleValue(extent.min, extent, bounds.bottom, bounds.top),
    x2: scaleValue(extent.max, extent, bounds.left, bounds.right),
    y2: scaleValue(extent.max, extent, bounds.bottom, bounds.top),
  }));
  for (const point of output.points) {
    const circle = svgNode("circle", {
      class: "validation-point",
      cx: scaleValue(point.observed, extent, bounds.left, bounds.right),
      cy: scaleValue(point.simulated, extent, bounds.bottom, bounds.top),
      r: 3,
    });
    circle.append(svgTitle(`row ${point.rowIndex}: measured ${shortNumber(point.observed)}, simulated ${shortNumber(point.simulated)}`));
    svg.append(circle);
  }
  return svg;
}

function residualPlot(output) {
  const svg = validationSVG(`${output.name} residuals`);
  const bounds = plotBounds();
  const yExtent = numericExtent([...output.points.map((point) => point.error), 0]);
  appendPlotFrame(svg, bounds);
  appendZeroLine(svg, bounds, yExtent);
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.error, "validation-series-residual");
  return svg;
}

function residualHistogram(output) {
  const svg = validationSVG(`${output.name} residual histogram`);
  const bounds = plotBounds();
  const errors = output.points.map((point) => point.error).filter((value) => Number.isFinite(value));
  const bins = histogramBins(errors, 8);
  appendPlotFrame(svg, bounds);
  if (!bins.length) return svg;
  const maxCount = Math.max(...bins.map((bin) => bin.count), 1);
  const gap = 3;
  const barWidth = ((bounds.right - bounds.left) / bins.length) - gap;
  bins.forEach((bin, index) => {
    const height = (bin.count / maxCount) * (bounds.bottom - bounds.top);
    const rect = svgNode("rect", {
      class: "validation-histogram-bar",
      x: bounds.left + index * ((bounds.right - bounds.left) / bins.length) + gap / 2,
      y: bounds.bottom - height,
      width: Math.max(1, barWidth),
      height,
    });
    rect.append(svgTitle(`${shortNumber(bin.min)} to ${shortNumber(bin.max)}: ${bin.count}`));
    svg.append(rect);
  });
  return svg;
}

function validationSVG(title) {
  const svg = svgNode("svg", {
    class: "validation-svg",
    viewBox: "0 0 320 180",
    role: "img",
    "aria-label": title,
  });
  svg.append(svgTitle(title));
  return svg;
}

function plotBounds() {
  return { left: 32, right: 302, top: 16, bottom: 152 };
}

function appendPlotFrame(svg, bounds) {
  svg.append(
    svgNode("line", { class: "validation-axis", x1: bounds.left, y1: bounds.bottom, x2: bounds.right, y2: bounds.bottom }),
    svgNode("line", { class: "validation-axis", x1: bounds.left, y1: bounds.top, x2: bounds.left, y2: bounds.bottom }),
  );
}

function appendPlotLegend(svg, items) {
  items.forEach(([label, className], index) => {
    const y = 168;
    const x = 32 + index * 96;
    svg.append(svgNode("line", { class: className, x1: x, y1: y, x2: x + 18, y2: y }));
    const text = svgNode("text", { class: "validation-legend", x: x + 24, y: y + 4 });
    text.textContent = label;
    svg.append(text);
  });
}

function appendLinePath(svg, points, bounds, yExtent, valueForPoint, className) {
  if (!points.length) return;
  const d = points.map((point, index) => {
    const x = scaleIndex(index, points.length, bounds.left, bounds.right);
    const y = scaleValue(valueForPoint(point), yExtent, bounds.bottom, bounds.top);
    return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(" ");
  svg.append(svgNode("path", { class: className, d }));
}

function appendZeroLine(svg, bounds, yExtent) {
  const y = scaleValue(0, yExtent, bounds.bottom, bounds.top);
  svg.append(svgNode("line", { class: "validation-reference", x1: bounds.left, y1: y, x2: bounds.right, y2: y }));
}

function histogramBins(values, binCount) {
  if (!values.length) return [];
  const extent = numericExtent(values);
  const width = (extent.max - extent.min) / binCount;
  const bins = Array.from({ length: binCount }, (_, index) => ({
    min: extent.min + index * width,
    max: extent.min + (index + 1) * width,
    count: 0,
  }));
  for (const value of values) {
    const index = Math.min(binCount - 1, Math.max(0, Math.floor((value - extent.min) / width)));
    bins[index].count += 1;
  }
  return bins;
}

function numericExtent(values) {
  const numbers = values.map((value) => Number(value)).filter((value) => Number.isFinite(value));
  if (!numbers.length) return { min: 0, max: 1 };
  let min = Math.min(...numbers);
  let max = Math.max(...numbers);
  if (min === max) {
    const pad = Math.max(Math.abs(min) * 0.1, 1);
    min -= pad;
    max += pad;
  } else {
    const pad = (max - min) * 0.08;
    min -= pad;
    max += pad;
  }
  return { min, max };
}

function scaleIndex(index, length, min, max) {
  if (length <= 1) return (min + max) / 2;
  return min + (index / (length - 1)) * (max - min);
}

function scaleValue(value, extent, min, max) {
  return min + ((Number(value) - extent.min) / (extent.max - extent.min)) * (max - min);
}

function finiteNumber(value) {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : null;
}

function svgNode(name, attrs = {}) {
  const node = document.createElementNS("http://www.w3.org/2000/svg", name);
  for (const [key, value] of Object.entries(attrs)) {
    node.setAttribute(key, String(value));
  }
  return node;
}

function highErrorRows(result) {
  const metrics = result.metrics || {};
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `
    <div class="result-block-title">High Error Rows</div>
    <table class="result-table">
      <thead><tr><th>Metric</th><th>Row</th><th>Time</th><th>Observed</th><th>Simulated</th><th>Error</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  const rows = [];
  for (const [metric, item] of Object.entries(metrics)) {
    for (const high of item.high_error_rows || []) {
      rows.push({
        metric,
        high,
      });
    }
  }
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="6" class="empty-cell">No high error rows</td></tr>`;
    return block;
  }
  for (const row of rows) {
    const tr = document.createElement("tr");
    tr.className = "clickable-result-row";
    tr.tabIndex = 0;
    tr.title = "Inspect timestep";
    const cells = [
      row.metric,
      String(row.high.row_index),
      row.high.time ?? "",
      shortNumber(row.high.observed),
      shortNumber(row.high.simulated),
      shortNumber(row.high.error),
    ];
    tr.innerHTML = cells.map((cell) => `<td>${escapeHTML(cell)}</td>`).join("");
    tr.addEventListener("click", () => openHighErrorInspection(result, row.metric, row.high));
    tr.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openHighErrorInspection(result, row.metric, row.high);
      }
    });
    tbody.append(tr);
  }
  return block;
}

function openHighErrorInspection(validationResult, metric, high) {
  state.latestWorkflowRecord = {
    kind: "high_error_inspection",
    metric,
    row_index: high.row_index,
    time: high.time,
    observed: high.observed,
    simulated: high.simulated,
    error: high.error,
    inspection: high.inspection || {},
    validation_result: validationResult,
  };
  renderResults();
  setBottomTab("results");
}

function highErrorInspectionSection(value) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const inspection = value.inspection || {};
  section.append(resultTable("Summary", [
    ["Metric", value.metric || ""],
    ["Row", String(value.row_index ?? "")],
    ["Time", value.time ?? ""],
    ["Observed", shortNumber(value.observed)],
    ["Simulated", shortNumber(value.simulated)],
    ["Error", shortNumber(value.error)],
  ]));
  section.append(resultTable("Component Inputs", componentValueRows(inspection.component_inputs), ["Component", "Input", "Value"]));
  section.append(resultTable("Component Outputs", componentValueRows(inspection.component_outputs), ["Component", "Output", "Value"]));
  section.append(resultTable("Node Values", nodeValueRows(inspection.node_values), ["Component", "Node", "Direction", "Metadata", "Value"]));
  section.append(resultTable("Connection Values", connectionValueRows(inspection.connection_values), ["Connection", "From", "To", "Metadata", "Value"]));
  section.append(resultTable("States", componentValueRows(inspection.states), ["Component", "State", "Value"]));
  const actions = document.createElement("div");
  actions.className = "result-actions";
  const back = document.createElement("button");
  back.type = "button";
  back.className = "small-action";
  back.textContent = "Back to Validation Result";
  back.addEventListener("click", () => {
    state.latestWorkflowRecord = value.validation_result || null;
    renderResults();
  });
  actions.append(back);
  section.append(actions);
  return section;
}

function componentValueRows(values) {
  const rows = [];
  for (const [component, fields] of Object.entries(values || {})) {
    for (const [name, value] of Object.entries(fields || {})) {
      rows.push([component, name, formatValue(value)]);
    }
  }
  return rows;
}

function nodeValueRows(values) {
  return (values || []).map((item) => [
    item.component || "",
    item.node || "",
    item.direction || "",
    [item.medium || "", item.value_type || "", item.unit || ""].filter(Boolean).join(" / "),
    formatValue(item.value),
  ]);
}

function connectionValueRows(values) {
  return (values || []).map((item) => [
    item.id || "",
    endpointLabel(item.from),
    endpointLabel(item.to),
    [
      item.source_medium && item.target_medium ? `${item.source_medium}->${item.target_medium}` : item.source_medium || item.target_medium || "",
      item.source_unit && item.target_unit ? `${item.source_unit}->${item.target_unit}` : "",
      item.value_type || "",
      item.unit || "",
      item.converted ? "converted" : "",
    ].filter(Boolean).join(" / "),
    item.converted ? `${formatValue(item.source_value)} -> ${formatValue(item.converted_value ?? item.value)}` : formatValue(item.value),
  ]);
}

function endpointLabel(endpoint) {
  if (!endpoint) return "";
  return [endpoint.component, endpoint.node].filter(Boolean).join(".");
}

function candidateResultSection(result, savedLabel, savedPath) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const candidates = result.candidates || [];
  const summaryRows = [
    ["Setup", result.setup_name || result.setup_id || ""],
  ];
  if (result.base_parameter_set !== undefined) summaryRows.push(["Base parameter set", result.base_parameter_set || "baseline"]);
  if (result.initial_objective !== undefined) summaryRows.push(["Initial objective", shortNumber(result.initial_objective)]);
  if (result.objective !== undefined) summaryRows.push(["Objective", shortNumber(result.objective)]);
  if (result.best_objective !== undefined) summaryRows.push(["Best objective", shortNumber(result.best_objective)]);
  summaryRows.push([savedLabel, savedPath || ""]);
  section.append(resultTable("Summary", summaryRows));
  if (result.changed_parameters) {
    section.append(resultTable("Parameter Changes", parameterChangeRows(result.changed_parameters), ["Component", "Parameter", "Initial", "Best", "Delta"]));
  }
  const bestDecisionRows = optimizationBestDecisionRows(result);
  if (bestDecisionRows.length) {
    section.append(resultTable("Best Decision Variables", bestDecisionRows, ["Kind", "Target", "Value"]));
  }
  if (result.best_outputs) {
    section.append(resultTable("Best Outputs", Object.entries(result.best_outputs || {}).map(([name, value]) => [name, formatValue(value)]), ["Output", "Value"]));
  }
  const objectiveHistory = candidateObjectiveHistory(result);
  if (objectiveHistory) {
    section.append(objectiveHistory);
  }
  const constraintRows = optimizationConstraintRows(result);
  if (constraintRows.length) {
    section.append(resultTable("Constraint Status", constraintRows, ["Item", "Status", "Detail"]));
  }
  const outputRows = optimizationOutputComparisonRows(result);
  if (outputRows.length) {
    section.append(resultTable("Output Comparison", outputRows, ["#", "Objective", "Status", "Outputs"]));
  }
  section.append(resultTable("Candidates", candidates.slice(0, 12).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
  ]), ["#", "Objective", "Status", "Values"]));
  const failed = candidates.filter((item) => item.error);
  if (failed.length) {
    section.append(resultTable("Failed Candidates", failed.slice(0, 12).map((item) => [
      String(item.index ?? ""),
      item.error || "",
      parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
    ]), ["#", "Error", "Values"]));
  }
  const actions = document.createElement("div");
  actions.className = "result-actions";
  if (candidates.length) {
    const exportCSV = document.createElement("button");
    exportCSV.type = "button";
    exportCSV.className = "small-action";
    exportCSV.textContent = "Export CSV";
    exportCSV.addEventListener("click", () => downloadCandidateCSV(result));
    actions.append(exportCSV);
  }
  const exportReport = document.createElement("button");
  exportReport.type = "button";
  exportReport.className = "small-action";
  exportReport.textContent = "Export Report";
  exportReport.addEventListener("click", () => downloadCandidateReport(result));
  actions.append(exportReport);
  if (isOptimizationResult(result) && result.setup) {
    const exportSDK = document.createElement("button");
    exportSDK.type = "button";
    exportSDK.className = "small-action";
    exportSDK.textContent = "Export SDK Script";
    exportSDK.addEventListener("click", () => downloadOptimizationSDKScript(result));
    actions.append(exportSDK);
  }
  if (savedLabel === "Saved parameter set" && savedPath && isWorkspaceProject()) {
    const useForRuns = document.createElement("button");
    useForRuns.type = "button";
    useForRuns.className = "small-action";
    useForRuns.textContent = "Use for Runs";
    useForRuns.addEventListener("click", () => activateParameterSetForRuns(savedPath));
    actions.append(useForRuns);
    const revertActive = document.createElement("button");
    revertActive.type = "button";
    revertActive.className = "small-action";
    revertActive.textContent = "Revert Active";
    revertActive.addEventListener("click", () => activateParameterSetForRuns(""));
    actions.append(revertActive);
    const compareValidation = document.createElement("button");
    compareValidation.type = "button";
    compareValidation.className = "small-action";
    compareValidation.textContent = "Validation Before/After";
    compareValidation.disabled = !result.mapping;
    compareValidation.addEventListener("click", () => runCalibrationValidationComparison(result));
    actions.append(compareValidation);
    const apply = document.createElement("button");
    apply.type = "button";
    apply.className = "small-action";
    apply.textContent = "Apply Parameter Set";
    apply.addEventListener("click", () => applyParameterSetToGraph(savedPath));
    actions.append(apply);
  }
  if (actions.childElementCount) {
    section.append(actions);
  }
  return section;
}

function isOptimizationResult(result) {
  return result.saved_scenario !== undefined || result.best_inputs !== undefined || result.objective?.output !== undefined;
}

function candidateObjectiveHistory(result) {
  const points = (result.candidates || [])
    .map((item, index) => ({ index: item.index ?? index + 1, objective: finiteNumber(item.objective) }))
    .filter((item) => item.objective !== null);
  if (!points.length) return null;
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Objective History</div>`;
  const svg = validationSVG("objective history");
  const bounds = plotBounds();
  const yExtent = numericExtent(points.map((point) => point.objective));
  appendPlotFrame(svg, bounds);
  appendLinePath(svg, points, bounds, yExtent, (point) => point.objective, "validation-series-simulated");
  for (const [index, point] of points.entries()) {
    const circle = svgNode("circle", {
      class: "validation-point",
      cx: scaleIndex(index, points.length, bounds.left, bounds.right),
      cy: scaleValue(point.objective, yExtent, bounds.bottom, bounds.top),
      r: 3,
    });
    circle.append(svgTitle(`candidate ${point.index}: ${shortNumber(point.objective)}`));
    svg.append(circle);
  }
  block.append(svg);
  return block;
}

function activateParameterSetForRuns(path) {
  state.activeParameterSetPath = path || "";
  renderRunInputs();
  renderProjectTree();
  renderStartRuntimeRows();
  log(`Active parameter set: ${state.activeParameterSetPath || "baseline"}`);
}

async function runCalibrationValidationComparison(result) {
  if (!result?.mapping || !result?.saved_parameter_set) {
    showInlineProblem("Calibration validation comparison requires a mapping and saved parameter set");
    return;
  }
  try {
    const beforeBody = await api("/api/validation/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: result.mapping,
        parameter_set_path: result.base_parameter_set || "",
        high_error_rows: 3,
        save: false,
      }),
    });
    const afterBody = await api("/api/validation/run", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        mapping_path: result.mapping,
        parameter_set_path: result.saved_parameter_set,
        high_error_rows: 3,
        save: false,
      }),
    });
    state.latestWorkflowRecord = {
      kind: "calibration_validation_comparison",
      calibration_result: result,
      before: beforeBody.validation_result,
      after: afterBody.validation_result,
    };
    renderResults();
    setBottomTab("results");
    log(`Calibration validation compared: ${result.saved_parameter_set}`);
  } catch (error) {
    state.latestValidation = { error: error.message, problems: error.body?.problems || [] };
    renderProblems();
    setBottomTab("problems");
    log(`Calibration validation comparison failed: ${error.message}`);
  }
}

function calibrationValidationComparisonSection(value) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const before = value.before || {};
  const after = value.after || {};
  section.append(resultTable("Summary", [
    ["Mapping", after.mapping || before.mapping || value.calibration_result?.mapping || ""],
    ["Before", before.parameter_set || "baseline"],
    ["After", after.parameter_set || value.calibration_result?.saved_parameter_set || ""],
    ["Rows", `${before.row_count || 0} before / ${after.row_count || 0} after`],
  ]));
  const comparisonRows = validationComparisonRows(after, before);
  if (comparisonRows.length) {
    section.append(resultTable("Metric Deltas", comparisonRows, ["Metric", "Before", "After", "RMSE Delta", "MAE Delta", "R2 Delta"]));
  }
  const beforePlots = validationPlotSection(before);
  setResultBlockTitle(beforePlots, "Before Validation Plots");
  const afterPlots = validationPlotSection(after);
  setResultBlockTitle(afterPlots, "After Validation Plots");
  section.append(beforePlots, afterPlots);
  const actions = document.createElement("div");
  actions.className = "result-actions";
  const back = document.createElement("button");
  back.type = "button";
  back.className = "small-action";
  back.textContent = "Back to Calibration Result";
  back.addEventListener("click", () => {
    state.latestWorkflowRecord = value.calibration_result || null;
    renderResults();
  });
  actions.append(back);
  section.append(actions);
  return section;
}

function setResultBlockTitle(block, title) {
  const titleNode = block.querySelector(".result-block-title");
  if (titleNode) titleNode.textContent = title;
}

function optimizationBestDecisionRows(result) {
  const rows = [];
  for (const [name, value] of Object.entries(result.best_inputs || {})) {
    rows.push(["Public Input", name, formatValue(value)]);
  }
  for (const [component, values] of Object.entries(result.best_parameters || {})) {
    for (const [name, value] of Object.entries(values || {})) {
      rows.push(["Component Parameter", `${component}.${name}`, formatValue(value)]);
    }
  }
  return rows;
}

function optimizationConstraintRows(result) {
  const candidates = result.candidates || [];
  if (!candidates.length || !(result.best_inputs || result.best_parameters || result.best_outputs)) return [];
  const best = bestCandidate(result);
  const feasible = candidates.filter((item) => !item.error && item.feasible !== false).length;
  const infeasible = candidates.filter((item) => item.feasible === false).length;
  const failed = candidates.filter((item) => item.error).length;
  const rows = [
    ["Feasible candidates", String(feasible), ""],
    ["Infeasible candidates", String(infeasible), ""],
    ["Failed candidates", String(failed), ""],
  ];
  if (best) {
    const violations = best.constraint_violations || [];
    rows.push(["Best candidate", violations.length ? "violated" : "ok", violations.length ? `${violations.length} constraint violation(s)` : "constraints satisfied"]);
    for (const violation of violations) {
      rows.push([
        violation.output || "constraint",
        `${violation.operator || ""} ${shortNumber(violation.value)}`.trim(),
        [violation.message || "", `actual ${formatValue(violation.actual)}`, `violation ${shortNumber(violation.violation)}`].filter(Boolean).join(" / "),
      ]);
    }
  }
  return rows;
}

function optimizationOutputComparisonRows(result) {
  return (result.candidates || []).filter((item) => item.outputs && Object.keys(item.outputs).length).slice(0, 12).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    resultPublicOutputSummary(item.outputs || {}),
  ]);
}

function bestCandidate(result) {
  const candidates = result.candidates || [];
  const bestObjective = Number(result.best_objective);
  if (!Number.isFinite(bestObjective)) return candidates.find((item) => !item.error) || null;
  return candidates.find((item) => !item.error && Math.abs(Number(item.objective) - bestObjective) <= 1e-9) || candidates.find((item) => !item.error) || null;
}

function parameterChangeRows(changes) {
  const rows = [];
  for (const [component, params] of Object.entries(changes || {})) {
    for (const [name, change] of Object.entries(params || {})) {
      const initial = change?.initial;
      const best = change?.best;
      rows.push([
        component,
        name,
        formatValue(initial),
        formatValue(best),
        numericDelta(initial, best),
      ]);
    }
  }
  return rows.sort((a, b) => `${a[0]}.${a[1]}`.localeCompare(`${b[0]}.${b[1]}`));
}

function numericDelta(initial, best) {
  const before = Number(initial);
  const after = Number(best);
  if (!Number.isFinite(before) || !Number.isFinite(after)) return "";
  return shortNumber(after - before);
}

function downloadCandidateReport(result) {
  const isCalibration = result.saved_parameter_set !== undefined && result.changed_parameters !== undefined;
  const title = isCalibration ? "Calibration Result Report" : "Optimization Result Report";
  const lines = [`# ${title}`, ""];
  lines.push(...markdownTable([
    ["Setup", result.setup_name || result.setup_id || ""],
    ["Algorithm", result.algorithm || ""],
    ["Base parameter set", result.base_parameter_set || "baseline"],
    ["Initial objective", result.initial_objective !== undefined ? shortNumber(result.initial_objective) : ""],
    ["Best objective", result.best_objective !== undefined ? shortNumber(result.best_objective) : ""],
    ["Saved parameter set", result.saved_parameter_set || ""],
    ["Saved scenario", result.saved_scenario || ""],
    ["Saved record", result.saved_record || ""],
  ].filter(([, value]) => value !== "")));
  if (result.changed_parameters) {
    lines.push("", "## Parameter Changes", "");
    lines.push(...markdownTable(parameterChangeRows(result.changed_parameters), ["Component", "Parameter", "Initial", "Best", "Delta"]));
  }
  const decisionRows = optimizationBestDecisionRows(result);
  if (decisionRows.length) {
    lines.push("", "## Best Decision Variables", "");
    lines.push(...markdownTable(decisionRows, ["Kind", "Target", "Value"]));
  }
  if (result.best_outputs) {
    lines.push("", "## Best Outputs", "");
    lines.push(...markdownTable(Object.entries(result.best_outputs || {}).map(([name, value]) => [name, formatValue(value)]), ["Output", "Value"]));
  }
  const constraintRows = optimizationConstraintRows(result);
  if (constraintRows.length) {
    lines.push("", "## Constraint Status", "");
    lines.push(...markdownTable(constraintRows, ["Item", "Status", "Detail"]));
  }
  lines.push("", "## Candidates", "");
  lines.push(...markdownTable((result.candidates || []).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
  ]), ["#", "Objective", "Status", "Values"]));
  const name = `${safeFileName(result.setup_id || result.setup_name || "candidate-result")}-report.md`;
  downloadTextFile(name, `${lines.join("\n")}\n`, "text/markdown;charset=utf-8");
}

function markdownTable(rows, headers = ["Item", "Value"]) {
  const normalized = rows || [];
  if (!normalized.length) return ["No rows."];
  return [
    `| ${headers.map(markdownCell).join(" | ")} |`,
    `| ${headers.map(() => "---").join(" | ")} |`,
    ...normalized.map((row) => `| ${headers.map((_, index) => markdownCell(row[index] ?? "")).join(" | ")} |`),
  ];
}

function markdownCell(value) {
  return String(value ?? "").replace(/\|/g, "\\|").replace(/\r?\n/g, " ");
}

function downloadCandidateCSV(result) {
  const candidates = result.candidates || [];
  const flatRows = candidates.map((candidate, index) => {
    const row = {
      index: candidate.index ?? index + 1,
      objective: candidate.objective ?? "",
      status: candidateStatus(candidate),
      feasible: candidate.feasible ?? "",
      error: candidate.error || "",
      constraint_penalty: candidate.constraint_penalty ?? "",
    };
    flattenCandidateValues(candidate.inputs || {}, "inputs", row);
    flattenCandidateValues(candidate.parameters || {}, "parameters", row);
    flattenCandidateValues(candidate.outputs || {}, "outputs", row);
    return row;
  });
  const baseHeaders = ["index", "objective", "status", "feasible", "error", "constraint_penalty"];
  const dynamicHeaders = [...new Set(flatRows.flatMap((row) => Object.keys(row)))].filter((key) => !baseHeaders.includes(key)).sort();
  const headers = [...baseHeaders, ...dynamicHeaders];
  const csv = [
    headers.map(csvCell).join(","),
    ...flatRows.map((row) => headers.map((header) => csvCell(row[header] ?? "")).join(",")),
  ].join("\r\n");
  const name = `${safeFileName(result.setup_id || result.setup_name || "candidates")}-candidates.csv`;
  downloadTextFile(name, csv, "text/csv;charset=utf-8");
}

function flattenCandidateValues(value, prefix, row) {
  for (const [key, item] of Object.entries(value || {})) {
    const path = `${prefix}.${key}`;
    if (item && typeof item === "object" && !Array.isArray(item)) {
      flattenCandidateValues(item, path, row);
    } else {
      row[path] = item;
    }
  }
}

function csvCell(value) {
  const text = typeof value === "string" ? value : JSON.stringify(value);
  const normalized = text === undefined ? "" : String(text);
  return `"${normalized.replace(/"/g, '""')}"`;
}

function safeFileName(value) {
  return String(value || "export").replace(/[^A-Za-z0-9._-]+/g, "_").replace(/^_+|_+$/g, "") || "export";
}

function downloadTextFile(name, content, type) {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function downloadOptimizationSDKScript(result) {
  const setup = result.setup || "";
  const saveScenario = result.saved_scenario || "";
  const saveParameterSet = result.saved_parameter_set || "";
  const outputName = `${safeFileName(result.setup_id || result.setup_name || "optimization")}-sdk-result.json`;
  const lines = [
    "from pathlib import Path",
    "import json",
    "",
    "from bcs_sdk import RunnerClient",
    "",
    `PROJECT = Path(${pythonStringLiteral(state.currentProjectPath || "project.bcsproj")})`,
    `RUNNER = ${pythonStringLiteral("bcs-runner.exe")}`,
    `SETUP = ${pythonStringLiteral(setup)}`,
    `OUTPUT = Path(${pythonStringLiteral(outputName)})`,
    "",
    "client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)",
    "client.validate_project()",
    "result = client.run_optimization(",
    "    setup=SETUP,",
    saveScenario ? `    save_scenario=${pythonStringLiteral(saveScenario)},` : "    save_scenario=None,",
    saveParameterSet ? `    save_parameter_set=${pythonStringLiteral(saveParameterSet)},` : "    save_parameter_set=None,",
    "    save_record=True,",
    "    output=OUTPUT,",
    ")",
    "print(json.dumps({",
    "    \"ok\": result.get(\"ok\"),",
    "    \"best_objective\": result.get(\"best_objective\"),",
    "    \"saved_scenario\": result.get(\"saved_scenario\", \"\"),",
    "    \"saved_parameter_set\": result.get(\"saved_parameter_set\", \"\"),",
    "    \"output\": str(OUTPUT),",
    "}, indent=2, sort_keys=True))",
    "",
  ];
  const name = `${safeFileName(result.setup_id || result.setup_name || "optimization")}-sdk.py`;
  downloadTextFile(name, lines.join("\n"), "text/x-python;charset=utf-8");
}

function candidateStatus(item) {
  if (item.error) return `failed: ${item.error}`;
  if (item.feasible === false) {
    const count = (item.constraint_violations || []).length;
    return `${count || 1} constraint${count === 1 ? "" : "s"}`;
  }
  return "feasible";
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

function seriesResultSection(series) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const actions = document.createElement("div");
  actions.className = "result-actions";

  const exportCSV = document.createElement("button");
  exportCSV.type = "button";
  exportCSV.className = "small-action";
  exportCSV.textContent = "Export Series CSV";
  exportCSV.addEventListener("click", () => downloadSeriesCSV(series));

  const exportJSON = document.createElement("button");
  exportJSON.type = "button";
  exportJSON.className = "small-action";
  exportJSON.textContent = "Export Series JSON";
  exportJSON.addEventListener("click", () => downloadSeriesJSON(series));

  actions.append(exportCSV, exportJSON);
  section.append(actions);
  section.append(resultTable("Public Output Series", seriesOutputRows(series), ["Output", "Values"]));
  section.append(resultTable("Time Indexed Steps", seriesStepRows(series), ["#", "Step", "Time", "Inputs", "Outputs", "Trace"]));
  const componentRows = selectedComponentSeriesRows(series);
  if (componentRows.length) {
    section.append(resultTable(`Selected Component Timeline: ${state.selectedComponentId}`, componentRows, ["#", "Inputs", "Outputs", "State"]));
  }
  section.append(resultTable("Final States", Object.keys(series.final_states || {}).map((component) => [component, formatValue(series.final_states[component])]), ["Component", "State"]));
  return section;
}

function seriesOutputRows(series) {
  return Object.entries(series.outputs || {}).map(([name, values]) => {
    const list = Array.isArray(values) ? values : [];
    const last = list.length ? list[list.length - 1] : "";
    return [name, `${list.length} values / last ${formatValue(last)}`];
  });
}

function seriesStepRows(series) {
  return (series.series || []).map((point, index) => [
    String(point.index ?? index + 1),
    point.id || `step-${index + 1}`,
    formatSeriesTime(point),
    compactObjectSummary(point.inputs || {}),
    compactObjectSummary(point.outputs || {}),
    `${(point.node_values || []).length} nodes / ${(point.connection_values || []).length} connections`,
  ]);
}

function selectedComponentSeriesRows(series) {
  const componentID = state.selectedComponentId;
  if (!componentID) return [];
  return (series.series || []).map((point, index) => {
    const inputs = point.component_inputs?.[componentID] || {};
    const outputs = point.component_outputs?.[componentID] || {};
    const componentState = point.states?.[componentID] || {};
    if (!Object.keys(inputs).length && !Object.keys(outputs).length && !Object.keys(componentState).length) return null;
    return [
      String(point.index ?? index + 1),
      compactObjectSummary(inputs),
      compactObjectSummary(outputs),
      compactObjectSummary(componentState),
    ];
  }).filter(Boolean);
}

function formatSeriesTime(point) {
  if (point.time !== undefined && point.time !== null && point.time !== "") return formatValue(point.time);
  if (point.context?.time !== undefined) return formatValue(point.context.time);
  return "";
}

function compactObjectSummary(value) {
  const entries = Object.entries(value || {});
  if (!entries.length) return "";
  return entries.slice(0, 6).map(([key, item]) => `${key}=${formatValue(item)}`).join(", ");
}

function downloadSeriesJSON(series) {
  const project = state.detail?.project?.project_name || "series";
  const name = `${safeFileName(project)}-series.json`;
  downloadTextFile(name, `${JSON.stringify(series, null, 2)}\n`, "application/json;charset=utf-8");
}

function downloadSeriesCSV(series) {
  const points = series.series || [];
  const inputKeys = sortedSeriesKeys(points, "inputs");
  const contextKeys = sortedSeriesKeys(points, "context");
  const outputKeys = Array.from(new Set([
    ...Object.keys(series.outputs || {}),
    ...sortedSeriesKeys(points, "outputs"),
  ])).sort();
  const headers = [
    "index",
    "id",
    "time",
    ...inputKeys.map((key) => `input:${key}`),
    ...contextKeys.map((key) => `context:${key}`),
    ...outputKeys.map((key) => `output:${key}`),
  ];
  const rows = [headers, ...points.map((point, index) => [
    point.index ?? index + 1,
    point.id || `step-${index + 1}`,
    point.time ?? point.context?.time ?? "",
    ...inputKeys.map((key) => point.inputs?.[key] ?? ""),
    ...contextKeys.map((key) => point.context?.[key] ?? ""),
    ...outputKeys.map((key) => point.outputs?.[key] ?? ""),
  ])];
  const csv = `${rows.map((row) => row.map(csvCell).join(",")).join("\n")}\n`;
  const project = state.detail?.project?.project_name || "series";
  downloadTextFile(`${safeFileName(project)}-series.csv`, csv, "text/csv;charset=utf-8");
}

function sortedSeriesKeys(points, field) {
  const keys = new Set();
  for (const point of points || []) {
    for (const key of Object.keys(point[field] || {})) keys.add(key);
  }
  return Array.from(keys).sort();
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
  state.lastRuntimeAction = "run";
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
      state.latestRunRecord = { ...body.run_record, result: body.result };
      log(`Run saved: ${body.run_record.relative_path}`);
      renderProjectTree();
    } else {
      state.latestRunRecord = null;
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
  normalizeSeriesInputSelection();
  const selectedSeriesInput = activeSeriesInputSummary();
  const seriesInput = selectedSeriesInput ? null : buildSeriesInput();
  const comparisonBaseline = latestRuntimeComparisonContext();
  state.lastRuntimeAction = "series";
  const controller = beginRuntimeRequest("Series");
  if (!controller) return;
  try {
    const request = {
      project_path: state.currentProjectPath,
      parameter_set_path: state.activeParameterSetPath,
      timeout_ms: state.runTimeoutMS,
    };
    if (selectedSeriesInput) {
      request.input_path = selectedSeriesInput.relative_path;
    } else {
      request.schema_version = "0.1.0";
      request.context = seriesInput.context;
      request.steps = seriesInput.steps;
    }
    const body = await api("/api/run-series", {
      method: "POST",
      signal: controller.signal,
      body: JSON.stringify(request),
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
  state.lastRuntimeAction = "batch";
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
  el("runtimeStatus").textContent = `${label} in progress`;
  updateCommandState();
  renderRunWorkspace();
  return controller;
}

function finishRuntimeRequest(controller) {
  if (state.activeRunAbortController !== controller) return;
  state.activeRunAbortController = null;
  state.activeRunLabel = "";
  el("runtimeStatus").textContent = "Runtime ready";
  updateCommandState();
  renderRunWorkspace();
}

function cancelActiveRun() {
  if (!state.activeRunAbortController) return;
  const label = state.activeRunLabel || "Run";
  state.activeRunAbortController.abort();
  log(`${label} cancel requested`);
  updateCommandState();
}

async function retryLastRuntimeAction() {
  if (!state.detail || state.activeRunAbortController) return;
  const action = state.lastRuntimeAction;
  if (action === "series") {
    log("Retrying Series");
    await runSeries();
    return;
  }
  if (action === "batch") {
    log("Retrying Batch");
    await runBatch();
    return;
  }
  if (action === "run") {
    log("Retrying Run");
    await runProject();
    return;
  }
  log("No run action to retry");
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
  const comparisonBaseline = latestValidationComparisonSource();
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
    state.validationComparisonBaseline = comparisonBaseline;
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

function latestValidationComparisonSource() {
  if (state.latestWorkflowRecord?.result?.metrics) return state.latestWorkflowRecord.result;
  if (state.latestWorkflowRecord?.metrics) return state.latestWorkflowRecord;
  return state.latestDataValidation;
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

function seriesInputField() {
  const field = document.createElement("div");
  field.className = "input-field series-input-field";
  const selected = activeSeriesInputSummary();
  const meta = selected
    ? `${selected.step_count || 0} steps / time ${selected.time_key || "step index"}`
    : "current fields preview / context.time";
  field.innerHTML = `
    <label for="runSeriesInputSelect">
      <span class="input-label">Series Input</span>
      <span class="input-meta">${escapeHTML(meta)}</span>
    </label>
    <select id="runSeriesInputSelect" class="run-select"></select>
  `;
  const select = field.querySelector("select");
  select.append(new Option("Current fields preview", ""));
  for (const item of state.detail?.series_inputs || []) {
    const label = `${item.name || item.id || item.relative_path} (${item.step_count || 0} steps)`;
    select.append(new Option(label, item.relative_path || ""));
  }
  select.value = state.activeSeriesInputPath || "";
  select.addEventListener("change", () => {
    state.activeSeriesInputPath = select.value;
    markRunResultStale(false);
    renderSystemHeader();
    renderRunInputs();
    renderRunWorkspace();
    renderStartRuntimeRows();
    log(`Series input selected: ${state.activeSeriesInputPath || "current fields preview"}`);
  });
  return field;
}

function normalizeSeriesInputSelection() {
  if (!state.activeSeriesInputPath) return;
  const exists = (state.detail?.series_inputs || []).some((item) => item.relative_path === state.activeSeriesInputPath);
  if (!exists) state.activeSeriesInputPath = "";
}

function activeSeriesInputSummary() {
  if (!state.activeSeriesInputPath) return null;
  return (state.detail?.series_inputs || []).find((item) => item.relative_path === state.activeSeriesInputPath) || null;
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
    const includeRecords = el("exportIncludeRecordsInput")?.checked ?? true;
    const includeDatasets = el("exportIncludeDatasetsInput")?.checked ?? true;
    const includeCalibration = el("exportIncludeCalibrationInput")?.checked ?? true;
    const includeOptimization = el("exportIncludeOptimizationInput")?.checked ?? true;
    const includeMLAssets = el("exportIncludeMLAssetsInput")?.checked ?? true;
    const includeSDKExamples = el("exportIncludeSDKInput")?.checked ?? true;
    const body = await api("/api/export", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        profile: "runtime_package",
        include_datasets: includeDatasets,
        include_calibration_setups: includeCalibration,
        include_optimization_setups: includeOptimization,
        include_ml_assets: includeMLAssets,
        include_sdk_examples: includeSDKExamples,
        include_records: includeRecords,
      }),
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

async function createComponent(templateOverride = "") {
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
  const selectedTemplate = typeof templateOverride === "string" ? templateOverride : "";
  const template = selectedTemplate || el("componentTemplateSelect")?.value || state.componentTemplates[0]?.id || "";
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

async function createMLComponent() {
  if (!state.componentTemplates.some((template) => template.id === "ml_inference")) {
    showInlineProblem("ML component template is not available");
    return;
  }
  const nameInput = el("newComponentName");
  if (nameInput && !nameInput.value.trim()) nameInput.value = "ML Inference";
  const templateSelect = el("componentTemplateSelect");
  if (templateSelect) {
    templateSelect.value = "ml_inference";
    renderComponentTemplateMeta();
  }
  await createComponent("ml_inference");
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

async function replaceSelectedComponent() {
  const component = componentById(state.selectedComponentId);
  if (!component || !isWorkspaceProject()) return;
  const template = el("componentTemplateSelect")?.value || state.componentTemplates[0]?.id || "";
  const templateContract = selectedComponentTemplate();
  if (!template) {
    showInlineProblem("Component template is required");
    return;
  }
  const preview = templateContract ? replacementPreview(component, templateContract) : { problems: [] };
  if (preview.problems.length) {
    state.latestValidation = { error: "Replacement mapping is broken", problems: preview.problems };
    renderProblems();
    setBottomTab("problems");
    log(`Replace component blocked: ${preview.problems.length} broken mapping${preview.problems.length === 1 ? "" : "s"}`);
    return;
  }
  const name = `${component.name || component.id} Replacement`;
  const mapParameters = state.replacementMapParameters !== false;
  const parameterText = mapParameters ? "same-name parameters will be copied" : "parameters will use template defaults";
  if (!window.confirm(`Create a replacement for ${component.id} from template ${template}? The original component and source will be retained; ${parameterText}.`)) return;
  try {
    const body = await api("/api/project/components/replace", {
      method: "POST",
      body: JSON.stringify({ project_path: state.currentProjectPath, component_id: component.id, name, template, map_parameters: mapParameters }),
    });
    state.detail = body.project;
    state.selectedComponentId = body.component.id;
    markRunResultStale(false);
    renderAll();
    setMode("code");
    const replacement = body.replacement || {};
    log(`Component replacement created: ${component.id} -> ${body.component.id} connections=${replacement.rewired_connections || 0} parameters=${replacement.mapped_parameters || 0}`);
    await validateProject();
  } catch (error) {
    log(`Replace component failed: ${error.message}`);
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

async function updateConnectionUnitConversion(connectionId, unitConversion) {
  if (!connectionId || !isWorkspaceProject()) return;
  try {
    const body = await api("/api/project/connections/update", {
      method: "POST",
      body: JSON.stringify({
        project_path: state.currentProjectPath,
        connection_id: connectionId,
        unit_conversion: unitConversion,
      }),
    });
    state.detail = body.project;
    state.selectedConnectionId = body.connection?.id || connectionId;
    markRunResultStale(false);
    renderAll();
    log(unitConversion ? `Connection conversion saved: ${connectionId}` : `Connection conversion cleared: ${connectionId}`);
  } catch (error) {
    log(`Connection conversion failed: ${error.message}`);
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
    const minNumber = min === "" ? null : Number(min);
    const maxNumber = max === "" ? null : Number(max);
    if (min !== "" && !Number.isFinite(minNumber)) {
      showInlineProblem(`Parameter bounds min must be numeric: ${componentID}.${name}`);
      return;
    }
    if (max !== "" && !Number.isFinite(maxNumber)) {
      showInlineProblem(`Parameter bounds max must be numeric: ${componentID}.${name}`);
      return;
    }
    if (minNumber !== null && maxNumber !== null && minNumber > maxNumber) {
      showInlineProblem(`Parameter bounds min must be <= max: ${componentID}.${name}`);
      return;
    }
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
  el("retryRunButton").disabled = !hasProject || runtimeBusy || !state.lastRuntimeAction;
  el("schemaButton").disabled = !hasProject;
  el("serveButton").disabled = true;
  el("exportButton").disabled = !hasProject || !isWorkspaceProject();
  for (const id of [
    "exportIncludeDatasetsInput",
    "exportIncludeCalibrationInput",
    "exportIncludeOptimizationInput",
    "exportIncludeMLAssetsInput",
    "exportIncludeSDKInput",
    "exportIncludeRecordsInput",
  ]) {
    el(id).disabled = !hasProject || !isWorkspaceProject();
  }
  el("saveProjectButton").disabled = !hasProject || !isWorkspaceProject();
  el("copyProjectButton").disabled = !hasProject;
  el("datasetSourcePathInput").disabled = !hasProject || !isWorkspaceProject();
  el("datasetIDInput").disabled = !hasProject || !isWorkspaceProject();
  el("datasetDelimiterSelect").disabled = !hasProject || !isWorkspaceProject();
  el("validationMissingPolicySelect").disabled = !hasProject || !isWorkspaceProject();
  el("importDatasetButton").disabled = !hasProject || !isWorkspaceProject() || runtimeBusy;
  el("createCalibrationSetupButton").disabled = !hasProject || !isWorkspaceProject() || !(state.detail?.validation_mappings || []).length;
  el("createOptimizationSetupButton").disabled = !hasProject || !isWorkspaceProject() || runtimeBusy;
  el("addComponentButton").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("newMLComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.componentTemplates.some((template) => template.id === "ml_inference");
  el("newComponentName").disabled = !hasProject || !isWorkspaceProject();
  el("componentCategorySelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("componentExecutionModeSelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("componentTemplateSelect").disabled = !hasProject || !isWorkspaceProject() || state.componentTemplates.length === 0;
  el("includeComponentOnCreate").disabled = !hasProject || !isWorkspaceProject();
  el("autoLayoutButton").disabled = !hasProject || !isWorkspaceProject();
  el("includeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || selectedComponentInSystem();
  el("removeComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || !selectedComponentInSystem();
  el("replaceComponentButton").disabled = !hasProject || !isWorkspaceProject() || !state.selectedComponentId || state.componentTemplates.length === 0;
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
  el("newMLComponentButton").addEventListener("click", createMLComponent);
  el("autoLayoutButton").addEventListener("click", autoLayoutCanvas);
  el("newComponentName").addEventListener("keydown", (event) => {
    if (event.key === "Enter") createComponent();
  });
  el("componentCategorySelect").addEventListener("change", renderComponentTemplateSelect);
  el("componentExecutionModeSelect").addEventListener("change", renderComponentTemplateSelect);
  el("componentTemplateSelect").addEventListener("change", () => {
    renderComponentTemplateMeta();
    renderInspector();
  });
  el("includeComponentButton").addEventListener("click", includeSelectedComponent);
  el("removeComponentButton").addEventListener("click", removeSelectedComponentFromSystem);
  el("replaceComponentButton").addEventListener("click", replaceSelectedComponent);
  el("deleteComponentButton").addEventListener("click", deleteSelectedComponent);
  el("saveProjectButton").addEventListener("click", saveProjectEdits);
  el("validateButton").addEventListener("click", validateProject);
  el("dataValidateButton").addEventListener("click", runDataValidation);
  el("runButton").addEventListener("click", runProject);
  el("seriesButton").addEventListener("click", runSeries);
  el("scenarioButton").addEventListener("click", createScenario);
  el("batchButton").addEventListener("click", runBatch);
  el("cancelRunButton").addEventListener("click", cancelActiveRun);
  el("retryRunButton").addEventListener("click", retryLastRuntimeAction);
  el("schemaButton").addEventListener("click", exportSchema);
  el("exportButton").addEventListener("click", exportProject);
  el("importDatasetButton").addEventListener("click", importDataset);
  el("datasetSourcePathInput").addEventListener("keydown", (event) => {
    if (event.key === "Enter") importDataset();
  });
  el("createCalibrationSetupButton").addEventListener("click", () => openCalibrationSetupEditor());
  el("createOptimizationSetupButton").addEventListener("click", openOptimizationSetupEditor);
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
