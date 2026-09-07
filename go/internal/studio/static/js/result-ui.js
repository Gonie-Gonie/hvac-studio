import { escapeAttr, escapeHTML } from "./dom.js";

export function resultHeader(title, subtitle, status, helpPath = "") {
  const header = document.createElement("div");
  header.className = "result-header";
  header.innerHTML = `
    <div>
      <div class="result-title">${escapeHTML(title)}</div>
      <div class="result-subtitle">${escapeHTML(subtitle || "")}</div>
    </div>
    <div class="result-header-actions">
      <div class="result-status">${escapeHTML(status || "")}</div>
      ${helpPath ? `<a class="help-button result-help-button" href="${escapeAttr(helpPath)}" target="_blank" rel="noopener" title="Open related help" aria-label="Open related help">?</a>` : ""}
    </div>
  `;
  return header;
}

export function resultTable(title, rows, headers = ["Item", "Value"]) {
  const block = document.createElement("div");
  block.className = "result-block";
  const normalizedRows = rows || [];
  block.innerHTML = `
    <div class="result-block-title">${escapeHTML(title)}</div>
    <table class="result-table">
      <thead><tr>${headers.map((header) => `<th>${escapeHTML(header)}</th>`).join("")}</tr></thead>
      <tbody></tbody>
    </table>
  `;
  const tbody = block.querySelector("tbody");
  if (!normalizedRows.length) {
    tbody.innerHTML = `<tr><td colspan="${headers.length}" class="empty-cell">No ${escapeHTML(String(title || "rows").toLowerCase())}</td></tr>`;
    return block;
  }
  for (const row of normalizedRows) {
    const cells = Array.isArray(row) ? row : [row.name || row[0] || "", row.value || row[1] || ""];
    const tr = document.createElement("tr");
    tr.innerHTML = headers.map((_, index) => `<td>${escapeHTML(cells[index] ?? "")}</td>`).join("");
    tbody.append(tr);
  }
  return block;
}

export function objectRows(value) {
  return Object.entries(value || {}).filter(([, item]) => typeof item !== "object").map(([key, item]) => [key, item]);
}

export function rawJSONBlock(value) {
  const details = document.createElement("details");
  details.className = "result-raw";
  const summary = document.createElement("summary");
  summary.textContent = "Raw JSON / Diagnostics";
  const pre = document.createElement("pre");
  pre.className = "code-pane result-json";
  pre.textContent = JSON.stringify(value, null, 2);
  details.append(summary, pre);
  return details;
}
