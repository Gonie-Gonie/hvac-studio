import { escapeHTML } from "./dom.js";
import { parameterInputValue } from "./format.js";
import { roleLabel } from "./contract-labels.js";

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

export function sourceSnippet(kind, component) {
  const inputBindings = pythonInputBindings(component);
  const firstInput = inputBindings[0]?.id || "value";
  const firstOutput = (component.nodes.outputs || [])[0]?.id || "result";
  const firstParam = Object.keys(component.parameters || {})[0] || "gain";
  switch (kind) {
    case "initialize":
      return `\n    def initialize(self, params, context):\n        return {}\n`;
    case "output":
      return `${pythonStringLiteral(firstOutput)}: value`;
    case "input":
      return `inputs.get(${pythonStringLiteral(firstInput)}, 0.0)`;
    case "parameter":
      return `params.get(${pythonStringLiteral(firstParam)}, 1.0)`;
    case "vectorized":
      return vectorizedSnippet(component, inputBindings);
    case "external":
      return externalExecutableSnippet(component);
    default:
      return component.source?.layout === "generated_wrapper"
        ? stepSnippet(component, inputBindings)
        : evaluateSnippet(component, inputBindings);
  }
}

export function stepSnippet(component, inputBindings) {
  const bindings = inputBindings.length ? inputBindings : [{ id: "value", varName: "value" }];
  const inputLines = bindings.map((item) => `    ${item.varName} = float(inputs.get(${pythonStringLiteral(item.id)}, 0.0))`).join("\n");
  const primaryValue = bindings[0].varName;
  const outputs = component.nodes.outputs || [];
  const outputLines = (outputs.length ? outputs : [{ id: "result" }])
    .map((node) => `        ${pythonStringLiteral(node.id)}: ${primaryValue},`)
    .join("\n");
  return `\ndef step(inputs, state, params, context):\n${inputLines}\n    return {\n${outputLines}\n    }, state\n`;
}

export function evaluateSnippet(component, inputBindings) {
  const bindings = inputBindings.length ? inputBindings : [{ id: "value", varName: "value" }];
  const inputLines = bindings.map((item) => `        ${item.varName} = float(inputs.get(${pythonStringLiteral(item.id)}, 0.0))`).join("\n");
  const primaryValue = bindings[0].varName;
  const outputs = component.nodes.outputs || [];
  const outputLines = (outputs.length ? outputs : [{ id: "result" }])
    .map((node) => `            ${pythonStringLiteral(node.id)}: ${primaryValue},`)
    .join("\n");
  return `\n    def evaluate(self, inputs, state, params, context):\n${inputLines}\n        return {\n${outputLines}\n        }, state\n`;
}

function vectorizedSnippet(component, inputBindings) {
  return component.source?.layout === "generated_wrapper"
    ? vectorizedStepSnippet(component, inputBindings)
    : evaluateBatchSnippet(component, inputBindings);
}

function vectorizedStepSnippet(component, inputBindings) {
  const firstInput = inputBindings[0]?.id || "values";
  const firstOutput = (component.nodes.outputs || [])[0]?.id || "results";
  const firstParam = Object.keys(component.parameters || {})[0] || "gain";
  return `\ndef step(inputs, state, params, context):\n    values = inputs.get(${pythonStringLiteral(firstInput)}, [])\n    gain = float(params.get(${pythonStringLiteral(firstParam)}, 1.0))\n    return {\n        ${pythonStringLiteral(firstOutput)}: [float(value) * gain for value in values],\n    }, state\n`;
}

export function evaluateBatchSnippet(component, inputBindings) {
  const firstInput = inputBindings[0]?.id || "values";
  const firstOutput = (component.nodes.outputs || [])[0]?.id || "results";
  const firstParam = Object.keys(component.parameters || {})[0] || "gain";
  return `\n    def evaluate_batch(self, inputs, state, params, context):\n        values = inputs.get(${pythonStringLiteral(firstInput)}, [])\n        gain = float(params.get(${pythonStringLiteral(firstParam)}, 1.0))\n        return {\n            ${pythonStringLiteral(firstOutput)}: [float(value) * gain for value in values],\n        }, state\n`;
}

