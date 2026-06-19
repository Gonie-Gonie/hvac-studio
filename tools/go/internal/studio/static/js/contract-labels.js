export function roleLabel(role) {
  return String(role || "")
    .replace(/_target$/, "")
    .replace(/_/g, " ");
}
