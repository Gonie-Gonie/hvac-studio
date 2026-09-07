import { escapeAttr, escapeHTML } from "./dom.js";
import { finiteNumber, parameterInputValue } from "./format.js";
import {
  defaultCalibrationGridStep,
  formatExpectedRunCount,
  labeledEditorControl,
  labeledEditorInput,
} from "./setup-editor-ui.js";

export function optimizationSetupEditorSection(context) {
  const section = document.createElement("div");
  section.className = "result-grid";
  section.dataset.optimizationBaseInputs = JSON.stringify(context.base_inputs || context.collectRunInputs?.() || {});
  section.dataset.optimizationBaseContext = JSON.stringify(context.context || context.currentRunContext?.() || {});
  section.append(optimizationSetupControls(context));
  section.append(optimizationDecisionEditor(context.candidates || []));
  section.append(optimizationConstraintEditor(context.outputs || []));
  const actions = document.createElement("div");
  actions.className = "result-actions calibration-setup-actions";
  const runCount = document.createElement("span");
  runCount.className = "input-meta optimization-run-count";
  const warning = document.createElement("span");
  warning.className = "input-meta optimization-editor-warning";
  const create = document.createElement("button");
  create.type = "button";
  create.className = "small-action";
  create.dataset.optimizationCreate = "true";
  create.textContent = "Create Setup";
  create.addEventListener("click", () => {
    const payload = collectOptimizationSetupEditorPayload(section, context);
    if (payload) context.createSetup?.(payload);
  });
  actions.append(runCount, warning, create);
  section.append(actions);
  const baseSource = section.querySelector("[data-optimization-base-source]");
  baseSource?.addEventListener("change", () => applyOptimizationBaseSource(section, baseSource.value, context));
  section.querySelectorAll("[data-optimization-objective], [data-opt-var-check], [data-opt-var-field], [data-opt-constraint-check], [data-opt-constraint-field]").forEach((control) => {
    control.addEventListener("input", () => updateOptimizationEditorState(section));
    control.addEventListener("change", () => updateOptimizationEditorState(section));
  });
  updateOptimizationEditorState(section);
  return section;
}