export function externalExecutableSnippet(component) {
  const firstInput = (component.nodes.inputs || [])[0]?.id || "request";
  const outputs = component.nodes.outputs || [{ id: "response" }];
  const outputLines = outputs
    .map((node) => `            ${pythonStringLiteral(node.id)}: inputs.get(${pythonStringLiteral(firstInput)}, None),`)
    .join("\n");
  return `\nimport json\nimport sys\n\n\ndef main():\n    request = json.load(sys.stdin)\n    inputs = request.get("inputs", {})\n    state = request.get("state", {})\n    response = {\n        "ok": True,\n        "outputs": {\n${outputLines}\n        },\n        "state": state,\n        "logs": [\n            {"severity": "info", "message": "external call complete"},\n        ],\n    }\n    json.dump(response, sys.stdout)\n    sys.stdout.write("\\n")\n\n\nif __name__ == "__main__":\n    main()\n`;
}

export function pythonInputBindings(component) {
  const used = new Set();
  return (component.nodes.inputs || []).map((node, index) => {
    const fallback = `input_${index + 1}`;
    const base = pythonIdentifier(node.id) || fallback;
    let candidate = base;
    let suffix = 2;
    while (used.has(candidate)) {
      candidate = `${base}_${suffix}`;
      suffix += 1;
    }
    used.add(candidate);
    return { id: node.id || fallback, varName: candidate };
  });
}

export function sourceCompletionItems(component) {
  if (!component) return [];
  const items = [];
  const inputBindings = pythonInputBindings(component);
  for (const item of inputBindings) {
    const node = (component.nodes.inputs || []).find((candidate) => candidate.id === item.id) || {};
    items.push({
      name: `inputs[${pythonStringLiteral(item.id)}]`,
      meta: nodeTypeLabel(node) || "input",
      scope: "input node",
      details: nodeSourceDetails(node),
      snippet: `inputs.get(${pythonStringLiteral(item.id)}, 0.0)`,
    });
    items.push({
      name: item.varName,
      meta: `local from ${item.id}`,
      scope: "local binding",
      details: { node: item.id },
      snippet: item.varName,
    });
  }
  for (const node of component.nodes.outputs || []) {
    items.push({
      name: `${pythonStringLiteral(node.id)}: value`,
      meta: nodeTypeLabel(node) || "output",
      scope: "output node",
      details: nodeSourceDetails(node),
      snippet: `${pythonStringLiteral(node.id)}: value`,
    });
  }
  for (const item of parameterSourceItems(component)) {
    items.push({
      name: `params[${pythonStringLiteral(item.name)}]`,
      meta: item.meta,
      snippet: item.snippet,
    });
  }
  for (const item of stateSourceItems(component)) {
    items.push({
      name: `state[${pythonStringLiteral(item.name)}]`,
      meta: item.meta,
      snippet: item.snippet,
    });
  }
  for (const item of contextSourceItems()) {
    items.push({
      name: `context[${pythonStringLiteral(item.name)}]`,
      meta: item.meta,
      snippet: item.snippet,
    });
  }
  return items;
}

export function sourceReferenceCandidates(scope, component) {
  switch (scope) {
    case "input node":
      return (component.nodes?.inputs || []).map((node) => node.id);
    case "output node":
      return (component.nodes?.outputs || []).map((node) => node.id);
    case "parameter":
      return parameterSourceItems(component).map((item) => item.name);
    case "state":
      return stateSourceItems(component).map((item) => item.name);
    default:
      return [];
  }
}

export function closestSourceName(name, candidates) {
  const source = String(name || "").toLowerCase();
  if (!source || !candidates.length) return "";
  let best = "";
  let bestDistance = Infinity;
  for (const candidate of candidates) {
    const target = String(candidate || "");
    const distance = editDistance(source, target.toLowerCase());
    if (distance < bestDistance || (distance === bestDistance && target.localeCompare(best) < 0)) {
      best = target;
      bestDistance = distance;
    }
  }
  const limit = Math.max(1, Math.ceil(Math.max(source.length, best.length) * 0.4));
  return bestDistance <= limit ? best : "";
}

