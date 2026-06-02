export function sampleValueFor(id) {
  const samples = {
    value: 4,
    building_load_kw: 500,
    base_chw_setpoint_c: 7,
  };
  return samples[id] ?? "";
}

export function coerceInput(value) {
  const trimmed = value.trim();
  if (trimmed === "") return "";
  const numeric = Number(trimmed);
  return Number.isNaN(numeric) ? trimmed : numeric;
}

export function coerceParameter(value) {
  const trimmed = value.trim();
  if (trimmed === "") return "";
  if (trimmed === "true") return true;
  if (trimmed === "false") return false;
  if (trimmed === "null") return null;
  if (trimmed.startsWith("{") || trimmed.startsWith("[") || trimmed.startsWith('"')) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return trimmed;
    }
  }
  const numeric = Number(trimmed);
  return Number.isNaN(numeric) ? trimmed : numeric;
}

export function parameterInputValue(value) {
  if (typeof value === "object" && value !== null) return JSON.stringify(value);
  return String(value ?? "");
}

export function formatValue(value) {
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
