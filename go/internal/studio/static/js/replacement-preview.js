export function replacementPreview(component, template, system, graphConnections = [], mapParameters = true) {
  const diff = replacementContractDiff(component, template);
  const nodeMappings = replacementNodeMappings(component, template, system, graphConnections);
  const parameterMappings = replacementParameterMappings(component, template, mapParameters);
  const problems = nodeMappings
    .filter((mapping) => mapping.status === "missing")
    .map((mapping) => ({
      severity: "error",
      component_id: component.id,
      node_id: mapping.node_id || "",
      message: `replacement missing ${mapping.direction || "node"} for ${String(mapping.scope || "").replace(/_/g, " ")} ${mapping.id}: ${mapping.node_id || mapping.from}`,
    }));
  return { diff, nodeMappings, parameterMappings, problems };
}

export function replacementNodeMappings(component, template, system, graphConnections = []) {
  if (!system) return [];
  const templateInputs = new Set(contractNodeIDs(template.inputs || []));
  const templateOutputs = new Set(contractNodeIDs(template.outputs || []));
  const mappings = [];
  for (const input of system.public_inputs || []) {
    if (input.component !== component.id) continue;
    const found = templateInputs.has(input.node);
    mappings.push(replacementMapping("public_input", input.id, component.id, template.id, input.node, "input", found, found ? "public input preserved" : "replacement input node is missing"));
  }
  for (const output of system.public_outputs || []) {
    if (output.component !== component.id) continue;
    const found = templateOutputs.has(output.node);
    mappings.push(replacementMapping("public_output", output.id, component.id, template.id, output.node, "output", found, found ? "public output preserved" : "replacement output node is missing"));
  }
  for (const connectionID of system.connections || []) {
    const connection = (graphConnections || []).find((item) => item.id === connectionID);
    if (!connection) continue;
    if (connection.from?.component === component.id) {
      const nodeID = connection.from.node;
      const found = templateOutputs.has(nodeID);
      mappings.push(replacementMapping("connection_output", connection.id, component.id, template.id, nodeID, "output", found, found ? "connection source preserved" : "replacement output node is missing"));
    }
    if (connection.to?.component === component.id) {
      const nodeID = connection.to.node;
      const found = templateInputs.has(nodeID);
      mappings.push(replacementMapping("connection_input", connection.id, component.id, template.id, nodeID, "input", found, found ? "connection target preserved" : "replacement input node is missing"));
    }
  }
  return mappings;
}

export function replacementParameterMappings(component, template, mapParameters) {
  return contractParameterIDs(template).map((name) => {
    const found = contractParameterIDs(component).includes(name);
    return {
      scope: "parameter",
      id: name,
      from: `${component.id}.${name}`,
      to: `${template.id}.${name}`,
      status: mapParameters ? (found ? "copied" : "missing") : "skipped",
      detail: mapParameters ? (found ? "same-name parameter value copied" : "source parameter is not present") : "parameter mapping disabled",
    };
  });
}

export function replacementContractDiff(component, template) {
  const originalInputs = contractNodeIDs(component.nodes?.inputs || []);
  const replacementInputs = contractNodeIDs(template.inputs || []);
  const originalOutputs = contractNodeIDs(component.nodes?.outputs || []);
  const replacementOutputs = contractNodeIDs(template.outputs || []);
  const originalParameters = contractParameterIDs(component);
  const replacementParameters = contractParameterIDs(template);
  return {
    originalInputs,
    replacementInputs,
    matchedInputs: intersectLists(originalInputs, replacementInputs),
    missingInputs: differenceLists(originalInputs, replacementInputs),
    addedInputs: differenceLists(replacementInputs, originalInputs),
    originalOutputs,
    replacementOutputs,
    matchedOutputs: intersectLists(originalOutputs, replacementOutputs),
    missingOutputs: differenceLists(originalOutputs, replacementOutputs),
    addedOutputs: differenceLists(replacementOutputs, originalOutputs),
    originalParameters,
    replacementParameters,
    matchedParameters: intersectLists(originalParameters, replacementParameters),
    missingParameters: differenceLists(originalParameters, replacementParameters),
    addedParameters: differenceLists(replacementParameters, originalParameters),
  };
}

export function replacementDiffText(matched, missing, added) {
  return [
    `${matched.length} matched`,
    missing.length ? `missing ${missing.join(", ")}` : "",
    added.length ? `new ${added.join(", ")}` : "",
  ].filter(Boolean).join(" / ");
}

export function contractNodeIDs(nodes) {
  return [...new Set((nodes || []).map((node) => node.id).filter(Boolean))].sort();
}

export function contractParameterIDs(contract) {
  return [...new Set([
    ...Object.keys(contract.parameters || {}),
    ...Object.keys(contract.parameter_defs || {}),
  ].filter(Boolean))].sort();
}

export function intersectLists(left, right) {
  const rightSet = new Set(right);
  return left.filter((item) => rightSet.has(item));
}

export function differenceLists(left, right) {
  const rightSet = new Set(right);
  return left.filter((item) => !rightSet.has(item));
}

function replacementMapping(scope, id, sourceComponent, replacementTemplate, nodeID, direction, found, detail) {
  return {
    scope,
    id,
    node_id: nodeID,
    direction,
    from: `${sourceComponent}.${nodeID}`,
    to: `${replacementTemplate}.${nodeID}`,
    status: found ? "preserved" : "missing",
    detail,
  };
}
