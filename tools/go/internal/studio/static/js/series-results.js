import { csvCell, downloadTextFile, safeFileName } from "./download.js";
import { formatValue } from "./format.js";
import { resultTable } from "./result-ui.js";

export function seriesResultSection(series, context = {}) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const actions = document.createElement("div");
  actions.className = "result-actions";

  const exportCSV = document.createElement("button");
  exportCSV.type = "button";
  exportCSV.className = "small-action";
  exportCSV.textContent = "Export Series CSV";
  exportCSV.addEventListener("click", () => downloadSeriesCSV(series, context));

  const exportJSON = document.createElement("button");
  exportJSON.type = "button";
  exportJSON.className = "small-action";
  exportJSON.textContent = "Export Series JSON";
  exportJSON.addEventListener("click", () => downloadSeriesJSON(series, context));

  actions.append(exportCSV, exportJSON);
  section.append(actions);
  section.append(resultTable("Public Output Series", seriesOutputRows(series), ["Output", "Values"]));
  section.append(resultTable("Time Indexed Steps", seriesStepRows(series), ["#", "Step", "Time", "Inputs", "Outputs", "Trace"]));
  const componentRows = selectedComponentSeriesRows(series, context.selectedComponentId);
  if (componentRows.length) {
    section.append(resultTable(`Selected Component Timeline: ${context.selectedComponentId}`, componentRows, ["#", "Inputs", "Outputs", "State"]));
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

function selectedComponentSeriesRows(series, componentID) {
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

function downloadSeriesJSON(series, context) {
  const project = context.projectName || "series";
  const name = `${safeFileName(project)}-series.json`;
  downloadTextFile(name, `${JSON.stringify(series, null, 2)}\n`, "application/json;charset=utf-8");
}

function downloadSeriesCSV(series, context) {
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
  const project = context.projectName || "series";
  downloadTextFile(`${safeFileName(project)}-series.csv`, csv, "text/csv;charset=utf-8");
}

function sortedSeriesKeys(points, field) {
  const keys = new Set();
  for (const point of points || []) {
    for (const key of Object.keys(point[field] || {})) keys.add(key);
  }
  return Array.from(keys).sort();
}
