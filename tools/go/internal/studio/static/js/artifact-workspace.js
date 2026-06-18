import { el, escapeHTML } from "./dom.js";
import { state } from "./state.js";

export function renderArtifactWorkspace(context) {
  const tbody = el("artifactRows");
  if (!tbody) return;
  const rows = artifactRows(context);
  tbody.innerHTML = "";
  if (!rows.length) {
    tbody.append(context.emptyRow(6, "No artifacts yet"));
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

function artifactRows(context) {
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
      open: () => context.openArtifactSummary("dataset", item),
    });
  }
  for (const item of state.detail.validation_mappings || []) {
    rows.push({
      type: "Validation Mapping",
      name: item.name || item.id,
      path: item.relative_path,
      state: `${item.input_count || 0} in / ${item.output_count || 0} out`,
      ...sourcePolicy,
      open: () => context.openArtifactSummary("validation_mapping", item),
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
        context.openArtifactSummary("parameter_set", item);
        context.renderProjectTree();
        context.renderRunInputs();
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
      open: () => context.runCalibrationSetup(item),
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
      open: () => context.runOptimizationSetup(item),
    });
  }
  for (const item of state.detail.scenarios || []) {
    rows.push({
      type: "Scenario",
      name: item.name || item.id,
      path: item.relative_path,
      state: item.created_at_utc || "",
      ...sourcePolicy,
      open: () => context.loadScenario(item.id),
    });
  }
  for (const item of state.detail.runs || []) {
    rows.push({
      type: "Run Record",
      name: item.id,
      path: item.relative_path,
      state: item.created_at_utc || "",
      ...recordPolicy,
      open: () => context.loadRunRecord(item.id),
    });
  }
  for (const item of state.detail.batches || []) {
    rows.push({
      type: "Batch Record",
      name: item.id,
      path: item.relative_path,
      state: `${item.ok_count || 0}/${item.case_count || 0} ok`,
      ...recordPolicy,
      open: () => context.loadBatchRecord(item.id),
    });
  }
  for (const item of state.detail.validation_runs || []) {
    rows.push({
      type: "Validation Record",
      name: item.mapping_name || item.mapping_id || item.id,
      path: item.relative_path,
      state: `${item.row_count || 0} rows`,
      ...recordPolicy,
      open: () => context.loadWorkflowRecord("validation", item.id),
    });
  }
  for (const item of state.detail.calibration_results || []) {
    rows.push({
      type: "Calibration Record",
      name: item.setup_name || item.setup_id || item.id,
      path: item.relative_path,
      state: `best ${context.shortNumber(item.best_objective)}`,
      ...recordPolicy,
      open: () => context.loadWorkflowRecord("calibration", item.id),
    });
  }
  for (const item of state.detail.optimization_results || []) {
    rows.push({
      type: "Optimization Record",
      name: item.setup_name || item.setup_id || item.id,
      path: item.relative_path,
      state: `best ${context.shortNumber(item.best_objective)}`,
      ...recordPolicy,
      open: () => context.loadWorkflowRecord("optimization", item.id),
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
      open: () => context.loadExportRecord(item.profile),
    });
  }
  return rows;
}

export function datasetTreeItems(context) {
  return (state.detail?.datasets || []).map((item) => {
    const row = context.treeStatic(item.name || item.id, item.relative_path || "dataset");
    row.addEventListener("click", () => context.openArtifactSummary("dataset", item));
    return row;
  });
}

export function parameterSetTreeItems(context) {
  return (state.detail?.parameter_sets || []).map((item) => parameterSetTreeItem(item, context));
}

export function validationMappingTreeItems(context) {
  return (state.detail?.validation_mappings || []).map((item) => {
    const row = context.treeStatic(item.name || item.id, `${item.relative_path || "mapping"} / ${item.input_count || 0} in / ${item.output_count || 0} out`);
    row.addEventListener("click", () => context.openArtifactSummary("validation_mapping", item));
    return row;
  });
}

export function validationRunTreeItems(context) {
  return (state.detail?.validation_runs || []).map((item) => workflowRecordTreeItem("validation", item, item.mapping_name || item.mapping_id || item.id, `${item.row_count || 0} rows`, context));
}

export function calibrationResultTreeItems(context) {
  return (state.detail?.calibration_results || []).map((item) => workflowRecordTreeItem("calibration", item, item.setup_name || item.setup_id || item.id, `best ${context.shortNumber(item.best_objective)}`, context));
}

export function optimizationResultTreeItems(context) {
  return (state.detail?.optimization_results || []).map((item) => workflowRecordTreeItem("optimization", item, item.setup_name || item.setup_id || item.id, `best ${context.shortNumber(item.best_objective)}`, context));
}

function workflowRecordTreeItem(kind, item, label, meta, context) {
  const row = context.treeStatic(label, meta || item.relative_path || kind);
  row.addEventListener("click", () => context.loadWorkflowRecord(kind, item.id));
  return row;
}

function parameterSetTreeItem(item, context) {
  const row = context.treeStatic(item.name || item.id, item.relative_path || "parameter set");
  if (state.activeParameterSetPath === item.relative_path) row.classList.add("active");
  row.addEventListener("click", () => {
    state.activeParameterSetPath = item.relative_path || "";
    context.renderProjectTree();
    context.renderRunInputs();
    context.renderStartRuntimeRows();
    context.log(`Parameter set selected: ${state.activeParameterSetPath || "baseline"}`);
  });
  return row;
}
