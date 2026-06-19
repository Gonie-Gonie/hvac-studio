import { csvCell, downloadTextFile, markdownTable, safeFileName } from "./download.js";
import { formatValue, shortNumber } from "./format.js";
import { resultTable } from "./result-ui.js";
import { candidateObjectiveHistory } from "./validation-plots.js";

export function candidateResultSection(result, savedLabel, savedPath, context = {}) {
  const section = document.createElement("div");
  section.className = "result-grid";
  const candidates = result.candidates || [];
  const summaryRows = [
    ["Setup", result.setup_name || result.setup_id || ""],
  ];
  if (result.base_parameter_set !== undefined) summaryRows.push(["Base parameter set", result.base_parameter_set || "baseline"]);
  if (result.initial_objective !== undefined) summaryRows.push(["Initial objective", shortNumber(result.initial_objective)]);
  if (result.objective !== undefined && (typeof result.objective !== "object" || result.objective === null)) summaryRows.push(["Final objective", shortNumber(result.objective)]);
  if (result.best_objective !== undefined) summaryRows.push(["Best objective", shortNumber(result.best_objective)]);
  summaryRows.push([savedLabel, savedPath || ""]);
  if (result.saved_parameter_set && savedLabel !== "Saved parameter set") summaryRows.push(["Saved parameter set", result.saved_parameter_set]);
  section.append(resultTable("Summary", summaryRows));
  if (result.changed_parameters) {
    section.append(resultTable("Parameter Changes", parameterChangeRows(result.changed_parameters), ["Component", "Parameter", "Initial", "Best", "Delta"]));
  }
  const bestRows = bestCandidateRows(result);
  if (bestRows.length) {
    section.append(resultTable("Best Candidate", bestRows));
  }
  const bestDecisionRows = optimizationBestDecisionRows(result);
  if (bestDecisionRows.length) {
    section.append(resultTable("Best Decision Variables", bestDecisionRows, ["Kind", "Target", "Value"]));
  }
  if (result.best_outputs) {
    section.append(resultTable("Best Outputs", Object.entries(result.best_outputs || {}).map(([name, value]) => [name, formatValue(value)]), ["Output", "Value"]));
  }
  const objectiveHistory = candidateObjectiveHistory(result);
  if (objectiveHistory) {
    section.append(objectiveHistory);
  }
  const constraintRows = optimizationConstraintRows(result);
  if (constraintRows.length) {
    section.append(resultTable("Constraint Status", constraintRows, ["Item", "Status", "Detail"]));
  }
  const outputRows = optimizationOutputComparisonRows(result);
  if (outputRows.length) {
    section.append(resultTable("Output Comparison", outputRows, ["#", "Objective", "Status", "Outputs"]));
  }
  section.append(resultTable("Candidates", candidates.slice(0, 12).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
  ]), ["#", "Objective", "Status", "Values"]));
  const failed = candidates.filter((item) => item.error);
  if (failed.length) {
    section.append(resultTable("Failed Candidates", failed.slice(0, 12).map((item) => [
      String(item.index ?? ""),
      item.error || "",
      parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
    ]), ["#", "Error", "Values"]));
  }
  const actions = document.createElement("div");
  actions.className = "result-actions";
  if (candidates.length) {
    const exportCSV = document.createElement("button");
    exportCSV.type = "button";
    exportCSV.className = "small-action";
    exportCSV.textContent = "Export CSV";
    exportCSV.addEventListener("click", () => downloadCandidateCSV(result));
    actions.append(exportCSV);
  }
  const exportReport = document.createElement("button");
  exportReport.type = "button";
  exportReport.className = "small-action";
  exportReport.textContent = "Export Report";
  exportReport.addEventListener("click", () => downloadCandidateReport(result));
  actions.append(exportReport);
  if (isOptimizationResult(result) && result.setup) {
    const exportSDK = document.createElement("button");
    exportSDK.type = "button";
    exportSDK.className = "small-action";
    exportSDK.textContent = "Export SDK Script";
    exportSDK.addEventListener("click", () => downloadOptimizationSDKScript(result, context));
    actions.append(exportSDK);
  }
  if (result.saved_scenario && context.isWorkspaceProject) {
    const openScenario = document.createElement("button");
    openScenario.type = "button";
    openScenario.className = "small-action";
    openScenario.textContent = "Open Saved Scenario";
    openScenario.addEventListener("click", () => context.loadScenario?.(scenarioIDFromPath(result.saved_scenario)));
    actions.append(openScenario);
  }
  if (savedLabel !== "Saved parameter set" && result.saved_parameter_set && context.isWorkspaceProject) {
    const useSavedParameterSet = document.createElement("button");
    useSavedParameterSet.type = "button";
    useSavedParameterSet.className = "small-action";
    useSavedParameterSet.textContent = "Use Saved Parameter Set";
    useSavedParameterSet.addEventListener("click", () => context.activateParameterSetForRuns?.(result.saved_parameter_set));
    actions.append(useSavedParameterSet);
    const applySavedParameterSet = document.createElement("button");
    applySavedParameterSet.type = "button";
    applySavedParameterSet.className = "small-action";
    applySavedParameterSet.textContent = "Apply Saved Parameter Set";
    applySavedParameterSet.addEventListener("click", () => context.applyParameterSetToGraph?.(result.saved_parameter_set));
    actions.append(applySavedParameterSet);
  }
  if (savedLabel === "Saved parameter set" && savedPath && context.isWorkspaceProject) {
    const useForRuns = document.createElement("button");
    useForRuns.type = "button";
    useForRuns.className = "small-action";
    useForRuns.textContent = "Use for Runs";
    useForRuns.addEventListener("click", () => context.activateParameterSetForRuns?.(savedPath));
    actions.append(useForRuns);
    const revertActive = document.createElement("button");
    revertActive.type = "button";
    revertActive.className = "small-action";
    revertActive.textContent = "Revert Active";
    revertActive.addEventListener("click", () => context.activateParameterSetForRuns?.(""));
    actions.append(revertActive);
    const compareValidation = document.createElement("button");
    compareValidation.type = "button";
    compareValidation.className = "small-action";
    compareValidation.textContent = "Validation Before/After";
    compareValidation.disabled = !result.mapping;
    compareValidation.addEventListener("click", () => context.runCalibrationValidationComparison?.(result));
    actions.append(compareValidation);
    const compareSelect = document.createElement("select");
    compareSelect.className = "validation-compare-select";
    const compareChoices = calibrationCompareParameterSetChoices(savedPath, context.parameterSets || []);
    if (!compareChoices.length) {
      compareSelect.append(new Option("No comparison sets", savedPath));
    } else {
      for (const item of compareChoices) {
        compareSelect.append(new Option(item.label, item.value));
      }
    }
    const compareExisting = document.createElement("button");
    compareExisting.type = "button";
    compareExisting.className = "small-action";
    compareExisting.textContent = "Compare Existing Set";
    compareExisting.disabled = !result.mapping || !compareChoices.length;
    compareExisting.addEventListener("click", () => context.runCalibrationParameterSetComparison?.(result, compareSelect.value));
    actions.append(compareSelect, compareExisting);
    const apply = document.createElement("button");
    apply.type = "button";
    apply.className = "small-action";
    apply.textContent = "Apply Parameter Set";
    apply.addEventListener("click", () => context.applyParameterSetToGraph?.(savedPath));
    actions.append(apply);
  }
  if (actions.childElementCount) {
    section.append(actions);
  }
  return section;
}

