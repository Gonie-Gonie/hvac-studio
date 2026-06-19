import { escapeHTML } from "./dom.js";
import { finiteNumber, shortNumber } from "./format.js";

export function metricBars(metrics) {
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Metrics</div>`;
  const rows = document.createElement("div");
  rows.className = "metric-bars";
  const entries = Object.entries(metrics);
  if (!entries.length) {
    rows.innerHTML = `<div class="empty-cell">No metrics</div>`;
  } else {
    const maxRMSE = Math.max(...entries.map(([, item]) => Number(item.rmse) || 0), 1);
    for (const [name, item] of entries) {
      const width = Math.max(3, ((Number(item.rmse) || 0) / maxRMSE) * 100);
      const row = document.createElement("div");
      row.className = "metric-row";
      row.innerHTML = `
        <div class="metric-name">${escapeHTML(name)}</div>
        <div class="metric-track"><div class="metric-fill" style="width: ${width}%"></div></div>
        <div class="metric-values">RMSE ${escapeHTML(shortNumber(item.rmse))} / MAE ${escapeHTML(shortNumber(item.mae))} / R2 ${escapeHTML(shortNumber(item.r2))}</div>
      `;
      rows.append(row);
    }
  }
  block.append(rows);
  return block;
}

export function validationPlotSection(result) {
  const block = document.createElement("div");
  block.className = "result-block validation-plots-block";
  block.innerHTML = `<div class="result-block-title">Validation Plots</div>`;
  const content = document.createElement("div");
  content.className = "validation-plots";
  const outputs = validationOutputSeries(result);
  if (!outputs.length) {
    content.innerHTML = `<div class="empty-cell">No validation plot data</div>`;
    block.append(content);
    return block;
  }
  for (const output of outputs) {
    const group = document.createElement("div");
    group.className = "validation-output-plots";
    const title = document.createElement("div");
    title.className = "validation-output-title";
    title.textContent = `${output.name} (${output.points.length} rows)`;
    const grid = document.createElement("div");
    grid.className = "validation-plot-grid";
    grid.append(
      plotPanel("Measured vs Simulated", measuredSeriesPlot(output)),
      plotPanel("Scatter", scatterPlot(output)),
      plotPanel("Residuals", residualPlot(output)),
      plotPanel("Residual Histogram", residualHistogram(output)),
    );
    group.append(title, grid);
    content.append(group);
  }
  block.append(content);
  return block;
}

export function candidateObjectiveHistory(result) {
  const points = (result.candidates || [])
    .map((item, index) => ({ index: item.index ?? index + 1, objective: finiteNumber(item.objective) }))
    .filter((item) => item.objective !== null);
  if (!points.length) return null;
  const block = document.createElement("div");
  block.className = "result-block";
  block.innerHTML = `<div class="result-block-title">Objective History</div>`;
  const svg = validationSVG("objective history");
  const bounds = plotBounds();
  const yExtent = numericExtent(points.map((point) => point.objective));
  appendPlotFrame(svg, bounds);
  appendLinePath(svg, points, bounds, yExtent, (point) => point.objective, "validation-series-simulated");
  for (const [index, point] of points.entries()) {
    const circle = svgNode("circle", {
      class: "validation-point",
      cx: scaleIndex(index, points.length, bounds.left, bounds.right),
      cy: scaleValue(point.objective, yExtent, bounds.bottom, bounds.top),
      r: 3,
    });
    circle.append(svgTitle(`candidate ${point.index}: ${shortNumber(point.objective)}`));
    svg.append(circle);
  }
  block.append(svg);
  return block;
}

function validationOutputSeries(result) {
  const outputs = new Set(Object.keys(result.metrics || {}));
  for (const row of result.rows || []) {
    Object.keys(row.observed || {}).forEach((name) => outputs.add(name));
    Object.keys(row.simulated || {}).forEach((name) => outputs.add(name));
  }
  return [...outputs].sort().map((name) => {
    const points = (result.rows || []).map((row, index) => {
      if (!row || row.skipped) return null;
      const observed = finiteNumber(row.observed?.[name]);
      const simulated = finiteNumber(row.simulated?.[name]);
      if (observed === null || simulated === null) return null;
      const error = finiteNumber(row.errors?.[name]) ?? simulated - observed;
      return {
        rowIndex: row.row_index ?? index,
        label: row.time ?? row.row_index ?? index + 1,
        observed,
        simulated,
        error,
      };
    }).filter(Boolean);
    return { name, points };
  }).filter((output) => output.points.length);
}

function plotPanel(title, svg) {
  const panel = document.createElement("div");
  panel.className = "validation-plot-panel";
  const label = document.createElement("div");
  label.className = "validation-plot-title";
  label.textContent = title;
  panel.append(label, svg);
  return panel;
}

function measuredSeriesPlot(output) {
  const svg = validationSVG(`${output.name} measured vs simulated`);
  const bounds = plotBounds();
  const yExtent = numericExtent(output.points.flatMap((point) => [point.observed, point.simulated]));
  appendPlotFrame(svg, bounds);
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.observed, "validation-series-observed");
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.simulated, "validation-series-simulated");
  appendPlotLegend(svg, [["Measured", "validation-series-observed"], ["Simulated", "validation-series-simulated"]]);
  return svg;
}

function scatterPlot(output) {
  const svg = validationSVG(`${output.name} measured simulated scatter`);
  const bounds = plotBounds();
  const extent = numericExtent(output.points.flatMap((point) => [point.observed, point.simulated]));
  appendPlotFrame(svg, bounds);
  svg.append(svgNode("line", {
    class: "validation-reference",
    x1: scaleValue(extent.min, extent, bounds.left, bounds.right),
    y1: scaleValue(extent.min, extent, bounds.bottom, bounds.top),
    x2: scaleValue(extent.max, extent, bounds.left, bounds.right),
    y2: scaleValue(extent.max, extent, bounds.bottom, bounds.top),
  }));
  for (const point of output.points) {
    const circle = svgNode("circle", {
      class: "validation-point",
      cx: scaleValue(point.observed, extent, bounds.left, bounds.right),
      cy: scaleValue(point.simulated, extent, bounds.bottom, bounds.top),
      r: 3,
    });
    circle.append(svgTitle(`row ${point.rowIndex}: measured ${shortNumber(point.observed)}, simulated ${shortNumber(point.simulated)}`));
    svg.append(circle);
  }
  return svg;
}

function residualPlot(output) {
  const svg = validationSVG(`${output.name} residuals`);
  const bounds = plotBounds();
  const yExtent = numericExtent([...output.points.map((point) => point.error), 0]);
  appendPlotFrame(svg, bounds);
  appendZeroLine(svg, bounds, yExtent);
  appendLinePath(svg, output.points, bounds, yExtent, (point) => point.error, "validation-series-residual");
  return svg;
}

function residualHistogram(output) {
  const svg = validationSVG(`${output.name} residual histogram`);
  const bounds = plotBounds();
  const errors = output.points.map((point) => point.error).filter((value) => Number.isFinite(value));
  const bins = histogramBins(errors, 8);
  appendPlotFrame(svg, bounds);
  if (!bins.length) return svg;
  const maxCount = Math.max(...bins.map((bin) => bin.count), 1);
  const gap = 3;
  const barWidth = ((bounds.right - bounds.left) / bins.length) - gap;
  bins.forEach((bin, index) => {
    const height = (bin.count / maxCount) * (bounds.bottom - bounds.top);
    const rect = svgNode("rect", {
      class: "validation-histogram-bar",
      x: bounds.left + index * ((bounds.right - bounds.left) / bins.length) + gap / 2,
      y: bounds.bottom - height,
      width: Math.max(1, barWidth),
      height,
    });
    rect.append(svgTitle(`${shortNumber(bin.min)} to ${shortNumber(bin.max)}: ${bin.count}`));
    svg.append(rect);
  });
  return svg;
}

function validationSVG(title) {
  const svg = svgNode("svg", {
    class: "validation-svg",
    viewBox: "0 0 320 180",
    role: "img",
    "aria-label": title,
  });
  svg.append(svgTitle(title));
  return svg;
}

function plotBounds() {
  return { left: 32, right: 302, top: 16, bottom: 152 };
}

function appendPlotFrame(svg, bounds) {
  svg.append(
    svgNode("line", { class: "validation-axis", x1: bounds.left, y1: bounds.bottom, x2: bounds.right, y2: bounds.bottom }),
    svgNode("line", { class: "validation-axis", x1: bounds.left, y1: bounds.top, x2: bounds.left, y2: bounds.bottom }),
  );
}

function appendPlotLegend(svg, items) {
  items.forEach(([label, className], index) => {
    const y = 168;
    const x = 32 + index * 96;
    svg.append(svgNode("line", { class: className, x1: x, y1: y, x2: x + 18, y2: y }));
    const text = svgNode("text", { class: "validation-legend", x: x + 24, y: y + 4 });
    text.textContent = label;
    svg.append(text);
  });
}

function appendLinePath(svg, points, bounds, yExtent, valueForPoint, className) {
  if (!points.length) return;
  const d = points.map((point, index) => {
    const x = scaleIndex(index, points.length, bounds.left, bounds.right);
    const y = scaleValue(valueForPoint(point), yExtent, bounds.bottom, bounds.top);
    return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(" ");
  svg.append(svgNode("path", { class: className, d }));
}

function appendZeroLine(svg, bounds, yExtent) {
  const y = scaleValue(0, yExtent, bounds.bottom, bounds.top);
  svg.append(svgNode("line", { class: "validation-reference", x1: bounds.left, y1: y, x2: bounds.right, y2: y }));
}

function histogramBins(values, binCount) {
  if (!values.length) return [];
  const extent = numericExtent(values);
  const width = (extent.max - extent.min) / binCount;
  const bins = Array.from({ length: binCount }, (_, index) => ({
    min: extent.min + index * width,
    max: extent.min + (index + 1) * width,
    count: 0,
  }));
  for (const value of values) {
    const index = Math.min(binCount - 1, Math.max(0, Math.floor((value - extent.min) / width)));
    bins[index].count += 1;
  }
  return bins;
}

function numericExtent(values) {
  const numbers = values.map((value) => Number(value)).filter((value) => Number.isFinite(value));
  if (!numbers.length) return { min: 0, max: 1 };
  let min = Math.min(...numbers);
  let max = Math.max(...numbers);
  if (min === max) {
    const pad = Math.max(Math.abs(min) * 0.1, 1);
    min -= pad;
    max += pad;
  } else {
    const pad = (max - min) * 0.08;
    min -= pad;
    max += pad;
  }
  return { min, max };
}

function scaleIndex(index, length, min, max) {
  if (length <= 1) return (min + max) / 2;
  return min + (index / (length - 1)) * (max - min);
}

function scaleValue(value, extent, min, max) {
  return min + ((Number(value) - extent.min) / (extent.max - extent.min)) * (max - min);
}

function svgNode(name, attrs = {}) {
  const node = document.createElementNS("http://www.w3.org/2000/svg", name);
  for (const [key, value] of Object.entries(attrs)) {
    node.setAttribute(key, String(value));
  }
  return node;
}

function svgTitle(text) {
  const title = document.createElementNS("http://www.w3.org/2000/svg", "title");
  title.textContent = text;
  return title;
}
