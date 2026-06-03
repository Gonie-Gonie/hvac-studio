import { escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";

export function renderRunOutputWorkspace(state, summary, outputRows, chart, componentRows) {
  if (!summary || !outputRows || !chart) return;
  renderRunSummary(state, summary);
  renderPublicOutputs(state, outputRows);
  renderOutputChart(state, chart);
  renderSelectedComponentValues(state, componentRows);
}

function renderRunSummary(state, summary) {
  summary.innerHTML = "";
  const rows = latestSummaryRows(state);
  if (!rows.length) {
    summary.innerHTML = `<tr><td colspan="3" class="empty-cell">No run yet</td></tr>`;
    return;
  }
  for (const item of rows) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(item.name)}</td>
      <td>${escapeHTML(item.value)}</td>
      <td>${escapeHTML(item.source)}</td>
    `;
    summary.append(row);
  }
}

function renderPublicOutputs(state, tbody) {
  tbody.innerHTML = "";
  const context = latestResultContext(state);
  const outputs = context.result?.outputs || {};
  const entries = Object.entries(outputs);
  if (!entries.length) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty-cell">No outputs yet</td></tr>`;
    return;
  }
  for (const [name, value] of entries) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(name)}</td>
      <td>${escapeHTML(formatValue(value))}</td>
      <td>${escapeHTML(context.source)}</td>
    `;
    tbody.append(row);
  }
}

function renderSelectedComponentValues(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";

  const componentID = state.selectedComponentId;
  const context = latestResultContext(state);
  if (!componentID || !context.result) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No component values yet</td></tr>`;
    return;
  }

  const inputs = context.result.component_inputs?.[componentID] || {};
  const outputs = context.result.component_outputs?.[componentID] || {};
  const rows = [
    ...Object.entries(inputs).map(([node, value]) => ({
      direction: "input",
      node,
      value: formatValue(value),
      source: context.source,
    })),
    ...Object.entries(outputs).map(([node, value]) => ({
      direction: "output",
      node,
      value: formatValue(value),
      source: context.source,
    })),
  ];

  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No values for selected component</td></tr>`;
    return;
  }

  for (const item of rows) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(item.direction)}</td>
      <td>${escapeHTML(item.node)}</td>
      <td>${escapeHTML(item.value)}</td>
      <td>${escapeHTML(item.source)}</td>
    `;
    tbody.append(row);
  }
}

function renderOutputChart(state, chart) {
  chart.innerHTML = "";

  const outputs = latestNumericOutputs(state);
  if (!outputs.length) {
    chart.innerHTML = `<div class="chart-empty">Run a case to preview numeric public outputs.</div>`;
    return;
  }

  const maxAbs = Math.max(...outputs.map((item) => Math.abs(item.value)), 1);
  for (const output of outputs) {
    const row = document.createElement("div");
    row.className = "bar-row";
    const width = Math.max(3, (Math.abs(output.value) / maxAbs) * 100);
    row.innerHTML = `
      <div class="bar-label">${escapeHTML(output.id)}</div>
      <div class="bar-track">
        <div class="bar-fill ${output.value < 0 ? "negative" : ""}" style="width: ${width}%"></div>
      </div>
      <div class="bar-value">${escapeHTML(formatValue(output.value))}</div>
    `;
    chart.append(row);
  }
}

function latestSummaryRows(state) {
  if (state.latestBatchRecord) return batchSummaryRows(state.latestBatchRecord);
  if (state.latestRunRecord) return runRecordSummaryRows(state.latestRunRecord);
  if (state.latestResult) return resultSummaryRows(state.latestResult, "current run");
  return [];
}

function runRecordSummaryRows(record) {
  return [
    { name: "Record", value: record.id || "run", source: "saved run" },
    { name: "Created", value: record.created_at_utc || "", source: "saved run" },
    ...resultSummaryRows(record.result, "saved run"),
  ];
}

function batchSummaryRows(record) {
  const cases = record.cases || [];
  const okCount = cases.filter((item) => item.ok).length;
  const firstFailure = cases.find((item) => !item.ok);
  const rows = [
    { name: "Record", value: record.id || "batch", source: "batch" },
    { name: "Created", value: record.created_at_utc || "", source: "batch" },
    { name: "Cases", value: `${okCount}/${cases.length} ok`, source: "batch" },
  ];
  if (firstFailure) {
    rows.push({
      name: "First failure",
      value: `${firstFailure.scenario_name || firstFailure.scenario_id || "case"}: ${firstFailure.error || "failed"}`,
      source: "batch",
    });
  }
  rows.push(...resultSummaryRows(firstBatchResult({ latestBatchRecord: record }), "first ok case"));
  return rows;
}

function resultSummaryRows(result, source) {
  if (!result) return [];
  const outputs = Object.keys(result.outputs || {}).length;
  const components = Object.keys(result.component_outputs || {}).length;
  const executionOrder = (result.execution_order || []).join(" -> ");
  return [
    { name: "Public outputs", value: String(outputs), source },
    { name: "Components", value: String(components), source },
    { name: "Execution order", value: executionOrder || "n/a", source },
  ];
}

function latestNumericOutputs(state) {
  const outputs = latestResultContext(state).result?.outputs || {};
  return Object.entries(outputs)
    .filter(([, value]) => typeof value === "number" && Number.isFinite(value))
    .map(([id, value]) => ({ id, value }));
}

function latestResultContext(state) {
  if (state.latestBatchRecord) return { result: firstBatchResult(state), source: "first ok case" };
  if (state.latestRunRecord) return { result: state.latestRunRecord.result, source: "saved run" };
  if (state.latestResult) return { result: state.latestResult, source: "current run" };
  return { result: null, source: "" };
}

function firstBatchResult(state) {
  const cases = state.latestBatchRecord?.cases || [];
  const found = cases.find((item) => item.ok && item.result?.outputs);
  return found?.result || null;
}
