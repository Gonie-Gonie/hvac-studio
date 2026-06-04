package studio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

func TestProjectsEndpointListsExamples(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Projects []ProjectSummary `json:"projects"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Projects) < 2 {
		t.Fatalf("project count = %d", len(body.Projects))
	}
}

func TestStaticIndexServesWorkspace(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("HVAC Studio")) {
		t.Fatalf("index did not contain Studio shell")
	}
	if !bytes.Contains(body, []byte(`type="module"`)) {
		t.Fatalf("index did not load the Studio JavaScript module")
	}
	if !bytes.Contains(body, []byte("newComponentName")) {
		t.Fatalf("index did not include the component creation form")
	}
	if !bytes.Contains(body, []byte("componentTemplateSelect")) {
		t.Fatalf("index did not include the component template selector")
	}
	if !bytes.Contains(body, []byte("sourceEditorMeta")) {
		t.Fatalf("index did not include the source editor metadata status")
	}
	if !bytes.Contains(body, []byte("sourceCompletionPanel")) {
		t.Fatalf("index did not include the source completion panel")
	}
	if !bytes.Contains(body, []byte("sourceHighlight")) {
		t.Fatalf("index did not include the source syntax highlight layer")
	}
	if !bytes.Contains(body, []byte("formatSourceButton")) {
		t.Fatalf("index did not include the source formatting control")
	}
	if !bytes.Contains(body, []byte("projectNameInput")) {
		t.Fatalf("index did not include the project name field")
	}
	if !bytes.Contains(body, []byte("projectTemplateSelect")) {
		t.Fatalf("index did not include the project type selector")
	}
	if !bytes.Contains(body, []byte("dataValidateButton")) {
		t.Fatalf("index did not include the Data validation command slot")
	}
	if !bytes.Contains(body, []byte("serveButton")) {
		t.Fatalf("index did not include the Serve command slot")
	}
	if !bytes.Contains(body, []byte("startView")) {
		t.Fatalf("index did not include the Start workspace")
	}
	if !bytes.Contains(body, []byte("startRuntimeRows")) {
		t.Fatalf("index did not include Start runtime status rows")
	}
	if !bytes.Contains(body, []byte("startProjectTypeRows")) {
		t.Fatalf("index did not include Start project type rows")
	}
	if !bytes.Contains(body, []byte("startWorkspaceRows")) {
		t.Fatalf("index did not include Start workspace project rows")
	}
	if !bytes.Contains(body, []byte("startExampleRows")) {
		t.Fatalf("index did not include Start example project rows")
	}
	if !bytes.Contains(body, []byte("artifactsView")) {
		t.Fatalf("index did not include the Artifacts workspace")
	}
	if !bytes.Contains(body, []byte("artifactRows")) {
		t.Fatalf("index did not include artifact browser rows")
	}
	if !bytes.Contains(body, []byte("runComparisonRows")) {
		t.Fatalf("index did not include run comparison rows")
	}
	if !bytes.Contains(body, []byte("componentRunRows")) {
		t.Fatalf("index did not include selected component run values")
	}
	if !bytes.Contains(body, []byte("batchCaseRows")) {
		t.Fatalf("index did not include batch case rows")
	}
	if !bytes.Contains(body, []byte("executionTraceRows")) {
		t.Fatalf("index did not include execution trace rows")
	}
	if !bytes.Contains(body, []byte("componentLogRows")) {
		t.Fatalf("index did not include component log rows")
	}
	if !bytes.Contains(body, []byte("connectionTraceRows")) {
		t.Fatalf("index did not include connection trace rows")
	}
	if !bytes.Contains(body, []byte("nodeTraceRows")) {
		t.Fatalf("index did not include node trace rows")
	}
	if !bytes.Contains(body, []byte("autoLayoutButton")) {
		t.Fatalf("index did not include canvas auto layout control")
	}
}

func TestStaticModuleEntrypointServes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/app.js", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`from "./state.js"`)) {
		t.Fatalf("module entrypoint did not contain expected imports")
	}
	if !bytes.Contains(body, []byte(`from "./export-workspace.js"`)) {
		t.Fatalf("module entrypoint did not import export workspace renderer")
	}
	if !bytes.Contains(body, []byte("openProblem")) {
		t.Fatalf("module entrypoint did not include problem navigation")
	}
	if !bytes.Contains(body, []byte("applySourceSaveResponse")) {
		t.Fatalf("module entrypoint did not include source save response handling")
	}
	if !bytes.Contains(body, []byte("evaluateSnippet")) {
		t.Fatalf("module entrypoint did not include contract-aware evaluate snippets")
	}
	if !bytes.Contains(body, []byte("setProblems")) {
		t.Fatalf("module entrypoint did not include explicit problem state handling")
	}
	if !bytes.Contains(body, []byte("sourceIssueBlock")) {
		t.Fatalf("module entrypoint did not include source issue panel rendering")
	}
	if !bytes.Contains(body, []byte("handleSourceIndent")) {
		t.Fatalf("module entrypoint did not include source indentation handling")
	}
	if !bytes.Contains(body, []byte("showSourceCompletionPanel")) {
		t.Fatalf("module entrypoint did not include source completion handling")
	}
	if !bytes.Contains(body, []byte("sourceCompletionItems")) {
		t.Fatalf("module entrypoint did not include contract-derived source completion items")
	}
	if !bytes.Contains(body, []byte("bracketCheck")) {
		t.Fatalf("module entrypoint did not include bracket status checking")
	}
	if !bytes.Contains(body, []byte("highlightPython")) {
		t.Fatalf("module entrypoint did not include Python syntax highlighting")
	}
	if !bytes.Contains(body, []byte("handleSourceNewline")) {
		t.Fatalf("module entrypoint did not include source auto indentation")
	}
	if !bytes.Contains(body, []byte("formatCurrentSource")) {
		t.Fatalf("module entrypoint did not include source formatting")
	}
	if !bytes.Contains(body, []byte("formatPythonSource")) {
		t.Fatalf("module entrypoint did not include Python source formatting rules")
	}
	if !bytes.Contains(body, []byte("sourceLineProblemMap")) {
		t.Fatalf("module entrypoint did not include source gutter problem markers")
	}
	if !bytes.Contains(body, []byte("sourceQuickFixForProblem")) {
		t.Fatalf("module entrypoint did not include source quick fixes")
	}
	if !bytes.Contains(body, []byte("stepSnippet")) {
		t.Fatalf("module entrypoint did not include generated-wrapper step snippets")
	}
	if !bytes.Contains(body, []byte("Runtime Contract")) {
		t.Fatalf("module entrypoint did not render protected runtime contract context")
	}
	if !bytes.Contains(body, []byte("handleCanvasEndpointClick")) {
		t.Fatalf("module entrypoint did not include direct canvas endpoint connection handling")
	}
	if !bytes.Contains(body, []byte("pendingConnection")) {
		t.Fatalf("module entrypoint did not include pending canvas connection state")
	}
	if !bytes.Contains(body, []byte("latestCanvasNodeValue")) {
		t.Fatalf("module entrypoint did not include canvas node result rendering")
	}
	if !bytes.Contains(body, []byte("latestConnectionValue")) {
		t.Fatalf("module entrypoint did not include connection result rendering")
	}
	if !bytes.Contains(body, []byte("latestRuntimeResult")) {
		t.Fatalf("module entrypoint did not share latest runtime result state")
	}
	if !bytes.Contains(body, []byte("latestRuntimeComparisonContext")) {
		t.Fatalf("module entrypoint did not include runtime comparison baseline capture")
	}
	if !bytes.Contains(body, []byte("latestRunSource")) {
		t.Fatalf("module entrypoint did not preserve latest run source labels")
	}
	if !bytes.Contains(body, []byte("runComparisonBaseline")) {
		t.Fatalf("module entrypoint did not preserve run comparison baselines")
	}
	if !bytes.Contains(body, []byte("runTimeoutField")) {
		t.Fatalf("module entrypoint did not include run timeout control rendering")
	}
	if !bytes.Contains(body, []byte("timeout_ms")) {
		t.Fatalf("module entrypoint did not send run timeout requests")
	}
	if !bytes.Contains(body, []byte("runInputMeta")) {
		t.Fatalf("module entrypoint did not include run input metadata rendering")
	}
	if !bytes.Contains(body, []byte("resetRunInput")) {
		t.Fatalf("module entrypoint did not include run input reset handling")
	}
	if !bytes.Contains(body, []byte("scenarioNameField")) {
		t.Fatalf("module entrypoint did not include in-app scenario naming")
	}
	if !bytes.Contains(body, []byte("activeScenarioBadge")) {
		t.Fatalf("module entrypoint did not include active scenario state")
	}
	if !bytes.Contains(body, []byte("markRunInputsEdited")) {
		t.Fatalf("module entrypoint did not clear scenario state after input edits")
	}
	if bytes.Contains(body, []byte(`window.prompt("Scenario name"`)) {
		t.Fatalf("module entrypoint should not use a prompt for scenario creation")
	}
	if !bytes.Contains(body, []byte("selectedConnectionId")) {
		t.Fatalf("module entrypoint did not include canvas connection selection state")
	}
	if !bytes.Contains(body, []byte("saveRunSourceButton")) {
		t.Fatalf("module entrypoint did not include source save-and-run action binding")
	}
	if !bytes.Contains(body, []byte("openComponentCode")) {
		t.Fatalf("module entrypoint did not include component-to-code navigation")
	}
	if !bytes.Contains(body, []byte("componentTreeItem")) {
		t.Fatalf("module entrypoint did not include component tree state rendering")
	}
	if !bytes.Contains(body, []byte("includeComponentInSystem")) {
		t.Fatalf("module entrypoint did not include tree component system inclusion")
	}
	if !bytes.Contains(body, []byte("duplicateComponent")) {
		t.Fatalf("module entrypoint did not include component duplication action")
	}
	if !bytes.Contains(body, []byte("sourceRuntimeBlock")) {
		t.Fatalf("module entrypoint did not include code workspace runtime feedback")
	}
	if !bytes.Contains(body, []byte("sourceReferenceBlock")) {
		t.Fatalf("module entrypoint did not include source contract reference insertion")
	}
	if !bytes.Contains(body, []byte("sourceTreeItem")) {
		t.Fatalf("module entrypoint did not include source tree navigation")
	}
	if !bytes.Contains(body, []byte("insertSourceText")) {
		t.Fatalf("module entrypoint did not include shared source insertion")
	}
	if !bytes.Contains(body, []byte("loadExportRecord")) {
		t.Fatalf("module entrypoint did not include export record reopening")
	}
	if !bytes.Contains(body, []byte("exportReadyTreeItem")) {
		t.Fatalf("module entrypoint did not include export preview navigation")
	}
	if !bytes.Contains(body, []byte("/api/project/export")) {
		t.Fatalf("module entrypoint did not call export record endpoint")
	}
	if !bytes.Contains(body, []byte("latestResultStale")) {
		t.Fatalf("module entrypoint did not include stale run result state")
	}
	if !bytes.Contains(body, []byte("startCanvasNodeDrag")) {
		t.Fatalf("module entrypoint did not include canvas node dragging")
	}
	if !bytes.Contains(body, []byte("saveCanvasLayout")) {
		t.Fatalf("module entrypoint did not include canvas layout persistence")
	}
	if !bytes.Contains(body, []byte("autoLayoutPositions")) {
		t.Fatalf("module entrypoint did not include canvas auto layout")
	}
	if !bytes.Contains(body, []byte("resizeCanvasSurface")) {
		t.Fatalf("module entrypoint did not resize the canvas surface")
	}
	if !bytes.Contains(body, []byte("canvasParameterSummary")) {
		t.Fatalf("module entrypoint did not include readable canvas parameter rendering")
	}
	if !bytes.Contains(body, []byte("canvasNodeMeta")) {
		t.Fatalf("module entrypoint did not include readable canvas node metadata")
	}
	if !bytes.Contains(body, []byte("canvasNodeAnchorY")) {
		t.Fatalf("module entrypoint did not spread canvas connection anchors by node")
	}
	if !bytes.Contains(body, []byte("connectionMediumState")) {
		t.Fatalf("module entrypoint did not include canvas connection medium state markers")
	}
	if !bytes.Contains(body, []byte("connectionAnnotation")) {
		t.Fatalf("module entrypoint did not include canvas connection annotations")
	}
	if !bytes.Contains(body, []byte("node-medium")) {
		t.Fatalf("module entrypoint did not include canvas node medium badges")
	}
	if !bytes.Contains(body, []byte("medium-override")) {
		t.Fatalf("module entrypoint did not mark explicit canvas medium overrides")
	}
	if !bytes.Contains(body, []byte("long-path")) {
		t.Fatalf("module entrypoint did not mark long canvas connection paths")
	}
	if !bytes.Contains(body, []byte("backtracking")) {
		t.Fatalf("module entrypoint did not mark backtracking canvas connection paths")
	}
	if !bytes.Contains(body, []byte("parameterInspectorBlock")) {
		t.Fatalf("module entrypoint did not include inspector parameter editing")
	}
	if !bytes.Contains(body, []byte("parameterDefinitionBlock")) {
		t.Fatalf("module entrypoint did not include inspector parameter definition editing")
	}
	if !bytes.Contains(body, []byte("stateDefinitionBlock")) {
		t.Fatalf("module entrypoint did not include inspector state definition editing")
	}
	if !bytes.Contains(body, []byte("/api/project/component-contract")) {
		t.Fatalf("module entrypoint did not call component contract endpoint")
	}
	if !bytes.Contains(body, []byte("syncParameterInputs")) {
		t.Fatalf("module entrypoint did not include synchronized parameter input editing")
	}
	if !bytes.Contains(body, []byte("editableNodeRow")) {
		t.Fatalf("module entrypoint did not include editable node rows")
	}
	if !bytes.Contains(body, []byte("updateNodeFromInspector")) {
		t.Fatalf("module entrypoint did not include node metadata editing")
	}
	if !bytes.Contains(body, []byte("newNodeName")) {
		t.Fatalf("module entrypoint did not include detailed node creation fields")
	}
	if !bytes.Contains(body, []byte("/api/project/nodes/update")) {
		t.Fatalf("module entrypoint did not call node update endpoint")
	}
	if !bytes.Contains(body, []byte("CANVAS_NODE_WIDTH")) {
		t.Fatalf("module entrypoint did not include canvas sizing constants")
	}
	if !bytes.Contains(body, []byte("ensureEditableProject")) {
		t.Fatalf("module entrypoint did not create an editable first-run workspace")
	}
	if !bytes.Contains(body, []byte("/api/component-templates")) {
		t.Fatalf("module entrypoint did not load component templates")
	}
	if !bytes.Contains(body, []byte("defaultProjectName")) {
		t.Fatalf("module entrypoint did not include in-app project naming")
	}
	if !bytes.Contains(body, []byte("renderStartWorkspace")) {
		t.Fatalf("module entrypoint did not include the Start workspace renderer")
	}
	if !bytes.Contains(body, []byte("datasetTreeItems")) {
		t.Fatalf("module entrypoint did not include Dataset project tree objects")
	}
	if !bytes.Contains(body, []byte("validationMappingTreeItems")) {
		t.Fatalf("module entrypoint did not include Validation project tree objects")
	}
	if !bytes.Contains(body, []byte("parameterSetTreeItems")) {
		t.Fatalf("module entrypoint did not include Parameter Set project tree objects")
	}
	if !bytes.Contains(body, []byte("runDataValidation")) {
		t.Fatalf("module entrypoint did not include data validation action")
	}
	if !bytes.Contains(body, []byte("renderArtifactWorkspace")) {
		t.Fatalf("module entrypoint did not include artifact browser rendering")
	}
	if !bytes.Contains(body, []byte("openArtifactSummary")) {
		t.Fatalf("module entrypoint did not include artifact summary navigation")
	}
	if !bytes.Contains(body, []byte("/api/project/dataset")) {
		t.Fatalf("module entrypoint did not call dataset preview endpoint")
	}
	if !bytes.Contains(body, []byte("/api/project/parameter-set")) {
		t.Fatalf("module entrypoint did not call parameter-set detail endpoint")
	}
	if !bytes.Contains(body, []byte("/api/project/validation-mapping")) {
		t.Fatalf("module entrypoint did not create validation mappings from datasets")
	}
	if !bytes.Contains(body, []byte("/api/project/parameter-set/apply")) {
		t.Fatalf("module entrypoint did not apply parameter sets")
	}
	if !bytes.Contains(body, []byte("Raw JSON")) {
		t.Fatalf("module entrypoint did not keep raw workflow JSON available")
	}
	if !bytes.Contains(body, []byte("runCalibrationSetup")) {
		t.Fatalf("module entrypoint did not include calibration setup execution")
	}
	if !bytes.Contains(body, []byte("runOptimizationSetup")) {
		t.Fatalf("module entrypoint did not include optimization setup execution")
	}
	if !bytes.Contains(body, []byte("/api/calibration/run")) {
		t.Fatalf("module entrypoint did not call calibration run endpoint")
	}
	if !bytes.Contains(body, []byte("/api/optimization/run")) {
		t.Fatalf("module entrypoint did not call optimization run endpoint")
	}
	if !bytes.Contains(body, []byte("projectTemplateSelect")) {
		t.Fatalf("module entrypoint did not read the selected project type")
	}
	if !bytes.Contains(body, []byte("serveButton")) {
		t.Fatalf("module entrypoint did not reserve the Serve command state")
	}
	if bytes.Contains(body, []byte(`window.prompt("Project name"`)) || bytes.Contains(body, []byte(`window.prompt("Copy project as"`)) {
		t.Fatalf("module entrypoint should not use prompts for project creation or copy")
	}
	if bytes.Contains(body, []byte(`window.prompt("Component name"`)) {
		t.Fatalf("module entrypoint should not use a prompt for component creation")
	}
}

func TestComponentTemplatesEndpointListsManifests(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/component-templates", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Templates []ComponentTemplateSummary `json:"templates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Templates) == 0 {
		t.Fatal("expected at least one component template")
	}
	template := body.Templates[0]
	if template.ID != "scalar" || template.Name != "Scalar Component" || template.Kind != "user_python" {
		t.Fatalf("template summary = %#v", template)
	}
	if template.Category != "utility" || template.ExecutionMode != "step" {
		t.Fatalf("template authoring metadata = %#v", template)
	}
	if template.SourceLayout != "generated_wrapper" {
		t.Fatalf("template source layout = %s", template.SourceLayout)
	}
	if template.InputCount != 1 || template.OutputCount != 1 || template.ParameterCount != 1 {
		t.Fatalf("template counts = %#v", template)
	}
	if !hasComponentTemplate(body.Templates, "controller") ||
		!hasComponentTemplate(body.Templates, "stateful") ||
		!hasComponentTemplate(body.Templates, "data_source") ||
		!hasComponentTemplate(body.Templates, "data_sink") ||
		!hasComponentTemplate(body.Templates, "utility") ||
		!hasComponentTemplate(body.Templates, "external_executable") ||
		!hasComponentTemplate(body.Templates, "vectorized") {
		t.Fatalf("expected beta component templates, got %#v", body.Templates)
	}
}

func TestStaticExportWorkspaceModuleServes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/export-workspace.js", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("renderExportWorkspace")) {
		t.Fatalf("export workspace module did not contain renderer")
	}
	if !bytes.Contains(body, []byte("Interface schema")) {
		t.Fatalf("export workspace module did not render interface schema")
	}
	if !bytes.Contains(body, []byte("Export folder")) {
		t.Fatalf("export workspace module did not render export folder")
	}
	if !bytes.Contains(body, []byte("Commands")) {
		t.Fatalf("export workspace module did not render commands")
	}
	if !bytes.Contains(body, []byte("Records")) {
		t.Fatalf("export workspace module did not render record count")
	}
}

func TestStaticRunOutputModuleServes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/run-output.js", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("renderSelectedComponentValues")) {
		t.Fatalf("run output module did not render selected component values")
	}
	if !bytes.Contains(body, []byte("renderBatchCases")) {
		t.Fatalf("run output module did not render batch cases")
	}
	if !bytes.Contains(body, []byte("renderExecutionTrace")) {
		t.Fatalf("run output module did not render execution traces")
	}
	if !bytes.Contains(body, []byte("renderRunComparison")) {
		t.Fatalf("run output module did not render run comparisons")
	}
	if !bytes.Contains(body, []byte("renderComponentLogs")) {
		t.Fatalf("run output module did not render component logs")
	}
	if !bytes.Contains(body, []byte("renderConnectionTrace")) {
		t.Fatalf("run output module did not render connection traces")
	}
	if !bytes.Contains(body, []byte("renderNodeTrace")) {
		t.Fatalf("run output module did not render node traces")
	}
	if !bytes.Contains(body, []byte("case-status")) {
		t.Fatalf("run output module did not render batch case status badges")
	}
	if !bytes.Contains(body, []byte("timing-cell")) {
		t.Fatalf("run output module did not render component timing bars")
	}
	if !bytes.Contains(body, []byte("comparison-delta")) {
		t.Fatalf("run output module did not render comparison delta badges")
	}
	if !bytes.Contains(body, []byte("runComparisonBaseline")) {
		t.Fatalf("run output module did not read run comparison baselines")
	}
	if !bytes.Contains(body, []byte("log-severity")) {
		t.Fatalf("run output module did not render component log severity badges")
	}
	if !bytes.Contains(body, []byte("failureSummaryRows")) {
		t.Fatalf("run output module did not render failed run summaries")
	}
	if !bytes.Contains(body, []byte("batchCaseErrorSummary")) {
		t.Fatalf("run output module did not include batch failure problem summaries")
	}
	if !bytes.Contains(body, []byte("component_inputs")) {
		t.Fatalf("run output module did not read component input snapshots")
	}
	if !bytes.Contains(body, []byte("component_outputs")) {
		t.Fatalf("run output module did not read component output snapshots")
	}
	if !bytes.Contains(body, []byte("component_logs")) {
		t.Fatalf("run output module did not read component logs")
	}
}

