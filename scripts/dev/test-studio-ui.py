"""Exercise Studio's example canvas and run workflow in installed Edge/Chrome.

Run through test-studio-ui.ps1 to build a server with isolated example fixtures.
Screenshots and a machine-readable report are kept under the repo's .tmp folder.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path

from playwright.sync_api import Page, expect, sync_playwright


def graph_hashes(root: Path) -> dict[str, str]:
    return {
        str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted((root / "examples").rglob("graph.json"))
    }


def settle_canvas(page: Page) -> None:
    page.evaluate("() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))")


def open_example(page: Page, example: str, count: int) -> None:
    select = page.locator("#projectSelect")
    expect(select.locator("option").first).to_be_attached()
    options = select.locator("option").evaluate_all("options => options.map(option => option.value)")
    path = next(value for value in options if f"/examples/{example}/" in value.replace("\\", "/"))
    if select.input_value() != path:
        with page.expect_response(lambda response: "/api/project?" in response.url):
            select.select_option(path)
    page.locator('.mode-button[data-mode="canvas"]').click()
    expect(page.locator(".component-node")).to_have_count(count)
    expect(page.locator("#systemSubtitle")).to_have_attribute("title", re.compile(re.escape(example)))
    settle_canvas(page)


def assert_cards(page: Page, count: int) -> list[dict]:
    """Check actual browser boxes, including every port row, without fixed pixels."""
    cards = page.locator(".component-node").evaluate_all("""nodes => nodes.map(node => {
      const box = node.getBoundingClientRect();
      const ports = [...node.querySelectorAll('[data-node-endpoint]')];
      return {
        id: node.dataset.componentId, x: box.x, y: box.y,
        width: box.width, height: box.height,
        titleSize: parseFloat(getComputedStyle(node.querySelector('.component-title')).fontSize) * box.width / node.offsetWidth,
        clipped: node.scrollHeight > node.clientHeight + 2,
        ports: ports.map(port => {
          const rect = port.getBoundingClientRect();
          return {text: port.textContent.trim(), contained:
            rect.left >= box.left - 1 && rect.top >= box.top - 1 &&
            rect.right <= box.right + 1 && rect.bottom <= box.bottom + 1};
        }),
      };
    })""")
    assert len(cards) == count, cards
    for index, card in enumerate(cards):
        assert card["width"] > 0 and card["height"] > 0, card
        assert card["titleSize"] >= 10.5, f"Title is too small in the fitted diagram: {card}"
        assert not card["clipped"], f"Card content clipped: {card}"
        assert card["ports"] and all(port["text"] and port["contained"] for port in card["ports"]), card
        for other in cards[index + 1 :]:
            overlap_x = min(card["x"] + card["width"], other["x"] + other["width"]) - max(card["x"], other["x"])
            overlap_y = min(card["y"] + card["height"], other["y"] + other["height"]) - max(card["y"], other["y"])
            assert overlap_x <= 1 or overlap_y <= 1, f"Cards overlap: {card['id']} / {other['id']}"
    return cards


def assert_fit(page: Page) -> None:
    page.locator("#canvasFitButton").click()
    settle_canvas(page)
    outside = page.locator(".component-node").evaluate_all("""nodes => {
      const wrap = document.querySelector('.canvas-wrap');
      const viewport = wrap.getBoundingClientRect();
      return nodes.filter(node => {
        const box = node.getBoundingClientRect();
        return box.left < viewport.left - 2 || box.top < viewport.top - 2 ||
          box.right > viewport.left + wrap.clientWidth + 2 ||
          box.bottom > viewport.top + wrap.clientHeight + 2;
      }).map(node => node.dataset.componentId);
    }""")
    assert not outside, f"Fit leaves cards outside canvas viewport: {outside}"


def canvas_position(page: Page, component_id: str) -> dict:
    return page.locator(f'.component-node[data-component-id="{component_id}"]').evaluate(
        "node => ({x: parseFloat(node.style.left), y: parseFloat(node.style.top)})"
    )


def set_details(page: Page, selector: str, opened: bool) -> None:
    details = page.locator(selector)
    if details.evaluate("element => element.open") != opened:
        details.locator(":scope > summary").click()


def check_drag_and_reload(page: Page, example: str, count: int) -> dict:
    node = page.locator(".component-node").last
    component_id = node.get_attribute("data-component-id")
    before = canvas_position(page, component_id)
    box = node.locator(".component-head").bounding_box()
    assert box, "Component header is not visible for dragging"
    page.mouse.move(box["x"] + box["width"] / 2, box["y"] + box["height"] / 2)
    page.mouse.down()
    page.mouse.move(box["x"] + box["width"] / 2 + 70, box["y"] + box["height"] / 2 + 40, steps=10)
    page.mouse.up()
    settle_canvas(page)
    after = canvas_position(page, component_id)
    assert after["x"] > before["x"] + 20 and after["y"] > before["y"] + 10, (before, after)
    page.reload(wait_until="networkidle")
    open_example(page, example, count)
    reloaded = canvas_position(page, component_id)
    assert abs(reloaded["x"] - after["x"]) <= 1 and abs(reloaded["y"] - after["y"]) <= 1, (after, reloaded)
    return {"component": component_id, "before": before, "after": after, "reloaded": reloaded}


def check_edge_selection(page: Page) -> None:
    expect(page.locator(".connection-label:visible")).to_have_count(0)
    # SVG path bounding-box centers may be nowhere near the stroke. Sample actual
    # rendered path points, and click a point that the browser confirms is exposed.
    target = page.locator(".connection-line").evaluate_all("""paths => {
      for (const path of paths) {
        const id = path.closest('[data-connection-id]')?.dataset.connectionId;
        const length = path.getTotalLength();
        for (const fraction of [0.5, 0.35, 0.65, 0.2, 0.8]) {
          const local = path.getPointAtLength(length * fraction);
          const point = new DOMPoint(local.x, local.y).matrixTransform(path.getScreenCTM());
          const hit = document.elementFromPoint(point.x, point.y);
          if (id && hit?.closest('[data-connection-id]')?.dataset.connectionId === id) {
            return {x: point.x, y: point.y, id};
          }
        }
      }
      return null;
    }""")
    assert target, "No connection stroke can be clicked in the fitted canvas"
    page.mouse.click(target["x"], target["y"])
    expect(page.locator(".connection-label:visible")).to_have_count(1)
    expect(page.locator(".connection-group.selected")).to_have_attribute("data-connection-id", target["id"])


def check_run_input_workflow(page: Page, output: Path) -> dict:
    open_example(page, "001_scalar_component", 1)
    settings = page.locator("#runSettings")
    expect(settings).not_to_have_attribute("open", "")
    settings.locator("summary").click()
    control = page.locator('[data-input-id="value"]')
    expect(control).to_be_visible()
    control.fill("4")
    settings.locator("summary").click()
    expect(control).not_to_be_visible()
    settings.locator("summary").click()
    expect(control).to_have_value("4")
    page.screenshot(path=output / "001-inputs-expanded-1440x920.png")
    settings.locator("summary").click()
    with page.expect_response(lambda response: response.url.endswith("/api/run"), timeout=45000) as response_info:
        page.locator("#runButton").click()
    response = response_info.value
    result = response.json()
    assert response.ok, result
    assert response.request.post_data_json["inputs"]["value"] == 4, response.request.post_data_json
    assert result["result"]["ok"] and result["result"]["outputs"]["result"] == 10, result
    expect(page.locator("#runView")).to_be_visible()
    expect(page.locator("#runOutputRows")).to_contain_text("10")
    page.screenshot(path=output / "001-run-result-1440x920.png")
    return {"input": 4, "output": 10}


def check_inspector_and_workspace(page: Page, output: Path) -> dict:
    open_example(page, "015_rc_ahu_ann_composition", 7)
    contract = page.locator(".inspector-node-row").first
    expect(contract.locator(".inspector-node-contract")).not_to_be_visible()
    contract.locator("summary").click()
    expect(contract.locator(".inspector-node-contract")).to_be_visible()
    expect(contract).to_contain_text("outdoor_temperature_c")
    page.screenshot(path=output / "015-inspector-expanded-1440x920.png")
    contract.locator("summary").click()

    select = page.locator("#projectSelect")
    options = select.locator("option").evaluate_all("options => options.map(option => ({value: option.value, label: option.textContent}))")
    workspace = next(option["value"] for option in options if option["label"].startswith("Project"))
    with page.expect_response(lambda response: "/api/project?" in response.url):
        select.select_option(workspace)
    expect(page.locator("#projectAccessBadge")).to_have_text("Workspace")
    expect(page.locator("#createComponentDetails")).to_be_visible()
    page.locator("#createComponentDetails > summary").click()
    expect(page.locator("#newComponentName")).to_be_visible()
    expect(page.locator("#newComponentName")).to_be_enabled()
    expect(page.locator("#addComponentButton")).to_be_enabled()
    page.locator("#createComponentDetails > summary").click()

    set_details(page, "#runSettings", True)
    workspace_input = page.locator('[data-input-id="value"]')
    workspace_input.fill("17")
    editable_contract = page.locator(".inspector-node-row").first
    if not editable_contract.evaluate("details => details.open"):
        editable_contract.locator("summary").click()
    name = editable_contract.locator('[data-node-field="name"]')
    expect(name).to_be_visible()
    name.fill("Reviewed input")
    with page.expect_response(lambda response: response.url.endswith("/api/project/nodes/update")) as response_info:
        editable_contract.get_by_role("button", name="Save", exact=True).click()
    assert response_info.value.ok, response_info.value.text()
    expect(page.locator(".component-node .node-label")).to_contain_text(["Reviewed input", "Result"])
    expect(workspace_input).to_have_value("17")
    page.reload(wait_until="networkidle")
    expect(select).to_have_value(workspace)
    expect(page.locator(".component-node .node-label")).to_contain_text(["Reviewed input", "Result"])
    editable_contract = page.locator(".inspector-node-row").first
    if not editable_contract.evaluate("details => details.open"):
        editable_contract.locator("summary").click()
    expect(editable_contract.locator('[data-node-field="name"]')).to_have_value("Reviewed input")
    page.screenshot(path=output / "workspace-contract-edit-1440x920.png")
    return {"read_only_contract": "expanded", "workspace_contract_name": "Reviewed input", "persisted": True}


def check_run_input_drafts(page: Page) -> dict:
    open_example(page, "015_rc_ahu_ann_composition", 7)
    set_details(page, "#runSettings", True)
    outdoor = page.locator('[data-input-id="outdoor_temperature_c"]')
    solar = page.locator('[data-input-id="solar_gain_kw"]')
    internal = page.locator('[data-input-id="internal_gain_kw"]')
    outdoor.fill("40")
    solar.fill("8")
    internal.fill("")
    page.locator("#runParameterSetSelect").select_option(index=1)
    expect(outdoor).to_have_value("40")
    expect(solar).to_have_value("8")
    expect(internal).to_have_value("")
    page.locator("#runSeriesInputSelect").select_option(index=1)
    expect(outdoor).to_have_value("40")
    expect(internal).to_have_value("")
    outdoor.locator("..").get_by_role("button").click()
    expect(outdoor).to_have_value("32")
    expect(solar).to_have_value("8")
    expect(internal).to_have_value("")

    # Copy through the real UI so scenario writes remain isolated in the fixture.
    set_details(page, "#projectActions", True)
    page.locator("#projectNameInput").fill("UI scenario fixture")
    with page.expect_response(lambda response: response.url.endswith("/api/projects/copy")) as copied:
        page.locator("#copyProjectButton").click()
    assert copied.value.ok, copied.value.text()
    expect(page.locator("#projectAccessBadge")).to_have_text("Workspace")
    set_details(page, "#projectActions", False)
    set_details(page, "#runSettings", True)
    expect(outdoor).to_have_value("32")
    expect(solar).to_have_value("4")
    outdoor.fill("36")
    solar.fill("8")
    page.locator("#scenarioNameInput").fill("UI retained scenario")
    with page.expect_response(lambda response: response.url.endswith("/api/project/scenarios")) as saved:
        page.locator("#scenarioNameInput").press("Enter")
    assert saved.value.ok, saved.value.text()
    outdoor.fill("49")
    solar.fill("9")

    def load_scenario() -> None:
        set_details(page, '#projectTree details[data-tree-section="Scenarios"]', True)
        with page.expect_response(lambda response: "/api/project/scenario?" in response.url):
            page.locator("#projectTree").get_by_text("UI retained scenario", exact=True).click()
        expect(outdoor).to_have_value("36")
        expect(solar).to_have_value("8")

    load_scenario()
    outdoor.fill("37")
    page.locator("#runParameterSetSelect").select_option(index=1)
    expect(outdoor).to_have_value("37")
    expect(solar).to_have_value("8")
    load_scenario()
    page.locator(".active-scenario").get_by_role("button", name="Clear").click()
    expect(outdoor).to_have_value("32")
    expect(solar).to_have_value("4")
    outdoor.fill("44")
    open_example(page, "001_scalar_component", 1)
    open_example(page, "015_rc_ahu_ann_composition", 7)
    expect(outdoor).to_have_value("32")
    expect(solar).to_have_value("4")
    set_details(page, "#runSettings", False)
    return {"parameter_and_series_rerender": True, "empty_draft": True, "individual_reset": True,
            "scenario_restore_edit_and_clear": True, "project_switch_reset": True}


def check_small_viewport(page: Page, output: Path) -> dict:
    page.set_viewport_size({"width": 1024, "height": 768})
    settle_canvas(page)
    geometry = page.evaluate("""() => ({
      headerBottom: document.querySelector('.top-bar').getBoundingClientRect().bottom,
      workspaceTop: document.querySelector('.workspace').getBoundingClientRect().top,
      width: document.documentElement.scrollWidth,
    })""")
    assert geometry["headerBottom"] <= geometry["workspaceTop"] + 1, geometry
    assert geometry["width"] <= 1026, geometry
    toggle = page.locator("#toggleInspectorButton")
    if toggle.get_attribute("aria-expanded") == "true":
        toggle.click()
    expect(page.locator(".right-sidebar")).not_to_be_visible()
    expect(toggle).to_have_attribute("aria-expanded", "false")
    toggle.click()
    expect(page.locator(".right-sidebar")).to_be_visible()
    expect(toggle).to_have_attribute("aria-expanded", "true")
    page.screenshot(path=output / "015-inspector-overlay-1024x768.png")
    toggle.click()
    expect(page.locator(".right-sidebar")).not_to_be_visible()
    page.screenshot(path=output / "015-canvas-1024x768.png")
    return geometry


def run(args: argparse.Namespace) -> None:
    original_hashes = graph_hashes(args.repo)
    fixture_hashes = graph_hashes(args.fixture)
    errors: list[str] = []
    notices: list[str] = []
    report: dict = {"viewports": [], "errors": errors, "browser_notices": notices}
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(
            executable_path=str(args.browser), headless=True,
            args=["--no-first-run", "--no-default-browser-check"],
        )
        context = browser.new_context(viewport={"width": 1440, "height": 920}, device_scale_factor=1)
        page = context.new_page()
        page.set_default_timeout(15000)
        page.on("pageerror", lambda error: errors.append(f"pageerror: {error}"))
        def console_message(message) -> None:
            if message.type != "error":
                return
            location = message.location.get("url", "")
            target = notices if location.endswith("favicon.ico") else errors
            target.append(f"console: {message.text} ({location})")

        page.on("console", console_message)
        page.on("response", lambda response: errors.append(f"HTTP {response.status}: {response.url}") if response.status >= 400 and not response.url.endswith("favicon.ico") else None)
        try:
            page.goto(args.url + "/#canvas", wait_until="networkidle")
            open_example(page, "015_rc_ahu_ann_composition", 7)
            for detail_id in ("runSettings", "createComponentDetails", "projectActions", "moreCommands"):
                expect(page.locator(f"#{detail_id}")).not_to_have_attribute("open", "")
            assert_cards(page, 7)
            expect(page.locator("#autoLayoutButton")).to_be_enabled()
            assert_fit(page)
            fit_zoom = page.locator("#canvasZoomLabel").inner_text()
            page.locator("#canvasZoomInButton").click()
            expect(page.locator("#canvasZoomLabel")).not_to_have_text(fit_zoom)
            zoomed = page.locator("#canvasZoomLabel").inner_text()
            page.locator("#canvasZoomOutButton").click()
            expect(page.locator("#canvasZoomLabel")).not_to_have_text(zoomed)
            assert_fit(page)
            expect(page.locator(".component-node .node-label").first).not_to_be_visible()
            page.locator("#canvasResetButton").click()
            expect(page.locator("#canvasZoomLabel")).to_have_text("100%")
            expect(page.locator(".component-node .node-label").first).to_be_visible()
            page.screenshot(path=args.output / "015-canvas-detail-100-percent-1440x920.png")
            assert_fit(page)
            report["drag"] = check_drag_and_reload(page, "015_rc_ahu_ann_composition", 7)
            page.locator("#autoLayoutButton").click()
            settle_canvas(page)
            assert_cards(page, 7)
            assert_fit(page)
            check_edge_selection(page)
            report["run"] = check_run_input_workflow(page, args.output)
            report["inspector"] = check_inspector_and_workspace(page, args.output)
            report["run_input_drafts"] = check_run_input_drafts(page)

            for width, height in ((1440, 920), (1920, 1080)):
                page.set_viewport_size({"width": width, "height": height})
                for example, count in (("001_scalar_component", 1), ("015_rc_ahu_ann_composition", 7)):
                    open_example(page, example, count)
                    assert_fit(page)
                    cards = assert_cards(page, count)
                    filename = f"{example[:3]}-canvas-{width}x{height}.png"
                    page.screenshot(path=args.output / filename)
                    report["viewports"].append({"example": example, "width": width, "height": height, "cards": cards, "screenshot": filename})
            report["small_viewport"] = check_small_viewport(page, args.output)
            assert graph_hashes(args.fixture) == fixture_hashes, "Example graph files were modified by canvas interactions"
            assert graph_hashes(args.repo) == original_hashes, "Repository example graph files were modified"
            assert not errors, "\n".join(errors)
        except Exception:
            page.screenshot(path=args.output / "failure.png", full_page=True)
            (args.output / "failure.html").write_text(page.content(), encoding="utf-8")
            raise
        finally:
            (args.output / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
            context.close()
            browser.close()
    print("UI regression passed: draggable examples, retained layout, fit/zoom, readable cards, selected edges, and input/run workflow")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True)
    for argument in ("browser", "repo", "fixture", "output"):
        parser.add_argument(f"--{argument}", type=Path, required=True)
    run(parser.parse_args())
