import { escapeAttr, escapeHTML } from "./dom.js";
import { parameterInputValue } from "./format.js";
import {
  formatExpectedRunCount,
  labeledEditorControl,
  labeledEditorInput,
} from "./setup-editor-ui.js";

export function calibrationSetupEditorSection(context) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.append(calibrationSetupControls(context));
  section.append(calibrationObjectiveOutputEditor(context.mapping));
  section.append(calibrationCandidateEditor(context.candidates || []));
  const actions = document.createElement("div");
  actions.className = "result-actions calibration-setup-actions";
  const runCount = document.createElement("span");
  runCount.className = "input-meta calibration-run-count";
  const warning = document.createElement("span");
  warning.className = "input-meta calibration-editor-warning";
  const create = document.createElement("button");
  create.type = "button";
  create.className = "small-action";
  create.dataset.calibrationCreate = "true";
  create.textContent = "Create Setup";
  create.addEventListener("click", () => {
    const payload = collectCalibrationSetupEditorPayload(section, context);
    if (payload) context.createSetup?.(payload);
  });
  actions.append(runCount, warning, create);
  section.append(actions);
  section.querySelectorAll("[data-calibration-filter], [data-calibration-stop-max], [data-calibration-stop-tolerance], [data-cal-param-check], [data-cal-param-field], [data-cal-output-check], [data-cal-output-weight]").forEach((control) => {
    control.addEventListener("input", () => updateCalibrationEditorState(section));
    control.addEventListener("change", () => updateCalibrationEditorState(section));
  });
  updateCalibrationEditorState(section);
  return section;
}

