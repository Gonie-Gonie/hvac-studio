import { escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";

export function renderRunOutputWorkspace(state, summary, chart) {
  if (!summary || !chart) return;
  renderRunSummary(state, summary);
  renderOutputChart(state, chart);
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
  return [
    { name: "Record", value: record.id || "batch", source: "batch" },
    { name: "Created", value: record.created_at_utc || "", source: "batch" },
    { name: "Cases", value: `${okCount}/${cases.length} ok`, source: "batch" },
    ...resultSummaryRows(firstBatchResult({ latestBatchRecord: record }), "first ok case"),
  ];
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
  const result = state.latestResult || state.latestRunRecord?.result || firstBatchResult(state);
  const outputs = result?.outputs || {};
  return Object.entries(outputs)
    .filter(([, value]) => typeof value === "number" && Number.isFinite(value))
    .map(([id, value]) => ({ id, value }));
}

function firstBatchResult(state) {
  const cases = state.latestBatchRecord?.cases || [];
  const found = cases.find((item) => item.ok && item.result?.outputs);
  return found?.result || null;
}