func TestCreateProjectEndpointCreatesWorkspaceProject(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	payload := []byte(`{"name":"My First Project"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Project.Source != "workspace" {
		t.Fatalf("source = %s", body.Project.Source)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "my-first-project", "components", "scalar.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Load(body.Project.ProjectPath); err != nil {
		t.Fatal(err)
	}
}

func TestCopyProjectEndpointCreatesEditableWorkspaceCopy(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Seed Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	copyPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Copied Project",
	})
	if err != nil {
		t.Fatal(err)
	}
	copyResponse := httptest.NewRecorder()
	copyRequest := httptest.NewRequest(http.MethodPost, "/api/projects/copy", bytes.NewReader(copyPayload))
	server.Handler().ServeHTTP(copyResponse, copyRequest)
	if copyResponse.Code != http.StatusCreated {
		t.Fatalf("copy status = %d body=%s", copyResponse.Code, copyResponse.Body.String())
	}
	var copyBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(copyResponse.Body.Bytes(), &copyBody); err != nil {
		t.Fatal(err)
	}
	if copyBody.Project.Source != "workspace" {
		t.Fatalf("source = %s", copyBody.Project.Source)
	}
	if copyBody.Project.Name != "Copied Project" {
		t.Fatalf("name = %s", copyBody.Project.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "copied-project", "components", "scalar.py")); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(copyBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.ProjectName != "Copied Project" {
		t.Fatalf("project_name = %s", loaded.Project.ProjectName)
	}

	updatePayload, err := json.Marshal(map[string]any{
		"project_path": copyBody.Project.ProjectPath,
		"parameters": map[string]any{
			"scalar": map[string]any{"gain": 5.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(updatePayload))
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update copied project status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
}

func TestCreateComponentEndpointCreatesWorkspaceComponent(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Component Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
		"template":     "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(payload))

	server.Handler().ServeHTTP(componentResponse, componentRequest)

	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	var componentBody struct {
		Component model.Component `json:"component"`
	}
	if err := json.Unmarshal(componentResponse.Body.Bytes(), &componentBody); err != nil {
		t.Fatal(err)
	}
	if componentBody.Component.ID != "second_gain" {
		t.Fatalf("component id = %s, want second_gain", componentBody.Component.ID)
	}
	if componentBody.Component.Category != "utility" || componentBody.Component.ExecutionMode != "step" {
		t.Fatalf("component authoring metadata = %#v", componentBody.Component)
	}
	if componentBody.Component.Source.Layout != "generated_wrapper" ||
		componentBody.Component.Source.Metadata != "components/second_gain/component.json" ||
		componentBody.Component.Source.Step != "components/second_gain/user_step.py" ||
		componentBody.Component.Source.Wrapper != "components/second_gain/wrapper.py" {
		t.Fatalf("component source metadata = %#v", componentBody.Component.Source)
	}
	if componentBody.Component.Class != "components.second_gain.wrapper.SecondGainComponent" {
		t.Fatalf("component class = %s", componentBody.Component.Class)
	}
	if len(componentBody.Component.Nodes.Inputs) != 1 || componentBody.Component.Nodes.Inputs[0].Preset != "scalar_input" {
		t.Fatalf("input node metadata = %#v", componentBody.Component.Nodes.Inputs)
	}
	if len(componentBody.Component.Nodes.Outputs) != 1 || componentBody.Component.Nodes.Outputs[0].Preset != "scalar_output" {
		t.Fatalf("output node metadata = %#v", componentBody.Component.Nodes.Outputs)
	}
	gainDefinition, ok := componentBody.Component.ParameterDefinitions["gain"]
	if !ok {
		t.Fatalf("parameter definitions = %#v", componentBody.Component.ParameterDefinitions)
	}
	if gainDefinition.DisplayName != "Gain" || gainDefinition.Unit != "ratio" || gainDefinition.Role != "calibration_target" || gainDefinition.Group != "Model" {
		t.Fatalf("gain definition = %#v", gainDefinition)
	}
	if gainDefinition.Default != 2.0 || gainDefinition.Current != 2.0 {
		t.Fatalf("gain values = default %v current %v", gainDefinition.Default, gainDefinition.Current)
	}
	if gainDefinition.Bounds == nil || gainDefinition.Bounds.Min != 0.0 || gainDefinition.Bounds.Max != 100.0 {
		t.Fatalf("gain bounds = %#v", gainDefinition.Bounds)
	}
	sourcePath := filepath.Join(root, "projects", "component-project", "components", "second_gain", "user_step.py")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sourceBytes), `inputs["value"]`) {
		t.Fatalf("component source did not read the scalar input:\n%s", string(sourceBytes))
	}
	if !strings.Contains(string(sourceBytes), `params.get("gain", 2.0)`) {
		t.Fatalf("component source did not come from scalar template:\n%s", string(sourceBytes))
	}
	wrapperBytes, err := os.ReadFile(filepath.Join(root, "projects", "component-project", "components", "second_gain", "wrapper.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapperBytes), "class SecondGainComponent:") {
		t.Fatalf("component wrapper did not use generated class name:\n%s", string(wrapperBytes))
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "component-project", "components", "second_gain", "component.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted model.Component
	found := false
	for _, component := range loaded.Graph.Components {
		if component.ID == "second_gain" {
			persisted = component
			found = true
		}
	}
	if !found {
		t.Fatal("created component was not written to graph")
	}
	if persisted.Category != "utility" || persisted.ExecutionMode != "step" {
		t.Fatalf("persisted component authoring metadata = %#v", persisted)
	}
	if persisted.Source.Step != "components/second_gain/user_step.py" || persisted.Source.Wrapper != "components/second_gain/wrapper.py" {
		t.Fatalf("persisted source metadata = %#v", persisted.Source)
	}
	if _, ok := persisted.ParameterDefinitions["gain"]; !ok {
		t.Fatalf("persisted parameter definitions = %#v", persisted.ParameterDefinitions)
	}
	if got := componentBody.Component.Parameters["gain"]; got != 2.0 {
		t.Fatalf("created gain = %v, want 2", got)
	}
	for _, componentID := range loaded.Graph.Systems[0].Components {
		if componentID == "second_gain" {
			t.Fatal("new component should not be added to the runnable system yet")
		}
	}
}

func TestDuplicateComponentEndpointCopiesGraphAndSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Duplicate Component Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path":        createBody.Project.ProjectPath,
		"source_component_id": "scalar",
		"name":                "Scalar Copy",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/duplicate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("duplicate status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Component model.Component `json:"component"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Component.ID != "scalar_copy" {
		t.Fatalf("component id = %s, want scalar_copy", body.Component.ID)
	}
	if body.Component.Class != "components.scalar_copy.ScalarComponent" {
		t.Fatalf("component class = %s", body.Component.Class)
	}
	if got := body.Component.Parameters["gain"]; got != 2.0 {
		t.Fatalf("copied gain = %v, want 2", got)
	}
	sourcePath := filepath.Join(root, "projects", "duplicate-component-project", "components", "scalar_copy.py")
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scalar_copy"); !found {
		t.Fatal("duplicated component was not written to graph")
	}
	if containsString(loaded.Graph.Systems[0].Components, "scalar_copy") {
		t.Fatal("duplicated component should not be added to the runnable system yet")
	}
}

