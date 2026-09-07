import { escapeHTML } from "./dom.js";

const disclosureStates = new Map();

export function inspectorSection(title, options = {}) {
  const block = document.createElement(options.collapsible ? "details" : "div");
  block.className = `inspector-block${options.collapsible ? " inspector-section" : ""}`;
  const heading = document.createElement(options.collapsible ? "summary" : "div");
  heading.className = "inspector-title";
  const label = document.createElement("span");
  label.textContent = title;
  heading.append(label);
  if (options.count !== undefined) {
    const count = document.createElement("span");
    count.className = "inspector-count";
    count.textContent = String(options.count);
    heading.append(count);
  }
  block.append(heading);
  if (options.collapsible) {
    rememberDisclosure(block, options.stateKey, options.open !== false);
  }
  return block;
}

export function rememberDisclosure(details, key, initiallyOpen = false) {
  details.open = key && disclosureStates.has(key) ? disclosureStates.get(key) : initiallyOpen;
  if (key) details.addEventListener("toggle", () => disclosureStates.set(key, details.open));
  return details;
}

export function inspectorBlock(title, rows, options = {}) {
  const block = inspectorSection(title, options);
  if (!rows.length) {
    block.append(emptyKVRow(options.emptyMessage || "No values", {
      messagePlacement: options.emptyMessagePlacement || "value",
    }));
    return block;
  }
  for (const [key, value] of rows) block.append(inspectorKVRow(key, value));
  return block;
}

export function inspectorKVRow(key, value) {
  const row = document.createElement("div");
  row.className = "kv";
  const label = document.createElement("span");
  label.className = "kv-key";
  label.textContent = key;
  label.title = key;
  row.append(label, inspectorValue(value));
  return row;
}

export function inspectorValue(value) {
  const text = typeof value === "object" && value !== null ? JSON.stringify(value) : String(value ?? "");
  if (text.length <= 56 && !text.includes("\n")) {
    const span = document.createElement("span");
    span.className = "inspector-value";
    span.textContent = text;
    span.title = text;
    return span;
  }
  const details = document.createElement("details");
  details.className = "inspector-value inspector-value-expand";
  const summary = document.createElement("summary");
  summary.textContent = text.replace(/\s+/g, " ");
  summary.title = "Expand full value";
  summary.setAttribute("aria-label", "Expand full value");
  const full = document.createElement("pre");
  try {
    full.textContent = JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    full.textContent = text;
  }
  details.append(summary, full);
  return details;
}

export function inspectorLabel(value) {
  return String(value || "").replace(/_/g, " ").replace(/\b\w/g, (character) => character.toUpperCase());
}

export function inspectorUnit(value) {
  return ({ degC: "°C", degF: "°F", fraction: "0–1", dimensionless: "" })[value] ?? value ?? "";
}

export function emptyKVRow(message, options = {}) {
  const messageInKey = options.messagePlacement === "key";
  const row = document.createElement("div");
  row.className = "kv";
  row.innerHTML = messageInKey
    ? `<span class="kv-key">${escapeHTML(message)}</span><span></span>`
    : `<span class="kv-key"></span><span>${escapeHTML(message)}</span>`;
  return row;
}
