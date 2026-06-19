import { escapeHTML } from "./dom.js";
import { coerceInput, formatValue } from "./format.js";
import { resultTable } from "./result-ui.js";

export function datasetMappingEditorSection(dataset, context = {}) {
  const block = document.createElement("div");
  block.className = "result-block dataset-mapping-editor";
  block.innerHTML = `<div class="result-block-title">Mapping Editor</div>`;

  const timeField = document.createElement("div");
  timeField.className = "mapping-time-field";
  const timeSelect = columnSelect(dataset.columns || [], dataset.suggested_time_column || firstDatasetColumn(dataset.columns || [], "time", "timestamp"));
  timeSelect.id = "datasetTimeColumnSelect";
  timeSelect.setAttribute("aria-label", "Dataset time column");
  timeField.innerHTML = `<label for="datasetTimeColumnSelect">Time column</label>`;
  timeField.append(timeSelect);
  block.append(timeField);

  block.append(validationMappingEditorTable("Public Inputs", "input", dataset.suggested_inputs || [], dataset.columns || []));
  block.append(validationMappingEditorTable("Observed Outputs", "output", dataset.suggested_outputs || [], dataset.columns || []));
  const samplePreview = datasetSampleRowPreview(dataset);
  block.append(samplePreview);
  block.append(datasetSampleEvaluationSection(dataset, context));
  block.append(datasetUnitHintTable(dataset));
  block.addEventListener("change", (event) => {
    if (event.target?.matches("[data-validation-direction], #datasetTimeColumnSelect")) {
      updateSampleRowPreview(dataset, samplePreview);
    }
  });
  return block;
}

export function suggestionRows(direction, suggestions) {
  return suggestions.map((item) => [
    direction,
    item.public_id || "",
    item.column || "unmatched",
    [item.value_type || "", item.unit || "", item.required ? "required" : "optional"].filter(Boolean).join(" / "),
  ]);
}

export function previewRowsSection(dataset) {
  const columns = dataset.columns || [];
  const rows = (dataset.preview_rows || []).map((row) => columns.map((column) => row[column] ?? ""));
  return resultTable("Preview Rows", rows, columns);
}

export function collectValidationColumnMap(direction) {
  const values = {};
  document.querySelectorAll(`[data-validation-direction="${direction}"]`).forEach((select) => {
    if (select.dataset.publicId && select.value) values[select.dataset.publicId] = select.value;
  });
  return values;
}

export function collectDatasetUnitHints() {
  const values = {};
  document.querySelectorAll("[data-dataset-unit-hint]").forEach((input) => {
    const column = input.dataset.datasetUnitHint || "";
    const unit = input.value.trim();
    if (column && unit) values[column] = unit;
  });
  return values;
}

export function datasetSampleEvaluationPayload(dataset, root = document) {
  const sample = dataset.preview_rows?.[0] || null;
  if (!sample) {
    return { error: "Dataset preview has no sample row to evaluate" };
  }
  const inputs = {};
  const observed = {};
  root.querySelectorAll("[data-validation-direction]").forEach((select) => {
    const column = select.value || "";
    const publicID = select.dataset.publicId || "";
    if (!column || !publicID) return;
    const value = coerceInput(String(sample[column] ?? ""));
    if (select.dataset.validationDirection === "output") {
      observed[publicID] = value;
      return;
    }
    inputs[publicID] = value;
  });
  if (!Object.keys(inputs).length) {
    return { error: "Map at least one public input before evaluating a sample row" };
  }
  const context = {};
  const timeColumn = root.querySelector("#datasetTimeColumnSelect")?.value || "";
  if (timeColumn) {
    context.time = coerceInput(String(sample[timeColumn] ?? ""));
  }
  return {
    context,
    inputs,
    observed,
    sample_row: sample,
    time_column: timeColumn,
  };
}

