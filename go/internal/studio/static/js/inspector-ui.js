import { escapeHTML } from "./dom.js";

export function inspectorBlock(title, rows, options = {}) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
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
  row.innerHTML = `<span class="kv-key">${escapeHTML(key)}</span><span>${escapeHTML(value)}</span>`;
  return row;
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