function optimizationSetupControls(context) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Setup</div>`;
  const controls = document.createElement("div");
  controls.className = "calibration-editor-grid";
  const baseSourceSelect = document.createElement("select");
  baseSourceSelect.dataset.optimizationBaseSource = "true";
  baseSourceSelect.append(new Option("Current Fields", "current"));
  if (context.defaultRunInput) {
    baseSourceSelect.append(new Option("Default Input", "default"));
  }
  for (const scenario of context.scenarios || []) {
    baseSourceSelect.append(new Option(`Scenario: ${scenario.name || scenario.id}`, `scenario:${scenario.id}`));
  }
  const objectiveSelect = document.createElement("select");
  objectiveSelect.dataset.optimizationObjective = "true";
  for (const output of context.outputs || []) {
    objectiveSelect.append(new Option(output.name || output.id, output.id || ""));
  }
  const senseSelect = document.createElement("select");
  senseSelect.dataset.optimizationSense = "true";
  senseSelect.append(new Option("Minimize", "min"), new Option("Maximize", "max"));
  const baseSelect = document.createElement("select");
  baseSelect.dataset.optimizationBase = "true";
  baseSelect.append(new Option("Baseline", ""));
  for (const item of context.parameterSets || []) {
    baseSelect.append(new Option(item.name || item.id || item.relative_path, item.relative_path || ""));
  }
  baseSelect.value = context.activeParameterSetPath || "";
  const algorithmSelect = document.createElement("select");
  algorithmSelect.dataset.optimizationAlgorithm = "true";
  algorithmSelect.append(new Option("Grid Search", "grid"));
  algorithmSelect.append(new Option("Differential Evolution", "differential_evolution"));
  algorithmSelect.append(new Option("Custom SDK Script", "custom_sdk_script"));
  controls.append(
    labeledEditorControl("Base Input/Scenario", baseSourceSelect),
    labeledEditorControl("Objective", objectiveSelect),
    labeledEditorControl("Sense", senseSelect),
    labeledEditorControl("Base Parameter Set", baseSelect),
    labeledEditorControl("Algorithm", algorithmSelect),
    labeledEditorInput("Setup ID", "text", "auto", "optimization-setup-id"),
    labeledEditorInput("Setup Name", "text", "auto", "optimization-setup-name"),
  );
  block.append(controls);
  return block;
}

function optimizationDecisionEditor(candidates) {
  const block = document.createElement("div");
  block.className = "result-block calibration-candidate-block";
  block.innerHTML = `
    <div class="result-block-title">Decision Variables</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Kind</th><th>Target</th><th>Role</th><th>Unit</th><th>Current</th><th>Min</th><th>Max</th><th>Step</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!candidates.length) {
    tbody.innerHTML = `<tr><td colspan="9" class="empty-cell">No decision variables</td></tr>`;
    return block;
  }
  for (const candidate of candidates) {
    const row = document.createElement("tr");
    row.dataset.optVar = candidate.label;
    row.dataset.kind = candidate.kind;
    row.dataset.component = candidate.component;
    row.dataset.name = candidate.name;
    row.innerHTML = `
      <td><input type="checkbox" data-opt-var-check ${candidate.selected ? "checked" : ""} aria-label="Use ${escapeAttr(candidate.label)}" /></td>
      <td>${escapeHTML(candidate.kind)}</td>
      <td>${escapeHTML(candidate.label)}</td>
      <td>${escapeHTML(candidate.role)}</td>
      <td>${escapeHTML(candidate.unit)}</td>
      <td data-opt-current>${escapeHTML(parameterInputValue(candidate.current))}</td>
      <td><input type="number" data-opt-var-field="min" value="${escapeAttr(candidate.min)}" step="any" /></td>
      <td><input type="number" data-opt-var-field="max" value="${escapeAttr(candidate.max)}" step="any" /></td>
      <td><input type="number" data-opt-var-field="step" value="${escapeAttr(candidate.step)}" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

function optimizationConstraintEditor(outputs) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `
    <div class="result-block-title">Constraints</div>
    <table class="result-table">
      <thead><tr><th>Use</th><th>Output</th><th>Operator</th><th>Value</th><th>Tolerance</th><th>Penalty</th></tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!outputs.length) {
    tbody.innerHTML = `<tr><td colspan="6" class="empty-cell">No numeric outputs</td></tr>`;
    return block;
  }
  for (const output of outputs) {
    const row = document.createElement("tr");
    row.dataset.optConstraint = output.id || "";
    row.innerHTML = `
      <td><input type="checkbox" data-opt-constraint-check aria-label="Constrain ${escapeAttr(output.id)}" /></td>
      <td>${escapeHTML(output.name || output.id)}</td>
      <td><select data-opt-constraint-field="operator"><option value="<=">&lt;=</option><option value=">=">&gt;=</option><option value="==">==</option></select></td>
      <td><input type="number" data-opt-constraint-field="value" value="0" step="any" /></td>
      <td><input type="number" data-opt-constraint-field="tolerance" value="0" min="0" step="any" /></td>
      <td><input type="number" data-opt-constraint-field="penalty" value="1000" min="0" step="any" /></td>
    `;
    tbody.append(row);
  }
  return block;
}

async function applyOptimizationBaseSource(section, source, context) {
  try {
    const base = await context.loadBaseSource?.(source);
    if (!base) return;
    setOptimizationEditorBase(section, base.inputs || {}, base.context || {}, context);
    context.onBaseSourceLoaded?.(base.label || source || "current fields");
  } catch (error) {
    context.onBaseSourceError?.(error);
  }
}

