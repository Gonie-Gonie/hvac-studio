export function componentTemplateByID(templates, templateID) {
  return (templates || []).find((template) => template.id === templateID) || null;
}

export function filteredComponentTemplates(templates, filters = {}) {
  const category = filters.category || "";
  const executionMode = filters.executionMode || "";
  return (templates || []).filter((template) => {
    if (category && template.category !== category) return false;
    if (executionMode && template.execution_mode !== executionMode) return false;
    return true;
  });
}

export function defaultComponentName(templates, templateID, fallbackName = "") {
  const template = componentTemplateByID(templates, templateID);
  return template?.name || fallbackName || "Component";
}

export function componentTemplateOptionLabel(template) {
  const contract = `${template.input_count || 0} in / ${template.output_count || 0} out`;
  const layout = template.source_layout ? ` / ${String(template.source_layout).replace(/_/g, " ")}` : "";
  const mode = template.execution_mode ? ` / ${String(template.execution_mode).replace(/_/g, " ")}` : "";
  return `${template.name || template.id} (${contract}${mode}${layout})`;
}

export function componentTemplateMetaText(template) {
  if (!template) return "";
  return [
    template.category || "",
    template.execution_mode || "",
    template.source_layout ? String(template.source_layout).replace(/_/g, " ") : "",
  ].filter(Boolean).join(" / ");
}