function validationMappingEditorTable(title, direction, suggestions, columns) {
  const wrapper = document.createElement("div");
  wrapper.className = "mapping-editor-table";
  const rows = suggestions || [];
  wrapper.innerHTML = `
    <div class="result-block-subtitle">${escapeHTML(title)}</div>
    <table class="result-table">
      <thead><tr><th>Public ID</th><th>Contract</th><th>Dataset Column</th><th>Status</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = wrapper.querySelector("tbody");
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No ${escapeHTML(title.toLowerCase())}</td></tr>`;
    return wrapper;
  }
  for (const item of rows) {
    const row = document.createElement("tr");
    const select = columnSelect(columns, item.column || "");
    select.dataset.validationDirection = direction;
    select.dataset.publicId = item.public_id || "";
    const requiredMissing = item.required && !select.value;
    if (requiredMissing) row.classList.add("mapping-missing");
    row.innerHTML = `
      <td>${escapeHTML(item.public_id || "")}</td>
      <td>${escapeHTML([item.value_type || "", item.unit || "", item.required ? "required" : "optional"].filter(Boolean).join(" / "))}</td>
      <td></td>
      <td>${escapeHTML(requiredMissing ? "required column missing" : (select.value ? "mapped" : "unmapped"))}</td>
    `;
    row.children[2].append(select);
    tbody.append(row);
  }
  return wrapper;
}

function datasetSampleEvaluationSection(dataset, context) {
  const wrapper = document.createElement("div");
  wrapper.className = "mapping-editor-table sample-row-evaluation";
  wrapper.innerHTML = `
    <div class="result-block-subtitle">Sample Row Evaluation</div>
    <div class="result-actions"></div>
    <div class="sample-evaluation-output"><div class="empty-cell">Evaluate the first preview row before saving the mapping.</div></div>
  `;
  const actions = wrapper.querySelector(".result-actions");
  const output = wrapper.querySelector(".sample-evaluation-output");
  const button = document.createElement("button");
  button.type = "button";
  button.className = "small-action";
  button.textContent = "Evaluate Sample";
  button.disabled = typeof context.evaluateSample !== "function";
  button.addEventListener("click", async () => {
    const payload = datasetSampleEvaluationPayload(dataset, wrapper.closest(".dataset-mapping-editor") || document);
    if (payload.error) {
      renderSampleEvaluationError(output, payload.error);
      return;
    }
    button.disabled = true;
    output.innerHTML = `<div class="empty-cell">Evaluating sample row...</div>`;
    try {
      const evaluation = await context.evaluateSample(payload);
      renderSampleEvaluationResult(output, evaluation);
    } catch (error) {
      renderSampleEvaluationError(output, error.message || "Sample evaluation failed");
    } finally {
      button.disabled = false;
    }
  });
  actions.append(button);
  return wrapper;
}

function renderSampleEvaluationResult(container, evaluation) {
  if (!container) return;
  container.innerHTML = "";
  const result = evaluation?.result || {};
  const outputs = result.outputs || {};
  const observed = evaluation?.observed || {};
  const outputIDs = Array.from(new Set([...Object.keys(outputs), ...Object.keys(observed)])).sort();
  const rows = outputIDs.map((id) => {
    const simulated = outputs[id];
    const measured = observed[id];
    return [
      id,
      formatValue(simulated),
      measured === undefined ? "" : formatValue(measured),
      sampleDelta(simulated, measured),
    ];
  });
  container.append(resultTable("Sample Output Comparison", rows, ["Public Output", "Simulated", "Observed", "Delta"]));
  container.append(resultTable("Sample Inputs", Object.entries(evaluation?.inputs || {}).map(([id, value]) => [id, formatValue(value)]), ["Public Input", "Value"]));
}

function renderSampleEvaluationError(container, message) {
  if (!container) return;
  container.innerHTML = `<div class="empty-cell">Sample evaluation failed: ${escapeHTML(message)}</div>`;
}

function sampleDelta(simulated, observed) {
  const left = Number(simulated);
  const right = Number(observed);
  if (!Number.isFinite(left) || !Number.isFinite(right)) return "";
  return formatValue(left - right);
}

