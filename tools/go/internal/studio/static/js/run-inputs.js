import { escapeAttr, escapeHTML } from "./dom.js";
import { parameterInputValue, sampleValueFor } from "./format.js";
import { state } from "./state.js";

export function renderRunInputs(context) {
  const container = context.container();
  container.innerHTML = "";
  const inputs = context.currentSystem()?.public_inputs || [];
  const savedInputs = state.activeRunInput?.inputs || state.detail?.default_run_input?.inputs || {};
  context.normalizeSeriesInputSelection();
  container.append(context.parameterSetField());
  container.append(context.runTimeoutField());
  container.append(context.seriesInputField());
  for (const input of inputs) {
    const field = document.createElement("div");
    field.className = "input-field";
    const defaultValue = savedInputs[input.id] ?? input.default ?? sampleValueFor(input.id);
    const label = input.name || input.id;
    const meta = runInputMeta(input, label);
    field.innerHTML = `
      <label for="input-${escapeAttr(input.id)}">
        <span class="input-label">${escapeHTML(label)}</span>
        ${meta ? `<span class="input-meta">${escapeHTML(meta)}</span>` : ""}
      </label>
      <input id="input-${escapeAttr(input.id)}" data-input-id="${escapeAttr(input.id)}" value="${escapeAttr(defaultValue)}" />
    `;
    field.querySelector("input").addEventListener("input", () => markRunInputsEdited(context));
    const reset = document.createElement("button");
    reset.type = "button";
    reset.className = "input-reset";
    reset.textContent = "Default";
    reset.addEventListener("click", () => resetRunInput(input, context));
    field.append(reset);
    container.append(field);
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
    context.markRunResultStale();
    renderRunInputs(context);
    context.renderSystemHeader();
  });
  field.append(button);
  return field;
}
