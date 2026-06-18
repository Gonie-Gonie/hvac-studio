import { el, escapeHTML } from "./dom.js";
import { state } from "./state.js";

export function renderStartWorkspace(context) {
  renderStartRuntimeRows(context);
  renderStartWorkflowRows(context);
  renderProjectRows(el("startWorkspaceRows"), state.projects.filter((project) => project.source === "workspace"), context);
  renderProjectRows(el("startExampleRows"), state.projects.filter((project) => project.source === "example"), context);
  renderProjectTypeRows();
}

export function renderStartRuntimeRows(context) {
  const tbody = el("startRuntimeRows");
  if (!tbody) return;
  const project = state.detail?.project;
  const summary = context.currentProject();
  const seriesInput = context.activeSeriesInputSummary();
  const rows = [
    ["Runtime", el("runtimeStatus")?.textContent || "Runtime ready"],
    ["Current Project", project?.project_name || "No project open"],
    ["Project File", summary?.relative_path || state.detail?.project_path || "No project open"],
    ["Default Input", project?.default_input || "No default input"],
    ["Run Parameters", state.activeParameterSetPath || "Baseline graph parameters"],
    ["Series Input", seriesInput?.relative_path || "Current fields preview"],
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

function renderProjectRows(tbody, projects, context) {
  if (!tbody) return;
  tbody.innerHTML = "";
  if (!projects.length) {
    tbody.append(context.emptyRow(3, "No projects found"));
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
      context.loadProject(project.project_path);
      context.setMode("canvas");
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

export function renderStartWorkflowRows(context) {
  const tbody = el("startWorkflowRows");
  if (!tbody) return;
  tbody.innerHTML = "";
  const rows = workflowReadinessRows(context);
  if (!rows.length) {
    tbody.append(context.emptyRow(4, "No project open"));
    return;
  }
  for (const item of rows) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${escapeHTML(item.name)}</td>
      <td><span class="status-pill ${escapeHTML(item.level)}">${escapeHTML(item.status)}</span></td>
      <td>${escapeHTML(item.detail)}</td>
      <td class="action-cell"></td>
    `;
    if (item.mode) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "small-action table-action";
      button.textContent = item.action || "Open";
      button.disabled = item.disabled || !state.detail;
      button.addEventListener("click", () => context.setMode(item.mode));
      tr.querySelector(".action-cell").append(button);
    }
    tbody.append(tr);
  }
}

export function workflowReadinessRows(context) {
  if (!state.detail) return [];
  const system = context.currentSystem();
  const components = state.detail.graph?.components || [];
  const connections = state.detail.graph?.connections || [];
  const sourceChecks = Object.values(state.sourceCheckByComponent || {});
  const sourceProblems = sourceChecks.flatMap((check) => check?.problems || []);
  const validationProblems = state.latestValidation?.problems || [];
  const hasValidationError = Boolean(state.latestValidation?.error) || validationProblems.some((problem) => problem.severity === "error");
  const hasSourceError = sourceProblems.some((problem) => problem.severity === "error");
  const datasets = state.detail.datasets || [];
  const mappings = state.detail.validation_mappings || [];
  const calibrationSetups = state.detail.calibration_setups || [];
  const optimizationSetups = state.detail.optimization_setups || [];
  const exports = state.detail.exports || [];
  const run = context.latestRuntimeResult();
  const hasRun = Boolean(run);
  return [
    readinessRow(
      "Editable Project",
      context.isWorkspaceProject(),
      "Ready",
      "Blocked",
      context.isWorkspaceProject() ? context.currentProject()?.relative_path || state.detail.project_path || "Workspace project" : "Open or copy a workspace project",
      "start",
    ),
    readinessRow(
      "System Graph",
      Boolean(system && components.length),
      "Ready",
      "Blocked",
      system ? `${components.length} components / ${connections.length} connections` : "No entry system",
      "canvas",
    ),
    readinessRow(
      "Python Source",
      components.length > 0 && !hasSourceError,
      sourceChecks.length ? "Ready" : "Check",
      "Blocked",
      sourceChecks.length ? `${sourceChecks.length} checks / ${sourceProblems.length} problems` : `${components.length} components pending check`,
      "code",
      "Open",
      components.length === 0,
      sourceChecks.length ? "ready" : "warning",
    ),
    readinessRow(
      "Data Mapping",
      datasets.length > 0 && mappings.length > 0 && !hasValidationError,
      "Ready",
      mappings.length ? "Check" : "Blocked",
      mappings.length ? `${mappings.length} mappings / ${datasets.length} datasets` : "Create a validation mapping",
      "artifacts",
      "Open",
      false,
      mappings.length && hasValidationError ? "warning" : undefined,
    ),
    readinessRow(
      "Calibration",
      calibrationSetups.length > 0,
      "Ready",
      mappings.length ? "Check" : "Blocked",
      calibrationSetups.length ? `${calibrationSetups.length} setups` : "Create setup from validation data",
      "parameters",
      "Open",
      !mappings.length,
      mappings.length ? "warning" : undefined,
    ),
    readinessRow(
      "Optimization",
      optimizationSetups.length > 0,
      "Ready",
      components.length ? "Check" : "Blocked",
      optimizationSetups.length ? `${optimizationSetups.length} setups` : "Create setup from run variables",
      "parameters",
      "Open",
      components.length === 0,
      components.length ? "warning" : undefined,
    ),
    readinessRow(
      "Run",
      hasRun && !state.latestResultStale,
      "Ready",
      hasRun ? "Check" : "Blocked",
      hasRun ? (state.latestResultStale ? "Last result is stale" : "Last result is current") : "Run the current system",
      "run",
    ),
    readinessRow(
      "Export",
      exports.length > 0,
      "Ready",
      hasRun ? "Check" : "Blocked",
      exports.length ? `${exports.length} export records` : "Export after a verified run",
      "export",
      "Open",
      !hasRun,
      hasRun ? "warning" : undefined,
    ),
  ];
}

export function readinessRow(name, ok, okStatus, blockedStatus, detail, mode, action = "Open", disabled = false, levelOverride = "") {
  const level = levelOverride || (ok ? "ready" : "blocked");
  return {
    name,
    status: ok ? okStatus : blockedStatus,
    detail,
    mode,
    action,
    disabled,
    level,
  };
}
