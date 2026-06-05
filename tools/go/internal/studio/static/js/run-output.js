import { escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";

export function renderRunOutputWorkspace(state, summary, outputRows, comparisonRows, chart, componentRows, batchRows, executionRows, componentLogRows, connectionRows, nodeRows) {
  if (!summary || !outputRows || !chart) return;
  renderRunSummary(state, summary);
  renderPublicOutputs(state, outputRows);
  renderRunComparison(state, comparisonRows);
  renderOutputChart(state, chart);
  renderSelectedComponentValues(state, componentRows);
  renderBatchCases(state, batchRows);
  renderExecutionTrace(state, executionRows);
  renderComponentLogs(state, componentLogRows);
  renderConnectionTrace(state, connectionRows);
  renderNodeTrace(state, nodeRows);
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

function renderBatchCases(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const cases = state.latestBatchRecord?.cases || [];
  if (!cases.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No batch run yet</td></tr>`;
    return;
  }
  cases.forEach((item, index) => {
    const outputs = item.ok ? publicOutputSummary(item.result?.outputs || {}) : "";
    const status = item.ok ? "ok" : "failed";
    const error = batchCaseErrorSummary(item);
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(item.scenario_name || item.scenario_id || `case ${index + 1}`)}</td>
      <td><span class="case-status ${status}">${escapeHTML(status)}</span></td>
      <td>${escapeHTML(outputs)}</td>
      <td class="case-error">${escapeHTML(error)}</td>
    `;
    tbody.append(row);
  });
}

function renderExecutionTrace(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const context = latestResultContext(state);
  const timings = context.result?.component_timings || [];
  const maxDuration = Math.max(...timings.map((item) => Number(item.duration_ms) || 0), 0);
  const rows = timings.length
    ? timings.map((item) => ({
      component: item.component || "",
      stage: item.stage || "evaluate",
      duration: formatDuration(item.duration_ms),
      durationMS: Number(item.duration_ms) || 0,
    }))
    : (context.result?.execution_order || []).map((component) => ({
      component,
      stage: "evaluate",
      duration: "",
      durationMS: 0,
    }));
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty-cell">No execution trace yet</td></tr>`;
    return;
  }
  for (const item of rows) {
    const width = maxDuration > 0 ? Math.max(3, (item.durationMS / maxDuration) * 100) : 0;
    const duration = item.duration
      ? `<div class="timing-cell"><span>${escapeHTML(item.duration)}</span><div class="timing-track"><div class="timing-fill" style="width: ${width}%"></div></div></div>`
      : "";
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(item.component)}</td>
      <td>${escapeHTML(item.stage)}</td>
      <td>${duration}</td>
    `;
    tbody.append(row);
  }
}

function renderRunComparison(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const current = latestResultContext(state);
  const baseline = state.runComparisonBaseline;
  if (!current.result || !baseline?.result) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">Run twice or open a saved run to compare outputs</td></tr>`;
    return;
  }

  const currentOutputs = current.result.outputs || {};
  const baselineOutputs = baseline.result.outputs || {};
  const outputNames = Array.from(new Set([...Object.keys(currentOutputs), ...Object.keys(baselineOutputs)])).sort();
  if (!outputNames.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No public outputs to compare</td></tr>`;
    return;
  }

  for (const name of outputNames) {
    const currentValue = currentOutputs[name];
    const baselineValue = baselineOutputs[name];
    const delta = comparisonDelta(currentValue, baselineValue);
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(name)}</td>
      <td><span class="comparison-source">${escapeHTML(current.source)}</span>${escapeHTML(formatComparisonValue(currentValue))}</td>
      <td><span class="comparison-source">${escapeHTML(baseline.source || "baseline")}</span>${escapeHTML(formatComparisonValue(baselineValue))}</td>
      <td><span class="comparison-delta ${delta.className}">${escapeHTML(delta.label)}</span></td>
    `;
    tbody.append(row);
  }
}

function renderComponentLogs(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const context = latestResultContext(state);
  const logs = context.result?.component_logs || [];
  if (!logs.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No component logs yet</td></tr>`;
    return;
  }
  for (const log of logs) {
    const severity = String(log.severity || "info").toLowerCase();
    const severityLabel = [log.severity || "info", log.stream || ""].filter(Boolean).join(" / ");
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(log.component || "")}</td>
      <td>${escapeHTML(log.stage || "")}</td>
      <td><span class="log-severity ${logSeverityClass(severity)}">${escapeHTML(severityLabel)}</span></td>
      <td class="log-message">${escapeHTML(log.message || "")}</td>
    `;
    tbody.append(row);
  }
}

