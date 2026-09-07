import { shortNumber } from "./format.js";

export function defaultCalibrationGridStep(min, max) {
  const step = (Number(max) - Number(min)) / 4;
  if (!Number.isFinite(step) || step <= 0) return "1";
  return String(Math.round(step * 1e9) / 1e9);
}

export function labeledEditorControl(label, control) {
  const field = document.createElement("label");
  field.className = "editor-control";
  field.append(textSpan("input-label", label), control);
  return field;
}

export function labeledEditorInput(label, type, placeholder, dataName) {
  const input = document.createElement("input");
  input.type = type;
  input.placeholder = placeholder;
  input.setAttribute(`data-${dataName}`, "true");
  return labeledEditorControl(label, input);
}

export function formatExpectedRunCount(value) {
  if (!Number.isFinite(value)) return "too many";
  if (value > 999999) return `${shortNumber(value)}+`;
  return String(value);
}

function textSpan(className, text) {
  const span = document.createElement("span");
  span.className = className;
  span.textContent = text;
  return span;
}
