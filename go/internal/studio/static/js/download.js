export function downloadTextFile(name, content, type) {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function safeFileName(value) {
  return String(value || "export").replace(/[^A-Za-z0-9._-]+/g, "_").replace(/^_+|_+$/g, "") || "export";
}

export function csvCell(value) {
  const text = typeof value === "string" ? value : JSON.stringify(value);
  const normalized = text === undefined ? "" : String(text);
  return `"${normalized.replace(/"/g, '""')}"`;
}

export function markdownTable(rows, headers = ["Item", "Value"]) {
  const normalized = rows || [];
  if (!normalized.length) return ["No rows."];
  return [
    `| ${headers.map(markdownCell).join(" | ")} |`,
    `| ${headers.map(() => "---").join(" | ")} |`,
    ...normalized.map((row) => `| ${headers.map((_, index) => markdownCell(row[index] ?? "")).join(" | ")} |`),
  ];
}

function markdownCell(value) {
  return String(value ?? "").replace(/\|/g, "\\|").replace(/\r?\n/g, " ");
}
