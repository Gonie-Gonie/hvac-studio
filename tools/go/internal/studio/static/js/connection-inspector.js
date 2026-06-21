import {
  connectionUnitConversionPresetID,
  connectionUnitConversionSummary as connectionUnitConversionSummaryText,
  finiteNumberOrDefault,
  unitConversionInitialNumber,
  unitConversionPresetDefinition,
} from "./connections.js";
import { escapeAttr, escapeHTML } from "./dom.js";
import { formatValue } from "./format.js";
import { emptyKVRow } from "./inspector-ui.js";
import { UNIT_CONVERSION_PRESETS } from "./workspace-config.js";

export function connectionEditor(targetComponent, context, actions) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Connections</div>`;
  const existingRows = context.connectionRows || [];
  const canEditConnections = Boolean(context.canEditConnections);
  if (existingRows.length) {
    for (const connectionRow of existingRows) {
      const latest = context.latestConnectionValue(connectionRow.connection);
      const flowValue = latest.hasValue
        ? `<span class="connection-flow ${context.latestResultStale ? "stale" : ""}">${escapeHTML(formatValue(latest.value))}</span>`
        : "";
      const unitState = context.connectionUnitState(connectionRow.connection);
      const mediumValue = connectionMediumBadge(connectionRow.connection, context);
      const contractValue = connectionContractBadge(connectionRow.connection, unitState);
      const conversionValue = connectionRow.connection.unit_conversion
        ? `<span class="connection-flow converted">${escapeHTML(connectionUnitConversionSummary(connectionRow.connection, unitState))}</span>`
        : (unitState.status === "warning" ? `<span class="connection-flow warning">unit mismatch</span>` : "");
      const rowEl = document.createElement("div");
      rowEl.className = `kv connection-row ${connectionRow.id === context.selectedConnectionId ? "selected" : ""}`;
      rowEl.innerHTML = `
        <span class="kv-key">${escapeHTML(connectionRow.key)}</span>
        <span class="connection-value">
          <span>${escapeHTML(connectionRow.value)}</span>
          ${mediumValue}
          ${contractValue}
          ${conversionValue}
          ${flowValue}
        </span>
      `;
      rowEl.addEventListener("click", () => actions.onSelectConnection(connectionRow.id));
      if (canEditConnections) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "small-action";
        button.textContent = "Remove";
        button.addEventListener("click", (event) => {
          event.stopPropagation();
          actions.onDeleteConnection(connectionRow.id);
        });
        rowEl.querySelector(".connection-value").append(button);
      }
      block.append(rowEl);
    }
  }

  if (context.selectedConnection && canEditConnections) {
    block.append(connectionUnitConversionEditor(context.selectedConnection, context, actions));
  }

  if (!canEditConnections) {
    if (!existingRows.length) {
      block.append(emptyKVRow("No connections", { messagePlacement: "key" }));
    }
    return block;
  }

  const sourceOptions = context.sourceOptions || [];
  const targetOptions = targetComponent.nodes.inputs || [];
  if (!sourceOptions.length || !targetOptions.length) return block;

  const form = document.createElement("div");
  form.className = "connection-form";
  const sourceSelect = document.createElement("select");
  sourceSelect.dataset.connectionSource = "true";
  for (const endpoint of sourceOptions) {
    const option = document.createElement("option");
    option.value = `${endpoint.component}.${endpoint.node}`;
    option.textContent = `${endpoint.component}.${endpoint.node}`;
    sourceSelect.append(option);
  }
  const targetSelect = document.createElement("select");
  targetSelect.dataset.connectionTarget = "true";
  for (const node of targetOptions) {
    const option = document.createElement("option");
    option.value = node.id;
    option.textContent = `${targetComponent.id}.${node.id}`;
    targetSelect.append(option);
  }
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Connect";
  button.addEventListener("click", () => actions.onCreateConnection(sourceSelect.value, targetComponent.id, targetSelect.value));
  form.append(sourceSelect, targetSelect, button);
  block.append(form);
  return block;
}

function connectionContractBadge(connection, unitState) {
  const labels = [
    unitState.label ? `unit ${unitState.label}` : "",
    unitState.valueTypeLabel ? `value_type ${unitState.valueTypeLabel}` : "",
  ].filter(Boolean);
  if (!labels.length) return "";
  const classes = ["connection-flow", "contract-state"];
  if (unitState.status === "warning") classes.push("warning");
  if (unitState.status === "converted") classes.push("converted");
  const title = [
    unitState.sourceUnit ? `source unit ${unitState.sourceUnit}` : "",
    unitState.targetUnit ? `target unit ${unitState.targetUnit}` : "",
    unitState.sourceValueType ? `source value_type ${unitState.sourceValueType}` : "",
    unitState.targetValueType ? `target value_type ${unitState.targetValueType}` : "",
    connection.unit_conversion ? connectionUnitConversionSummary(connection, unitState) : "",
  ].filter(Boolean).join(" / ");
  return `<span class="${classes.join(" ")}"${title ? ` title="${escapeAttr(title)}"` : ""}>${escapeHTML(labels.join(" / "))}</span>`;
}

function connectionMediumBadge(connection, context) {
  const mediumState = context.connectionMediumState(connection);
  if (!mediumState.label && mediumState.status === "ok") return "";
  const classes = ["connection-flow", "medium-state"];
  let label = mediumState.label || "medium";
  if (mediumState.status === "error") {
    classes.push("error");
    label = `medium mismatch ${label}`;
  } else if (mediumState.status === "override") {
    classes.push("warning");
    label = `override ${label}`;
  } else if (mediumState.status === "warning") {
    classes.push("warning");
    label = `signal warning ${label}`;
  }
  const title = [
    mediumState.sourceMedium ? `source ${mediumState.sourceMedium}` : "",
    mediumState.targetMedium ? `target ${mediumState.targetMedium}` : "",
    connection.medium_override_reason || "",
  ].filter(Boolean).join(" / ");
  return `<span class="${classes.join(" ")}"${title ? ` title="${escapeAttr(title)}"` : ""}>${escapeHTML(label)}</span>`;
}

function connectionUnitConversionEditor(connection, context, actions) {
  const wrapper = document.createElement("div");
  wrapper.className = "connection-conversion-editor";
  const unitState = context.connectionUnitState(connection);
  const conversion = connection.unit_conversion || null;
  const presetID = connectionUnitConversionPresetID(connection, conversion, unitState, UNIT_CONVERSION_PRESETS);
  const activePresetDefinition = unitConversionPresetDefinition(UNIT_CONVERSION_PRESETS, presetID);

  const header = document.createElement("div");
  header.className = "connection-conversion-header";
  header.innerHTML = `
    <span>Unit Conversion</span>
    <span>${escapeHTML(unitState.label || "same unit")}</span>
  `;

  const form = document.createElement("div");
  form.className = "connection-conversion-form";

  const preset = document.createElement("select");
  preset.id = "connectionUnitConversionPreset";
  for (const [id, label] of UNIT_CONVERSION_PRESETS) {
    const option = document.createElement("option");
    option.value = id;
    option.textContent = label;
    preset.append(option);
  }
  preset.value = presetID;

  const factor = document.createElement("input");
  factor.id = "connectionUnitConversionFactor";
  factor.type = "number";
  factor.step = "any";
  factor.value = String(unitConversionInitialNumber(conversion, activePresetDefinition, "factor", 1));
  factor.placeholder = "Factor";

  const offset = document.createElement("input");
  offset.id = "connectionUnitConversionOffset";
  offset.type = "number";
  offset.step = "any";
  offset.value = String(unitConversionInitialNumber(conversion, activePresetDefinition, "offset", 0));
  offset.placeholder = "Offset";

  const sample = document.createElement("input");
  sample.id = "connectionUnitConversionSample";
  sample.type = "number";
  sample.step = "any";
  sample.value = "1";
  sample.placeholder = "Sample";

  const description = document.createElement("input");
  description.id = "connectionUnitConversionDescription";
  description.value = conversion?.description || activePresetDefinition?.description || "";
  description.placeholder = "Description";

  const preview = document.createElement("div");
  preview.id = "connectionUnitConversionPreview";
  preview.className = "connection-conversion-preview";

  const save = document.createElement("button");
  save.type = "button";
  save.id = "saveConnectionUnitConversionButton";
  save.textContent = "Save Conversion";
  save.addEventListener("click", () => {
    const parsedFactor = finiteNumberOrDefault(factor.value, 1);
    const parsedOffset = finiteNumberOrDefault(offset.value, 0);
    if (!Number.isFinite(parsedFactor) || parsedFactor === 0) {
      actions.onProblem("Conversion factor must be a non-zero number");
      return;
    }
    if (!Number.isFinite(parsedOffset)) {
      actions.onProblem("Conversion offset must be numeric");
      return;
    }
    actions.onUpdateUnitConversion(connection.id, {
      mode: "linear",
      factor: parsedFactor,
      offset: parsedOffset,
      description: description.value.trim(),
    });
  });

  const clear = document.createElement("button");
  clear.type = "button";
  clear.id = "clearConnectionUnitConversionButton";
  clear.className = "ghost";
  clear.textContent = "Clear";
  clear.addEventListener("click", () => actions.onUpdateUnitConversion(connection.id, null));

  const updatePreview = () => {
    const parsedFactor = finiteNumberOrDefault(factor.value, 1);
    const parsedOffset = finiteNumberOrDefault(offset.value, 0);
    const sampleValue = finiteNumberOrDefault(sample.value, 1);
    if (!Number.isFinite(parsedFactor) || !Number.isFinite(parsedOffset) || !Number.isFinite(sampleValue) || parsedFactor === 0) {
      preview.textContent = "Invalid conversion";
      preview.className = "connection-conversion-preview invalid";
      return;
    }
    const converted = sampleValue * parsedFactor + parsedOffset;
    const units = [unitState.sourceUnit, unitState.targetUnit].filter(Boolean).join(" to ");
    preview.textContent = `${formatValue(sampleValue)}${unitState.sourceUnit ? ` ${unitState.sourceUnit}` : ""} = ${formatValue(converted)}${unitState.targetUnit ? ` ${unitState.targetUnit}` : ""}${units ? ` / ${units}` : ""}`;
    preview.className = "connection-conversion-preview";
  };

  preset.addEventListener("change", () => {
    const definition = unitConversionPresetDefinition(UNIT_CONVERSION_PRESETS, preset.value);
    if (!definition) {
      updatePreview();
      return;
    }
    factor.value = String(definition.factor);
    offset.value = String(definition.offset);
    description.value = definition.description || "";
    updatePreview();
  });
  [factor, offset, sample, description].forEach((input) => input.addEventListener("input", updatePreview));
  form.append(preset, factor, offset, sample, description, preview, save, clear);
  wrapper.append(header, form);
  updatePreview();
  return wrapper;
}

function connectionUnitConversionSummary(connection, unitState) {
  return connectionUnitConversionSummaryText(connection, unitState, formatValue);
}