export function connectionMediumStateForNodes(connection, sourceNode, targetNode) {
  const sourceMedium = sourceNode?.medium || "";
  const targetMedium = targetNode?.medium || "";
  const normalizedSource = normalizedMedium(sourceMedium);
  const normalizedTarget = normalizedMedium(targetMedium);
  let status = "ok";
  if (!mediumCompatible(sourceMedium, targetMedium)) {
    if (connection.allow_medium_mismatch) {
      status = "override";
    } else if (normalizedSource === "signal" && normalizedTarget && normalizedTarget !== "signal") {
      status = "warning";
    } else {
      status = "error";
    }
  }
  const label = sourceMedium && targetMedium && normalizedSource !== normalizedTarget
    ? `${sourceMedium}->${targetMedium}`
    : sourceMedium || targetMedium || "";
  return { sourceNode, targetNode, sourceMedium, targetMedium, label, status };
}

export function mediumCompatible(source, target) {
  const normalizedSource = normalizedMedium(source);
  const normalizedTarget = normalizedMedium(target);
  if (!normalizedSource || !normalizedTarget) return true;
  if (normalizedSource === "generic" || normalizedTarget === "generic") return true;
  return normalizedSource === normalizedTarget;
}

export function normalizedMedium(value) {
  return String(value || "").trim().toLowerCase();
}

export function connectionUnitStateForNodes(connection, sourceNode, targetNode) {
  const sourceUnit = sourceNode?.unit || "";
  const targetUnit = targetNode?.unit || "";
  const sourceValueType = sourceNode?.value_type || "";
  const targetValueType = targetNode?.value_type || "";
  const unitMismatch = normalizedUnit(sourceUnit) && normalizedUnit(targetUnit) && normalizedUnit(sourceUnit) !== normalizedUnit(targetUnit);
  const hasConversion = Boolean(connection.unit_conversion);
  const label = sourceUnit || targetUnit
    ? (unitMismatch ? `${sourceUnit || "?"}->${targetUnit || "?"}` : sourceUnit || targetUnit)
    : "";
  const valueTypeLabel = sourceValueType || targetValueType
    ? (sourceValueType && targetValueType && sourceValueType !== targetValueType ? `${sourceValueType}->${targetValueType}` : sourceValueType || targetValueType)
    : "";
  let status = "ok";
  if (hasConversion) status = "converted";
  else if (unitMismatch) status = "warning";
  return {
    sourceNode,
    targetNode,
    sourceUnit,
    targetUnit,
    sourceValueType,
    targetValueType,
    label,
    valueTypeLabel,
    status,
  };
}

export function normalizedUnit(value) {
  return String(value || "").trim().toLowerCase();
}

export function connectionStatusLabel(connection, mediumState, route, unitState) {
  if (mediumState.status === "error") return "medium mismatch";
  if (mediumState.status === "override") return connection.medium_override_reason ? "override" : "medium override";
  if (mediumState.status === "warning") return "signal warning";
  if (unitState.status === "converted") return "converted";
  if (unitState.status === "warning") return "unit mismatch";
  if (route?.backtracking) return "backtracking";
  if (route?.longPath) return "long path";
  return "";
}

export function connectionUnitConversionSummary(connection, unitState, formatValue) {
  const conversion = connection.unit_conversion;
  if (!conversion) return "";
  const factor = Number(conversion.factor ?? 1);
  const offset = Number(conversion.offset ?? 0);
  const offsetLabel = offset === 0 ? "" : (offset > 0 ? ` + ${formatValue(offset)}` : ` - ${formatValue(Math.abs(offset))}`);
  return [
    unitState.label ? `${unitState.label}` : "converted",
    `x ${formatValue(factor)}${offsetLabel}`,
  ].filter(Boolean).join(" ");
}

export function connectionUnitConversionPresetID(connection, conversion, unitState, presets) {
  if (conversion) {
    const factor = Number(conversion.factor ?? 1);
    const offset = Number(conversion.offset ?? 0);
    const match = (presets || []).find(([, , definition]) => (
      definition && approximatelyEqual(definition.factor, factor) && approximatelyEqual(definition.offset, offset)
    ));
    return match?.[0] || "custom";
  }
  const sourceUnit = normalizedUnit(unitState?.sourceUnit);
  const targetUnit = normalizedUnit(unitState?.targetUnit);
  if (sourceUnit === "w" && targetUnit === "kw") return "w_to_kw";
  if (sourceUnit === "kw" && targetUnit === "w") return "kw_to_w";
  if (sourceUnit === "degc" && targetUnit === "k") return "degc_to_k";
  if (sourceUnit === "kg/s" && targetUnit === "kg/h") return "kgs_to_kgh";
  if (sourceUnit === "fraction" && targetUnit === "percent") return "fraction_to_percent";
  return "custom";
}

export function unitConversionPresetDefinition(presets, presetID) {
  return (presets || []).find(([id]) => id === presetID)?.[2] || null;
}

export function unitConversionInitialNumber(conversion, preset, key, fallback) {
  if (conversion && conversion[key] !== undefined && conversion[key] !== null) return Number(conversion[key]);
  if (preset && preset[key] !== undefined) return preset[key];
  return fallback;
}

export function finiteNumberOrDefault(value, fallback) {
  const text = String(value ?? "").trim();
  if (text === "") return fallback;
  const parsed = Number(text);
  return Number.isFinite(parsed) ? parsed : Number.NaN;
}

export function connectionContractLabels(mediumState, unitState) {
  return [
    mediumState.label ? `medium ${mediumState.label}` : "",
    unitState.label ? `unit ${unitState.label}` : "",
    unitState.valueTypeLabel ? `value_type ${unitState.valueTypeLabel}` : "",
  ].filter(Boolean);
}

function approximatelyEqual(a, b) {
  return Math.abs(Number(a) - Number(b)) < 1e-12;
}