function datasetSampleRowPreview(dataset) {
  const wrapper = document.createElement("div");
  wrapper.className = "mapping-editor-table sample-row-preview";
  wrapper.innerHTML = `
    <div class="result-block-subtitle">Sample Row Preview</div>
    <table class="result-table">
      <thead><tr><th>Role</th><th>Public ID</th><th>Dataset Column</th><th>Sample Value</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  queueMicrotask(() => updateSampleRowPreview(dataset, wrapper));
  return wrapper;
}

function updateSampleRowPreview(dataset, wrapper) {
  const tbody = wrapper.querySelector("tbody");
  if (!tbody) return;
  renderSampleRowPreview(dataset, tbody, wrapper.closest(".dataset-mapping-editor") || document);
}

function renderSampleRowPreview(dataset, tbody, root) {
  const sample = dataset.preview_rows?.[0] || {};
  const rows = [];
  const timeColumn = root.querySelector("#datasetTimeColumnSelect")?.value || "";
  if (timeColumn) {
    rows.push({ role: "time", publicID: "context.time", column: timeColumn, value: sample[timeColumn] ?? "" });
  }
  root.querySelectorAll("[data-validation-direction]").forEach((select) => {
    const direction = select.dataset.validationDirection === "output" ? "observed output" : "public input";
    const column = select.value || "";
    rows.push({
      role: direction,
      publicID: select.dataset.publicId || "",
      column: column || "unmapped",
      value: column ? sample[column] ?? "" : "",
    });
  });
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No mapped sample values</td></tr>`;
    return;
  }
  tbody.innerHTML = rows.map((row) => `
    <tr class="${row.column === "unmapped" ? "mapping-missing" : ""}">
      <td>${escapeHTML(row.role)}</td>
      <td>${escapeHTML(row.publicID)}</td>
      <td>${escapeHTML(row.column)}</td>
      <td>${escapeHTML(row.value)}</td>
    </tr>
  `).join("");
}

function datasetUnitHintTable(dataset) {
  const wrapper = document.createElement("div");
  wrapper.className = "mapping-editor-table";
  const columns = dataset.columns || [];
  wrapper.innerHTML = `
    <div class="result-block-subtitle">Column Unit Hints</div>
    <table class="result-table">
      <thead><tr><th>Column</th><th>Detected Type</th><th>Unit Hint</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = wrapper.querySelector("tbody");
  if (!columns.length) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty-cell">No columns</td></tr>`;
    return wrapper;
  }
  const profiles = new Map((dataset.column_profiles || []).map((item) => [item.column, item]));
  for (const column of columns) {
    const input = document.createElement("input");
    input.type = "text";
    input.className = "unit-hint-input";
    input.dataset.datasetUnitHint = column;
    input.value = unitHintForColumn(dataset, column);
    input.placeholder = "unit";
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(column)}</td>
      <td>${escapeHTML(profiles.get(column)?.value_type || "")}</td>
      <td></td>
    `;
    row.children[2].append(input);
    tbody.append(row);
  }
  return wrapper;
}

function columnSelect(columns, selected) {
  const select = document.createElement("select");
  select.className = "run-select mapping-column-select";
  select.append(new Option("Unmapped", ""));
  for (const column of columns || []) {
    select.append(new Option(column, column));
  }
  select.value = selected || "";
  return select;
}

function firstDatasetColumn(columns, ...candidates) {
  for (const candidate of candidates) {
    const normalized = normalizeColumnLabel(candidate);
    const match = (columns || []).find((column) => normalizeColumnLabel(column) === normalized);
    if (match) return match;
  }
  return "";
}

function normalizeColumnLabel(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, "");
}

function unitHintForColumn(dataset, column) {
  for (const item of [...(dataset.suggested_inputs || []), ...(dataset.suggested_outputs || [])]) {
    if (item.column === column && item.unit) return item.unit;
  }
  return "";
}
