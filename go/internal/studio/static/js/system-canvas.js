import { el, escapeAttr, escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";
import { autoLayoutPositions, canvasBounds, canvasConnectionRoute, canvasLayoutOverlaps, canvasNodeHeight } from "./canvas-layout.js";
import { CANVAS_NODE_WIDTH } from "./workspace-config.js";

const SVG = "http://www.w3.org/2000/svg";
const MIN_ZOOM = 0.3;
const MAX_ZOOM = 1.75;

export function createSystemCanvas(context) {
  const { state } = context;
  const views = new Map();
  let activeKey = "";
  let view;
  let viewport;
  let surface;
  let spacer;
  let components = [];
  let connections = [];
  let heights = {};
  let positions = {};
  let bounds = { width: 320, height: 240 };
  let renderedNodes = new Map();
  let scheduled = 0;
  let resizeObserver;
  let suppressClick = false;

  function initialize() {
    if (surface) return;
    const canvas = el("systemCanvas");
    const layer = el("connectionLayer");
    viewport = canvas.closest(".canvas-wrap");
    viewport.setAttribute("aria-label", "System diagram. Drag component headers to arrange. Use the zoom controls to explore.");
    spacer = document.createElement("div");
    spacer.className = "canvas-scroll-space";
    surface = document.createElement("div");
    surface.className = "canvas-surface";
    surface.append(layer, canvas);
    spacer.append(surface);
    viewport.append(spacer);
    viewport.addEventListener("click", (event) => {
      if (event.target !== canvas && event.target !== surface && event.target !== spacer && event.target !== viewport) return;
      state.selectedComponentId = "";
      state.selectedConnectionId = "";
      state.pendingConnection = null;
      render();
      context.selectionChanged();
    });
    viewport.addEventListener("wheel", (event) => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      const rect = viewport.getBoundingClientRect();
      setZoom((view?.zoom || 1) * Math.exp(-event.deltaY * 0.0015), event.clientX - rect.left, event.clientY - rect.top);
    }, { passive: false });
    el("canvasFitButton")?.addEventListener("click", fit);
    el("canvasZoomOutButton")?.addEventListener("click", () => setZoom((view?.zoom || 1) / 1.2));
    el("canvasZoomInButton")?.addEventListener("click", () => setZoom((view?.zoom || 1) * 1.2));
    el("canvasResetButton")?.addEventListener("click", () => setZoom(1));
    resizeObserver = new ResizeObserver(() => {
      if (view?.fit) fit();
      else applyTransform();
    });
    resizeObserver.observe(viewport);
  }

  function storageKey() { return `hvac-studio:canvas:v2:${activeKey}`; }

  function saveView() {
    if (!view) return;
    view.positions = { ...positions };
    try {
      localStorage.setItem(storageKey(), JSON.stringify({ positions: view.positions, zoom: view.zoom, fit: view.fit, autoArranged: view.autoArranged }));
    } catch { /* The current view remains usable when browser storage is unavailable. */ }
  }

  function readView() {
    try { return JSON.parse(localStorage.getItem(storageKey()) || "null"); } catch { return null; }
  }

  function render() {
    initialize();
    const canvas = el("systemCanvas");
    const layer = el("connectionLayer");
    const system = context.currentSystem();
    const graph = state.detail?.graph;
    canvas.innerHTML = "";
    layer.innerHTML = "";
    renderedNodes = new Map();
    if (!system || !graph) return;
    components = (system.components || []).map(context.componentById).filter(Boolean);
    const connectionIDs = new Set(system.connections || []);
    connections = (graph.connections || []).filter((connection) => connectionIDs.has(connection.id));
    heights = Object.fromEntries(components.map((component) => [component.id, canvasNodeHeight(component)]));
    const key = `${state.currentProjectPath}:${system.id}`;
    const changed = activeKey !== key;
    activeKey = key;
    if (!views.has(key)) {
      const stored = readView();
      const savedPositions = context.isWorkspaceProject() ? state.detail?.layout?.components : stored?.positions;
      const saved = Object.fromEntries(components.filter((component) => savedPositions?.[component.id]).map((component) => [component.id, savedPositions[component.id]]));
      const knownView = components.every((component) => {
        const point = saved[component.id];
        const previous = stored?.positions?.[component.id];
        return point && Number.isFinite(point.x) && Number.isFinite(point.y) && point.x >= 0 && point.y >= 0
          && previous?.x === point.x && previous?.y === point.y;
      });
      const needsLayout = !knownView && canvasLayoutOverlaps(saved, components, heights);
      views.set(key, {
        positions: needsLayout ? autoLayoutPositions(components, connections, heights) : saved,
        autoArranged: needsLayout || (knownView && stored?.autoArranged === true),
        zoom: Number.isFinite(stored?.zoom) ? Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, stored.zoom)) : 1,
        fit: stored?.fit !== false,
      });
    }
    view = views.get(key);
    const fallback = autoLayoutPositions(components, connections, heights);
    positions = Object.fromEntries(components.map((component) => [component.id, view.positions[component.id] || fallback[component.id]]));
    view.positions = positions;
    for (const component of components) {
      const point = positions[component.id];
      const node = document.createElement("div");
      node.className = `component-node${state.selectedComponentId === component.id ? " selected" : ""}`;
      node.dataset.componentId = component.id;
      node.style.left = `${point.x}px`;
      node.style.top = `${point.y}px`;
      const inputs = component.nodes?.inputs || [];
      const outputs = component.nodes?.outputs || [];
      node.innerHTML = `
        <button type="button" class="component-head" aria-label="Select ${escapeAttr(component.name || component.id)}. Drag to move." title="${escapeAttr(component.name || component.id)} · Drag to move">
          <span class="component-grip" aria-hidden="true">⠿</span><span class="component-title">${escapeHTML(component.name || component.id)}</span>
        </button>
        <div class="node-list">
          <div class="node-column"><span class="node-column-title">IN <span>${inputs.length}</span></span>${inputs.map((port) => canvasNodePill(component.id, port, "input")).join("")}</div>
          <div class="node-column"><span class="node-column-title">OUT <span>${outputs.length}</span></span>${outputs.map((port) => canvasNodePill(component.id, port, "output")).join("")}</div>
        </div>`;
      node.addEventListener("click", () => {
        if (suppressClick) { suppressClick = false; return; }
        state.selectedComponentId = component.id;
        state.selectedConnectionId = "";
        render();
        context.selectionChanged();
      });
      node.querySelectorAll("[data-node-endpoint]").forEach((endpoint) => {
        endpoint.addEventListener("click", (event) => {
          event.stopPropagation();
          context.endpointClick(endpoint.dataset.componentId, endpoint.dataset.nodeId, endpoint.dataset.direction);
        });
      });
      const head = node.querySelector(".component-head");
      head.addEventListener("pointerdown", (event) => startCanvasNodeDrag(event, node, component.id));
      head.addEventListener("keydown", (event) => {
        if (!event.altKey || !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(event.key)) return;
        event.preventDefault();
        const step = event.shiftKey ? 32 : 8;
        positions[component.id] = {
          x: Math.max(16, positions[component.id].x + (event.key === "ArrowRight" ? step : event.key === "ArrowLeft" ? -step : 0)),
          y: Math.max(16, positions[component.id].y + (event.key === "ArrowDown" ? step : event.key === "ArrowUp" ? -step : 0)),
        };
        view.fit = false;
        view.autoArranged = false;
        persist("component position");
        render();
        renderedNodes.get(component.id)?.querySelector(".component-head")?.focus();
      });
      renderedNodes.set(component.id, node);
      canvas.append(node);
    }
    if (!components.length) canvas.innerHTML = `<div class="canvas-empty"><strong>This system is empty</strong><span>Add a component from the Project panel to start building.</span></div>`;
    cancelAnimationFrame(scheduled);
    scheduled = requestAnimationFrame(() => {
      for (const [id, node] of renderedNodes) heights[id] = node.offsetHeight || heights[id];
      drawConnections();
      if (view.fit) fit();
      else applyTransform();
      if (changed && !view.fit) viewport.scrollTo(0, 0);
    });
  }

  function canvasNodePill(componentID, port, direction) {
    const pending = state.pendingConnection;
    const latest = context.latestNodeValue(componentID, port.id, direction);
    const selected = direction === "output" && pending?.component === componentID && pending?.node === port.id;
    const targetable = direction === "input" && pending && pending.component !== componentID;
    const medium = String(port.medium || "signal").toLowerCase();
    const knownMedium = ["air", "water", "electricity", "electric", "control"].includes(medium) ? medium : "signal";
    const title = [port.name || port.id, port.id, port.medium, port.value_type, port.unit,
      latest.hasValue ? `${state.latestResultStale ? "Previous " : ""}value: ${formatValue(latest.value)}` : "",
    ].filter(Boolean).join(" · ");
    return `<button type="button" class="node-pill ${direction} medium-${knownMedium}${selected ? " pending-source" : ""}${targetable ? " targetable" : ""}" data-node-endpoint="true" data-component-id="${escapeAttr(componentID)}" data-node-id="${escapeAttr(port.id)}" data-direction="${direction}" title="${escapeAttr(title)}" aria-label="${escapeAttr(`${port.name || port.id}, ${direction}${port.unit ? `, ${port.unit}` : ""}`)}"><span class="port-dot" aria-hidden="true"></span><span class="node-label">${escapeHTML(port.name || port.id)}</span></button>`;
  }

  function startCanvasNodeDrag(event, node, componentID) {
    if (event.button !== 0) return;
    event.preventDefault();
    const capturedView = view;
    const pointer = event.pointerId;
    const head = node.querySelector(".component-head");
    const original = { ...positions[componentID] };
    const start = { x: event.clientX, y: event.clientY, scrollX: viewport.scrollLeft, scrollY: viewport.scrollTop };
    const zoom = view.zoom;
    let moved = false;
    state.selectedComponentId = componentID;
    state.selectedConnectionId = "";
    renderedNodes.forEach((item, id) => item.classList.toggle("selected", id === componentID));
    context.selectionChanged();
    drawConnections();
    head.setPointerCapture(pointer);
    const onMove = (move) => {
      if (move.pointerId !== pointer || view !== capturedView) return;
      const dx = move.clientX - start.x + viewport.scrollLeft - start.scrollX;
      const dy = move.clientY - start.y + viewport.scrollTop - start.scrollY;
      if (!moved && Math.hypot(dx, dy) < 3) return;
      moved = true;
      node.classList.add("dragging");
      view.fit = false;
      view.autoArranged = false;
      positions[componentID] = { x: Math.max(16, original.x + dx / zoom), y: Math.max(16, original.y + dy / zoom) };
      node.style.left = `${positions[componentID].x}px`;
      node.style.top = `${positions[componentID].y}px`;
      drawConnections();
      applyTransform();
    };
    const onUp = (up) => {
      if (up.pointerId !== pointer) return;
      node.classList.remove("dragging");
      head.removeEventListener("pointermove", onMove);
      head.removeEventListener("pointerup", onUp);
      head.removeEventListener("pointercancel", onUp);
      head.removeEventListener("lostpointercapture", onUp);
      if (head.hasPointerCapture(pointer)) head.releasePointerCapture(pointer);
      if (moved && view === capturedView) {
        suppressClick = true;
        setTimeout(() => { suppressClick = false; }, 0);
        persist("component position");
      }
    };
    head.addEventListener("pointermove", onMove);
    head.addEventListener("pointerup", onUp);
    head.addEventListener("pointercancel", onUp);
    head.addEventListener("lostpointercapture", onUp);
  }

  function persist(label) {
    positions = Object.fromEntries(Object.entries(positions).map(([id, point]) => [id, { x: Math.round(point.x), y: Math.round(point.y) }]));
    saveView();
    if (context.isWorkspaceProject()) context.saveLayout(positions, label);
  }

  function autoLayout() {
    if (!view) return;
    positions = autoLayoutPositions(components, connections, heights);
    view.positions = positions;
    view.autoArranged = true;
    view.fit = true;
    persist("auto layout");
    render();
  }

  function applyTransform() {
    if (!view || !surface) return;
    const zoom = view.zoom;
    const overview = zoom < 0.6;
    surface.classList.toggle("canvas-overview", overview);
    surface.style.setProperty("--canvas-title-size", `${Math.max(13, 11 / zoom)}px`);
    surface.style.setProperty("--canvas-port-size", `${Math.max(10, 9 / zoom)}px`);
    surface.style.setProperty("--canvas-caption-size", `${Math.max(9, 9 / zoom)}px`);
    for (const [id, node] of renderedNodes) heights[id] = node.offsetHeight || heights[id];
    if (view.autoArranged) {
      positions = autoLayoutPositions(components, connections, heights);
      view.positions = positions;
      for (const [id, node] of renderedNodes) {
        node.style.left = `${positions[id].x}px`;
        node.style.top = `${positions[id].y}px`;
      }
    }
    drawConnections();
    const width = Math.max(viewport.clientWidth, Math.ceil(bounds.width * zoom));
    const height = Math.max(viewport.clientHeight, Math.ceil(bounds.height * zoom));
    spacer.style.width = `${width}px`;
    spacer.style.height = `${height}px`;
    surface.style.width = `${bounds.width}px`;
    surface.style.height = `${bounds.height}px`;
    surface.style.left = `${Math.max(0, (width - bounds.width * zoom) / 2)}px`;
    surface.style.top = `${Math.max(0, (height - bounds.height * zoom) / 2)}px`;
    surface.style.transform = `scale(${zoom})`;
    const label = el("canvasZoomLabel");
    if (label) label.textContent = `${Math.round(zoom * 100)}%`;
    const hint = viewport.parentElement.querySelector(".canvas-hint");
    if (hint) hint.textContent = overview ? "Overview · Zoom in for port names" : "Drag headers to arrange · Ctrl + scroll to zoom";
    if (el("canvasZoomOutButton")) el("canvasZoomOutButton").disabled = zoom <= MIN_ZOOM;
    if (el("canvasZoomInButton")) el("canvasZoomInButton").disabled = zoom >= MAX_ZOOM;
    el("canvasFitButton")?.classList.toggle("active", Boolean(view.fit));
  }

  function fit() {
    if (!view || viewport.clientWidth < 10 || viewport.clientHeight < 10) return;
    view.fit = true;
    view.zoom = Math.min(1, Math.max(MIN_ZOOM, Math.min((viewport.clientWidth - 24) / bounds.width, (viewport.clientHeight - 24) / bounds.height)));
    applyTransform();
    const heightZoom = Math.max(MIN_ZOOM, (viewport.clientHeight - 24) / bounds.height);
    if (view.zoom > heightZoom) {
      view.zoom = heightZoom;
      applyTransform();
    }
    viewport.scrollTo(0, 0);
    saveView();
  }

  function setZoom(value, anchorX = viewport.clientWidth / 2, anchorY = viewport.clientHeight / 2) {
    if (!view) return;
    const x = (viewport.scrollLeft + anchorX - (Number.parseFloat(surface.style.left) || 0)) / view.zoom;
    const y = (viewport.scrollTop + anchorY - (Number.parseFloat(surface.style.top) || 0)) / view.zoom;
    view.zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value));
    view.fit = false;
    applyTransform();
    viewport.scrollLeft = x * view.zoom + (Number.parseFloat(surface.style.left) || 0) - anchorX;
    viewport.scrollTop = y * view.zoom + (Number.parseFloat(surface.style.top) || 0) - anchorY;
    saveView();
  }

  function canvasNodeAnchor(componentID, portID, direction) {
    const node = renderedNodes.get(componentID);
    const endpoint = [...(node?.querySelectorAll("[data-node-endpoint]") || [])].find((item) => item.dataset.nodeId === portID && item.dataset.direction === direction);
    const point = positions[componentID];
    if (!point) return null;
    // DOM rectangles account for wrapped titles and the current zoom.
    const nodeRect = node?.getBoundingClientRect();
    const portRect = endpoint?.getBoundingClientRect();
    const actualZoom = nodeRect?.width ? nodeRect.width / CANVAS_NODE_WIDTH : view.zoom;
    return { component: componentID, x: point.x + (direction === "output" ? CANVAS_NODE_WIDTH : 0), y: point.y + (portRect && nodeRect && actualZoom ? (portRect.top + portRect.height / 2 - nodeRect.top) / actualZoom : 88) };
  }

  function drawConnections() {
    const layer = el("connectionLayer");
    layer.innerHTML = "";
    bounds = canvasBounds(positions, heights);
    const boxes = components.map((component) => ({ id: component.id, ...positions[component.id], width: CANVAS_NODE_WIDTH, height: heights[component.id] }));
    const defs = document.createElementNS(SVG, "defs");
    defs.innerHTML = `<marker id="canvas-arrow" viewBox="0 0 8 8" markerWidth="5" markerHeight="5" refX="7" refY="4" orient="auto-start-reverse"><path d="M0 0 L8 4 L0 8 Z" fill="context-stroke"/></marker>`;
    layer.append(defs);
    const pairCounts = new Map();
    const pairKey = (connection) => `${connection.from.component}->${connection.to.component}`;
    connections.forEach((connection) => pairCounts.set(pairKey(connection), (pairCounts.get(pairKey(connection)) || 0) + 1));
    const pairIndices = new Map();
    const lanes = new Map();
    const routes = new Map();
    // Calculate routes before changing drawing order: selecting a link must not move it.
    for (const connection of connections) {
      const source = canvasNodeAnchor(connection.from.component, connection.from.node, "output");
      const target = canvasNodeAnchor(connection.to.component, connection.to.node, "input");
      if (!source || !target) continue;
      const key = pairKey(connection);
      const pairIndex = pairIndices.get(key) || 0;
      pairIndices.set(key, pairIndex + 1);
      const fanOffset = (pairIndex - (pairCounts.get(key) - 1) / 2) * 5;
      let route = canvasConnectionRoute(source, target, boxes, 0, fanOffset);
      if (route.external) {
        if (!lanes.has(key)) lanes.set(key, lanes.size * 2);
        route = canvasConnectionRoute(source, target, boxes, lanes.get(key), fanOffset);
      }
      bounds.height = Math.max(bounds.height, route.bottom + 24);
      routes.set(connection.id, route);
    }
    const ordered = [...connections].sort((a, b) => Number(a.id === state.selectedConnectionId) - Number(b.id === state.selectedConnectionId));
    ordered.forEach((connection) => {
      const route = routes.get(connection.id);
      if (!route) return;
      const medium = context.connectionMediumState(connection);
      const unit = context.connectionUnitState(connection);
      const selected = state.selectedConnectionId === connection.id;
      const related = state.selectedConnectionId ? selected : [connection.from.component, connection.to.component].includes(state.selectedComponentId);
      const focused = Boolean(state.selectedConnectionId || state.selectedComponentId);
      const classes = ["connection-group", selected ? "selected" : "", related ? "related" : "", focused && !related ? "dimmed" : "",
        medium.status === "error" ? "medium-mismatch" : "", ["warning", "override"].includes(medium.status) || unit.status === "warning" ? "connection-warning" : "", unit.status === "converted" ? "unit-converted" : "",
        route.backtracking ? "backtracking" : "", route.longPath ? "long-path" : ""].filter(Boolean).join(" ");
      const group = document.createElementNS(SVG, "g");
      group.setAttribute("class", classes);
      group.dataset.connectionId = connection.id;
      const fromName = context.componentById(connection.from.component)?.name || connection.from.component;
      const toName = context.componentById(connection.to.component)?.name || connection.to.component;
      const title = `${fromName}: ${medium.sourceNode?.name || connection.from.node} → ${toName}: ${medium.targetNode?.name || connection.to.node}`;
      const titleNode = document.createElementNS(SVG, "title");
      titleNode.textContent = [title, medium.label, unit.label, unit.conversionLabel].filter(Boolean).join(" · ");
      group.append(titleNode);
      const hit = document.createElementNS(SVG, "path");
      hit.setAttribute("class", "connection-hit");
      hit.setAttribute("d", route.path);
      const path = document.createElementNS(SVG, "path");
      path.setAttribute("class", "connection-line");
      path.dataset.connectionId = connection.id;
      path.setAttribute("d", route.path);
      path.setAttribute("marker-end", "url(#canvas-arrow)");
      group.append(hit, path);
      drawConnectionLabel(group, `${medium.sourceNode?.name || connection.from.node} → ${medium.targetNode?.name || connection.to.node}`, route);
      group.addEventListener("click", (event) => { event.stopPropagation(); context.selectConnection(connection.id); });
      layer.append(group);
    });
    layer.setAttribute("width", String(bounds.width));
    layer.setAttribute("height", String(bounds.height));
    layer.style.width = `${bounds.width}px`;
    layer.style.height = `${bounds.height}px`;
  }

  function drawConnectionLabel(group, text, route) {
    const label = document.createElementNS(SVG, "g");
    label.setAttribute("class", "connection-label");
    const visibleText = text.length > 45 ? `${text.slice(0, 42)}…` : text;
    const width = Math.max(96, visibleText.length * 5.7 + 20);
    const x = Math.max(width / 2 + 8, Math.min(bounds.width - width / 2 - 8, route.labelX));
    const y = Math.max(20, route.labelY);
    const rect = document.createElementNS(SVG, "rect");
    rect.setAttribute("class", "connection-label-bg");
    rect.setAttribute("x", String(x - width / 2));
    rect.setAttribute("y", String(y - 13));
    rect.setAttribute("width", String(width));
    rect.setAttribute("height", "26");
    rect.setAttribute("rx", "6");
    const line = document.createElementNS(SVG, "text");
    line.setAttribute("class", "connection-label-text");
    line.setAttribute("x", String(x));
    line.setAttribute("y", String(y + 4));
    line.setAttribute("text-anchor", "middle");
    line.textContent = visibleText;
    label.append(rect, line);
    group.append(label);
  }

  return { render, autoLayout, fit };
}