function calibrationSetupControls(context) {
  const block = document.createElement("div");
  block.className = "result-block calibration-editor-block";
  block.innerHTML = `<div class="result-block-title">Setup</div>`;
  const controls = document.createElement("div");
  controls.className = "calibration-editor-grid";
  const mappingSelect = document.createElement("select");
  mappingSelect.dataset.calibrationMapping = "true";
  for (const item of context.validationMappings || []) {
    mappingSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  mappingSelect.value = context.mapping_summary?.relative_path || "";
  mappingSelect.addEventListener("change", () => context.openMapping?.(mappingSelect.value));
  const baseSelect = document.createElement("select");
  baseSelect.dataset.calibrationBase = "true";
  baseSelect.append(new Option("Baseline", ""));
  for (const item of context.parameterSets || []) {
    baseSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  baseSelect.value = context.activeParameterSetPath || "";
  const algorithmSelect = document.createElement("select");
  algorithmSelect.dataset.calibrationAlgorithm = "true";
  algorithmSelect.append(new Option("Grid Search", "grid"));
  algorithmSelect.append(new Option("Differential Evolution", "differential_evolution"));
  algorithmSelect.append(new Option("Least Squares", "least_squares"));
  const maxCandidates = document.createElement("input");
  maxCandidates.type = "number";
  maxCandidates.min = "1";
  maxCandidates.step = "1";
  maxCandidates.placeholder = "optional";
  maxCandidates.dataset.calibrationStopMax = "true";
  const tolerance = document.createElement("input");
  tolerance.type = "number";
  tolerance.min = "0";
  tolerance.step = "any";
  tolerance.placeholder = "optional";
  tolerance.dataset.calibrationStopTolerance = "true";
  controls.append(
    labeledEditorControl("Mapping", mappingSelect),
    labeledEditorControl("Base Parameter Set", baseSelect),
    labeledEditorControl("Algorithm", algorithmSelect),
    labeledEditorControl("Max Candidates", maxCandidates),
    labeledEditorControl("Objective Tolerance", tolerance),
    labeledEditorInput("Setup ID", "text", "auto", "calibration-setup-id"),
    labeledEditorInput("Setup Name", "text", "auto", "calibration-setup-name"),
  );
  block.append(controls);
  return block;
}

function calibrationObjectiveOutputEditor(mapping) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `
    <div class="result-block-title">Target Outputs</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Output</th><th>Dataset Column</th><th>Weight</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  const outputs = Object.entries(mapping?.observed_output_columns || {});
  if (!outputs.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No observed outputs</td></tr>`;
    return block;
  }
  for (const [output, column] of outputs) {
    const row = document.createElement("tr");
    row.dataset.calOutput = output;
    row.innerHTML = `
      <td><input type="checkbox" data-cal-output-check checked aria-label="Use ${escapeAttr(output)}" /></td>
      <td>${escapeHTML(output)}</td>
      <td>${escapeHTML(column)}</td>
      <td><input type="number" data-cal-output-weight value="1" min="0" step="0.1" aria-label="${escapeAttr(output)} weight" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function calibrationCandidateEditor(candidates) {
  const block = document.createElement("div");
  block.className = "result-block calibration-candidate-block";
  block.innerHTML = `
    <div class="result-block-title">Candidate Parameters</div>
    <div class="calibration-filters">
      <select data-calibration-filter="role" aria-label="Role filter">
        <option value="">All roles</option>
        <option value="calibration_target" selected>calibration_target</option>
      </select>
      <select data-calibration-filter="component" aria-label="Component filter"></select>
      <select data-calibration-filter="unit" aria-label="Unit filter"></select>
      <label class="compact-toggle"><input type="checkbox" data-calibration-filter="bounds" checked /> Bounds present</label>
    </div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Component</th><th>Parameter</th><th>Role</th><th>Unit</th><th>Current</th><th>Min</th><th>Max</th><th>Step</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const componentFilter = block.querySelector('[data-calibration-filter="component"]');
  componentFilter.append(new Option("All components", ""));
  [...new Set(candidates.map((item) => item.component))].sort().forEach((component) => componentFilter.append(new Option(component, component)));
  const unitFilter = block.querySelector('[data-calibration-filter="unit"]');
  unitFilter.append(new Option("All units", ""));
  [...new Set(candidates.map((item) => item.unit).filter(Boolean))].sort().forEach((unit) => unitFilter.append(new Option(unit, unit)));
  const tbody = block.querySelector("tbody");
  if (!candidates.length) {
    tbody.innerHTML = `<tr><td colspan="9" class="empty-cell">No parameters</td></tr>`;
    return block;
  }
  for (const candidate of candidates) {
    const row = document.createElement("tr");
    row.dataset.calParam = `${candidate.component}.${candidate.name}`;
    row.dataset.paramComponent = candidate.component;
    row.dataset.paramName = candidate.name;
    row.dataset.role = candidate.role;
    row.dataset.component = candidate.component;
    row.dataset.unit = candidate.unit;
    row.dataset.hasBounds = candidate.hasBounds ? "true" : "false";
    row.innerHTML = `
      <td><input type="checkbox" data-cal-param-check ${candidate.selected ? "checked" : ""} aria-label="Use ${escapeAttr(candidate.component)}.${escapeAttr(candidate.name)}" /></td>
      <td>${escapeHTML(candidate.componentName)}</td>
      <td>${escapeHTML(candidate.name)}</td>
      <td>${escapeHTML(candidate.role)}</td>
      <td>${escapeHTML(candidate.unit)}</td>
      <td>${escapeHTML(parameterInputValue(candidate.current))}</td>
      <td><input type="number" data-cal-param-field="min" value="${escapeAttr(candidate.min ?? "")}" step="any" /></td>
      <td><input type="number" data-cal-param-field="max" value="${escapeAttr(candidate.max ?? "")}" step="any" /></td>
      <td><input type="number" data-cal-param-field="step" value="${escapeAttr(candidate.step ?? "")}" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function updateCalibrationEditorState(section) {
  const roleFilter = section.querySelector('[data-calibration-filter="role"]')?.value || "";
  const componentFilter = section.querySelector('[data-calibration-filter="component"]')?.value || "";
  const unitFilter = section.querySelector('[data-calibration-filter="unit"]')?.value || "";
  const boundsOnly = section.querySelector('[data-calibration-filter="bounds"]')?.checked || false;
  for (const row of section.querySelectorAll("[data-cal-param]")) {
    const visible = (!roleFilter || row.dataset.role === roleFilter) &&
      (!componentFilter || row.dataset.component === componentFilter) &&
      (!unitFilter || row.dataset.unit === unitFilter) &&
      (!boundsOnly || row.dataset.hasBounds === "true");
    row.hidden = !visible;
  }
  const selectedRows = [...section.querySelectorAll("[data-cal-param]")].filter((row) => row.querySelector("[data-cal-param-check]")?.checked);
  const invalidRows = selectedRows.filter((row) => calibrationGridPointCount(row) === 0);
  for (const row of section.querySelectorAll("[data-cal-param]")) {
    const checked = row.querySelector("[data-cal-param-check]")?.checked;
    row.classList.toggle("calibration-invalid", Boolean(checked && calibrationGridPointCount(row) === 0));
  }
  const selectedOutputs = [...section.querySelectorAll("[data-cal-output]")].filter((row) => row.querySelector("[data-cal-output-check]")?.checked);
  const invalidOutputs = selectedOutputs.filter((row) => !validCalibrationWeight(row));
  let expectedRuns = selectedRows.length ? selectedRows.reduce((product, row) => product * calibrationGridPointCount(row), 1) : 0;
  const maxCandidates = Number(section.querySelector("[data-calibration-stop-max]")?.value);
  if (Number.isFinite(maxCandidates) && maxCandidates > 0) expectedRuns = Math.min(expectedRuns, Math.floor(maxCandidates));
  const runCount = section.querySelector(".calibration-run-count");
  if (runCount) {
    runCount.textContent = `Selected ${selectedRows.length} / Expected Runs ${formatExpectedRunCount(expectedRuns)}`;
  }
  const status = calibrationEditorStatus(selectedOutputs, invalidOutputs, selectedRows, invalidRows);
  const warning = section.querySelector(".calibration-editor-warning");
  if (warning) {
    warning.textContent = status.message;
    warning.classList.toggle("ready", status.valid);
  }
  const create = section.querySelector("[data-calibration-create]");
  if (create) create.disabled = !status.valid;
}

function calibrationEditorStatus(selectedOutputs, invalidOutputs, selectedRows, invalidRows) {
  if (!selectedOutputs.length) return { valid: false, message: "Select at least one target output" };
  if (invalidOutputs.length) return { valid: false, message: "Fix invalid output weights" };
  if (!selectedRows.length) return { valid: false, message: "Select at least one calibration parameter" };
  if (invalidRows.length) return { valid: false, message: "Fix invalid parameter bounds" };
  return { valid: true, message: "Ready to create setup" };
}

function validCalibrationWeight(row) {
  const weight = Number(row.querySelector("[data-cal-output-weight]")?.value);
  return Number.isFinite(weight) && weight >= 0;
}

function calibrationGridPointCount(row) {
  const min = Number(row.querySelector('[data-cal-param-field="min"]')?.value);
  const max = Number(row.querySelector('[data-cal-param-field="max"]')?.value);
  const step = Number(row.querySelector('[data-cal-param-field="step"]')?.value);
  if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) return 0;
  return Math.max(1, Math.floor(((max - min) / step) + 1.000000001));
}

function collectCalibrationSetupEditorPayload(section, context) {
  const mappingPath = section.querySelector("[data-calibration-mapping]")?.value || context.mapping_summary?.relative_path || "";
  const outputs = {};
  for (const row of section.querySelectorAll("[data-cal-output]")) {
    if (!row.querySelector("[data-cal-output-check]")?.checked) continue;
    const weight = Number(row.querySelector("[data-cal-output-weight]")?.value);
    if (!Number.isFinite(weight) || weight < 0) {
      context.showProblem?.(`Invalid calibration output weight: ${row.dataset.calOutput}`);
      return null;
    }
    outputs[row.dataset.calOutput] = weight;
  }
  if (!Object.keys(outputs).length) {
    context.showProblem?.("Select at least one calibration target output");
    return null;
  }
  const parameters = [];
  for (const row of section.querySelectorAll("[data-cal-param]")) {
    if (!row.querySelector("[data-cal-param-check]")?.checked) continue;
    const min = Number(row.querySelector('[data-cal-param-field="min"]')?.value);
    const max = Number(row.querySelector('[data-cal-param-field="max"]')?.value);
    const step = Number(row.querySelector('[data-cal-param-field="step"]')?.value);
    if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) {
      context.showProblem?.(`Invalid calibration bounds: ${row.dataset.paramComponent}.${row.dataset.paramName}`);
      return null;
    }
    parameters.push({
      component: row.dataset.paramComponent,
      name: row.dataset.paramName,
      min,
      max,
      step,
    });
  }
  if (!parameters.length) {
    context.showProblem?.("Select at least one calibration parameter");
    return null;
  }
  const stoppingRules = {};
  const maxCandidates = Number(section.querySelector("[data-calibration-stop-max]")?.value);
  if (Number.isFinite(maxCandidates) && maxCandidates > 0) stoppingRules.max_candidates = Math.floor(maxCandidates);
  const objectiveTolerance = Number(section.querySelector("[data-calibration-stop-tolerance]")?.value);
  if (Number.isFinite(objectiveTolerance) && objectiveTolerance > 0) stoppingRules.objective_tolerance = objectiveTolerance;
  return {
    mapping_path: mappingPath,
    id: section.querySelector("[data-calibration-setup-id]")?.value.trim() || "",
    name: section.querySelector("[data-calibration-setup-name]")?.value.trim() || "",
    algorithm: section.querySelector("[data-calibration-algorithm]")?.value || "grid",
    base_parameter_set: section.querySelector("[data-calibration-base]")?.value || "",
    objective_outputs: outputs,
    parameters,
    stopping_rules: stoppingRules,
  };
}
