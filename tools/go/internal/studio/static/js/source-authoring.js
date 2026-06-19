import { parameterInputValue } from "./format.js";
import { roleLabel } from "./contract-labels.js";
import {
  pythonIdentifier,
  pythonLiteral,
  pythonStringLiteral,
} from "./python-source.js";

export {
  bracketCheck,
  formatPythonSource,
  highlightPython,
  pythonIdentifier,
  pythonLiteral,
  pythonStringLiteral,
  sourceOffsetForLineColumn,
} from "./python-source.js";

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

export function sourceQuickFixForProblem(problem, component) {
  if (!component) return null;
  const message = String(problem.message || "");
  let match = message.match(/^required input node is not referenced in source: (.+)$/);
  if (match) {
    const nodeID = match[1];
    const variable = pythonIdentifier(nodeID) || "value";
    return {
      title: `Insert input read for ${nodeID}`,
      snippet: `${variable} = inputs.get(${pythonStringLiteral(nodeID)}, 0.0)`,
    };
  }
  match = message.match(/^output node is not obviously returned by source: (.+)$/);
  if (match) {
    const nodeID = match[1];
    return {
      title: `Insert output entry for ${nodeID}`,
      snippet: `${pythonStringLiteral(nodeID)}: value`,
    };
  }
  if (message === "evaluate method is missing") {
    return {
      title: "Insert evaluate method scaffold",
      snippet: evaluateSnippet(component, pythonInputBindings(component)),
    };
  }
  if (message === "step function is missing") {
    return {
      title: "Insert step function scaffold",
      snippet: stepSnippet(component, pythonInputBindings(component)),
    };
  }
  match = message.match(/^(input node|output node|parameter|state) reference is not in component contract: (.+)$/);
  if (match && problem.line && problem.column) {
    const scope = match[1];
    const missingName = match[2];
    const candidates = sourceReferenceCandidates(scope, component);
    const replacement = closestSourceName(missingName, candidates);
    if (!replacement) return null;
    return {
      title: `Replace ${scope} reference with ${replacement}`,
      replacement: {
        expected: missingName,
        value: replacement,
      },
    };
  }
  return null;
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
