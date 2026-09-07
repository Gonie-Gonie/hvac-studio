import { el } from "./dom.js";

export function setActivityExpanded(expanded) {
  document.querySelector(".app-shell").classList.toggle("bottom-collapsed", !expanded);
  const toggle = el("toggleBottomPanelButton");
  toggle.textContent = expanded ? "Collapse" : "Expand";
  toggle.setAttribute("aria-expanded", String(expanded));
  toggle.setAttribute("aria-label", `${expanded ? "Collapse" : "Expand"} activity panel`);
}

export function initWorkspaceChrome() {
  const compactWindow = window.matchMedia("(max-width: 1100px)");
  const setInspectorVisible = (visible) => {
    document.querySelector(".app-shell").classList.toggle("inspector-hidden", !visible);
    el("toggleInspectorButton").setAttribute("aria-expanded", String(visible));
  };
  setInspectorVisible(!compactWindow.matches);
  compactWindow.addEventListener("change", (event) => setInspectorVisible(!event.matches));
  el("toggleBottomPanelButton").addEventListener("click", () => {
    setActivityExpanded(document.querySelector(".app-shell").classList.contains("bottom-collapsed"));
  });
  el("toggleInspectorButton").addEventListener("click", () => {
    setInspectorVisible(document.querySelector(".app-shell").classList.contains("inspector-hidden"));
  });
  document.addEventListener("click", (event) => {
    for (const menu of document.querySelectorAll(".toolbar-menu[open]")) {
      if (!menu.contains(event.target) || event.target.closest("button:not(:disabled)")) menu.open = false;
    }
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      document.querySelectorAll(".toolbar-menu[open]").forEach((menu) => { menu.open = false; });
    }
  });
}

export function projectDisplayName(project) {
  const name = String(project?.name || project?.project_name || "Project");
  return name.replace(/^\d+_/, "").replace(/_/g, " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
    .replace(/\b(rc|ahu|ann|sdk|ml|hvac)\b/gi, (word) => word.toUpperCase());
}