function setOptimizationEditorBase(section, inputs, baseContext, context) {
  section.dataset.optimizationBaseInputs = JSON.stringify(inputs || {});
  section.dataset.optimizationBaseContext = JSON.stringify(baseContext || {});
  for (const row of section.querySelectorAll('[data-opt-var][data-kind="public_input"]')) {
    const value = finiteNumber((inputs || {})[row.dataset.name]);
    const checkbox = row.querySelector("[data-opt-var-check]");
    const fields = row.querySelectorAll("[data-opt-var-field]");
    const currentCell = row.querySelector("[data-opt-current]");
    if (value === null) {
      if (checkbox) {
        checkbox.checked = false;
        checkbox.disabled = true;
      }
      fields.forEach((field) => {
        field.value = "";
        field.disabled = true;
      });
      if (currentCell) currentCell.textContent = "";
      continue;
    }
    if (checkbox) checkbox.disabled = false;
    fields.forEach((field) => {
      field.disabled = false;
    });
    const [min, max] = context.defaultBounds?.(value) || defaultOptimizationBounds(value);
    row.querySelector('[data-opt-var-field="min"]').value = min;
    row.querySelector('[data-opt-var-field="max"]').value = max;
    row.querySelector('[data-opt-var-field="step"]').value = defaultCalibrationGridStep(min, max);
    if (currentCell) currentCell.textContent = parameterInputValue(value);
  }
  updateOptimizationEditorState(section);
}

function defaultOptimizationBounds(value) {
  const numeric = Number(value);
  const delta = Math.max(Math.abs(numeric) * 0.2, 1);
  return [numeric - delta, numeric + delta];
}

function optimizationEditorBaseInputs(section, context) {
  return parseEditorJSON(section.dataset.optimizationBaseInputs, context.collectRunInputs?.() || {});
}

function optimizationEditorBaseContext(section, context) {
  return parseEditorJSON(section.dataset.optimizationBaseContext, context.currentRunContext?.() || {});
}

function parseEditorJSON(value, fallback) {
  if (!value) return fallback;
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}

function updateOptimizationEditorState(section) {
  const selectedRows = [...section.querySelectorAll("[data-opt-var]")].filter((row) => row.querySelector("[data-opt-var-check]")?.checked);
  const invalidRows = selectedRows.filter((row) => optimizationGridPointCount(row) === 0);
  for (const row of section.querySelectorAll("[data-opt-var]")) {
    const checked = row.querySelector("[data-opt-var-check]")?.checked;
    row.classList.toggle("optimization-invalid", Boolean(checked && optimizationGridPointCount(row) === 0));
  }
  const selectedConstraints = [...section.querySelectorAll("[data-opt-constraint]")].filter((row) => row.querySelector("[data-opt-constraint-check]")?.checked);
  const invalidConstraints = selectedConstraints.filter((row) => !validOptimizationConstraint(row));
  for (const row of section.querySelectorAll("[data-opt-constraint]")) {
    const checked = row.querySelector("[data-opt-constraint-check]")?.checked;
    row.classList.toggle("optimization-invalid", Boolean(checked && !validOptimizationConstraint(row)));
  }
  const expectedRuns = selectedRows.length ? selectedRows.reduce((product, row) => product * optimizationGridPointCount(row), 1) : 0;
  const runCount = section.querySelector(".optimization-run-count");
  if (runCount) {
    runCount.textContent = `Selected ${selectedRows.length} / Constraints ${selectedConstraints.length} / Estimated Runs ${formatExpectedRunCount(expectedRuns)}`;
  }
  const objectiveOutput = section.querySelector("[data-optimization-objective]")?.value || "";
  const status = optimizationEditorStatus(objectiveOutput, selectedRows, invalidRows, invalidConstraints);
  const warning = section.querySelector(".optimization-editor-warning");
  if (warning) {
    warning.textContent = status.message;
    warning.classList.toggle("ready", status.valid);
  }
  const create = section.querySelector("[data-optimization-create]");
  if (create) create.disabled = !status.valid;
}

function optimizationEditorStatus(objectiveOutput, selectedRows, invalidRows, invalidConstraints) {
  if (!objectiveOutput) return { valid: false, message: "Select an objective output" };
  if (!selectedRows.length) return { valid: false, message: "Select at least one decision variable" };
  if (invalidRows.length) return { valid: false, message: "Fix invalid decision bounds" };
  if (invalidConstraints.length) return { valid: false, message: "Fix invalid constraints" };
  return { valid: true, message: "Ready to create setup" };
}