export function isOptimizationResult(result) {
  return result.saved_scenario !== undefined || result.best_inputs !== undefined || result.objective?.output !== undefined;
}

export function resultPublicOutputSummary(outputs) {
  const entries = Object.entries(outputs || {});
  if (!entries.length) return "";
  return entries.map(([name, value]) => `${name}: ${formatValue(value)}`).join(", ");
}

function optimizationBestDecisionRows(result) {
  const rows = [];
  for (const [name, value] of Object.entries(result.best_inputs || {})) {
    rows.push(["Public Input", name, formatValue(value)]);
  }
  for (const [component, values] of Object.entries(result.best_parameters || {})) {
    for (const [name, value] of Object.entries(values || {})) {
      rows.push(["Component Parameter", `${component}.${name}`, formatValue(value)]);
    }
  }
  return rows;
}

function optimizationConstraintRows(result) {
  const candidates = result.candidates || [];
  if (!candidates.length || !(result.best_inputs || result.best_parameters || result.best_outputs)) return [];
  const best = bestCandidate(result);
  const feasible = candidates.filter((item) => !item.error && item.feasible !== false).length;
  const infeasible = candidates.filter((item) => item.feasible === false).length;
  const failed = candidates.filter((item) => item.error).length;
  const rows = [
    ["Feasible candidates", String(feasible), ""],
    ["Infeasible candidates", String(infeasible), ""],
    ["Failed candidates", String(failed), ""],
  ];
  if (best) {
    const violations = best.constraint_violations || [];
    rows.push(["Best candidate", violations.length ? "violated" : "ok", violations.length ? `${violations.length} constraint violation(s)` : "constraints satisfied"]);
    for (const violation of violations) {
      rows.push([
        violation.output || "constraint",
        `${violation.operator || ""} ${shortNumber(violation.value)}`.trim(),
        [violation.message || "", `actual ${formatValue(violation.actual)}`, `violation ${shortNumber(violation.violation)}`].filter(Boolean).join(" / "),
      ]);
    }
  }
  return rows;
}

