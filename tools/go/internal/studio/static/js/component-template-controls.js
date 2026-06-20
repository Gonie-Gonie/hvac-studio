import {
  availableComponentFilterOptions,
  componentTemplateByID,
  componentTemplateMetaText,
  componentTemplateOptionLabel,
  filteredComponentTemplates,
} from "./component-templates.js";

export function renderComponentTemplateControls({ templates, categoryOptions, modeOptions, elements }) {
  renderComponentFilterSelect(
    elements.categorySelect,
    availableComponentFilterOptions(templates, categoryOptions, "category"),
  );
  const category = elements.categorySelect?.value || "";
  renderComponentFilterSelect(
    elements.modeSelect,
    availableComponentFilterOptions(
      filteredComponentTemplates(templates, { category }),
      modeOptions,
      "execution_mode",
    ),
  );
  const visibleTemplates = currentComponentTemplateOptions(templates, elements);
  renderComponentTemplateSelect(elements.templateSelect, visibleTemplates);
  renderSelectedComponentTemplateMeta(templates, elements);
  return visibleTemplates;
}

export function currentComponentTemplateOptions(templates, elements) {
  const category = elements.categorySelect?.value || "";
  const executionMode = elements.modeSelect?.value || "";
  return filteredComponentTemplates(templates, { category, executionMode });
}

export function renderSelectedComponentTemplateMeta(templates, elements) {
  const meta = elements.meta;
  if (!meta) return;
  const templateID = elements.templateSelect?.value || "";
  const template = componentTemplateByID(templates, templateID);
  meta.textContent = template ? componentTemplateMetaText(template) : "";
}

function renderComponentFilterSelect(select, options) {
  if (!select) return;
  const previous = select.value;
  select.innerHTML = "";
  for (const [value, label] of options || []) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    select.append(option);
  }
  if ((options || []).some(([value]) => value === previous)) {
    select.value = previous;
  }
}

function renderComponentTemplateSelect(select, templates) {
  if (!select) return;
  const previous = select.value;
  select.innerHTML = "";
  for (const template of templates || []) {
    const option = document.createElement("option");
    option.value = template.id;
    option.textContent = componentTemplateOptionLabel(template);
    select.append(option);
  }
  if (!(templates || []).length) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = "No matching templates";
    select.append(option);
  } else if ((templates || []).some((template) => template.id === previous)) {
    select.value = previous;
  }
}
