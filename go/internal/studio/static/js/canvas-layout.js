import { CANVAS_COLUMN_GAP, CANVAS_NODE_WIDTH, CANVAS_PADDING, CANVAS_ROW_GAP } from "./workspace-config.js";

export function canvasNodeHeight(component) {
  return 80 + Math.max(1, component.nodes?.inputs?.length || 0, component.nodes?.outputs?.length || 0) * 40;
}

// Collapse feedback loops before assigning columns, so cyclic models stay compact.
export function autoLayoutPositions(components, connections, heights = {}) {
  const ids = components.map((component) => component.id);
  const idSet = new Set(ids);
  const edges = connections.filter((edge) => idSet.has(edge.from.component) && idSet.has(edge.to.component));
  const adjacent = new Map(ids.map((id) => [id, []]));
  for (const edge of edges) adjacent.get(edge.from.component).push(edge.to.component);
  let nextIndex = 0;
  const indices = new Map();
  const low = new Map();
  const stack = [];
  const onStack = new Set();
  const groups = [];
  const visit = (id) => {
    indices.set(id, nextIndex);
    low.set(id, nextIndex++);
    stack.push(id);
    onStack.add(id);
    for (const target of adjacent.get(id)) {
      if (!indices.has(target)) {
        visit(target);
        low.set(id, Math.min(low.get(id), low.get(target)));
      } else if (onStack.has(target)) low.set(id, Math.min(low.get(id), indices.get(target)));
    }
    if (low.get(id) !== indices.get(id)) return;
    const group = [];
    let member;
    do {
      member = stack.pop();
      onStack.delete(member);
      group.push(member);
    } while (member !== id);
    groups.push(group);
  };
  ids.forEach((id) => { if (!indices.has(id)) visit(id); });
  const groupFor = new Map(groups.flatMap((group, index) => group.map((id) => [id, index])));
  const levels = groups.map(() => 0);
  const outgoing = groups.map(() => new Set());
  const indegrees = groups.map(() => 0);
  for (const edge of edges) {
    const source = groupFor.get(edge.from.component);
    const target = groupFor.get(edge.to.component);
    if (source === target || outgoing[source].has(target)) continue;
    outgoing[source].add(target);
    indegrees[target] += 1;
  }
  const queue = groups.map((_, index) => index).filter((index) => !indegrees[index]);
  for (let cursor = 0; cursor < queue.length; cursor += 1) {
    const source = queue[cursor];
    for (const target of outgoing[source]) {
      levels[target] = Math.max(levels[target], levels[source] + 1);
      if (--indegrees[target] === 0) queue.push(target);
    }
  }
  const columns = new Map();
  for (const id of ids) {
    const level = levels[groupFor.get(id)];
    if (!columns.has(level)) columns.set(level, []);
    columns.get(level).push(id);
  }
  const heightFor = new Map(components.map((component) => [component.id, heights[component.id] || canvasNodeHeight(component)]));
  const columnHeight = (column) => column.reduce((sum, id) => sum + heightFor.get(id), 0) + Math.max(0, column.length - 1) * CANVAS_ROW_GAP;
  const tallest = Math.max(0, ...[...columns.values()].map(columnHeight));
  const positions = {};
  for (const [level, column] of [...columns.entries()].sort(([a], [b]) => a - b)) {
    // Preserve model order for ties; align branches with already placed predecessors.
    const parentCenter = (id) => {
      const parents = edges.filter((edge) => edge.to.component === id).map((edge) => positions[edge.from.component]).filter(Boolean);
      return parents.length ? parents.reduce((sum, point) => sum + point.y, 0) / parents.length : 0;
    };
    column.sort((a, b) => parentCenter(a) - parentCenter(b));
    let y = CANVAS_PADDING + (tallest - columnHeight(column)) / 2;
    for (const id of column) {
      positions[id] = { x: CANVAS_PADDING + level * CANVAS_COLUMN_GAP, y: Math.round(y) };
      y += heightFor.get(id) + CANVAS_ROW_GAP;
    }
  }
  return positions;
}

export function canvasLayoutOverlaps(positions, components, heights = {}) {
  return components.some((component, index) => {
    const a = positions[component.id];
    if (!a || !Number.isFinite(a.x) || !Number.isFinite(a.y) || a.x < 0 || a.y < 0) return true;
    return components.slice(index + 1).some((other) => {
      const b = positions[other.id];
      if (!b) return true;
      return a.x < b.x + CANVAS_NODE_WIDTH + 16 && a.x + CANVAS_NODE_WIDTH + 16 > b.x
        && a.y < b.y + (heights[other.id] || canvasNodeHeight(other)) + 16
        && a.y + (heights[component.id] || canvasNodeHeight(component)) + 16 > b.y;
    });
  });
}

export function canvasBounds(positions, heights = {}) {
  return {
    width: Math.max(320, ...Object.values(positions).map((point) => point.x + CANVAS_NODE_WIDTH + CANVAS_PADDING)),
    height: Math.max(240, ...Object.entries(positions).map(([id, point]) => point.y + (heights[id] || 120) + CANVAS_PADDING)),
  };
}

// Long and returning links travel outside the cards, keeping every port visible.
export function canvasConnectionRoute(source, target, boxes, index = 0, fanOffset = 0) {
  const x1 = source.x;
  const y1 = source.y;
  const x2 = target.x;
  const y2 = target.y;
  const backtracking = x2 <= x1 + 30;
  const longPath = x2 - x1 > CANVAS_COLUMN_GAP;
  const blocked = boxes.some((box) => box.id !== source.component && box.id !== target.component
    && box.x < Math.max(x1, x2) && box.x + box.width > Math.min(x1, x2)
    && box.y < Math.max(y1, y2) + 12 && box.y + box.height > Math.min(y1, y2) - 12);
  if (backtracking || blocked) {
    const laneY = Math.max(y1, y2, ...boxes.map((box) => box.y + box.height)) + 28 + index * 12 + fanOffset;
    const exit = x1 + 20 + Math.abs(fanOffset);
    const enter = Math.max(8, x2 - 20 - Math.abs(fanOffset));
    const radius = 10;
    return {
      path: `M ${x1} ${y1} H ${exit - radius} Q ${exit} ${y1} ${exit} ${y1 + radius} V ${laneY - radius} Q ${exit} ${laneY} ${exit + (enter > exit ? radius : -radius)} ${laneY} H ${enter + (enter > exit ? -radius : radius)} Q ${enter} ${laneY} ${enter} ${laneY - radius} V ${y2 + radius} Q ${enter} ${y2} ${enter + radius} ${y2} H ${x2}`,
      labelX: (exit + enter) / 2, labelY: laneY - 12, bottom: laneY + 20, backtracking, longPath, external: true,
    };
  }
  const bend = Math.max(28, (x2 - x1) / 2 + fanOffset);
  return {
    path: `M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`,
    labelX: (x1 + x2) / 2, labelY: (y1 + y2) / 2 - 14, bottom: Math.max(y1, y2), backtracking, longPath, external: false,
  };
}
