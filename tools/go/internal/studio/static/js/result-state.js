export function latestResultValue(state) {
  return state?.latestWorkflowRecord ||
    state?.latestDataValidation ||
    state?.latestSeriesResult ||
    state?.latestBatchRecord ||
    state?.latestRunRecord ||
    state?.latestResult ||
    null;
}

export function latestRuntimeResult(state) {
  return latestRuntimeComparisonContext(state)?.result || null;
}

export function latestRuntimeComparisonContext(state) {
  if (state?.latestSeriesResult) {
    return { result: seriesLastResult(state.latestSeriesResult), source: seriesSourceLabel(state.latestSeriesResult) };
  }
  if (state?.latestBatchRecord) {
    const found = firstSuccessfulBatchCase(state.latestBatchRecord);
    if (found?.result) {
      return { result: found.result, source: batchRunSourceLabel(state.latestBatchRecord, found) };
    }
  }
  if (state?.latestRunRecord?.result) {
    return { result: state.latestRunRecord.result, source: runRecordSourceLabel(state.latestRunRecord) };
  }
  if (state?.latestResult) {
    return { result: state.latestResult, source: state.latestRunSource || currentRunSourceLabel(state) };
  }
  return null;
}

export function latestResultContext(state) {
  return latestRuntimeComparisonContext(state) || { result: null, source: "" };
}

export function seriesLastResult(series) {
  const points = series?.series || [];
  const point = points[points.length - 1];
  if (!point) return null;
  return {
    ok: true,
    parameter_set: series.parameter_set || "",
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

export function firstSuccessfulBatchCase(record) {
  return (record?.cases || []).find((item) => item.ok && item.result) || null;
}

export function firstBatchResult(stateOrRecord) {
  const record = stateOrRecord?.latestBatchRecord || stateOrRecord;
  return firstSuccessfulBatchCase(record)?.result || null;
}

export function currentRunSourceLabel(state) {
  const parts = ["current run"];
  if (state?.activeRunInput) {
    parts.push(`scenario ${state.activeRunInput.name || state.activeRunInput.id || "active"}`);
  }
  parts.push(state?.activeParameterSetPath ? `parameter set ${state.activeParameterSetPath}` : "baseline");
  return parts.join(" / ");
}

export function seriesSourceLabel(series) {
  const parts = ["series preview"];
  if (series?.step_count) parts.push(`${series.step_count} steps`);
  parts.push(series?.parameter_set ? `parameter set ${series.parameter_set}` : "baseline");
  return parts.join(" / ");
}

export function runRecordSourceLabel(record) {
  const parts = [record?.id || "saved run"];
  parts.push(record?.parameter_set ? `parameter set ${record.parameter_set}` : "baseline");
  return parts.join(" / ");
}

export function batchRunSourceLabel(record, firstCase) {
  const parts = [record?.id || "batch"];
  const caseName = firstCase?.scenario_name || firstCase?.scenario_id || "";
  if (caseName) parts.push(`case ${caseName}`);
  parts.push(record?.parameter_set ? `parameter set ${record.parameter_set}` : "baseline");
  return parts.join(" / ");
}