func TestUpdateComponentEndpointRenamesWorkspaceComponent(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Rename Component Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"name":         "Outdoor Air Signal",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/update", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Graph.Components[0].Name != "Outdoor Air Signal" {
		t.Fatalf("component name = %s", loaded.Graph.Components[0].Name)
	}
}

func TestCreateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"name": "Example Edit"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDuplicateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"source_component_id": "scalar",
		"name": "Example Copy"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/duplicate", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"name": "Example Rename"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/update", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteComponentEndpointRemovesUnusedWorkspaceComponent(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Delete Component Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Scratch Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	sourcePath := filepath.Join(root, "projects", "delete-component-project", "components", "scratch_gain")
	if info, err := os.Stat(sourcePath); err != nil || !info.IsDir() {
		t.Fatal(err)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scratch_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scratch_gain"); found {
		t.Fatal("deleted component should be removed from graph")
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("component source should be removed, stat err=%v", err)
	}
}

func TestDeleteComponentEndpointRejectsSystemComponent(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Reject Delete Component Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateNodeEndpointAddsPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Node Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	inputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "input",
		"id":           "bias",
		"value_type":   "float",
		"default":      4.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	inputResponse := httptest.NewRecorder()
	inputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(inputPayload))
	server.Handler().ServeHTTP(inputResponse, inputRequest)
	if inputResponse.Code != http.StatusCreated {
		t.Fatalf("input node status = %d body=%s", inputResponse.Code, inputResponse.Body.String())
	}

	outputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "output",
		"id":           "adjusted",
		"value_type":   "float",
	})
	if err != nil {
		t.Fatal(err)
	}
	outputResponse := httptest.NewRecorder()
	outputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(outputPayload))
	server.Handler().ServeHTTP(outputResponse, outputRequest)
	if outputResponse.Code != http.StatusCreated {
		t.Fatalf("output node status = %d body=%s", outputResponse.Code, outputResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component := loaded.Graph.Components[0]
	if !componentHasNode(component, "bias") {
		t.Fatal("input node was not written to graph")
	}
	if !componentHasNode(component, "adjusted") {
		t.Fatal("output node was not written to graph")
	}
	foundPublicInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "scalar_bias" {
			foundPublicInput = true
			break
		}
	}
	if !foundPublicInput {
		t.Fatal("input node was not exposed as public input")
	}
	foundPublicOutput := false
	for _, output := range loaded.Graph.Systems[0].PublicOutputs {
		if output.ID == "scalar_adjusted" {
			foundPublicOutput = true
			break
		}
	}
	if !foundPublicOutput {
		t.Fatal("output node was not exposed as public output")
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["scalar_bias"]; got != 4.0 {
		t.Fatalf("scalar_bias default = %v, want 4", got)
	}
}

func TestUpdateNodeEndpointUpdatesPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Update Node Project")
	required := false
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"node_id":      "value",
		"name":         "Room temperature",
		"medium":       "air",
		"value_type":   "int",
		"unit":         "C",
		"required":     required,
		"default":      21,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/update", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(projectSummary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	node, found := findInputNode(loaded.Graph.Components[0], "value")
	if !found {
		t.Fatal("value node not found")
	}
	if node.Name != "Room temperature" || node.Medium != "air" || node.ValueType != "int" || node.Unit != "C" {
		t.Fatalf("node metadata = %#v", node)
	}
	if node.Required == nil || *node.Required {
		t.Fatalf("node required = %#v, want false", node.Required)
	}
	if got := node.Default; got != float64(21) {
		t.Fatalf("node default = %#v, want 21", got)
	}
	publicInput := loaded.Graph.Systems[0].PublicInputs[0]
	if publicInput.Name != node.Name || publicInput.Medium != node.Medium || publicInput.ValueType != node.ValueType || publicInput.Unit != node.Unit {
		t.Fatalf("public input metadata = %#v, want node metadata", publicInput)
	}
	if publicInput.Required == nil || *publicInput.Required {
		t.Fatalf("public input required = %#v, want false", publicInput.Required)
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["value"]; got != float64(21) {
		t.Fatalf("value default input = %#v, want 21", got)
	}
}

func TestDeleteNodeEndpointCleansPublicIOAndConnections(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Delete Node Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	nodePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "input",
		"id":           "bias",
		"default":      4.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	nodeResponse := httptest.NewRecorder()
	nodeRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(nodePayload))
	server.Handler().ServeHTTP(nodeResponse, nodeRequest)
	if nodeResponse.Code != http.StatusCreated {
		t.Fatalf("node status = %d body=%s", nodeResponse.Code, nodeResponse.Body.String())
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"node_id":      "bias",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete input node status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if componentHasNode(loaded.Graph.Components[0], "bias") {
		t.Fatal("deleted input node should be removed from component")
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "scalar_bias" {
			t.Fatal("deleted input node public input should be removed")
		}
	}
	runInput, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := runInput.Inputs["scalar_bias"]; exists {
		t.Fatal("deleted input node default input should be removed")
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}
	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}
	deleteOutputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"node_id":      "result",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteOutputResponse := httptest.NewRecorder()
	deleteOutputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(deleteOutputPayload))
	server.Handler().ServeHTTP(deleteOutputResponse, deleteOutputRequest)
	if deleteOutputResponse.Code != http.StatusOK {
		t.Fatalf("delete output node status = %d body=%s", deleteOutputResponse.Code, deleteOutputResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if componentHasNode(loaded.Graph.Components[0], "result") {
		t.Fatal("deleted output node should be removed from component")
	}
	if len(loaded.Graph.Connections) != 0 || len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("connections after output delete = graph:%d system:%d", len(loaded.Graph.Connections), len(loaded.Graph.Systems[0].Connections))
	}
	foundRestoredInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			foundRestoredInput = true
		}
	}
	if !foundRestoredInput {
		t.Fatal("target input should be restored as public after source output node deletion")
	}
}

func TestCreateNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"direction": "input",
		"id": "bias"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"node_id": "value"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"node_id": "value",
		"name": "Edited"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/update", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestIncludeComponentEndpointAddsPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"System Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}

	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(loaded.Graph.Systems[0].Components, "second_gain") {
		t.Fatal("component was not added to the entry system")
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; !exists {
		t.Fatal("default input was not extended with second_gain_value")
	}

	runPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if _, exists := runBody.Result.Outputs["second_gain_result"]; !exists {
		t.Fatal("run output did not include second_gain_result")
	}
}

func TestIncludeComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateConnectionEndpointConnectsComponents(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Connection Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}
	var connectionBody struct {
		Connection model.Connection `json:"connection"`
	}
	if err := json.Unmarshal(connectionResponse.Body.Bytes(), &connectionBody); err != nil {
		t.Fatal(err)
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Connections) != 1 {
		t.Fatalf("connection count = %d", len(loaded.Graph.Connections))
	}
	if len(loaded.Graph.Systems[0].Connections) != 1 {
		t.Fatalf("system connection count = %d", len(loaded.Graph.Systems[0].Connections))
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			t.Fatal("connected target input should no longer be public")
		}
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; exists {
		t.Fatal("connected target default input should be removed")
	}

	runResponse := httptest.NewRecorder()
	runPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if runBody.Result.Outputs["second_gain_result"] != 16 {
		t.Fatalf("second_gain_result = %v, want 16", runBody.Result.Outputs["second_gain_result"])
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path":  createBody.Project.ProjectPath,
		"connection_id": connectionBody.Connection.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete connection status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Connections) != 0 {
		t.Fatalf("connection count after delete = %d", len(loaded.Graph.Connections))
	}
	if len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("system connection count after delete = %d", len(loaded.Graph.Systems[0].Connections))
	}
	foundPublicInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			foundPublicInput = true
			break
		}
	}
	if !foundPublicInput {
		t.Fatal("deleted connection target input should become public again")
	}
	input, err = runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; !exists {
		t.Fatal("deleted connection target default input should be restored")
	}
}

func TestValidateEndpointReportsConnectionMediumWarnings(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	summary := createWorkspaceProject(t, server, "Medium Warning Project")

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": summary.ProjectPath,
		"name":         "Water Sink",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}

	includePayload, err := json.Marshal(map[string]any{
		"project_path": summary.ProjectPath,
		"component_id": "water_sink",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	loaded, err := project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == "water_sink" {
			loaded.Graph.Components[index].Nodes.Inputs[0].Medium = "water"
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   summary.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "water_sink",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}

	validatePayload, err := json.Marshal(map[string]any{"project_path": summary.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	validateResponse := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(validateResponse, validateRequest)
	if validateResponse.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateResponse.Code, validateResponse.Body.String())
	}
	var body struct {
		Validation struct {
			Problems []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(validateResponse.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Validation.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Validation.Problems)
	}
	problem := body.Validation.Problems[0]
	if problem.Severity != "warning" || problem.ComponentID != "water_sink" || problem.NodeID != "value" {
		t.Fatalf("problem = %#v", problem)
	}
	if !strings.Contains(problem.Message, "connection scalar_result_to_water_sink_value medium mismatch") {
		t.Fatalf("problem message = %s", problem.Message)
	}
}

func TestWorkspaceWorkflowEditsSourceConnectsAndRuns(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Workflow Project")
	source := strings.TrimLeft(`
class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        return {"result": value * 3.0}, state
`, "\n")

	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"component_id": "scalar",
		"content":      source,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceResponse := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(sourcePayload))
	server.Handler().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); err != nil {
		t.Fatal(err)
	}
	if hasErrorProblems(sourceBody.Check.Problems) {
		t.Fatalf("source check errors = %#v", sourceBody.Check.Problems)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"name":         "Second Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}

	includePayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}

	runPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if got := runBody.Result.Outputs["second_gain_result"]; got != 24.0 {
		t.Fatalf("second_gain_result = %v, want 24", got)
	}
	if got := runBody.Result.ComponentOutputs["scalar"]["result"]; got != 12.0 {
		t.Fatalf("scalar result = %v, want 12", got)
	}
	if got := runBody.Result.ComponentInputs["second_gain"]["value"]; got != 12.0 {
		t.Fatalf("second_gain value input = %v, want 12", got)
	}
}

func TestRemoveComponentFromSystemEndpointCleansRuntimeSurface(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Removal Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}
	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}

	removePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	removeResponse := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components/remove", bytes.NewReader(removePayload))
	server.Handler().ServeHTTP(removeResponse, removeRequest)
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("remove status = %d body=%s", removeResponse.Code, removeResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !componentHasNode(loaded.Graph.Components[1], "value") {
		t.Fatal("removed system component artifact should remain in graph")
	}
	if containsString(loaded.Graph.Systems[0].Components, "second_gain") {
		t.Fatal("removed component should not remain in system")
	}
	if len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("system connection count = %d, want 0", len(loaded.Graph.Systems[0].Connections))
	}
	if len(loaded.Graph.Connections) != 0 {
		t.Fatalf("graph connection count = %d, want 0", len(loaded.Graph.Connections))
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.Component == "second_gain" {
			t.Fatal("removed component public input should be removed")
		}
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; exists {
		t.Fatal("removed component default input should be removed")
	}
}

func TestCreateConnectionEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"from_component": "load_model",
		"from_node": "adjusted_load_kw",
		"to_component": "controller",
		"to_node": "cooling_load_kw"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRemoveComponentFromSystemEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"component_id": "chiller"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/system/components/remove", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteConnectionEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"connection_id": "load_to_controller"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/connections/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateParametersEndpointWritesWorkspaceGraph(t *testing.T) {
	_, server := newIsolatedTestServer(t)

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Editable Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"parameters": map[string]any{
			"scalar": map[string]any{"gain": 3.0, "offset": 2.0, "scratch": 7.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(payload))

	server.Handler().ServeHTTP(updateResponse, updateRequest)

	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Graph.Components[0].Parameters["gain"]; got != 3.0 {
		t.Fatalf("gain = %v, want 3", got)
	}
	if got := loaded.Graph.Components[0].Parameters["offset"]; got != 2.0 {
		t.Fatalf("offset = %v, want 2", got)
	}
	if got := loaded.Graph.Components[0].Parameters["scratch"]; got != 7.0 {
		t.Fatalf("scratch = %v, want 7", got)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"name":         "scratch",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters/delete", bytes.NewReader(deletePayload))

	server.Handler().ServeHTTP(deleteResponse, deleteRequest)

	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := loaded.Graph.Components[0].Parameters["scratch"]; exists {
		t.Fatal("deleted parameter should be removed from component")
	}
	if got := loaded.Graph.Components[0].Parameters["gain"]; got != 3.0 {
		t.Fatalf("gain after delete = %v, want 3", got)
	}
	if got := loaded.Graph.Components[0].Parameters["offset"]; got != 2.0 {
		t.Fatalf("offset after delete = %v, want 2", got)
	}
}

func TestUpdateComponentContractEndpointWritesDefinitionsAndMetadata(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Contract Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
		"template":     "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}

	visible := false
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
		"parameters": map[string]any{
			"gain": 4.5,
		},
		"parameter_defs": map[string]model.ParameterDefinition{
			"gain": {
				DisplayName: "Test Gain",
				Unit:        "ratio",
				Default:     2.0,
				Current:     4.5,
				Bounds:      &model.ValueBounds{Min: 0.5, Max: 10.0},
				Role:        "optimization_variable",
				Group:       "Tuning",
				Description: "Edited from Studio",
				Visible:     &visible,
			},
		},
		"state_defs": map[string]model.StateDefinition{
			"accumulator": {
				DisplayName: "Accumulator",
				Unit:        "count",
				Initial:     1.0,
				Description: "Editable state",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("contract status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, found := findComponent(loaded.Graph, "second_gain")
	if !found {
		t.Fatal("second_gain missing")
	}
	if got := component.Parameters["gain"]; got != 4.5 {
		t.Fatalf("gain = %v, want 4.5", got)
	}
	gainDefinition := component.ParameterDefinitions["gain"]
	if gainDefinition.DisplayName != "Test Gain" || gainDefinition.Role != "optimization_variable" || gainDefinition.Group != "Tuning" {
		t.Fatalf("gain definition = %#v", gainDefinition)
	}
	if gainDefinition.Visible == nil || *gainDefinition.Visible {
		t.Fatalf("gain visible = %#v, want false", gainDefinition.Visible)
	}
	if gainDefinition.Bounds == nil || gainDefinition.Bounds.Min != 0.5 || gainDefinition.Bounds.Max != 10.0 {
		t.Fatalf("gain bounds = %#v", gainDefinition.Bounds)
	}
	if component.StateDefinitions["accumulator"].Initial != 1.0 {
		t.Fatalf("state definitions = %#v", component.StateDefinitions)
	}

	metadataPath := filepath.Join(root, "projects", "contract-project", "components", "second_gain", "component.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Parameters           map[string]any                       `json:"parameters"`
		ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs"`
		StateDefinitions     map[string]model.StateDefinition     `json:"state_defs"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Parameters["gain"] != 4.5 {
		t.Fatalf("metadata gain = %v, want 4.5", metadata.Parameters["gain"])
	}
	if metadata.ParameterDefinitions["gain"].Visible == nil || *metadata.ParameterDefinitions["gain"].Visible {
		t.Fatalf("metadata visible = %#v", metadata.ParameterDefinitions["gain"].Visible)
	}
	if metadata.StateDefinitions["accumulator"].Unit != "count" {
		t.Fatalf("metadata state definitions = %#v", metadata.StateDefinitions)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path":          createBody.Project.ProjectPath,
		"component_id":          "second_gain",
		"delete_state_defs":     []string{"accumulator"},
		"delete_parameter_defs": []string{"gain"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete contract status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, _ = findComponent(loaded.Graph, "second_gain")
	if _, exists := component.StateDefinitions["accumulator"]; exists {
		t.Fatalf("state definition should be deleted: %#v", component.StateDefinitions)
	}
	if _, exists := component.ParameterDefinitions["gain"]; exists {
		t.Fatalf("parameter definition should be cleared: %#v", component.ParameterDefinitions)
	}
}

func TestUpdateParametersEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"parameters": {
			"scalar": {"gain": 3}
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteParameterEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"name": "gain"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameters/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectEndpointIncludesDefaultRunInput(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/project?project_path=examples/001_scalar_component/project.bcsproj", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Project.DefaultRunInput == nil {
		t.Fatal("default_run_input is nil")
	}
	if got := body.Project.DefaultRunInput.Inputs["value"]; got != 4.0 {
		t.Fatalf("default value = %v, want 4", got)
	}
}

func TestProjectEndpointIncludesDatasetAndParameterSetSummaries(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/project?project_path=examples/005_chiller_plant_like_system/project.bcsproj", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Project.Datasets) != 1 {
		t.Fatalf("dataset count = %d", len(body.Project.Datasets))
	}
	dataset := body.Project.Datasets[0]
	if dataset.ID != "plant_validation" || dataset.RowCount != 3 || dataset.ColumnCount != 6 {
		t.Fatalf("dataset summary = %#v", dataset)
	}
	if len(body.Project.ParameterSets) != 2 {
		t.Fatalf("parameter set count = %d", len(body.Project.ParameterSets))
	}
	if body.Project.ParameterSets[0].ParameterCount == 0 {
		t.Fatalf("parameter set summary = %#v", body.Project.ParameterSets[0])
	}
	if len(body.Project.ValidationMappings) != 1 {
		t.Fatalf("validation mapping count = %d", len(body.Project.ValidationMappings))
	}
	mapping := body.Project.ValidationMappings[0]
	if mapping.ID != "plant_validation" || mapping.InputCount != 3 || mapping.OutputCount != 2 {
		t.Fatalf("validation mapping summary = %#v", mapping)
	}
	if len(body.Project.CalibrationSetups) != 1 {
		t.Fatalf("calibration setup count = %d", len(body.Project.CalibrationSetups))
	}
	if body.Project.CalibrationSetups[0].ID != "chiller_cop_grid" || body.Project.CalibrationSetups[0].ParameterCount != 1 {
		t.Fatalf("calibration setup summary = %#v", body.Project.CalibrationSetups[0])
	}
}

func TestDataValidationEndpointRunsMapping(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/005_chiller_plant_like_system/project.bcsproj",
		"mapping_path": "validation/mappings/plant_validation.json",
		"parameter_set_path": "parameter_sets/high_efficiency.json",
		"high_error_rows": 1
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validation/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		OK               bool `json:"ok"`
		ValidationResult struct {
			RowCount     int    `json:"row_count"`
			ParameterSet string `json:"parameter_set"`
			Metrics      map[string]struct {
				Count         int `json:"count"`
				HighErrorRows []struct {
					RowIndex int `json:"row_index"`
				} `json:"high_error_rows"`
			} `json:"metrics"`
		} `json:"validation_result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.ValidationResult.RowCount != 3 {
		t.Fatalf("validation response = %#v", body)
	}
	if body.ValidationResult.ParameterSet != "parameter_sets/high_efficiency.json" {
		t.Fatalf("parameter_set = %q", body.ValidationResult.ParameterSet)
	}
	if body.ValidationResult.Metrics["total_power_kw"].Count != 3 || len(body.ValidationResult.Metrics["total_power_kw"].HighErrorRows) != 1 {
		t.Fatalf("validation metrics = %#v", body.ValidationResult.Metrics)
	}
}

func TestDatasetPreviewEndpointSuggestsPublicIOMapping(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/dataset?project_path="+url.QueryEscape("examples/005_chiller_plant_like_system/project.bcsproj")+"&path="+url.QueryEscape("datasets/plant_validation.csv"),
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Dataset DatasetPreview `json:"dataset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Dataset.Summary.RowCount != 3 || len(body.Dataset.Columns) != 6 || len(body.Dataset.PreviewRows) == 0 {
		t.Fatalf("dataset preview = %#v", body.Dataset)
	}
	if !hasColumnSuggestion(body.Dataset.SuggestedInputs, "building_load_kw", "building_load_kw") {
		t.Fatalf("input suggestions = %#v", body.Dataset.SuggestedInputs)
	}
	if !hasColumnSuggestion(body.Dataset.SuggestedOutputs, "total_power_kw", "measured_total_power_kw") {
		t.Fatalf("output suggestions = %#v", body.Dataset.SuggestedOutputs)
	}
}

func TestParameterSetDetailEndpointReturnsDiffs(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/parameter-set?project_path="+url.QueryEscape("examples/005_chiller_plant_like_system/project.bcsproj")+"&path="+url.QueryEscape("parameter_sets/high_efficiency.json"),
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ParameterSet ParameterSetDetail `json:"parameter_set"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ParameterSet.Summary.ID != "high_efficiency" || len(body.ParameterSet.Differences) == 0 {
		t.Fatalf("parameter set detail = %#v", body.ParameterSet)
	}
	if !hasParameterDiff(body.ParameterSet.Differences, "chiller", "cop") {
		t.Fatalf("parameter diffs = %#v", body.ParameterSet.Differences)
	}
}

func TestCreateValidationMappingEndpointWritesSuggestedMapping(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "mapping-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path":         projectPath,
		"dataset_path":         filepath.Join("datasets", "plant_validation.csv"),
		"id":                   "suggested_validation",
		"missing_value_policy": "fail_fast",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ValidationMappingSummary `json:"summary"`
		Mapping struct {
			InputColumns          map[string]string `json:"input_columns"`
			ObservedOutputColumns map[string]string `json:"observed_output_columns"`
			MissingValuePolicy    string            `json:"missing_value_policy"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "validation/mappings/suggested_validation.json" || body.Summary.MissingValuePolicy != "fail_fast" {
		t.Fatalf("summary = %#v", body.Summary)
	}
	if body.Mapping.InputColumns["building_load_kw"] != "building_load_kw" {
		t.Fatalf("input columns = %#v", body.Mapping.InputColumns)
	}
	if body.Mapping.ObservedOutputColumns["total_power_kw"] != "measured_total_power_kw" {
		t.Fatalf("output columns = %#v", body.Mapping.ObservedOutputColumns)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "validation", "mappings", "suggested_validation.json")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyParameterSetEndpointPersistsGraphParameters(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "parameter-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"path":         filepath.Join("parameter_sets", "high_efficiency.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameter-set/apply", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, ok := findComponent(loaded.Graph, "chiller")
	if !ok {
		t.Fatal("chiller component not found")
	}
	if component.Parameters["cop"] != float64(6.8) {
		t.Fatalf("chiller cop = %#v", component.Parameters["cop"])
	}
}

func TestDataValidationEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "plant-validation")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path":       projectPath,
		"mapping_path":       filepath.Join("validation", "mappings", "plant_validation.json"),
		"parameter_set_path": filepath.Join("parameter_sets", "high_efficiency.json"),
		"high_error_rows":    1,
		"save":               true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validation/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ValidationResult struct {
			SavedRecord string `json:"saved_record"`
		} `json:"validation_result"`
		ValidationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
			RowCount     int    `json:"row_count"`
		} `json:"validation_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ValidationRecord.ID == "" || body.ValidationRecord.RowCount != 3 {
		t.Fatalf("validation record = %#v", body.ValidationRecord)
	}
	if body.ValidationResult.SavedRecord != body.ValidationRecord.RelativePath {
		t.Fatalf("saved record = %q, summary path = %q", body.ValidationResult.SavedRecord, body.ValidationRecord.RelativePath)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.ValidationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}

	detailResponse := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/project?project_path="+url.QueryEscape(projectPath), nil)
	server.Handler().ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detailBody struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailBody); err != nil {
		t.Fatal(err)
	}
	if len(detailBody.Project.ValidationRuns) != 1 || detailBody.Project.ValidationRuns[0].ID != body.ValidationRecord.ID {
		t.Fatalf("validation run summaries = %#v", detailBody.Project.ValidationRuns)
	}

	openResponse := httptest.NewRecorder()
	openRequest := httptest.NewRequest(http.MethodGet, "/api/project/validation-record?project_path="+url.QueryEscape(projectPath)+"&record_id="+url.QueryEscape(body.ValidationRecord.ID), nil)
	server.Handler().ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK {
		t.Fatalf("open status = %d body=%s", openResponse.Code, openResponse.Body.String())
	}
	var openBody struct {
		ValidationRecord struct {
			ID     string `json:"id"`
			Result struct {
				RowCount int `json:"row_count"`
			} `json:"result"`
		} `json:"validation_record"`
	}
	if err := json.Unmarshal(openResponse.Body.Bytes(), &openBody); err != nil {
		t.Fatal(err)
	}
	if openBody.ValidationRecord.ID != body.ValidationRecord.ID || openBody.ValidationRecord.Result.RowCount != 3 {
		t.Fatalf("opened record = %#v", openBody.ValidationRecord)
	}
}

func TestCalibrationRunEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "calibration-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"setup_path":   filepath.Join("calibration", "setups", "chiller_cop_grid.json"),
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/calibration/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CalibrationResult struct {
			OK                bool   `json:"ok"`
			SavedParameterSet string `json:"saved_parameter_set"`
			SavedRecord       string `json:"saved_record"`
		} `json:"calibration_result"`
		CalibrationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
		} `json:"calibration_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.CalibrationResult.OK || body.CalibrationResult.SavedParameterSet != "parameter_sets/chiller_cop_grid_calibrated.json" {
		t.Fatalf("calibration result = %#v", body.CalibrationResult)
	}
	if body.CalibrationRecord.ID == "" || body.CalibrationResult.SavedRecord != body.CalibrationRecord.RelativePath {
		t.Fatalf("calibration record = %#v result=%#v", body.CalibrationRecord, body.CalibrationResult)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "parameter_sets", "chiller_cop_grid_calibrated.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.CalibrationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestOptimizationRunEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "optimization-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "006_optimization_case"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"setup_path":   filepath.Join("optimization", "setups", "chw_setpoint_grid.json"),
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/optimization/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		OptimizationResult struct {
			OK            bool   `json:"ok"`
			SavedScenario string `json:"saved_scenario"`
			SavedRecord   string `json:"saved_record"`
		} `json:"optimization_result"`
		OptimizationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
		} `json:"optimization_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OptimizationResult.OK || body.OptimizationResult.SavedScenario != "scenarios/chw_setpoint_grid_optimized.json" {
		t.Fatalf("optimization result = %#v", body.OptimizationResult)
	}
	if body.OptimizationRecord.ID == "" || body.OptimizationResult.SavedRecord != body.OptimizationRecord.RelativePath {
		t.Fatalf("optimization record = %#v result=%#v", body.OptimizationRecord, body.OptimizationResult)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "scenarios", "chw_setpoint_grid_optimized.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.OptimizationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateLayoutEndpointWritesWorkspaceLayout(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Layout Project")
	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"components": map[string]CanvasPosition{
			"scalar":  {X: 132, Y: 96},
			"missing": {X: 10, Y: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/layout", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Project.Layout.Components["scalar"]; got.X != 132 || got.Y != 96 {
		t.Fatalf("layout position = %#v, want 132,96", got)
	}
	if _, exists := body.Project.Layout.Components["missing"]; exists {
		t.Fatal("layout should ignore unknown components")
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "layout-project", "studio", "layout.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateInputEndpointWritesWorkspaceDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Input Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{
		"project_path": "` + filepath.ToSlash(createBody.Project.ProjectPath) + `",
		"inputs": {"value": 7},
		"context": {"time": 0, "dt": 30}
	}`)
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/input", bytes.NewReader(payload))
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["value"]; got != 7.0 {
		t.Fatalf("input value = %v, want 7", got)
	}
}

func TestUpdateInputEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"inputs": {"value": 7}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/input", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRunEndpointRunsFeedForwardExample(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"inputs": {
			"building_load_kw": 500,
			"base_chw_setpoint_c": 7
		},
		"context": {
			"time": 0,
			"dt": 60
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.Outputs["total_power_kw"] != 122 {
		t.Fatalf("total_power_kw = %v", body.Result.Outputs["total_power_kw"])
	}
}

func TestRunEndpointCapturesComponentLogs(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Noisy Run Project")
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	source := strings.TrimLeft(`
import sys

class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        print("stdout from scalar")
        print("stderr from scalar", file=sys.stderr)
        value = float(inputs["value"])
        gain = float(params.get("gain", 2.0))
        return {"result": value * gain}, state
`, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Result.Outputs["result"]; got != 8.0 {
		t.Fatalf("result = %v, want 8", got)
	}
	if !hasComponentLog(body.Result.ComponentLogs, "scalar", "evaluate", "info", "stdout from scalar") {
		t.Fatalf("stdout log missing from %#v", body.Result.ComponentLogs)
	}
	if !hasComponentLog(body.Result.ComponentLogs, "scalar", "evaluate", "error", "stderr from scalar") {
		t.Fatalf("stderr log missing from %#v", body.Result.ComponentLogs)
	}
}

func TestRunEndpointHonorsTimeout(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Slow Run Project")
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	source := strings.TrimLeft(`
import time

class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        time.sleep(2)
        return {"result": float(inputs["value"])}, state
`, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
		"timeout_ms":   200,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Kind != "runtime" || !strings.Contains(body.Message, "run timed out after 200ms") {
		t.Fatalf("timeout body = %#v", body)
	}
}

func TestRunEndpointAppliesParameterSet(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/005_chiller_plant_like_system/project.bcsproj",
		"parameter_set_path": "parameter_sets/high_efficiency.json",
		"inputs": {
			"building_load_kw": 600,
			"base_chw_setpoint_c": 7,
			"condenser_entering_temp_c": 32
		},
		"context": {
			"time": 0,
			"dt": 60
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result struct {
			ParameterSet string             `json:"parameter_set"`
			Outputs      map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.ParameterSet != "parameter_sets/high_efficiency.json" {
		t.Fatalf("parameter_set = %q", body.Result.ParameterSet)
	}
	if body.Result.Outputs["total_power_kw"] == 140.96 {
		t.Fatalf("parameter set did not change total_power_kw")
	}
}

func TestRunEndpointReturnsComponentLinkedRuntimeProblem(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Run Problem Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "projects", "run-problem-project", "components", "scalar.py")
	source := "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"result\": 1, \"debug\": 2}, state\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"inputs":       map[string]any{"value": 5},
		"context":      map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Schema != "hvac-studio.error.v1" || body.Error.Kind != "python_worker" {
		t.Fatalf("error payload = %#v", body.Error)
	}
	if len(body.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
	if !strings.Contains(body.Problems[0].Message, "returned undeclared output node: debug") {
		t.Fatalf("problem = %#v", body.Problems[0])
	}
}

func TestRunEndpointMapsPythonTracebackToSourceLine(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Traceback Line Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "projects", "traceback-line-project", "components", "scalar.py")
	source := "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        scale = 1 / 0\n        return {\"result\": scale}, state\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"inputs":       map[string]any{"value": 5},
		"context":      map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	problem := body.Problems[0]
	if problem.ComponentID != "scalar" || problem.Source != "components/scalar.py" || problem.Line != 3 {
		t.Fatalf("problem location = %#v", problem)
	}
	if len(body.Error.Problems) != 1 || body.Error.Problems[0].Source != "components/scalar.py" || body.Error.Problems[0].Line != 3 {
		t.Fatalf("structured error problems = %#v", body.Error.Problems)
	}
	if !strings.Contains(problem.Message, "ZeroDivisionError") {
		t.Fatalf("problem message = %s", problem.Message)
	}
}

func TestRunEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Run Source Gate Project")
	writeBrokenScalarSource(t, project)

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !hasProblemMessage(body.Problems, "evaluate method is missing") {
		t.Fatalf("source problem missing from %#v", body.Problems)
	}
}

func TestValidateEndpointReturnsLinkedProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Invalid Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Graph.Systems[0].PublicInputs[0].Node = "missing"
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) != 1 {
		t.Fatalf("problem count = %d", len(body.Problems))
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
}

func TestValidateEndpointIncludesSourceChecks(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Validate Source Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Validation struct {
			SourceChecks int       `json:"source_checks"`
			Problems     []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Validation.SourceChecks != 1 {
		t.Fatalf("source checks = %d, want 1", body.Validation.SourceChecks)
	}
	if hasErrorProblems(body.Validation.Problems) {
		t.Fatalf("unexpected source validation errors = %#v", body.Validation.Problems)
	}
}

func TestValidateEndpointReportsSourceContractProblems(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Validate Broken Source Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "projects", "validate-broken-source-project", "components", "scalar.py")
	if err := os.WriteFile(sourcePath, []byte("class WrongName:\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) < 2 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
	if !strings.Contains(body.Message, "project source validation failed") {
		t.Fatalf("message = %s", body.Message)
	}
}

func TestRunRecordEndpointReturnsSavedRecord(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Run Record Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	runPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		RunRecord RunSummary `json:"run_record"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/run?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&run_id="+url.QueryEscape(runBody.RunRecord.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		RunRecord RunRecord `json:"run_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if recordBody.RunRecord.ID != runBody.RunRecord.ID {
		t.Fatalf("record id = %s, want %s", recordBody.RunRecord.ID, runBody.RunRecord.ID)
	}
	if recordBody.RunRecord.Result.Outputs["result"] != 8.0 {
		t.Fatalf("record result = %v, want 8", recordBody.RunRecord.Result.Outputs["result"])
	}
}

func TestSourceEndpointReadsExampleSource(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/source?project_path=examples/001_scalar_component/project.bcsproj&component_id=gain",
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Source SourceDetail `json:"source"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Source.ReadOnly {
		t.Fatal("example source should be read-only")
	}
	if body.Source.RelativePath != "components/scalar.py" {
		t.Fatalf("relative path = %s", body.Source.RelativePath)
	}
	if !strings.Contains(body.Source.Content, "class Gain") {
		t.Fatal("source did not include Gain")
	}
}

func TestUpdateSourceEndpointWritesWorkspaceSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	content := "class ScalarComponent:\n    pass\n"
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      content,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Source SourceDetail `json:"source"`
		Check  SourceCheck  `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source.Content != content {
		t.Fatalf("response source = %q", body.Source.Content)
	}
	if body.Check.OK {
		t.Fatal("source save check should report missing evaluate")
	}
	if len(body.Check.Problems) == 0 {
		t.Fatal("source save check returned no problems")
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "projects", "source-project", "components", "scalar.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceBytes) != content {
		t.Fatalf("source = %q", string(sourceBytes))
	}
}

func TestCheckSourceEndpointReportsContractProblems(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Check Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class WrongName:\n    pass\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Check.OK {
		t.Fatal("source check should fail when class and evaluate are missing")
	}
	if body.Check.ExpectedClass != "ScalarComponent" {
		t.Fatalf("expected class = %s", body.Check.ExpectedClass)
	}
	if len(body.Check.Problems) < 2 {
		t.Fatalf("problems = %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointReportsEvaluateSignatureProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Signature Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs):\n        return {\"result\": 1}, {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Check.OK {
		t.Fatal("source check should fail when evaluate signature is wrong")
	}
	var signatureProblem *Problem
	for index := range body.Check.Problems {
		if strings.Contains(body.Check.Problems[index].Message, "evaluate signature") {
			signatureProblem = &body.Check.Problems[index]
			break
		}
	}
	if signatureProblem == nil {
		t.Fatalf("signature problem missing from %#v", body.Check.Problems)
	}
	if signatureProblem.Line != 2 {
		t.Fatalf("line = %d, want 2", signatureProblem.Line)
	}
}

func TestCheckSourceEndpointReportsImportProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Import Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "import definitely_missing_hvac_studio_package\n\nclass ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        value = float(inputs.get(\"value\", 0.0))\n        return {\"result\": value}, state\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Check.OK {
		t.Fatal("source check should fail when source imports a missing package")
	}
	if !hasProblemMessageContaining(body.Check.Problems, "source load failed:") {
		t.Fatalf("load problem missing from %#v", body.Check.Problems)
	}
	if !hasProblemMessageContaining(body.Check.Problems, "definitely_missing_hvac_studio_package") {
		t.Fatalf("missing import name was not reported in %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointReportsUndefinedNameWarning(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Undefined Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        value = float(inputs.get(\"value\", 0.0)) * missing_factor\n        return {\"result\": value}, state\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Check.OK {
		t.Fatalf("undefined-name hint should warn without blocking: %#v", body.Check.Problems)
	}
	var found *Problem
	for index := range body.Check.Problems {
		if strings.Contains(body.Check.Problems[index].Message, "missing_factor") {
			found = &body.Check.Problems[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("undefined-name warning missing from %#v", body.Check.Problems)
	}
	if found.Severity != "warning" || found.Line != 3 {
		t.Fatalf("undefined-name problem = %#v", found)
	}
}

func TestCheckSourceEndpointAcceptsWorkspaceSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Check Valid Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "projects", "source-check-valid-project", "components", "scalar.py"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      string(sourceBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Check.OK {
		t.Fatalf("source check problems = %#v", body.Check.Problems)
	}
}

func TestGeneratedWrapperComponentUsesUserStepSource(t *testing.T) {
	server := newTestServer(t)
	projectPath := filepath.Join("examples", "008_generated_wrapper_component", "project.bcsproj")

	sourceResponse := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/source?project_path="+url.QueryEscape(projectPath)+"&component_id=wrapped_gain",
		nil,
	)
	server.Handler().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Source SourceDetail `json:"source"`
	}
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); err != nil {
		t.Fatal(err)
	}
	if sourceBody.Source.RelativePath != "components/custom_gain/user_step.py" {
		t.Fatalf("source relative path = %s", sourceBody.Source.RelativePath)
	}
	if !sourceBody.Source.ReadOnly {
		t.Fatal("example source should be read only")
	}
	if !strings.Contains(sourceBody.Source.Content, "def step(inputs, state, params, context):") {
		t.Fatalf("source content = %s", sourceBody.Source.Content)
	}

	checkPayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"component_id": "wrapped_gain",
		"content":      sourceBody.Source.Content,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkResponse := httptest.NewRecorder()
	checkRequest := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(checkPayload))
	server.Handler().ServeHTTP(checkResponse, checkRequest)
	if checkResponse.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", checkResponse.Code, checkResponse.Body.String())
	}
	var checkBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(checkResponse.Body.Bytes(), &checkBody); err != nil {
		t.Fatal(err)
	}
	if !checkBody.Check.OK {
		t.Fatalf("source check problems = %#v", checkBody.Check.Problems)
	}
	if checkBody.Check.ExpectedFunction != "step" {
		t.Fatalf("expected function = %s", checkBody.Check.ExpectedFunction)
	}

	validatePayload, err := json.Marshal(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatal(err)
	}
	validateResponse := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(validateResponse, validateRequest)
	if validateResponse.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateResponse.Code, validateResponse.Body.String())
	}

	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64            `json:"outputs"`
			States  map[string]map[string]float64 `json:"states"`
		} `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if runBody.Result.Outputs["result"] != 13 {
		t.Fatalf("result = %v, want 13", runBody.Result.Outputs["result"])
	}
	if runBody.Result.States["wrapped_gain"]["calls"] != 1 {
		t.Fatalf("calls state = %#v, want 1", runBody.Result.States["wrapped_gain"])
	}
}

func TestCheckSourceEndpointWarnsAboutUnreferencedContractNodes(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Contract Warning Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"other\": 1}, state\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Check.OK {
		t.Fatalf("warnings should not fail source check = %#v", body.Check.Problems)
	}
	if !hasProblemMessage(body.Check.Problems, "required input node is not referenced in source: value") {
		t.Fatalf("input warning missing from %#v", body.Check.Problems)
	}
	if !hasProblemMessage(body.Check.Problems, "output node is not obviously returned by source: result") {
		t.Fatalf("output warning missing from %#v", body.Check.Problems)
	}
}

func TestUpdateSourceEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"content": "class ScalarComponent:\n    pass\n"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateScenarioEndpointWritesWorkspaceScenario(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Scenario Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Design Day",
		"inputs":       map[string]any{"value": 9},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ScenarioSummary `json:"summary"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "scenarios/design-day.json" {
		t.Fatalf("relative path = %s", body.Summary.RelativePath)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "scenario-project", "scenarios", "design-day.json")); err != nil {
		t.Fatal(err)
	}
}

