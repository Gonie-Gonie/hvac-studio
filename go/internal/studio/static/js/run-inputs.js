import { escapeAttr, escapeHTML } from "./dom.js";
import { parameterInputValue, sampleValueFor } from "./format.js";
import { state } from "./state.js";

export function renderRunInputs(context) {
  const container = context.container();
  container.innerHTML = "";
  const inputs = context.currentSystem()?.public_inputs || [];
  const savedInputs = state.activeRunInput?.inputs || state.detail?.default_run_input?.inputs || {};
  context.normalizeSeriesInputSelection();
  const environment = document.createElement("div");
  environment.className = "run-environment-fields";
  environment.append(context.parameterSetField(), context.runTimeoutField(), context.seriesInputField());
  const publicFields = document.createElement("div");
  publicFields.className = "run-public-fields";
  container.append(environment, publicFields);
  const summary = document.getElementById("runSettingsSummary");
  if (summary) summary.textContent = `${inputs.length} inputs · ${state.activeParameterSetPath ? "Custom parameters" : "Baseline parameters"}`;
  for (const input of inputs) {
    const field = document.createElement("div");
    field.className = "input-field";
    const hasDraft = state.runInputDraft && Object.prototype.hasOwnProperty.call(state.runInputDraft, input.id);
    const defaultValue = hasDraft ? state.runInputDraft[input.id] : savedInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
    const label = input.name || input.id;
    const meta = runInputMeta(input, label);
    field.innerHTML = `
      <label for="input-${escapeAttr(input.id)}" title="${escapeAttr(meta)}">
        <span class="input-label">${escapeHTML(label)}</span>
        ${input.unit ? `<span class="input-unit">${escapeHTML(input.unit)}</span>` : ""}
      </label>
      <input id="input-${escapeAttr(input.id)}" data-input-id="${escapeAttr(input.id)}" value="${escapeAttr(parameterInputValue(defaultValue))}" />
    `;
    field.querySelector("input").addEventListener("input", () => markRunInputsEdited(context));
    const reset = document.createElement("button");
    reset.type = "button";
    reset.className = "input-reset";
    reset.textContent = "↺";
    reset.title = `Reset ${label} to default`;
    reset.setAttribute("aria-label", `Reset ${label} to default`);
    reset.addEventListener("click", () => resetRunInput(input, context));
    field.append(reset);
    publicFields.append(field);
  }
  if (context.isWorkspaceProject()) {
    const activeScenario = activeScenarioBadge(context);
    if (activeScenario) container.append(activeScenario);
    container.append(scenarioNameField(context));
  }
}

export function runInputMeta(input, label) {
  return [
    input.id && input.id !== label ? input.id : "",
    input.value_type || "",
    input.unit || "",
    input.required === false ? "optional" : "required",
  ].filter(Boolean).join(" / ");
}

export function resetRunInput(input, context) {
  const control = [...document.querySelectorAll("[data-input-id]")].find((item) => item.dataset.inputId === input.id);
  if (!control) return;
  const defaultInputs = state.detail?.default_run_input?.inputs || {};
  const value = defaultInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
  control.value = parameterInputValue(value);
  markRunInputsEdited(context);
}

export function markRunInputsEdited(context) {
  // Preserve the entire visible input set before detaching a scenario: its other
  // values must remain intact when a selector or an inspector edit rerenders it.
  state.runInputDraft = Object.fromEntries(
    [...context.container().querySelectorAll("[data-input-id]")].map((input) => [input.dataset.inputId, input.value]),
  );
  if (state.activeRunInput) {
    state.activeRunInput = null;
    document.querySelector(".active-scenario")?.remove();
  }
  context.markProjectDirty();
}

export function scenarioNameField(context) {
  const field = document.createElement("div");
  field.className = "scenario-name-field";
  const input = document.createElement("input");
  input.id = "scenarioNameInput";
  input.placeholder = "Scenario name";
  input.value = state.scenarioDraftName;
  input.setAttribute("aria-label", "Scenario name");
  input.addEventListener("input", () => {
    state.scenarioDraftName = input.value;
  });
  input.addEventListener("keydown", (event) => {
    if (event.key === "Enter") context.createScenario();
  });
  field.append(input);
  return field;
}

export function activeScenarioBadge(context) {
  if (!state.activeRunInput) return null;
  const field = document.createElement("div");
  field.className = "active-scenario";
  const name = state.activeRunInput.name || state.activeRunInput.id || "scenario";
  field.innerHTML = `<span>${escapeHTML(`Scenario: ${name}`)}</span>`;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "input-reset";
  button.textContent = "Clear";
  button.addEventListener("click", () => {
    state.activeRunInput = null;
    state.runInputDraft = null;
    context.markRunResultStale();
    renderRunInputs(context);
    context.renderSystemHeader();
  });
  field.append(button);
  return field;
}