function validOptimizationConstraint(row) {
  const value = Number(row.querySelector('[data-opt-constraint-field="value"]')?.value);
  const tolerance = Number(row.querySelector('[data-opt-constraint-field="tolerance"]')?.value);
  const penalty = Number(row.querySelector('[data-opt-constraint-field="penalty"]')?.value);
  return Number.isFinite(value) && Number.isFinite(tolerance) && Number.isFinite(penalty) && tolerance >= 0 && penalty >= 0;
}

function optimizationGridPointCount(row) {
  const min = Number(row.querySelector('[data-opt-var-field="min"]')?.value);
  const max = Number(row.querySelector('[data-opt-var-field="max"]')?.value);
  const step = Number(row.querySelector('[data-opt-var-field="step"]')?.value);
  if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) return 0;
  return Math.max(1, Math.floor(((max - min) / step) + 1.000000001));
}

function collectOptimizationSetupEditorPayload(section, context) {
  const objectiveOutput = section.querySelector("[data-optimization-objective]")?.value || "";
  if (!objectiveOutput) {
    context.showProblem?.("Select an optimization objective output");
    return null;
  }
  const variables = [];
  for (const row of section.querySelectorAll("[data-opt-var]")) {
    if (!row.querySelector("[data-opt-var-check]")?.checked) continue;
    const min = Number(row.querySelector('[data-opt-var-field="min"]')?.value);
    const max = Number(row.querySelector('[data-opt-var-field="max"]')?.value);
    const step = Number(row.querySelector('[data-opt-var-field="step"]')?.value);
    if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || step <= 0 || max < min) {
      context.showProblem?.(`Invalid optimization bounds: ${row.dataset.optVar}`);
      return null;
    }
    const variable = {
      kind: row.dataset.kind,
      name: row.dataset.name,
      min,
      max,
      step,
    };
    if (row.dataset.kind === "component_parameter") {
      variable.component = row.dataset.component;
    }
    if (row.dataset.kind === "system_parameter") {
      variable.component = row.dataset.component;
    }
    variables.push(variable);
  }
  if (!variables.length) {
    context.showProblem?.("Select at least one optimization decision variable");
    return null;
  }
  const constraints = [];
  for (const row of section.querySelectorAll("[data-opt-constraint]")) {
    if (!row.querySelector("[data-opt-constraint-check]")?.checked) continue;
    const value = Number(row.querySelector('[data-opt-constraint-field="value"]')?.value);
    const tolerance = Number(row.querySelector('[data-opt-constraint-field="tolerance"]')?.value);
    const penalty = Number(row.querySelector('[data-opt-constraint-field="penalty"]')?.value);
    if (!Number.isFinite(value) || !Number.isFinite(tolerance) || !Number.isFinite(penalty) || tolerance < 0 || penalty < 0) {
      context.showProblem?.(`Invalid optimization constraint: ${row.dataset.optConstraint}`);
      return null;
    }
    constraints.push({
      output: row.dataset.optConstraint,
      operator: row.querySelector('[data-opt-constraint-field="operator"]')?.value || "<=",
      value,
      tolerance,
      penalty,
    });
  }
  return {
    id: section.querySelector("[data-optimization-setup-id]")?.value.trim() || "",
    name: section.querySelector("[data-optimization-setup-name]")?.value.trim() || "",
    algorithm: section.querySelector("[data-optimization-algorithm]")?.value || "grid",
    base_parameter_set: section.querySelector("[data-optimization-base]")?.value || "",
    base_inputs: optimizationEditorBaseInputs(section, context),
    context: optimizationEditorBaseContext(section, context),
    objective: {
      output: objectiveOutput,
      sense: section.querySelector("[data-optimization-sense]")?.value || "min",
    },
    decision_variables: variables,
    constraints,
  };
}
