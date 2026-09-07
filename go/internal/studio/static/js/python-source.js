import { escapeHTML } from "./dom.js";
import { parameterInputValue } from "./format.js";

export function formatPythonSource(value) {
  const normalized = String(value || "").replace(/\r\n?/g, "\n");
  const lines = normalized.split("\n").map((line) => {
    const withoutTrailing = line.replace(/[ \t]+$/g, "");
    return withoutTrailing.replace(/^\t+/, (tabs) => "    ".repeat(tabs.length));
  });
  while (lines.length > 1 && lines[lines.length - 1] === "") {
    lines.pop();
  }
  return `${lines.join("\n")}\n`;
}

export function insertEditorText(editor, text) {
  const insert = String(text || "");
  const start = editor.selectionStart ?? editor.value.length;
  const end = editor.selectionEnd ?? editor.value.length;
  editor.value = `${editor.value.slice(0, start)}${insert}${editor.value.slice(end)}`;
  editor.selectionStart = editor.selectionEnd = start + insert.length;
}

export function applyEditorNewline(editor) {
  const start = editor.selectionStart ?? 0;
  const end = editor.selectionEnd ?? start;
  const value = editor.value;
  const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  const currentLine = value.slice(lineStart, start);
  const indent = currentLine.match(/^\s*/)?.[0] || "";
  const extra = currentLine.trimEnd().endsWith(":") ? "    " : "";
  const insert = `\n${indent}${extra}`;
  editor.value = `${value.slice(0, start)}${insert}${value.slice(end)}`;
  editor.selectionStart = editor.selectionEnd = start + insert.length;
}

export function applyEditorIndent(editor, outdent) {
  const start = editor.selectionStart ?? 0;
  const end = editor.selectionEnd ?? start;
  if (!outdent && start === end) {
    editor.value = `${editor.value.slice(0, start)}    ${editor.value.slice(end)}`;
    editor.selectionStart = editor.selectionEnd = start + 4;
    return;
  }

  const value = editor.value;
  const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  const adjustedEnd = end > start && value[end - 1] === "\n" ? end - 1 : end;
  const nextLineBreak = value.indexOf("\n", adjustedEnd);
  const lineEnd = nextLineBreak < 0 ? value.length : nextLineBreak;
  const selected = value.slice(lineStart, lineEnd);
  const lines = selected.split("\n");
  const transformed = lines.map((line) => (outdent ? outdentEditorLine(line) : `    ${line}`));
  const replacement = transformed.join("\n");
  const delta = replacement.length - selected.length;
  const firstLineDelta = transformed[0].length - lines[0].length;

  editor.value = `${value.slice(0, lineStart)}${replacement}${value.slice(lineEnd)}`;
  editor.selectionStart = Math.max(lineStart, start + firstLineDelta);
  editor.selectionEnd = Math.max(editor.selectionStart, end + delta);
}

export function syncEditorScroll(source, gutter, highlight) {
  if (gutter) gutter.scrollTop = source.scrollTop;
  if (highlight) {
    highlight.scrollTop = source.scrollTop;
    highlight.scrollLeft = source.scrollLeft;
  }
}

function outdentEditorLine(line) {
  if (line.startsWith("    ")) return line.slice(4);
  if (line.startsWith("\t")) return line.slice(1);
  const leadingSpaces = line.match(/^ +/)?.[0]?.length || 0;
  return line.slice(Math.min(4, leadingSpaces));
}

export function sourceOffsetForLineColumn(value, line, column) {
  let offset = 0;
  for (let currentLine = 1; currentLine < line; currentLine++) {
    const next = value.indexOf("\n", offset);
    if (next < 0) return value.length;
    offset = next + 1;
  }
  return Math.min(value.length, offset + column - 1);
}

export function bracketCheck(value) {
  const openers = new Map([["(", ")"], ["[", "]"], ["{", "}"]]);
  const closers = new Set([...openers.values()]);
  const stack = [];
  let quote = "";
  let escaped = false;
  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === quote) {
        quote = "";
      }
      continue;
    }
    if (char === "#") {
      const nextLine = value.indexOf("\n", index);
      if (nextLine < 0) break;
      index = nextLine;
      continue;
    }
    if (char === "\"" || char === "'") {
      quote = char;
      continue;
    }
    if (openers.has(char)) {
      stack.push({ char, index });
      continue;
    }
    if (closers.has(char)) {
      const expected = stack.length ? openers.get(stack[stack.length - 1].char) : "";
      if (expected !== char) return { ok: false, message: "bracket mismatch" };
      stack.pop();
    }
  }
  if (stack.length) return { ok: false, message: `open ${stack[stack.length - 1].char}` };
  return { ok: true, message: "brackets ok" };
}

export function highlightPython(value) {
  const keywords = new Set([
    "and", "as", "assert", "break", "class", "continue", "def", "del", "elif", "else", "except",
    "False", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "None",
    "nonlocal", "not", "or", "pass", "raise", "return", "True", "try", "while", "with", "yield",
  ]);
  const builtins = new Set(["abs", "bool", "dict", "enumerate", "float", "int", "len", "list", "max", "min", "range", "round", "str", "sum"]);
  let output = "";
  for (let index = 0; index < value.length;) {
    const char = value[index];
    if (char === "#") {
      const end = value.indexOf("\n", index);
      const next = end < 0 ? value.length : end;
      output += `<span class="tok-comment">${escapeHTML(value.slice(index, next))}</span>`;
      index = next;
      continue;
    }
    if (char === "\"" || char === "'") {
      const start = index;
      const quote = char;
      index += 1;
      let escaped = false;
      while (index < value.length) {
        const nextChar = value[index];
        index += 1;
        if (escaped) {
          escaped = false;
        } else if (nextChar === "\\") {
          escaped = true;
        } else if (nextChar === quote) {
          break;
        }
      }
      output += `<span class="tok-string">${escapeHTML(value.slice(start, index))}</span>`;
      continue;
    }
    if (/[0-9]/.test(char)) {
      const match = value.slice(index).match(/^[0-9]+(?:\.[0-9]+)?/);
      output += `<span class="tok-number">${escapeHTML(match[0])}</span>`;
      index += match[0].length;
      continue;
    }
    if (/[A-Za-z_]/.test(char)) {
      const match = value.slice(index).match(/^[A-Za-z_][A-Za-z0-9_]*/);
      const token = match[0];
      if (keywords.has(token)) {
        output += `<span class="tok-keyword">${escapeHTML(token)}</span>`;
      } else if (builtins.has(token)) {
        output += `<span class="tok-builtin">${escapeHTML(token)}</span>`;
      } else {
        output += escapeHTML(token);
      }
      index += token.length;
      continue;
    }
    output += escapeHTML(char);
    index += 1;
  }
  return output.endsWith("\n") ? `${output} ` : output || " ";
}

export function pythonIdentifier(value) {
  const identifier = String(value || "")
    .replace(/[^A-Za-z0-9_]/g, "_")
    .replace(/^([0-9])/, "_$1")
    .replace(/^_+$/, "");
  return identifier || "";
}

export function pythonStringLiteral(value) {
  return JSON.stringify(String(value || ""));
}

export function pythonLiteral(value) {
  if (typeof value === "number") return Number.isFinite(value) ? String(value) : "0.0";
  if (typeof value === "boolean") return value ? "True" : "False";
  if (value === null || value === undefined) return "None";
  if (typeof value === "string") return pythonStringLiteral(value);
  return pythonStringLiteral(parameterInputValue(value));
}
