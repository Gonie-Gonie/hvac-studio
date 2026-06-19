import {
  datasetMappingEditorSection,
  previewRowsSection,
  suggestionRows,
} from "./dataset-mapping.js";
import { formatValue } from "./format.js";
import { resultTable } from "./result-ui.js";

export function datasetResultSection(dataset, context = {}) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(resultTable("Dataset", [
    ["Path", dataset.summary?.relative_path || ""],
    ["Shape", `${dataset.summary?.row_count || 0} rows / ${dataset.summary?.column_count || 0} columns`],
    ["Format", dataset.summary?.format || ""],
    ["Source Encoding", dataset.source_encoding || ""],
    ["Detected Delimiter", dataset.detected_delimiter || ""],
    ["Suggested Time", dataset.suggested_time_column || ""],
    ["SHA256", dataset.summary?.sha256 || ""],
  ].filter(([, value]) => value)));
  section.append(resultTable("Column Profiles", (dataset.column_profiles || []).map((item) => [
    item.column || "",
    item.value_type || "",
    String(item.missing_count || 0),
    (item.samples || []).join(", "),
  ]), ["Column", "Type", "Missing", "Samples"]));
  section.append(datasetMappingEditorSection(dataset, context));
  section.append(resultTable("Public IO Mapping Preview", [
    ...suggestionRows("input", dataset.suggested_inputs || []),
    ...suggestionRows("output", dataset.suggested_outputs || []),
  ], ["Direction", "Public ID", "Column", "Unit"]));
  section.append(previewRowsSection(dataset));
  if (context.isWorkspaceProject) {
    const actions = document.createElement("div");
    actions.className = "result-actions";
    const button = document.createElement("button");
    button.type = "button";
    button.className = "small-action";
    button.textContent = "Create Mapping";
    button.addEventListener("click", () => context.createMapping?.(dataset));
    actions.append(button);
    section.append(actions);
  }
  return section;
}

export function validationMappingArtifactSection(summary, mapping = null, context = {}) {
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
  if (mapping?.unit_hints && Object.keys(mapping.unit_hints).length) {
    section.append(resultTable("Unit Hints", Object.entries(mapping.unit_hints).map(([column, unit]) => [column, unit]), ["Dataset Column", "Unit"]));
  }
  if (context.isWorkspaceProject) {
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
    save.addEventListener("click", () => context.renameMapping?.(summary, nameInput.value));
    const copy = document.createElement("button");
    copy.type = "button";
    copy.className = "small-action";
    copy.textContent = "Copy";
    copy.addEventListener("click", () => context.copyMapping?.(summary));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "small-action danger-action";
    remove.textContent = "Delete";
    remove.addEventListener("click", () => context.deleteMapping?.(summary));
    actions.append(nameInput, save, copy, remove);
    section.append(actions);
  }
  return section;
}

export function parameterSetResultSection(detail, context = {}) {
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
  activate.textContent = context.activeParameterSetPath === detail.summary?.relative_path ? "Active" : "Use for Runs";
  activate.disabled = context.activeParameterSetPath === detail.summary?.relative_path;
  activate.addEventListener("click", () => context.activateForRuns?.(detail.summary?.relative_path || ""));
  actions.append(activate);
  if (context.isWorkspaceProject) {
    const apply = document.createElement("button");
    apply.type = "button";
    apply.className = "small-action";
    apply.textContent = "Apply to Graph";
    apply.addEventListener("click", () => context.applyToGraph?.(detail.summary?.relative_path || ""));
    actions.append(apply);
  }
  section.append(actions);
  return section;
}