func TestScenarioEndpointReturnsSavedScenario(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Scenario Read Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Design Day",
		"inputs":       map[string]any{"value": 9},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	createScenarioResponse := httptest.NewRecorder()
	createScenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
	server.Handler().ServeHTTP(createScenarioResponse, createScenarioRequest)
	if createScenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", createScenarioResponse.Code, createScenarioResponse.Body.String())
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/scenario?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&scenario_id=design-day",
		nil,
	)
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Scenario ScenarioRecord `json:"scenario"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scenario.Inputs["value"] != 9.0 {
		t.Fatalf("scenario input = %v, want 9", body.Scenario.Inputs["value"])
	}
}

func TestBatchEndpointRunsSavedScenarios(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Batch Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	parameterSetPath := filepath.Join(root, "projects", "batch-project", "parameter_sets", "triple_gain.json")
	if err := os.MkdirAll(filepath.Dir(parameterSetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parameterSetPath, []byte(`{
  "id": "triple_gain",
  "name": "Triple Gain",
  "components": {
    "scalar": {
      "gain": 3
    }
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name  string
		value float64
	}{
		{name: "Low", value: 2},
		{name: "High", value: 3},
	} {
		payload, err := json.Marshal(map[string]any{
			"project_path": createBody.Project.ProjectPath,
			"name":         scenario.name,
			"inputs":       map[string]any{"value": scenario.value},
			"context":      map[string]any{"time": 0, "dt": 60},
		})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("scenario status = %d body=%s", response.Code, response.Body.String())
		}
	}

	batchPayload, err := json.Marshal(map[string]any{
		"project_path":       createBody.Project.ProjectPath,
		"parameter_set_path": filepath.Join("parameter_sets", "triple_gain.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	batchResponse := httptest.NewRecorder()
	batchRequest := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
	server.Handler().ServeHTTP(batchResponse, batchRequest)
	if batchResponse.Code != http.StatusOK {
		t.Fatalf("batch status = %d body=%s", batchResponse.Code, batchResponse.Body.String())
	}
	var batchBody struct {
		Summary BatchSummary `json:"summary"`
		Batch   BatchRecord  `json:"batch"`
	}
	if err := json.Unmarshal(batchResponse.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody.Summary.CaseCount != 2 || batchBody.Summary.OKCount != 2 {
		t.Fatalf("batch counts = %d/%d, want 2/2", batchBody.Summary.OKCount, batchBody.Summary.CaseCount)
	}
	if batchBody.Summary.ParameterSet != "parameter_sets/triple_gain.json" || batchBody.Batch.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("batch parameter set = summary:%q record:%q", batchBody.Summary.ParameterSet, batchBody.Batch.ParameterSet)
	}
	if len(batchBody.Batch.Cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(batchBody.Batch.Cases))
	}
	if got := batchBody.Batch.Cases[0].Result.Outputs["result"]; got != 6.0 {
		t.Fatalf("first output = %v, want 6", got)
	}
	if got := batchBody.Batch.Cases[1].Result.Outputs["result"]; got != 9.0 {
		t.Fatalf("second output = %v, want 9", got)
	}
	if batchBody.Batch.Cases[0].Result.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("case parameter set = %q", batchBody.Batch.Cases[0].Result.ParameterSet)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "batch-project", batchBody.Summary.RelativePath)); err != nil {
		t.Fatal(err)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/batch?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&batch_id="+url.QueryEscape(batchBody.Summary.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("batch record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		BatchRecord BatchRecord `json:"batch_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if recordBody.BatchRecord.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("opened batch parameter set = %q", recordBody.BatchRecord.ParameterSet)
	}
}

func TestBatchEndpointRecordsProblemsForFailedCases(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Batch Failure Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	scenarioPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Broken",
		"inputs":       map[string]any{"value": 2},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	scenarioResponse := httptest.NewRecorder()
	scenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(scenarioPayload))
	server.Handler().ServeHTTP(scenarioResponse, scenarioRequest)
	if scenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", scenarioResponse.Code, scenarioResponse.Body.String())
	}

	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"result\": float(inputs[\"value\"]), \"debug\": 1}, state\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceResponse := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(sourcePayload))
	server.Handler().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}

	batchPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	batchResponse := httptest.NewRecorder()
	batchRequest := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
	server.Handler().ServeHTTP(batchResponse, batchRequest)
	if batchResponse.Code != http.StatusOK {
		t.Fatalf("batch status = %d body=%s", batchResponse.Code, batchResponse.Body.String())
	}
	var batchBody struct {
		Summary BatchSummary `json:"summary"`
		Batch   BatchRecord  `json:"batch"`
	}
	if err := json.Unmarshal(batchResponse.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody.Summary.CaseCount != 1 || batchBody.Summary.OKCount != 0 {
		t.Fatalf("batch counts = %d/%d, want 0/1", batchBody.Summary.OKCount, batchBody.Summary.CaseCount)
	}
	if len(batchBody.Batch.Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(batchBody.Batch.Cases))
	}
	failed := batchBody.Batch.Cases[0]
	if failed.OK {
		t.Fatal("failed case was marked ok")
	}
	if !strings.Contains(failed.Error, "returned undeclared output node: debug") {
		t.Fatalf("case error = %s", failed.Error)
	}
	if len(failed.Problems) != 1 || failed.Problems[0].ComponentID != "scalar" {
		t.Fatalf("case problems = %#v", failed.Problems)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/batch?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&batch_id="+url.QueryEscape(batchBody.Summary.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("batch record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		BatchRecord BatchRecord `json:"batch_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if len(recordBody.BatchRecord.Cases) != 1 || len(recordBody.BatchRecord.Cases[0].Problems) != 1 {
		t.Fatalf("record problems = %#v", recordBody.BatchRecord.Cases)
	}
	if recordBody.BatchRecord.Cases[0].Problems[0].ComponentID != "scalar" {
		t.Fatalf("record problem component = %s", recordBody.BatchRecord.Cases[0].Problems[0].ComponentID)
	}
}

func TestBatchEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Batch Source Gate Project")
	scenarioPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"name":         "Gate",
		"inputs":       map[string]any{"value": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	scenarioResponse := httptest.NewRecorder()
	scenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(scenarioPayload))
	server.Handler().ServeHTTP(scenarioResponse, scenarioRequest)
	if scenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", scenarioResponse.Code, scenarioResponse.Body.String())
	}
	writeBrokenScalarSource(t, project)

	batchPayload, err := json.Marshal(map[string]any{"project_path": project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !hasProblemMessage(body.Problems, "evaluate method is missing") {
		t.Fatalf("source problem missing from %#v", body.Problems)
	}
}

func TestCreateScenarioEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"name": "Example Scenario",
		"inputs": {"value": 5}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestBatchEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{"project_path":"examples/001_scalar_component/project.bcsproj"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestExportEndpointWritesRuntimeArtifact(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	seedTestRuntimeSupport(t, root)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Export Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	seedExportWorkflowArtifacts(t, filepath.Join(root, "projects", "export-project"))

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ExportSummary  `json:"summary"`
		Export  ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "exports/runtime_package/manifest.json" {
		t.Fatalf("relative path = %s", body.Summary.RelativePath)
	}
	if body.Export.ProjectRoot != "project" {
		t.Fatalf("project root = %s", body.Export.ProjectRoot)
	}
	if body.Export.ProjectPath != "project/project.bcsproj" {
		t.Fatalf("project path = %s", body.Export.ProjectPath)
	}
	if body.Export.GraphPath != "project/graph.json" {
		t.Fatalf("graph path = %s", body.Export.GraphPath)
	}
	if body.Export.DefaultInput != "project/inputs/case01.json" {
		t.Fatalf("default input = %s", body.Export.DefaultInput)
	}
	if body.Export.EnvironmentLockfile != "project/requirements.lock.txt" {
		t.Fatalf("environment lockfile = %s", body.Export.EnvironmentLockfile)
	}
	if body.Export.InterfaceSchema != "schema/public-io.json" {
		t.Fatalf("interface schema = %s", body.Export.InterfaceSchema)
	}
	if body.Export.Runner != "bin/bcs-runner.exe" {
		t.Fatalf("runner = %s", body.Export.Runner)
	}
	expectedFiles := []string{
		"README.md",
		"bin/bcs-env.exe",
		"bin/bcs-runner.exe",
		"calibrate.ps1",
		"optimize.ps1",
		"project/project.bcsproj",
		"project/graph.json",
		"project/components/__init__.py",
		"project/components/scalar.py",
		"project/datasets/scalar_validation.csv",
		"project/parameter_sets/baseline.json",
		"project/scenarios/case01.json",
		"project/validation/mappings/scalar_validation.json",
		"project/calibration/setups/scalar_gain.json",
		"project/optimization/setups/scalar_grid.json",
		"project/inputs/case01.json",
		"project/requirements.lock.txt",
		"runtime/manifest.json",
		"runtime/python/python.exe",
		"run-batch.ps1",
		"run-default.ps1",
		"schema/public-io.json",
		"validate-data.ps1",
	}
	exportRoot := filepath.Join(root, "projects", "export-project", "exports", "runtime_package")
	for _, rel := range expectedFiles {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
	if !containsString(body.Export.ParameterSets, "project/parameter_sets/baseline.json") {
		t.Fatalf("export parameter sets = %v", body.Export.ParameterSets)
	}
	if !containsString(body.Export.Datasets, "project/datasets/scalar_validation.csv") {
		t.Fatalf("export datasets = %v", body.Export.Datasets)
	}
	if !containsString(body.Export.ValidationMappings, "project/validation/mappings/scalar_validation.json") {
		t.Fatalf("export validation mappings = %v", body.Export.ValidationMappings)
	}
	if !containsString(body.Export.CalibrationSetups, "project/calibration/setups/scalar_gain.json") {
		t.Fatalf("export calibration setups = %v", body.Export.CalibrationSetups)
	}
	if !containsString(body.Export.OptimizationSetups, "project/optimization/setups/scalar_grid.json") {
		t.Fatalf("export optimization setups = %v", body.Export.OptimizationSetups)
	}
	for _, rel := range body.Export.Files {
		if strings.HasPrefix(rel, "project/runs/") || strings.HasPrefix(rel, "project/batches/") || strings.HasPrefix(rel, "project/validation/runs/") || strings.HasPrefix(rel, "project/calibration/results/") || strings.HasPrefix(rel, "project/optimization/results/") || strings.HasPrefix(rel, "project/exports/") {
			t.Fatalf("export should not include generated project artifact %s", rel)
		}
	}
	if body.Export.IncludeRecords {
		t.Fatal("default API export should not include generated records")
	}
	for _, command := range []string{"run-default.ps1", "run-batch.ps1", "validate-data.ps1", "calibrate.ps1", "optimize.ps1"} {
		if !containsString(body.Export.Commands, command) {
			t.Fatalf("export commands missing %s in %v", command, body.Export.Commands)
		}
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "manifest.json")); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var exportedSchema schemaexport.InterfaceSchema
	schemaBytes, err := os.ReadFile(filepath.Join(exportRoot, "schema", "public-io.json"))
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if err := json.Unmarshal(schemaBytes, &exportedSchema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	if len(exportedSchema.Inputs) != 1 || len(exportedSchema.Outputs) != 1 {
		t.Fatalf("schema inputs/outputs = %d/%d", len(exportedSchema.Inputs), len(exportedSchema.Outputs))
	}
	exportedProjectPath := filepath.Join(exportRoot, "project", "project.bcsproj")
	exportedLoaded, err := project.Load(exportedProjectPath)
	if err != nil {
		t.Fatalf("load exported project: %v", err)
	}
	if _, err := compiler.Compile(exportedLoaded); err != nil {
		t.Fatalf("compile exported project: %v", err)
	}

	openResponse := httptest.NewRecorder()
	openRequest := httptest.NewRequest(http.MethodGet, "/api/project/export?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&profile=runtime_package", nil)
	server.Handler().ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK {
		t.Fatalf("open export status = %d body=%s", openResponse.Code, openResponse.Body.String())
	}
	var openBody struct {
		Summary ExportSummary  `json:"summary"`
		Export  ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(openResponse.Body.Bytes(), &openBody); err != nil {
		t.Fatal(err)
	}
	if openBody.Summary.RelativePath != body.Summary.RelativePath {
		t.Fatalf("opened export relative path = %s, want %s", openBody.Summary.RelativePath, body.Summary.RelativePath)
	}
	if len(openBody.Export.Files) != len(body.Export.Files) {
		t.Fatalf("opened export file count = %d, want %d", len(openBody.Export.Files), len(body.Export.Files))
	}

	recordPayload, err := json.Marshal(map[string]any{
		"project_path":    createBody.Project.ProjectPath,
		"profile":         "runtime_package",
		"include_records": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(recordPayload))
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("record export status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	expectedRecords := []string{
		"project/runs/run-test.json",
		"project/batches/batch-test.json",
		"project/validation/runs/validation-test.json",
		"project/calibration/results/calibration-test.json",
		"project/optimization/results/optimization-test.json",
	}
	for _, rel := range expectedRecords {
		if !containsString(recordBody.Export.Files, rel) {
			t.Fatalf("record export files missing %s in %v", rel, recordBody.Export.Files)
		}
	}
	if !recordBody.Export.IncludeRecords || !containsString(recordBody.Export.RunRecords, "project/runs/run-test.json") {
		t.Fatalf("record manifest = %#v", recordBody.Export)
	}
}

func TestExportEndpointIncludesGeneratedWrapperSources(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	seedTestRuntimeSupport(t, root)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "generated-wrapper-export")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "008_generated_wrapper_component"), projectRoot); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": filepath.Join(projectRoot, "project.bcsproj"),
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	expectedFiles := []string{
		"project/components/custom_gain/component.json",
		"project/components/custom_gain/helpers.py",
		"project/components/custom_gain/user_init.py",
		"project/components/custom_gain/user_step.py",
		"project/components/custom_gain/wrapper.py",
	}
	exportRoot := filepath.Join(projectRoot, "exports", "runtime_package")
	for _, rel := range expectedFiles {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
}

func TestExportEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Export Source Gate Project")
	writeBrokenScalarSource(t, project)

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !hasProblemMessage(body.Problems, "evaluate method is missing") {
		t.Fatalf("source problem missing from %#v", body.Problems)
	}
}

func TestExportEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"profile": "runtime_package"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRunRecordsRoundTrip(t *testing.T) {
	projectRoot := t.TempDir()
	loaded := &project.LoadedProject{
		Project: &model.Project{ProjectName: "recorded"},
		Root:    projectRoot,
	}
	input := runtimecore.RunInput{
		Inputs:  map[string]any{"value": 4.0},
		Context: map[string]any{"time": 0.0, "dt": 60.0},
	}
	result := &runtimecore.RunResult{
		OK:      true,
		Outputs: map[string]any{"result": 8.0},
	}

	summary, err := writeRunRecord(loaded, input, result, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.RelativePath == "" {
		t.Fatal("run summary did not include relative path")
	}
	summaries := loadRunSummaries(projectRoot)
	if len(summaries) != 1 {
		t.Fatalf("run summary count = %d", len(summaries))
	}
	if summaries[0].ID != summary.ID {
		t.Fatalf("run id = %s, want %s", summaries[0].ID, summary.ID)
	}
}

func hasProblemMessage(problems []Problem, message string) bool {
	for _, problem := range problems {
		if problem.Message == message {
			return true
		}
	}
	return false
}

func hasProblemMessageContaining(problems []Problem, text string) bool {
	for _, problem := range problems {
		if strings.Contains(problem.Message, text) {
			return true
		}
	}
	return false
}

func hasComponentLog(logs []runtimecore.ComponentLog, component string, stage string, severity string, message string) bool {
	for _, log := range logs {
		if log.Component == component && log.Stage == stage && log.Severity == severity && log.Message == message {
			return true
		}
	}
	return false
}

func createWorkspaceProject(t *testing.T, server *Server, name string) ProjectSummary {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Project
}

func writeBrokenScalarSource(t *testing.T, project ProjectSummary) {
	t.Helper()
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	if err := os.WriteFile(sourcePath, []byte("class ScalarComponent:\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func newIsolatedTestServer(t *testing.T) (string, *Server) {
	t.Helper()
	root := t.TempDir()
	seedTestTemplates(t, root)
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, server
}

func seedTestTemplates(t *testing.T, root string) {
	t.Helper()
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyProjectTree(filepath.Join(repoRoot, "templates"), filepath.Join(root, "templates")); err != nil {
		t.Fatal(err)
	}
}

func seedTestRuntimeSupport(t *testing.T, root string) {
	t.Helper()
	for rel, content := range map[string]string{
		"bin/bcs-runner.exe":        "runner",
		"bin/bcs-env.exe":           "env",
		"runtime/manifest.json":     `{"runtime":"test"}`,
		"runtime/python/python.exe": "python",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func hasColumnSuggestion(values []ColumnSuggestion, publicID string, column string) bool {
	for _, item := range values {
		if item.PublicID == publicID && item.Column == column {
			return true
		}
	}
	return false
}

func hasComponentTemplate(values []ComponentTemplateSummary, id string) bool {
	for _, item := range values {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasParameterDiff(values []ParameterDiff, component string, parameter string) bool {
	for _, item := range values {
		if item.Component == component && item.Parameter == parameter && item.Exists {
			return true
		}
	}
	return false
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "examples")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "tools", "go", "go.mod")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedExportWorkflowArtifacts(t *testing.T, projectRoot string) {
	t.Helper()
	writeTestFile(t, filepath.Join(projectRoot, "parameter_sets", "baseline.json"), `{
  "id": "baseline",
  "components": {
    "scalar": {
      "gain": 2
    }
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "datasets", "scalar_validation.csv"), "value,observed_result\n4,8\n")
	writeTestFile(t, filepath.Join(projectRoot, "scenarios", "case01.json"), `{
  "id": "case01",
  "name": "Case 01",
  "inputs": {
    "value": 4
  },
  "context": {
    "time": 0
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "validation", "mappings", "scalar_validation.json"), `{
  "id": "scalar_validation",
  "dataset": "datasets/scalar_validation.csv",
  "input_columns": {
    "value": "value"
  },
  "observed_output_columns": {
    "result": "observed_result"
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "calibration", "setups", "scalar_gain.json"), `{
  "id": "scalar_gain",
  "algorithm": "grid",
  "mapping": "validation/mappings/scalar_validation.json",
  "objective": {
    "metric": "rmse"
  },
  "parameters": [
    {
      "component": "scalar",
      "name": "gain",
      "min": 1,
      "max": 3,
      "step": 1
    }
  ]
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "optimization", "setups", "scalar_grid.json"), `{
  "id": "scalar_grid",
  "algorithm": "grid",
  "base_inputs": {
    "value": 4
  },
  "objective": {
    "output": "result",
    "sense": "max"
  },
  "decision_variables": [
    {
      "kind": "public_input",
      "name": "value",
      "min": 2,
      "max": 4,
      "step": 1
    }
  ]
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "runs", "run-test.json"), `{"id":"run-test","result":{"outputs":{"result":8}}}`)
	writeTestFile(t, filepath.Join(projectRoot, "batches", "batch-test.json"), `{"id":"batch-test","cases":[]}`)
	writeTestFile(t, filepath.Join(projectRoot, "validation", "runs", "validation-test.json"), `{"id":"validation-test","result":{"row_count":1}}`)
	writeTestFile(t, filepath.Join(projectRoot, "calibration", "results", "calibration-test.json"), `{"id":"calibration-test","result":{"best_objective":0}}`)
	writeTestFile(t, filepath.Join(projectRoot, "optimization", "results", "optimization-test.json"), `{"id":"optimization-test","result":{"best_objective":0}}`)
}
