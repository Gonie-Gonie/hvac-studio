import { escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";

export function mlMetadataBlock(component, inspectorBlock) {
  const metadata = component.ml_metadata || {};
  const rows = [
    ["Model Format", metadata.model_format || ""],
    ["Model File", metadata.model_file || ""],
    ["Feature Schema", metadata.feature_schema_file || ""],
    ["Target Schema", metadata.target_schema_file || ""],
    ["Validation Report", metadata.validation_report_file || ""],
    ["Required Packages", (metadata.required_packages || []).join(", ")],
    ["Time Resolution", metadata.valid_time_resolution || ""],
    ["Input Ranges", Object.keys(metadata.valid_input_ranges || {}).join(", ")],
  ].filter(([, value]) => value);
  return inspectorBlock("ML Metadata", rows);
}

export function mlValidationReportBlock(report, inspectorBlock) {
  if (!report) return null;
  const rows = [
    ["Dataset", report.dataset || ""],
    ["Report", report.report_path || ""],
    ["Feature Schema", report.feature_schema_version || ""],
    ["Model SHA256", report.model_asset_checksum || ""],
    ["Training Period", report.training_period || ""],
    ["Validation Period", report.validation_period || ""],
    ["Time Resolution", report.time_resolution || ""],
  ].filter(([, value]) => value);
  const block = inspectorBlock("ML Validation", rows);
  const metricRows = [];
  for (const [target, metrics] of Object.entries(report.metrics || {})) {
    for (const [metric, value] of Object.entries(metrics || {})) {
      metricRows.push([target, metric, formatValue(value)]);
    }
  }
  if (metricRows.length) {
    const table = document.createElement("table");
    table.className = "feature-preview-table";
    table.innerHTML = "<thead><tr><th>Target</th><th>Metric</th><th>Value</th></tr></thead>";
    const tbody = document.createElement("tbody");
    for (const rowValues of metricRows) {
      const row = document.createElement("tr");
      row.innerHTML = rowValues.map((value) => `<td>${escapeHTML(value)}</td>`).join("");
      tbody.append(row);
    }
    table.append(tbody);
    block.append(table);
  }
  return block;
}

export function mlAssetEditorBlock(component, config) {
  const metadata = component.ml_metadata || {};
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">ML Assets</div>`;

  const form = document.createElement("div");
  form.className = "connection-form ml-asset-form";

  const format = document.createElement("select");
  format.dataset.mlMetadataField = "model_format";
  format.setAttribute("aria-label", "Model format");
  for (const value of config.modelFormats) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    format.append(option);
  }
  format.value = metadata.model_format || "custom";

  const packages = document.createElement("input");
  packages.dataset.mlMetadataField = "required_packages";
  packages.placeholder = "required packages";
  packages.value = (metadata.required_packages || []).join(", ");
  packages.setAttribute("aria-label", "Required packages");

  const resolution = document.createElement("input");
  resolution.dataset.mlMetadataField = "valid_time_resolution";
  resolution.placeholder = "time resolution";
  resolution.value = metadata.valid_time_resolution || "";
  resolution.setAttribute("aria-label", "Valid time resolution");

  const ranges = document.createElement("textarea");
  ranges.dataset.mlMetadataField = "valid_input_ranges";
  ranges.placeholder = '{"feature_name": {"min": 0, "max": 1}}';
  ranges.value = metadata.valid_input_ranges ? JSON.stringify(metadata.valid_input_ranges, null, 2) : "";
  ranges.rows = 4;
  ranges.setAttribute("aria-label", "Valid input ranges");

  form.append(format, packages, resolution, ranges);

  for (const [field, label] of config.assetFields) {
    const row = document.createElement("div");
    row.className = "ml-asset-row";
    row.dataset.mlAssetField = field;
    const caption = document.createElement("span");
    caption.textContent = label;
    const file = document.createElement("input");
    file.type = "file";
    file.setAttribute("aria-label", label);
    row.append(caption, file);
    form.append(row);
  }

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Save ML Assets";
  button.addEventListener("click", () => config.onSave(component.id, block));
  const schemaButton = document.createElement("button");
  schemaButton.type = "button";
  schemaButton.textContent = "Apply Schema Nodes";
  schemaButton.addEventListener("click", () => config.onApplySchema(component.id));
  form.append(button, schemaButton);
  block.append(form);
  return block;
}

