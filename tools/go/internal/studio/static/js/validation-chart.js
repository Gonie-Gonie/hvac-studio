import { escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";

export function renderValidationWorkspace(state, metrics, chart) {
  if (!metrics || !chart) return;
  metrics.innerHTML = `<tr><td colspan="6" class="empty-cell">No validation run</td></tr>`;
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

function latestNumericOutputs(state) {
  const result = state.latestResult || state.latestRunRecord?.result || firstBatchResult(state);
  const outputs = result?.outputs || {};
  return Object.entries(outputs)
    .filter(([, value]) => typeof value === "number" && Number.isFinite(value))
    .map(([id, value]) => ({ id, value }));
}

function firstBatchResult(state) {
  const cases = state.latestBatchRecord?.cases || [];
  const found = cases.find((item) => item.result?.outputs);
  return found?.result || null;
}