function renderConnectionTrace(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const context = latestResultContext(state);
  const traces = context.result?.connection_values || [];
  if (!traces.length) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty-cell">No connection values yet</td></tr>`;
    return;
  }
  for (const trace of traces) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(connectionLabel(trace))}</td>
      <td>${escapeHTML(formatValue(trace.value))}</td>
      <td>${escapeHTML(traceMeta(trace))}</td>
    `;
    tbody.append(row);
  }
}

function renderNodeTrace(state, tbody) {
  if (!tbody) return;
  tbody.innerHTML = "";
  const context = latestResultContext(state);
  const traces = context.result?.node_values || [];
  if (!traces.length) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty-cell">No node values yet</td></tr>`;
    return;
  }
  for (const trace of traces) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHTML(`${trace.component}.${trace.node} ${trace.direction || ""}`.trim())}</td>
      <td>${escapeHTML(formatValue(trace.value))}</td>
      <td>${escapeHTML(traceMeta(trace))}</td>
    `;
    tbody.append(row);
  }
}

function publicOutputSummary(outputs) {
  const entries = Object.entries(outputs);
  if (!entries.length) return "";
  return entries.map(([name, value]) => `${name}: ${formatValue(value)}`).join(", ");
}

function renderOutputChart(state, chart) {
  chart.innerHTML = "";

  if (state.latestSeriesResult) {
    renderSeriesChart(state.latestSeriesResult, chart);
    return;
  }

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
  if (state.latestSeriesResult) return seriesSummaryRows(state.latestSeriesResult);
  if (state.latestBatchRecord) return batchSummaryRows(state.latestBatchRecord);
  if (state.latestRunRecord) return runRecordSummaryRows(state.latestRunRecord);
  if (state.latestResult) return resultSummaryRows(state.latestResult, "current run");
  if (state.latestValidation?.error) return failureSummaryRows(state.latestValidation);
  return pendingRunSummaryRows(state);
}

function pendingRunSummaryRows(state) {
  const project = state.detail?.project;
  const projectSummary = currentProject(state);
  const inputSource = state.activeRunInput
    ? `scenario ${state.activeRunInput.name || state.activeRunInput.id || "selected"}`
    : project?.default_input || "current fields";
  const parameterSet = state.activeParameterSetPath || "Baseline graph parameters";
  const saveTarget = projectSummary?.source === "workspace" ? "New record under runs/" : "Not saved for read-only project";
  return [
    { name: "Run target", value: project?.project_name || "No project open", source: projectSummary?.relative_path || state.detail?.project_path || "" },
    { name: "Input source", value: inputSource, source: state.activeRunInput?.relative_path || project?.default_input || "Run fields" },
    { name: "Parameter set", value: parameterSet, source: state.activeParameterSetPath ? "parameter_sets" : "graph.json" },
    { name: "Context", value: runContextSummary(state), source: "Run fields" },
    { name: "Save target", value: saveTarget, source: "Run command" },
    { name: "Timeout", value: timeoutSummary(state.runTimeoutMS), source: "Run command" },
  ];
}

function currentProject(state) {
  return (state.projects || []).find((project) => project.project_path === state.currentProjectPath);
}

function runContextSummary(state) {
  const context = state.activeRunInput?.context || state.detail?.default_run_input?.context || { time: 0, dt: 60 };
  const keys = Object.keys(context || {});
  if (!keys.length) return "No context";
  return keys.map((key) => `${key}=${formatValue(context[key])}`).join(", ");
}

function timeoutSummary(timeoutMS) {
  const numeric = Number(timeoutMS);
  if (!Number.isFinite(numeric) || numeric <= 0) return "default";
  return formatDuration(numeric);
}

function seriesSummaryRows(series) {
  const source = seriesSource(series);
  const rows = [
    { name: "Series", value: `${series.step_count || 0} steps`, source },
  ];
  if (typeof series.duration_ms === "number") {
    rows.push({ name: "Duration", value: formatDuration(series.duration_ms), source });
  }
  rows.push({ name: "Final states", value: String(Object.keys(series.final_states || {}).length), source });
  rows.push({ name: "Execution order", value: (series.execution_order || []).join(" -> ") || "n/a", source });
  return rows;
}

function renderSeriesChart(series, chart) {
  const rows = Object.entries(series.outputs || {})
    .map(([id, values]) => ({ id, values: Array.isArray(values) ? values.map((value) => Number(value)) : [] }))
    .filter((item) => item.values.length && item.values.every((value) => Number.isFinite(value)));
  if (!rows.length) {
    chart.innerHTML = `<div class="chart-empty">Series has no numeric public output arrays.</div>`;
    return;
  }
  const allValues = rows.flatMap((item) => item.values);
  const min = Math.min(...allValues);
  const max = Math.max(...allValues);
  const range = Math.max(max - min, 1e-9);
  for (const output of rows) {
    const points = output.values.map((value, index) => {
      const height = max === min ? 50 : 12 + ((value - min) / range) * 88;
      return `<span class="series-point" style="height:${height}%" title="step ${index + 1}: ${escapeHTML(formatValue(value))}"></span>`;
    }).join("");
    const last = output.values[output.values.length - 1];
    const row = document.createElement("div");
    row.className = "series-row";
    row.innerHTML = `
      <div class="bar-label">${escapeHTML(output.id)}</div>
      <div class="series-track">${points}</div>
      <div class="bar-value">${escapeHTML(formatValue(last))}</div>
    `;
    chart.append(row);
  }
}

function seriesSource(series) {
  return series.parameter_set ? `series / ${series.parameter_set}` : "series";
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
  const rows = [
    { name: "Public outputs", value: String(outputs), source },
    { name: "Components", value: String(components), source },
  ];
  if (typeof result.duration_ms === "number") {
    rows.push({ name: "Duration", value: formatDuration(result.duration_ms), source });
  }
  rows.push({ name: "Execution order", value: executionOrder || "n/a", source });
  return rows;
}

function failureSummaryRows(validation) {
  const rows = [
    { name: "Status", value: "failed", source: "latest failure" },
    { name: "Error", value: validation.error || "run failed", source: "latest failure" },
  ];
  const firstProblem = (validation.problems || [])[0];
  if (firstProblem) {
    rows.push({
      name: "First problem",
      value: problemSummary(firstProblem),
      source: "Problems",
    });
  }
  return rows;
}

function batchCaseErrorSummary(item) {
  const problems = item.problems || [];
  const problemText = problems.map(problemSummary).filter(Boolean).join("; ");
  return [item.error || "", problemText].filter(Boolean).join(" / ");
}

function problemSummary(problem) {
  return [
    problem.component_id || "",
    problem.node_id ? `node ${problem.node_id}` : "",
    problem.source ? sourceLocation(problem) : "",
    problem.message || "",
  ].filter(Boolean).join(" / ");
}

function sourceLocation(problem) {
  const line = problem.line ? `:${problem.line}${problem.column ? `:${problem.column}` : ""}` : "";
  return `${problem.source}${line}`;
}

function connectionLabel(trace) {
  const from = trace.from ? `${trace.from.component}.${trace.from.node}` : "";
  const to = trace.to ? `${trace.to.component}.${trace.to.node}` : "";
  return from && to ? `${from} -> ${to}` : trace.id || "connection";
}

function traceMeta(trace) {
  return [
    trace.source_medium && trace.target_medium ? `${trace.source_medium}->${trace.target_medium}` : trace.medium || "",
    trace.value_type || "",
    trace.unit || "",
  ].filter(Boolean).join(" / ");
}

function formatDuration(value) {
  if (typeof value !== "number" || !Number.isFinite(value)) return "";
  if (value <= 0.001) return "<0.001 ms";
  if (value < 1) return `${value.toFixed(3)} ms`;
  if (value < 100) return `${value.toFixed(2)} ms`;
  return `${value.toFixed(0)} ms`;
}

function logSeverityClass(severity) {
  if (severity === "error" || severity === "warning" || severity === "info") return severity;
  return "info";
}

function latestNumericOutputs(state) {
  const outputs = latestResultContext(state).result?.outputs || {};
  return Object.entries(outputs)
    .filter(([, value]) => typeof value === "number" && Number.isFinite(value))
    .map(([id, value]) => ({ id, value }));
}

function latestResultContext(state) {
  if (state.latestSeriesResult) return { result: seriesLastResult(state.latestSeriesResult), source: "last series step" };
  if (state.latestBatchRecord) return { result: firstBatchResult(state), source: "first ok case" };
  if (state.latestRunRecord) return { result: state.latestRunRecord.result, source: "saved run" };
  if (state.latestResult) return { result: state.latestResult, source: state.latestRunSource || "current run" };
  return { result: null, source: "" };
}

function seriesLastResult(series) {
  const points = series?.series || [];
  const point = points[points.length - 1];
  if (!point) return null;
  return {
    outputs: point.outputs || {},
    component_inputs: point.component_inputs || {},
    component_outputs: point.component_outputs || {},
    node_values: point.node_values || [],
    connection_values: point.connection_values || [],
    states: point.states || {},
    context: point.context || {},
    execution_order: series.execution_order || point.execution_order || [],
    component_timings: point.component_timings || [],
    component_logs: point.component_logs || [],
    duration_ms: point.duration_ms,
  };
}

function firstBatchResult(state) {
  const cases = state.latestBatchRecord?.cases || [];
  const found = cases.find((item) => item.ok && item.result?.outputs);
  return found?.result || null;
}

function comparisonDelta(currentValue, baselineValue) {
  if (typeof currentValue === "number" && Number.isFinite(currentValue) && typeof baselineValue === "number" && Number.isFinite(baselineValue)) {
    const delta = currentValue - baselineValue;
    const percent = baselineValue !== 0 ? ` (${((delta / Math.abs(baselineValue)) * 100).toFixed(2)}%)` : "";
    const label = `${delta >= 0 ? "+" : ""}${formatValue(delta)}${percent}`;
    return { label, className: delta > 0 ? "positive" : delta < 0 ? "negative" : "same" };
  }
  const currentText = formatValue(currentValue);
  const baselineText = formatValue(baselineValue);
  if (currentText === baselineText) return { label: "same", className: "same" };
  if (baselineValue === undefined) return { label: "added", className: "changed" };
  if (currentValue === undefined) return { label: "removed", className: "changed" };
  return { label: "changed", className: "changed" };
}

function formatComparisonValue(value) {
  return value === undefined ? "n/a" : formatValue(value);
}