export function featureMappingSuggestionBlock(targetComponent, suggestions, onConnect) {
  if (!suggestions.length) return null;
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Feature Mapping Suggestion</div>`;
  const form = document.createElement("div");
  form.className = "connection-form";
  const select = document.createElement("select");
  select.setAttribute("aria-label", "Feature mapper source");
  for (const suggestion of suggestions) {
    const option = document.createElement("option");
    option.value = `${suggestion.component}.${suggestion.node}`;
    option.textContent = `${suggestion.component}.${suggestion.node}`;
    select.append(option);
  }
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Connect Feature Mapper";
  button.addEventListener("click", () => onConnect(select.value, targetComponent.id, "features"));
  form.append(select, button);
  block.append(form);
  return block;
}

export function featurePreviewValue(latestOutputs, latestInputs, titleFor) {
  if (isPlainObject(latestOutputs?.features)) {
    return { title: titleFor("Feature Preview"), features: latestOutputs.features };
  }
  if (isPlainObject(latestInputs?.features)) {
    return { title: titleFor("Received Features"), features: latestInputs.features };
  }
  return null;
}

export function featurePreviewBlock(title, features) {
  const block = document.createElement("div");
  block.className = "inspector-block feature-preview-block";
  block.innerHTML = `<div class="inspector-title">${escapeHTML(title)}</div>`;
  const table = document.createElement("table");
  table.className = "feature-preview-table";
  table.innerHTML = "<thead><tr><th>Feature</th><th>Value</th></tr></thead>";
  const tbody = document.createElement("tbody");
  const rows = Object.entries(features || {});
  if (!rows.length) {
    tbody.innerHTML = `<tr><td colspan="2" class="empty-cell">No features</td></tr>`;
  } else {
    for (const [name, value] of rows) {
      const row = document.createElement("tr");
      row.innerHTML = `<td>${escapeHTML(name)}</td><td>${escapeHTML(formatValue(value))}</td>`;
      tbody.append(row);
    }
  }
  table.append(tbody);
  block.append(table);
  return block;
}

export function splitRequiredPackages(value) {
  return String(value || "")
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function parseValidInputRanges(value) {
  const text = String(value || "").trim();
  if (!text) return undefined;

  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new Error('Valid input ranges must be JSON such as {"feature": {"min": 0, "max": 1}}');
  }
  if (!isPlainObject(parsed)) {
    throw new Error("Valid input ranges must be an object keyed by feature name");
  }

  const ranges = {};
  for (const [name, bounds] of Object.entries(parsed)) {
    const feature = String(name || "").trim();
    if (!feature) {
      throw new Error("Valid input ranges cannot include an empty feature name");
    }
    if (!isPlainObject(bounds)) {
      throw new Error(`Valid input range for ${feature} must be an object`);
    }
    const clean = {};
    for (const key of ["min", "max"]) {
      if (bounds[key] === undefined || bounds[key] === "") continue;
      const value = Number(bounds[key]);
      if (!Number.isFinite(value)) {
        throw new Error(`Valid input range ${key} must be numeric: ${feature}`);
      }
      clean[key] = value;
    }
    if (clean.min !== undefined && clean.max !== undefined && clean.min > clean.max) {
      throw new Error(`Valid input range min must be <= max: ${feature}`);
    }
    ranges[feature] = clean;
  }
  return ranges;
}

export function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => {
      const value = String(reader.result || "");
      const comma = value.indexOf(",");
      resolve(comma >= 0 ? value.slice(comma + 1) : value);
    });
    reader.addEventListener("error", () => reject(reader.error || new Error("File read failed")));
    reader.readAsDataURL(file);
  });
}

function isPlainObject(value) {
  return !!value && typeof value === "object" && !Array.isArray(value);
}
