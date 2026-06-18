import { el, escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";
import { state } from "./state.js";

export function renderLogs(context) {
  const panel = el("logsPanel");
  if (!panel) return;
  panel.innerHTML = "";
  const controls = document.createElement("div");
  controls.className = "log-controls";

  const severity = document.createElement("select");
  severity.id = "logSeverityFilter";
  for (const [value, label] of [["all", "All"], ["app", "Studio"], ["info", "Info"], ["warning", "Warning"], ["error", "Error"]]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    severity.append(option);
  }
  severity.value = state.logSeverityFilter || "all";

  const search = document.createElement("input");
  search.id = "logTextFilter";
  search.type = "search";
  search.placeholder = "Filter logs";
  search.value = state.logTextFilter || "";

  const exportButton = document.createElement("button");
  exportButton.type = "button";
  exportButton.id = "exportLogBundleButton";
  exportButton.textContent = "Export Logs";
  exportButton.addEventListener("click", () => downloadLogBundle(context));

  const rows = document.createElement("div");
  rows.className = "log-rows";
  const updateRows = () => renderLogRows(rows, context);
  severity.addEventListener("change", () => {
    state.logSeverityFilter = severity.value;
    updateRows();
  });
  search.addEventListener("input", () => {
    state.logTextFilter = search.value;
    updateRows();
  });
  controls.append(severity, search, exportButton);
  panel.append(controls, rows);
  updateRows();
}

function renderLogRows(container, context) {
  const rows = filteredLogRows(context);
  container.innerHTML = "";
  if (!rows.length) {
    const empty = document.createElement("div");
    empty.className = "log-empty";
    empty.textContent = "No logs match the current filter";
    container.append(empty);
    return;
  }
  for (const item of rows) {
    const row = document.createElement("div");
    row.className = `log-row ${logSeverityClassName(item.severity)}`;
    const time = item.time !== undefined && item.time !== null && item.time !== "" ? `time ${formatValue(item.time)}` : "";
    row.innerHTML = `
      <span class="log-row-meta">${escapeHTML([item.source, item.component, item.stage, item.stream, time, item.location].filter(Boolean).join(" / "))}</span>
      <span class="log-row-severity">${escapeHTML(item.severity || "info")}</span>
      <span class="log-row-message">${escapeHTML(item.message || "")}</span>
    `;
    container.append(row);
  }
}

function filteredLogRows(context) {
  const severity = state.logSeverityFilter || "all";
  const needle = String(state.logTextFilter || "").trim().toLowerCase();
  return combinedLogRows(context).filter((item) => {
    if (severity !== "all" && item.severity !== severity && item.source !== severity) return false;
    if (!needle) return true;
    return [item.source, item.component, item.stage, item.stream, item.severity, item.time, item.location, item.message]
      .filter(Boolean)
      .join(" ")
      .toLowerCase()
      .includes(needle);
  });
}

function combinedLogRows(context) {
  const appLogs = (state.logs || []).map((message) => ({
    source: "app",
    severity: "app",
    message,
  }));
  const runtimeLogs = (context.latestRuntimeResult()?.component_logs || []).map((entry) => ({
    source: "runtime",
    component: entry.component || "",
    stage: entry.stage || "",
    stream: entry.stream || "",
    severity: String(entry.severity || "info").toLowerCase(),
    time: entry.time ?? "",
    location: logSourceLocation(entry),
    message: entry.message || "",
  }));
  return [...runtimeLogs, ...appLogs];
}

function logSourceLocation(entry) {
  const source = entry?.source || "";
  const line = entry?.line ? `:${entry.line}${entry.column ? `:${entry.column}` : ""}` : "";
  return `${source}${line}`;
}

function logSeverityClassName(severity) {
  const normalized = String(severity || "info").toLowerCase();
  if (normalized === "error" || normalized === "warning" || normalized === "info" || normalized === "app") return normalized;
  return "info";
}

function downloadLogBundle(context) {
  const project = state.detail?.project?.project_name || "hvac-studio";
  const bundle = {
    project: state.detail?.project?.project_name || "",
    project_path: state.currentProjectPath || "",
    active_component: state.selectedComponentId || "",
    latest_result_source: state.latestRunSource || "",
    filters: {
      severity: state.logSeverityFilter || "all",
      text: state.logTextFilter || "",
    },
    logs: combinedLogRows(context),
  };
  context.downloadTextFile(`${context.safeFileName(project)}-logs.json`, `${JSON.stringify(bundle, null, 2)}\n`, "application/json;charset=utf-8");
}