function optimizationOutputComparisonRows(result) {
  return (result.candidates || []).filter((item) => item.outputs && Object.keys(item.outputs).length).slice(0, 12).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    resultPublicOutputSummary(item.outputs || {}),
  ]);
}

function bestCandidateRows(result) {
  const candidates = result.candidates || [];
  const best = bestCandidate(result);
  if (!best) return [];
  const ordinal = candidates.indexOf(best);
  const rows = [
    ["Index", String(best.index ?? (ordinal >= 0 ? ordinal + 1 : ""))],
    ["Objective", shortNumber(best.objective)],
    ["Status", candidateStatus(best)],
  ];
  const parameterSummary = parameterCandidateSummary(best.parameters || {});
  const inputSummary = parameterCandidateSummary(best.inputs || {});
  const outputSummary = resultPublicOutputSummary(best.outputs || {});
  if (parameterSummary) rows.push(["Parameters", parameterSummary]);
  if (inputSummary) rows.push(["Inputs", inputSummary]);
  if (outputSummary) rows.push(["Outputs", outputSummary]);
  if (best.error) rows.push(["Error", best.error]);
  return rows;
}

function bestCandidate(result) {
  const candidates = result.candidates || [];
  const bestObjective = Number(result.best_objective);
  if (!Number.isFinite(bestObjective)) return candidates.find((item) => !item.error) || null;
  return candidates.find((item) => !item.error && Math.abs(Number(item.objective) - bestObjective) <= 1e-9) || candidates.find((item) => !item.error) || null;
}

function scenarioIDFromPath(path) {
  return String(path || "").split(/[\\/]/).pop().replace(/\.json$/i, "");
}

function calibrationCompareParameterSetChoices(savedPath, parameterSets) {
  return [{ label: "Baseline", value: "" }, ...(parameterSets || []).map((item) => ({
    label: item.name || item.id || item.relative_path,
    value: item.relative_path || "",
  }))].filter((item) => item.value !== savedPath);
}

function parameterChangeRows(changes) {
  const rows = [];
  for (const [component, params] of Object.entries(changes || {})) {
    for (const [name, change] of Object.entries(params || {})) {
      const initial = change?.initial;
      const best = change?.best;
      rows.push([
        component,
        name,
        formatValue(initial),
        formatValue(best),
        numericDelta(initial, best),
      ]);
    }
  }
  return rows.sort((a, b) => `${a[0]}.${a[1]}`.localeCompare(`${b[0]}.${b[1]}`));
}

function numericDelta(initial, best) {
  const before = Number(initial);
  const after = Number(best);
  if (!Number.isFinite(before) || !Number.isFinite(after)) return "";
  return shortNumber(after - before);
}

function downloadCandidateReport(result) {
  const isCalibration = result.saved_parameter_set !== undefined && result.changed_parameters !== undefined;
  const title = isCalibration ? "Calibration Result Report" : "Optimization Result Report";
  const lines = [`# ${title}`, ""];
  lines.push(...markdownTable([
    ["Setup", result.setup_name || result.setup_id || ""],
    ["Algorithm", result.algorithm || ""],
    ["Base parameter set", result.base_parameter_set || "baseline"],
    ["Initial objective", result.initial_objective !== undefined ? shortNumber(result.initial_objective) : ""],
    ["Best objective", result.best_objective !== undefined ? shortNumber(result.best_objective) : ""],
    ["Saved parameter set", result.saved_parameter_set || ""],
    ["Saved scenario", result.saved_scenario || ""],
    ["Saved record", result.saved_record || ""],
  ].filter(([, value]) => value !== "")));
  if (result.changed_parameters) {
    lines.push("", "## Parameter Changes", "");
    lines.push(...markdownTable(parameterChangeRows(result.changed_parameters), ["Component", "Parameter", "Initial", "Best", "Delta"]));
  }
  const decisionRows = optimizationBestDecisionRows(result);
  if (decisionRows.length) {
    lines.push("", "## Best Decision Variables", "");
    lines.push(...markdownTable(decisionRows, ["Kind", "Target", "Value"]));
  }
  if (result.best_outputs) {
    lines.push("", "## Best Outputs", "");
    lines.push(...markdownTable(Object.entries(result.best_outputs || {}).map(([name, value]) => [name, formatValue(value)]), ["Output", "Value"]));
  }
  const constraintRows = optimizationConstraintRows(result);
  if (constraintRows.length) {
    lines.push("", "## Constraint Status", "");
    lines.push(...markdownTable(constraintRows, ["Item", "Status", "Detail"]));
  }
  lines.push("", "## Candidates", "");
  lines.push(...markdownTable((result.candidates || []).map((item, index) => [
    String(item.index ?? index + 1),
    shortNumber(item.objective),
    candidateStatus(item),
    parameterCandidateSummary(item.parameters || item.inputs || item.outputs || {}),
  ]), ["#", "Objective", "Status", "Values"]));
  const name = `${safeFileName(result.setup_id || result.setup_name || "candidate-result")}-report.md`;
  downloadTextFile(name, `${lines.join("\n")}\n`, "text/markdown;charset=utf-8");
}

