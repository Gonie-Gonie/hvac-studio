import assert from "node:assert/strict";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import test from "node:test";
import { autoLayoutPositions, canvasBounds, canvasConnectionRoute, canvasLayoutOverlaps, canvasNodeHeight } from "../../go/internal/studio/static/js/canvas-layout.js";
import { CANVAS_COLUMN_GAP, CANVAS_NODE_WIDTH } from "../../go/internal/studio/static/js/workspace-config.js";

const component = (id, ports = 1) => ({ id, nodes: { inputs: Array.from({ length: ports }, (_, i) => ({ id: `in${i}` })), outputs: [{ id: "out" }] } });
const edge = (source, target) => ({ from: { component: source, node: "out" }, to: { component: target, node: "in0" } });

test("every supplied example fits all port rows in nonoverlapping, finite cards", () => {
  const examples = fileURLToPath(new URL("../../examples/", import.meta.url));
  let count = 0;
  for (const name of readdirSync(examples)) {
    const graphPath = join(examples, name, "graph.json");
    if (!existsSync(graphPath)) continue;
    const graph = JSON.parse(readFileSync(graphPath, "utf8"));
    for (const system of graph.systems) {
      const components = graph.components.filter((item) => system.components.includes(item.id));
      const connections = graph.connections.filter((item) => system.connections.includes(item.id));
      const heights = Object.fromEntries(components.map((item) => [item.id, canvasNodeHeight(item)]));
      const positions = autoLayoutPositions(components, connections, heights);
      assert.equal(canvasLayoutOverlaps(positions, components, heights), false, name);
      assert.deepEqual(positions, autoLayoutPositions(components, [...connections].reverse(), heights), `${name}: edge order should not shift layout`);
      const bounds = canvasBounds(positions, heights);
      for (const item of components) {
        assert.ok(Number.isFinite(positions[item.id].x) && Number.isFinite(positions[item.id].y), name);
        assert.ok(bounds.width >= positions[item.id].x + CANVAS_NODE_WIDTH, name);
        assert.ok(bounds.height >= positions[item.id].y + heights[item.id], name);
      }
      count += 1;
    }
  }
  assert.ok(count >= 15, `Only inspected ${count} example systems`);
});

test("long titles and dense components use measured heights instead of a fixed row gap", () => {
  const components = [component("source"), component("tall", 18), component("short"), component("sink")];
  const connections = [edge("source", "tall"), edge("source", "short"), edge("tall", "sink"), edge("short", "sink")];
  const heights = { tall: 900, short: 173 };
  const positions = autoLayoutPositions(components, connections, heights);
  assert.equal(canvasLayoutOverlaps(positions, components, heights), false);
  assert.ok(positions.short.y >= positions.tall.y + heights.tall);
  for (const connection of connections) assert.ok(positions[connection.from.component].x < positions[connection.to.component].x);
});

test("feedback loops collapse into one column without runaway levels", () => {
  const components = [component("a"), component("b"), component("out"), component("isolated")];
  const positions = autoLayoutPositions(components, [edge("a", "b"), edge("b", "a"), edge("b", "out"), edge("missing", "a")]);
  assert.equal(positions.a.x, positions.b.x);
  assert.equal(positions.out.x - positions.b.x, CANVAS_COLUMN_GAP);
  assert.equal(canvasLayoutOverlaps(positions, components), false);
});

test("legacy overlapping or incomplete layouts are recognized for repair", () => {
  const components = [component("a", 8), component("b")];
  assert.equal(canvasLayoutOverlaps({ a: { x: 40, y: 40 }, b: { x: 80, y: 80 } }, components), true);
  assert.equal(canvasLayoutOverlaps({ a: { x: 40, y: 40 } }, components), true);
  assert.equal(canvasLayoutOverlaps({ a: { x: NaN, y: 40 }, b: { x: 600, y: 80 } }, components), true);
  assert.equal(canvasLayoutOverlaps(autoLayoutPositions(components, []), components), false);
});

test("links that cross a card or return upstream route below the cards", () => {
  const boxes = [
    { id: "a", x: 40, y: 40, width: 252, height: 160 },
    { id: "middle", x: 376, y: 40, width: 252, height: 300 },
    { id: "b", x: 712, y: 40, width: 252, height: 160 },
  ];
  const route = canvasConnectionRoute({ component: "a", x: 292, y: 120 }, { component: "b", x: 712, y: 130 }, boxes, 2);
  assert.ok(route.bottom > 340);
  assert.ok(route.path.includes(" H "));
  const returning = canvasConnectionRoute({ component: "b", x: 964, y: 120 }, { component: "a", x: 40, y: 130 }, boxes);
  assert.equal(returning.backtracking, true);
  assert.ok(returning.bottom > 340);
  const direct = canvasConnectionRoute({ component: "a", x: 292, y: 120 }, { component: "middle", x: 376, y: 130 }, boxes);
  assert.ok(direct.path.includes(" C "));
  assert.equal(direct.bottom, 130);
});
