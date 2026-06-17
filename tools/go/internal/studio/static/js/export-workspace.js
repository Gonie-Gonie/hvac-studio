import { escapeHTML } from "./dom.js";

export function renderExportWorkspace(state, summaryRows, fileRows, manifestPane) {
  const manifest = exportManifestFor(state);
  renderSummaryRows(state, manifest, summaryRows);
  renderFileRows(manifest, fileRows);
  if (manifestPane) manifestPane.textContent = JSON.stringify(manifest, null, 2);
}

function exportManifestFor(state) {
  return state.latestExport || {
    profile: "runtime_package",
    project_root: "project",
    project_path: "project/project.bcsproj",
    graph_path: "project/graph.json",
    default_input: state.detail?.project?.default_input ? `project/${state.detail.project.default_input}` : "",
    interface_schema: "schema/public-io.json",
    runner: "bin/bcs-runner.exe",
    runtime_python: "runtime/python/python.exe",
    files: [],
  };
}

function renderSummaryRows(state, manifest, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const project = currentProject(state);
  const status = manifest.created_at_utc ? "written" : project?.source === "workspace" ? "ready" : "read only";
  const exportFolder = exportFolderFor(state.latestExportSummary, manifest);
  const rows = [
    ["Profile", manifest.profile || "runtime_package"],
    ["Status", status],
    ["Export folder", exportFolder],
    ["Project", manifest.project_path || ""],
    ["Default input", manifest.default_input || ""],
    ["Interface schema", manifest.interface_schema || ""],
    ["Self check", "bin/bcs-env.exe check --root ."],
    ["Commands", (manifest.commands || ["run-default.ps1"]).join(", ")],
    ["Include datasets", includeOptionLabel(manifest, "include_datasets", "exportIncludeDatasetsInput", true)],
    ["Include calibration setups", includeOptionLabel(manifest, "include_calibration_setups", "exportIncludeCalibrationInput", true)],
    ["Include optimization setups", includeOptionLabel(manifest, "include_optimization_setups", "exportIncludeOptimizationInput", true)],
    ["Include ML assets", includeOptionLabel(manifest, "include_ml_assets", "exportIncludeMLAssetsInput", true)],
    ["Include SDK examples", includeOptionLabel(manifest, "include_sdk_examples", "exportIncludeSDKInput", true)],
    ["Include records", includeOptionLabel(manifest, "include_records", "exportIncludeRecordsInput", true)],
    ["Records", recordCount(manifest).toString()],
    ["Files", String((manifest.files || []).length)],
  ];
  if (manifest.created_at_utc) rows.splice(2, 0, ["Created", manifest.created_at_utc]);
  for (const [name, value] of rows) tbody.append(row([name, value]));
}

function includeOptionLabel(manifest, key, inputID, defaultValue) {
  if (manifest.created_at_utc && manifest[key] !== undefined) return manifest[key] ? "yes" : "no";
  const input = document.getElementById(inputID);
  return (input?.checked ?? defaultValue) ? "yes" : "no";
}

function recordCount(manifest) {
  return [
    ...(manifest.run_records || []),
    ...(manifest.batch_records || []),
    ...(manifest.validation_records || []),
    ...(manifest.calibration_records || []),
    ...(manifest.optimization_records || []),
  ].length;
}

function exportFolderFor(summary, manifest) {
  if (summary?.relative_path) return summary.relative_path.replace(/\/manifest\.json$/, "");
  return `exports/${manifest.profile || "runtime_package"}`;
}

function renderFileRows(manifest, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const files = manifest.files || [];
  if (!files.length) {
    tbody.innerHTML = `<tr><td colspan="2" class="empty-cell">No files written yet</td></tr>`;
    return;
  }
  for (const file of files) tbody.append(row([file, "written"]));
}

function row(values) {
  const tr = document.createElement("tr");
  tr.innerHTML = values.map((value) => `<td>${escapeHTML(value)}</td>`).join("");
  return tr;
}

function currentProject(state) {
  return state.projects.find((project) => project.project_path === state.currentProjectPath);
}