function downloadCandidateCSV(result) {
  const candidates = result.candidates || [];
  const flatRows = candidates.map((candidate, index) => {
    const row = {
      index: candidate.index ?? index + 1,
      objective: candidate.objective ?? "",
      status: candidateStatus(candidate),
      feasible: candidate.feasible ?? "",
      error: candidate.error || "",
      constraint_penalty: candidate.constraint_penalty ?? "",
    };
    flattenCandidateValues(candidate.inputs || {}, "inputs", row);
    flattenCandidateValues(candidate.parameters || {}, "parameters", row);
    flattenCandidateValues(candidate.outputs || {}, "outputs", row);
    return row;
  });
  const baseHeaders = ["index", "objective", "status", "feasible", "error", "constraint_penalty"];
  const dynamicHeaders = [...new Set(flatRows.flatMap((row) => Object.keys(row)))].filter((key) => !baseHeaders.includes(key)).sort();
  const headers = [...baseHeaders, ...dynamicHeaders];
  const csv = [
    headers.map(csvCell).join(","),
    ...flatRows.map((row) => headers.map((header) => csvCell(row[header] ?? "")).join(",")),
  ].join("\r\n");
  const name = `${safeFileName(result.setup_id || result.setup_name || "candidates")}-candidates.csv`;
  downloadTextFile(name, csv, "text/csv;charset=utf-8");
}

function flattenCandidateValues(value, prefix, row) {
  for (const [key, item] of Object.entries(value || {})) {
    const path = `${prefix}.${key}`;
    if (item && typeof item === "object" && !Array.isArray(item)) {
      flattenCandidateValues(item, path, row);
    } else {
      row[path] = item;
    }
  }
}

function downloadOptimizationSDKScript(result, context) {
  const setup = result.setup || "";
  const saveScenario = result.saved_scenario || "";
  const saveParameterSet = result.saved_parameter_set || "";
  const outputName = `${safeFileName(result.setup_id || result.setup_name || "optimization")}-sdk-result.json`;
  const pythonStringLiteral = context.pythonStringLiteral || ((value) => JSON.stringify(String(value ?? "")));
  const lines = [
    "from pathlib import Path",
    "import json",
    "",
    "from bcs_sdk import RunnerClient",
    "",
    `PROJECT = Path(${pythonStringLiteral(context.currentProjectPath || "project.bcsproj")})`,
    `RUNNER = ${pythonStringLiteral("bcs-runner.exe")}`,
    `SETUP = ${pythonStringLiteral(setup)}`,
    `OUTPUT = Path(${pythonStringLiteral(outputName)})`,
    "",
    "client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)",
    "client.validate_project()",
    "result = client.run_optimization(",
    "    setup=SETUP,",
    saveScenario ? `    save_scenario=${pythonStringLiteral(saveScenario)},` : "    save_scenario=None,",
    saveParameterSet ? `    save_parameter_set=${pythonStringLiteral(saveParameterSet)},` : "    save_parameter_set=None,",
    "    save_record=True,",
    "    output=OUTPUT,",
    ")",
    "print(json.dumps({",
    "    \"ok\": result.get(\"ok\"),",
    "    \"best_objective\": result.get(\"best_objective\"),",
    "    \"saved_scenario\": result.get(\"saved_scenario\", \"\"),",
    "    \"saved_parameter_set\": result.get(\"saved_parameter_set\", \"\"),",
    "    \"output\": str(OUTPUT),",
    "}, indent=2, sort_keys=True))",
    "",
  ];
  const name = `${safeFileName(result.setup_id || result.setup_name || "optimization")}-sdk.py`;
  downloadTextFile(name, lines.join("\n"), "text/x-python;charset=utf-8");
}

function candidateStatus(item) {
  if (item.error) return `failed: ${item.error}`;
  if (item.feasible === false) {
    const count = (item.constraint_violations || []).length;
    return `${count || 1} constraint${count === 1 ? "" : "s"}`;
  }
  return "feasible";
}

function parameterCandidateSummary(values) {
  const entries = [];
  for (const [component, parameters] of Object.entries(values || {})) {
    if (parameters && typeof parameters === "object" && !Array.isArray(parameters)) {
      for (const [name, value] of Object.entries(parameters)) {
        entries.push(`${component}.${name}=${formatValue(value)}`);
      }
    } else {
      entries.push(`${component}=${formatValue(parameters)}`);
    }
  }
  return entries.slice(0, 5).join(", ");
}