function editDistance(left, right) {
  const previous = Array.from({ length: right.length + 1 }, (_, index) => index);
  for (let i = 1; i <= left.length; i += 1) {
    let diagonal = previous[0];
    previous[0] = i;
    for (let j = 1; j <= right.length; j += 1) {
      const saved = previous[j];
      const cost = left[i - 1] === right[j - 1] ? 0 : 1;
      previous[j] = Math.min(previous[j] + 1, previous[j - 1] + 1, diagonal + cost);
      diagonal = saved;
    }
  }
  return previous[right.length];
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

export function nodeTypeLabel(node) {
  return [node.medium || "", node.value_type || "", node.unit || ""].filter(Boolean).join(" / ");
}

export function nodeSourceItem(scope, node, snippet) {
  return {
    name: node.id,
    meta: nodeTypeLabel(node),
    scope,
    details: nodeSourceDetails(node),
    snippet,
  };
}

function nodeSourceDetails(node) {
  return {
    medium: node.medium || "",
    value_type: node.value_type || "",
    unit: node.unit || "",
    required: node.required,
    default: node.default !== undefined ? parameterInputValue(node.default) : undefined,
  };
}

export function sourceItemTitle(item) {
  const details = item?.details || {};
  return [
    item?.name ? `Name: ${item.name}` : "",
    item?.scope ? `Scope: ${item.scope}` : "",
    details.node ? `Node: ${details.node}` : "",
    details.medium ? `Medium: ${details.medium}` : "",
    details.value_type ? `Value type: ${details.value_type}` : "",
    details.unit ? `Unit: ${details.unit}` : "",
    details.required !== undefined ? `Required: ${details.required ? "yes" : "no"}` : "",
    details.default !== undefined ? `Default: ${details.default}` : "",
    details.current !== undefined ? `Current: ${details.current}` : "",
    details.role ? `Role: ${details.role}` : "",
    details.group ? `Group: ${details.group}` : "",
    details.initial !== undefined ? `Initial: ${details.initial}` : "",
    item?.meta ? `Summary: ${item.meta}` : "",
    item?.snippet ? `Insert: ${item.snippet}` : "",
  ].filter(Boolean).join("\n");
}

export function parameterSourceItems(component) {
  const definitions = component.parameter_defs || {};
  const names = new Set([...Object.keys(component.parameters || {}), ...Object.keys(definitions)]);
  return [...names].sort().map((name) => {
    const definition = definitions[name] || {};
    const value = component.parameters?.[name] ?? definition.current ?? definition.default ?? 0.0;
    return {
      name,
      meta: [
        parameterInputValue(value),
        definition.unit || "",
        roleLabel(definition.role || "parameter"),
      ].filter(Boolean).join(" / "),
      scope: "parameter",
      details: {
        current: parameterInputValue(value),
        default: definition.default !== undefined ? parameterInputValue(definition.default) : undefined,
        unit: definition.unit || "",
        role: roleLabel(definition.role || "parameter"),
        group: definition.group || "",
      },
      snippet: `params.get(${pythonStringLiteral(name)}, ${pythonLiteral(value)})`,
    };
  });
}

export function stateSourceItems(component) {
  return Object.entries(component.state_defs || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, definition]) => ({
      name,
      meta: [definition.unit || "", "state"].filter(Boolean).join(" / "),
      scope: "state",
      details: {
        unit: definition.unit || "",
        initial: parameterInputValue(definition.initial),
      },
      snippet: `state.get(${pythonStringLiteral(name)}, ${pythonLiteral(definition.initial)})`,
    }));
}

export function contextSourceItems() {
  return ["time", "dt"].map((name) => ({
    name,
    meta: "context",
    scope: "context",
    snippet: `context.get(${pythonStringLiteral(name)}, 0.0)`,
  }));
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
