export function nodeDeleteImpact(component, node, system, graphConnections = []) {
  const nodeID = typeof node === "string" ? node : node?.id;
  const componentID = component?.id || "";
  if (!componentID || !nodeID || !system) {
    return emptyImpact();
  }

  const direction = (component.nodes?.inputs || []).some((item) => item.id === nodeID)
    ? "input"
    : ((component.nodes?.outputs || []).some((item) => item.id === nodeID) ? "output" : "");
  const publicInputs = (system.public_inputs || [])
    .filter((ref) => ref.component === componentID && ref.node === nodeID)
    .map(publicRefLabel);
  const publicOutputs = (system.public_outputs || [])
    .filter((ref) => ref.component === componentID && ref.node === nodeID)
    .map(publicRefLabel);
  const connections = (system.connections || [])
    .map((id) => (graphConnections || []).find((connection) => connection.id === id))
    .filter(Boolean)
    .filter((connection) => endpointMatches(connection.from, componentID, nodeID) || endpointMatches(connection.to, componentID, nodeID));
  const removedConnectionIDs = new Set(connections.map((connection) => connection.id));
  const restoredPublicInputs = direction === "output"
    ? uniqueSorted(connections
      .filter((connection) => endpointMatches(connection.from, componentID, nodeID))
      .filter((connection) => (system.components || []).includes(connection.to?.component))
      .filter((connection) => !hasPublicInputFor(system, connection.to?.component, connection.to?.node))
      .filter((connection) => !hasIncomingConnectionAfterRemoval(system, graphConnections, connection.to?.component, connection.to?.node, removedConnectionIDs))
      .map((connection) => endpointLabel(connection.to)))
    : [];

  return {
    component_id: componentID,
    node_id: nodeID,
    direction,
    public_inputs: publicInputs,
    public_outputs: publicOutputs,
    default_inputs: direction === "input" ? publicInputs : [],
    connections: connections.map(connectionLabel),
    restored_public_inputs: restoredPublicInputs,
  };
}

export function nodeDeleteImpactSummary(impact) {
  const removals = [
    countLabel(impact.public_inputs, "public input"),
    countLabel(impact.public_outputs, "public output"),
    countLabel(impact.default_inputs, "default input value"),
    countLabel(impact.connections, "connection"),
  ].filter(Boolean);
  const segments = [];
  if (removals.length) segments.push(`Removes ${removals.join(", ")}`);
  const restored = countLabel(impact.restored_public_inputs, "target public input");
  if (restored) segments.push(`Restores ${restored}`);
  return segments.join("; ");
}

export function nodeDeleteImpactDetails(impact) {
  return [
    detailLine("Public inputs", impact.public_inputs),
    detailLine("Public outputs", impact.public_outputs),
    detailLine("Default inputs", impact.default_inputs),
    detailLine("Connections", impact.connections),
    detailLine("Restored public inputs", impact.restored_public_inputs),
  ].filter(Boolean).join("\n");
}

export function nodeDeleteImpactConfirmText(impact) {
  const summary = nodeDeleteImpactSummary(impact) || "No system references will be removed.";
  const details = nodeDeleteImpactDetails(impact);
  return details ? `${summary}\n${details}` : summary;
}

export function nodeRenameImpact(component, node, nextNodeID, system, graphConnections = []) {
  const impact = nodeDeleteImpact(component, node, system, graphConnections);
  return {
    ...impact,
    next_node_id: nextNodeID || "",
  };
}

export function nodeRenameImpactSummary(impact) {
  const updates = [
    countLabel(impact.public_inputs, "public input"),
    countLabel(impact.public_outputs, "public output"),
    countLabel(impact.connections, "connection"),
  ].filter(Boolean);
  return updates.length ? `Updates ${updates.join(", ")}` : "No system references will be updated.";
}

export function nodeRenameImpactDetails(impact) {
  return [
    impact.node_id && impact.next_node_id ? `Node id: ${impact.node_id} -> ${impact.next_node_id}` : "",
    detailLine("Public inputs", impact.public_inputs),
    detailLine("Public outputs", impact.public_outputs),
    detailLine("Connections", impact.connections),
  ].filter(Boolean).join("\n");
}

export function nodeRenameImpactConfirmText(impact, sourceDetails = "") {
  const summary = nodeRenameImpactSummary(impact);
  const details = [nodeRenameImpactDetails(impact), sourceDetails].filter(Boolean).join("\n");
  return details ? `${summary}\n${details}` : summary;
}

function emptyImpact() {
  return {
    component_id: "",
    node_id: "",
    direction: "",
    public_inputs: [],
    public_outputs: [],
    default_inputs: [],
    connections: [],
    restored_public_inputs: [],
  };
}

function endpointMatches(endpoint, componentID, nodeID) {
  return endpoint?.component === componentID && endpoint?.node === nodeID;
}

function endpointLabel(endpoint) {
  return `${endpoint?.component || ""}.${endpoint?.node || ""}`;
}

function connectionLabel(connection) {
  return `${endpointLabel(connection.from)} -> ${endpointLabel(connection.to)}`;
}

function publicRefLabel(ref) {
  return ref.id || `${ref.component}.${ref.node}`;
}

function hasPublicInputFor(system, componentID, nodeID) {
  return (system.public_inputs || []).some((ref) => ref.component === componentID && ref.node === nodeID);
}

function hasIncomingConnectionAfterRemoval(system, graphConnections, componentID, nodeID, removedConnectionIDs) {
  return (system.connections || [])
    .filter((id) => !removedConnectionIDs.has(id))
    .map((id) => (graphConnections || []).find((connection) => connection.id === id))
    .filter(Boolean)
    .some((connection) => endpointMatches(connection.to, componentID, nodeID));
}

function uniqueSorted(values) {
  return [...new Set(values.filter(Boolean))].sort();
}

function countLabel(values, singular) {
  const count = (values || []).length;
  if (!count) return "";
  return `${count} ${singular}${count === 1 ? "" : "s"}`;
}

function detailLine(label, values) {
  return values?.length ? `${label}: ${values.join(", ")}` : "";
}
