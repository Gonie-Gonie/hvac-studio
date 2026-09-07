import { escapeHTML } from "./dom.js";
import { formatValue, shortNumber } from "./format.js";
import { resultTable } from "./result-ui.js";
import { metricBars, validationPlotSection } from "./validation-plots.js";

export function validationResultSection(result, context = {}) {
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
  const comparisonRows = validationComparisonRows(result, context.comparisonBaseline);
  if (comparisonRows.length) {
    section.append(resultTable("Parameter Set Comparison", comparisonRows, ["Metric", "Baseline", "Current", "RMSE Delta", "MAE Delta", "R2 Delta"]));
  }
  section.append(validationResultActions(result, context));
  section.append(validationComparisonControls(result, context));
  section.append(validationPlotSection(result));
  section.append(highErrorRows(result, context));
  return section;
}

export function validationResultMappingPath(result, validationMappings = []) {
  if (result.mapping) return result.mapping;
  const mappingID = result.mapping_id || "";
  const found = (validationMappings || []).find((item) => item.id === mappingID || item.name === result.mapping_name);
  return found?.relative_path || "";
}

export function highErrorInspectionSection(value, context = {}) {
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
  back.addEventListener("click", () => context.showWorkflowRecord?.(value.validation_result || null));
  actions.append(back);
  section.append(actions);
  return section;
}

export function calibrationValidationComparisonSection(value, context = {}) {
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
  back.addEventListener("click", () => context.showWorkflowRecord?.(value.calibration_result || null));
  actions.append(back);
  section.append(actions);
  return section;
}

function validationResultActions(result, context) {
  const actions = document.createElement("div");
  actions.className = "result-actions validation-result-actions";
  const calibration = document.createElement("button");
  calibration.type = "button";
  calibration.className = "small-action";
  calibration.textContent = "Create Calibration Setup";
  const mappingPath = validationResultMappingPath(result, context.validationMappings);
  calibration.disabled = !context.isWorkspaceProject || !mappingPath;
  calibration.addEventListener("click", () => context.createCalibrationSetup?.(mappingPath));
  actions.append(calibration);
  return actions;
}

function validationComparisonControls(result, context) {
  const actions = document.createElement("div");
  actions.className = "result-actions validation-compare-actions";
  const select = document.createElement("select");
  select.className = "validation-compare-select";
  const currentPath = result.parameter_set || "";
  const choices = [{ label: "Baseline", value: "" }, ...(context.parameterSets || []).map((item) => ({
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
  button.disabled = !choices.length || !validationResultMappingPath(result, context.validationMappings);
  button.addEventListener("click", () => context.compareParameterSet?.(result, select.value));
  actions.append(select, button);
  return actions;
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

function highErrorRows(result, context) {
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
    tr.addEventListener("click", () => context.inspectHighError?.(result, row.metric, row.high));
    tr.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        context.inspectHighError?.(result, row.metric, row.high);
      }
    });
    tbody.append(tr);
  }
  return block;
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

function setResultBlockTitle(block, title) {
  const titleNode = block.querySelector(".result-block-title");
  if (titleNode) titleNode.textContent = title;
}
