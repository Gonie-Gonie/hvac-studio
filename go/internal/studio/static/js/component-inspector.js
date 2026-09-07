import { escapeHTML } from "./dom.js";
import { emptyKVRow, inspectorKVRow } from "./inspector-ui.js";
import {
  replacementDiffText,
  replacementPreview,
} from "./replacement-preview.js";

export function componentEditor(component, actions) {
  const block = document.createElement("div");
  block.className = "inspector-block";
  block.innerHTML = `<div class="inspector-title">Component Settings</div>`;

  const form = document.createElement("div");
  form.className = "connection-form";
  const name = document.createElement("input");
  name.id = "componentNameInput";
  name.value = component.name || component.id;
  name.setAttribute("aria-label", "Component name");

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Rename";
  button.addEventListener("click", () => actions.onRename(component.id));
  const duplicateButton = document.createElement("button");
  duplicateButton.type = "button";
  duplicateButton.textContent = "Duplicate";
  duplicateButton.addEventListener("click", () => actions.onDuplicate(component.id));
  const codeButton = document.createElement("button");
  codeButton.type = "button";
  codeButton.textContent = "Code";
  codeButton.addEventListener("click", () => actions.onOpenCode(component.id));
  name.addEventListener("keydown", (event) => {
    if (event.key === "Enter") actions.onRename(component.id);
  });

  form.append(name, button, duplicateButton, codeButton);
  block.append(form);
  return block;
}

export function replacementPreviewForComponent(component, template, context) {
  return replacementPreview(
    component,
    template,
    context.system,
    context.connections || [],
    context.mapParameters !== false,
  );
}

export function replacementPreviewBlock(component, context, actions) {
  const block = document.createElement("div");
  block.className = "inspector-block replacement-preview-block";
  block.innerHTML = `<div class="inspector-title">Replacement Preview</div>`;
  const template = context.selectedTemplate;
  if (!template) {
    block.append(emptyKVRow("No replacement template selected"));
    return block;
  }
  const preview = replacementPreviewForComponent(component, template, context);
  block.append(inspectorKVRow("Template", `${template.name || template.id} (${template.id})`));
  block.append(inspectorKVRow("Contract", `${preview.diff.matchedInputs.length}/${preview.diff.originalInputs.length} inputs, ${preview.diff.matchedOutputs.length}/${preview.diff.originalOutputs.length} outputs, ${preview.diff.matchedParameters.length}/${preview.diff.originalParameters.length} parameters`));
  block.append(inspectorKVRow("Status", preview.problems.length ? `${preview.problems.length} broken mapping${preview.problems.length === 1 ? "" : "s"}` : "Compatible"));
  block.append(replacementMappingTable("Node Mapping", preview.nodeMappings));
  block.append(replacementMappingTable("Parameter Mapping", preview.parameterMappings));
  block.append(replacementDiffSummary(preview.diff));

  const form = document.createElement("div");
  form.className = "connection-form replacement-form";
  const mapLabel = document.createElement("label");
  mapLabel.className = "node-required-toggle contract-toggle";
  const mapParameters = document.createElement("input");
  mapParameters.id = "replacementMapParameters";
  mapParameters.type = "checkbox";
  mapParameters.checked = context.mapParameters !== false;
  mapParameters.setAttribute("aria-label", "Copy same-name parameters");
  mapParameters.addEventListener("change", () => actions.onMapParametersChange(mapParameters.checked));
  mapLabel.append(mapParameters, document.createTextNode("Copy same-name parameters"));
  const replaceButton = document.createElement("button");
  replaceButton.type = "button";
  replaceButton.textContent = "Replace And Validate";
  replaceButton.disabled = Boolean(preview.problems.length);
  replaceButton.addEventListener("click", actions.onReplace);
  form.append(mapLabel, replaceButton);
  block.append(form);
  return block;
}

function replacementMappingTable(title, mappings) {
  const wrap = document.createElement("div");
  wrap.className = "replacement-table-wrap";
  const heading = document.createElement("div");
  heading.className = "replacement-subtitle";
  heading.textContent = title;
  const table = document.createElement("table");
  table.className = "feature-preview-table replacement-preview-table";
  table.innerHTML = "<thead><tr><th>Scope</th><th>From</th><th>To</th><th>Status</th></tr></thead>";
  const tbody = document.createElement("tbody");
  if (!mappings.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No entry-system references</td></tr>`;
  } else {
    for (const mapping of mappings) {
      const row = document.createElement("tr");
      row.className = mapping.status === "missing" ? "replacement-missing" : "";
      row.innerHTML = `
        <td>${escapeHTML(String(mapping.scope || "").replace(/_/g, " "))}</td>
        <td>${escapeHTML(mapping.from || "")}</td>
        <td>${escapeHTML(mapping.to || "")}</td>
        <td>${escapeHTML(mapping.status || "")}</td>
      `;
      if (mapping.detail) row.title = mapping.detail;
      tbody.append(row);
    }
  }
  table.append(tbody);
  wrap.append(heading, table);
  return wrap;
}

function replacementDiffSummary(diff) {
  const wrap = document.createElement("div");
  wrap.className = "replacement-diff";
  const rows = [
    ["Input diff", replacementDiffText(diff.matchedInputs, diff.missingInputs, diff.addedInputs)],
    ["Output diff", replacementDiffText(diff.matchedOutputs, diff.missingOutputs, diff.addedOutputs)],
    ["Parameter diff", replacementDiffText(diff.matchedParameters, diff.missingParameters, diff.addedParameters)],
  ];
  for (const [key, value] of rows) wrap.append(inspectorKVRow(key, value));
  return wrap;
}
