package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
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

type staticOption struct {
	value string
	label string
}

func assertStaticOptions(t *testing.T, body []byte, group string, options []staticOption) {
	t.Helper()
	for _, option := range options {
		token := `["` + option.value + `", "` + option.label + `"`
		if !bytes.Contains(body, []byte(token)) {
			t.Fatalf("%s did not include %s", group, token)
		}
	}
}

func assertStaticTokens(t *testing.T, body []byte, group string, tokens []string) {
	t.Helper()
	for _, token := range tokens {
		if !bytes.Contains(body, []byte(token)) {
			t.Fatalf("%s did not include %s", group, token)
		}
	}
}

func TestStaticIndexServesWorkspace(t *testing.T) {
	server := newTestServer(t)
	body := getRouteBody(t, server, "/")
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
	if !bytes.Contains(body, []byte("componentCategorySelect")) {
		t.Fatalf("index did not include the component category selector")
	}
	if !bytes.Contains(body, []byte("componentExecutionModeSelect")) {
		t.Fatalf("index did not include the component execution mode selector")
	}
	if !bytes.Contains(body, []byte("includeComponentOnCreate")) {
		t.Fatalf("index did not include the component system inclusion control")
	}
	if !bytes.Contains(body, []byte("newMLComponentButton")) || !bytes.Contains(body, []byte("New ML")) {
		t.Fatalf("index did not include the ML component quick-create control")
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
	if !bytes.Contains(body, []byte("startWorkflowRows")) {
		t.Fatalf("index did not include the Start workflow readiness rows")
	}
	if bytes.Contains(body, []byte("serveButton")) {
		t.Fatalf("index should not include a disabled Serve command slot")
	}
	for _, id := range []string{
		"exportIncludeDatasetsInput",
		"exportIncludeCalibrationInput",
		"exportIncludeOptimizationInput",
		"exportIncludeMLAssetsInput",
		"exportIncludeSDKInput",
		"exportIncludeRecordsInput",
	} {
		if !bytes.Contains(body, []byte(id)) {
			t.Fatalf("index did not include runtime export option %s", id)
		}
		if bytes.Contains(body, []byte(`id="`+id+`" type="checkbox" checked disabled`)) {
			t.Fatalf("runtime export option %s should be selectable", id)
		}
	}
	if !bytes.Contains(body, []byte("SDK Examples")) || !bytes.Contains(body, []byte("Records")) {
		t.Fatalf("index did not include the runtime export record selection control")
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
	if !bytes.Contains(body, []byte("workspaceHelpLink")) {
		t.Fatalf("index did not include workspace help link")
	}
	if bytes.Contains(body, []byte("Empty System")) || bytes.Contains(body, []byte("HVAC System")) || bytes.Contains(body, []byte("Runtime-only")) {
		t.Fatalf("index should not expose unavailable project types")
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
	if !bytes.Contains(body, []byte("cancelRunButton")) {
		t.Fatalf("index did not include run cancellation control")
	}
	if !bytes.Contains(body, []byte("retryRunButton")) {
		t.Fatalf("index did not include run retry control")
	}
	if !bytes.Contains(body, []byte("seriesButton")) {
		t.Fatalf("index did not include time-series run control")
	}
	if !bytes.Contains(body, []byte("datasetEncodingSelect")) {
		t.Fatalf("index did not include dataset encoding selection")
	}
	if !bytes.Contains(body, []byte(`value="auto">Auto`)) ||
		!bytes.Contains(body, []byte(`value="utf-16">UTF-16`)) ||
		!bytes.Contains(body, []byte(`value="cp949">CP949 / EUC-KR`)) {
		t.Fatalf("index did not include practical dataset encoding options")
	}
	if !bytes.Contains(body, []byte("executionTraceRows")) {
		t.Fatalf("index did not include execution trace rows")
	}
	if !bytes.Contains(body, []byte("runExportActions")) {
		t.Fatalf("index did not include run result export actions")
	}
	if !bytes.Contains(body, []byte("componentLogRows")) {
		t.Fatalf("index did not include component log rows")
	}
	if !bytes.Contains(body, []byte("<th>Time</th>")) || !bytes.Contains(body, []byte("<th>Source</th>")) {
		t.Fatalf("index did not include component log time/source columns")
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

func TestDocsRouteServesUserGuide(t *testing.T) {
	server := newTestServer(t)
	body := getRouteBody(t, server, "/docs/user/run-simulation.md")
	if !bytes.Contains(body, []byte("# Run Simulation")) {
		t.Fatalf("docs route did not serve the run simulation guide")
	}

	tutorialBody := getRouteBody(t, server, "/docs/user/tutorials.md")
	if !bytes.Contains(tutorialBody, []byte("assets/tutorials/studio-canvas.png")) {
		t.Fatalf("docs route did not serve screenshot tutorial references")
	}
}

func TestStaticModuleEntrypointServes(t *testing.T) {
	server := newTestServer(t)
	body := getRouteBody(t, server, "/js/app.js")
	configBody := getRouteBody(t, server, "/js/workspace-config.js")
	connectionsBody := getRouteBody(t, server, "/js/connections.js")
	assertStaticOptions(t, configBody, "component category options", []staticOption{
		{"physical_component", "Physical Component"},
		{"controller", "Controller"},
		{"data_source", "Data Source"},
		{"data_sink", "Data Sink"},
		{"utility", "Utility"},
		{"solver", "Solver"},
		{"composite_wrapper", "Composite Wrapper"},
	})
	assertStaticOptions(t, configBody, "execution mode options", []staticOption{
		{"step", "Step"},
		{"vectorized", "Vectorized"},
		{"external_executable", "External Executable"},
		{"initialization_only", "Initialization Only"},
	})
	assertStaticOptions(t, configBody, "node preset options", []staticOption{
		{"water_inlet", "Water Inlet"},
		{"water_outlet", "Water Outlet"},
		{"air_inlet", "Air Inlet"},
		{"air_outlet", "Air Outlet"},
		{"control_signal_input", "Control Signal Input"},
		{"electric_power_output", "Electric Power Output"},
		{"scalar_input", "Scalar Input"},
		{"scalar_output", "Scalar Output"},
		{"time_series_input", "Time Series Input"},
	})
	assertStaticTokens(t, configBody, "parameter role options", []string{
		`"fixed"`,
		`"scenario_input"`,
		`"calibration_target"`,
		`"optimization_variable"`,
		`"derived"`,
	})
	assertStaticTokens(t, configBody, "ML model format options", []string{
		`"pickle"`,
		`"joblib"`,
		`"onnx"`,
		`"torch"`,
		`"tensorflow"`,
		`"custom"`,
	})
	assertStaticOptions(t, configBody, "ML asset fields", []staticOption{
		{"model_file", "Model File"},
		{"input_scaler_file", "Input Scaler"},
		{"output_scaler_file", "Output Scaler"},
		{"feature_schema_file", "Feature Schema"},
		{"target_schema_file", "Target Schema"},
		{"training_metadata_file", "Training Metadata"},
		{"validation_report_file", "Validation Report"},
	})
	startBody := getRouteBody(t, server, "/js/start-workspace.js")
	logsBody := getRouteBody(t, server, "/js/logs-panel.js")
	resultUIBody := getRouteBody(t, server, "/js/result-ui.js")
	artifactResultsBody := getRouteBody(t, server, "/js/artifact-results.js")
	stylesBody := getRouteBody(t, server, "/styles.css")
	mlInspectorBody := getRouteBody(t, server, "/js/ml-inspector.js")
	componentTemplatesBody := getRouteBody(t, server, "/js/component-templates.js")
	datasetMappingBody := getRouteBody(t, server, "/js/dataset-mapping.js")
	downloadBody := getRouteBody(t, server, "/js/download.js")
	validationPlotsBody := getRouteBody(t, server, "/js/validation-plots.js")
	validationResultsBody := getRouteBody(t, server, "/js/validation-results.js")
	candidateResultsBody := getRouteBody(t, server, "/js/candidate-results.js")
	seriesResultsBody := getRouteBody(t, server, "/js/series-results.js")
	sourceAuthoringBody := getRouteBody(t, server, "/js/source-authoring.js")
	pythonSourceBody := getRouteBody(t, server, "/js/python-source.js")
	replacementPreviewBody := getRouteBody(t, server, "/js/replacement-preview.js")
	nodeImpactBody := getRouteBody(t, server, "/js/node-impact.js")
	contractImpactBody := getRouteBody(t, server, "/js/contract-impact.js")
	contractAuthoringBody := getRouteBody(t, server, "/js/contract-authoring.js")
	contractLabelsBody := getRouteBody(t, server, "/js/contract-labels.js")
	calibrationSetupEditorBody := getRouteBody(t, server, "/js/calibration-setup-editor.js")
	optimizationSetupEditorBody := getRouteBody(t, server, "/js/optimization-setup-editor.js")
	workflowCandidatesBody := getRouteBody(t, server, "/js/workflow-candidates.js")
	setupEditorUIBody := getRouteBody(t, server, "/js/setup-editor-ui.js")
	if !bytes.Contains(body, []byte(`from "./state.js"`)) {
		t.Fatalf("module entrypoint did not contain expected imports")
	}
	if !bytes.Contains(body, []byte(`from "./artifact-results.js"`)) {
		t.Fatalf("module entrypoint did not import artifact result renderers")
	}
	if !bytes.Contains(body, []byte(`from "./calibration-setup-editor.js"`)) {
		t.Fatalf("module entrypoint did not import calibration setup editor")
	}
	if !bytes.Contains(body, []byte(`from "./optimization-setup-editor.js"`)) {
		t.Fatalf("module entrypoint did not import optimization setup editor")
	}
	if !bytes.Contains(body, []byte(`from "./workflow-candidates.js"`)) {
		t.Fatalf("module entrypoint did not import workflow candidate helpers")
	}
	if !bytes.Contains(body, []byte(`from "./source-authoring.js"`)) {
		t.Fatalf("module entrypoint did not import source authoring helpers")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte(`from "./python-source.js"`)) {
		t.Fatalf("source authoring module did not import Python source helpers")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("bracketCheck")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("formatPythonSource")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("pythonStringLiteral")) {
		t.Fatalf("source authoring module did not re-export Python source helpers")
	}
	if !bytes.Contains(body, []byte(`from "./replacement-preview.js"`)) {
		t.Fatalf("module entrypoint did not import replacement preview helpers")
	}
	if !bytes.Contains(body, []byte(`from "./node-impact.js"`)) {
		t.Fatalf("module entrypoint did not import node impact helpers")
	}
	if !bytes.Contains(body, []byte(`from "./contract-impact.js"`)) {
		t.Fatalf("module entrypoint did not import contract impact helpers")
	}
	if !bytes.Contains(body, []byte(`from "./contract-authoring.js"`)) ||
		!bytes.Contains(contractAuthoringBody, []byte("newNodePayload")) ||
		!bytes.Contains(contractAuthoringBody, []byte("nodeUpdatePayload")) ||
		!bytes.Contains(contractAuthoringBody, []byte("newParameterDefinition")) ||
		!bytes.Contains(contractAuthoringBody, []byte("parameterDefinitionFromFields")) ||
		!bytes.Contains(contractAuthoringBody, []byte("newStateDefinition")) ||
		!bytes.Contains(contractAuthoringBody, []byte("stateDefinitionFromFields")) ||
		!bytes.Contains(contractAuthoringBody, []byte("PARAMETER_ROLES")) {
		t.Fatalf("module entrypoint did not import shared contract authoring helpers")
	}
	if !bytes.Contains(body, []byte(`from "./contract-labels.js"`)) ||
		!bytes.Contains(sourceAuthoringBody, []byte(`from "./contract-labels.js"`)) ||
		!bytes.Contains(contractImpactBody, []byte(`from "./contract-labels.js"`)) ||
		!bytes.Contains(contractLabelsBody, []byte("roleLabel")) {
		t.Fatalf("module entrypoint did not import shared contract label helpers")
	}
	if !bytes.Contains(body, []byte(`from "./component-templates.js"`)) {
		t.Fatalf("module entrypoint did not import component template helpers")
	}
	if !bytes.Contains(componentTemplatesBody, []byte("componentTemplateOptionLabel")) ||
		!bytes.Contains(componentTemplatesBody, []byte("componentTemplateMetaText")) {
		t.Fatalf("component template module did not expose selector helpers")
	}
	if !bytes.Contains(body, []byte(`from "./export-workspace.js"`)) {
		t.Fatalf("module entrypoint did not import export workspace renderer")
	}
	if !bytes.Contains(body, []byte("openProblem")) {
		t.Fatalf("module entrypoint did not include problem navigation")
	}
	if !bytes.Contains(body, []byte("problemLocationLabel")) {
		t.Fatalf("module entrypoint did not include source-aware problem locations")
	}
	if !bytes.Contains(body, []byte("applySourceSaveResponse")) {
		t.Fatalf("module entrypoint did not include source save response handling")
	}
	if !bytes.Contains(body, []byte("markComponentContractChanged")) ||
		!bytes.Contains(body, []byte("delete state.sourceCheckByComponent[componentID]")) {
		t.Fatalf("module entrypoint did not clear stale source checks after contract edits")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("evaluateSnippet")) {
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
	if !bytes.Contains(sourceAuthoringBody, []byte("sourceCompletionItems")) {
		t.Fatalf("module entrypoint did not include contract-derived source completion items")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("sourceItemTitle")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("Medium:")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("Value type:")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("Unit:")) {
		t.Fatalf("module entrypoint did not include source hover metadata")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("parameterSourceItems")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("stateSourceItems")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("contextSourceItems")) {
		t.Fatalf("module entrypoint did not include state/context source reference inserts")
	}
	if !bytes.Contains(body, []byte("parameterDeleteImpact")) ||
		!bytes.Contains(body, []byte("stateDeleteImpact")) ||
		!bytes.Contains(contractImpactBody, []byte("parameterDeleteImpactConfirmText")) ||
		!bytes.Contains(contractImpactBody, []byte("stateDeleteImpactConfirmText")) {
		t.Fatalf("module entrypoint did not include parameter/state delete impact hints")
	}
	if !bytes.Contains(pythonSourceBody, []byte("bracketCheck")) {
		t.Fatalf("module entrypoint did not include bracket status checking")
	}
	if !bytes.Contains(pythonSourceBody, []byte("highlightPython")) {
		t.Fatalf("module entrypoint did not include Python syntax highlighting")
	}
	if !bytes.Contains(body, []byte("handleSourceNewline")) {
		t.Fatalf("module entrypoint did not include source auto indentation")
	}
	if !bytes.Contains(body, []byte("formatCurrentSource")) {
		t.Fatalf("module entrypoint did not include source formatting")
	}
	if !bytes.Contains(pythonSourceBody, []byte("formatPythonSource")) {
		t.Fatalf("module entrypoint did not include Python source formatting rules")
	}
	if !bytes.Contains(body, []byte("sourceLineProblemMap")) {
		t.Fatalf("module entrypoint did not include source gutter problem markers")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("sourceQuickFixForProblem")) ||
		!bytes.Contains(body, []byte("sourceQuickFixForProblem")) {
		t.Fatalf("module entrypoint did not include source quick fixes")
	}
	if !bytes.Contains(body, []byte("replaceSourceIssueText")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("replacement")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("closestSourceName")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("sourceReferenceCandidates")) {
		t.Fatalf("module entrypoint did not include typo-like source quick fixes")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("stepSnippet")) {
		t.Fatalf("module entrypoint did not include generated-wrapper step snippets")
	}
	if !bytes.Contains(sourceAuthoringBody, []byte("evaluateBatchSnippet")) ||
		!bytes.Contains(sourceAuthoringBody, []byte("externalExecutableSnippet")) {
		t.Fatalf("module entrypoint did not include vectorized and external executable snippets")
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
	if !bytes.Contains(body, []byte("WORKSPACE_HELP")) {
		t.Fatalf("module entrypoint did not define workspace help links")
	}
	if !bytes.Contains(body, []byte("updateWorkspaceHelp")) {
		t.Fatalf("module entrypoint did not update contextual help links")
	}
	if !bytes.Contains(body, []byte("workspaceStateFromHash")) || !bytes.Contains(body, []byte("applyWorkspaceHash")) {
		t.Fatalf("module entrypoint did not support workspace hash links")
	}
	if !bytes.Contains(body, []byte("isKnownBottomTab")) {
		t.Fatalf("module entrypoint did not support bottom-panel hash links")
	}
	if !bytes.Contains(body, []byte("hashchange")) {
		t.Fatalf("module entrypoint did not react to workspace hash changes")
	}
	if !bytes.Contains(body, []byte(`from "./result-ui.js"`)) {
		t.Fatalf("module entrypoint did not import result UI helpers")
	}
	if !bytes.Contains(body, []byte(`from "./ml-inspector.js"`)) {
		t.Fatalf("module entrypoint did not import ML inspector helpers")
	}
	if !bytes.Contains(body, []byte(`from "./connections.js"`)) {
		t.Fatalf("module entrypoint did not import connection display helpers")
	}
	if !bytes.Contains(body, []byte(`from "./dataset-mapping.js"`)) {
		t.Fatalf("module entrypoint did not import dataset mapping helpers")
	}
	if !bytes.Contains(body, []byte(`from "./download.js"`)) {
		t.Fatalf("module entrypoint did not import download helpers")
	}
	if !bytes.Contains(body, []byte(`from "./validation-results.js"`)) {
		t.Fatalf("module entrypoint did not import validation result helpers")
	}
	if !bytes.Contains(validationResultsBody, []byte(`from "./validation-plots.js"`)) {
		t.Fatalf("validation result module did not import validation plot helpers")
	}
	if !bytes.Contains(body, []byte(`from "./candidate-results.js"`)) {
		t.Fatalf("module entrypoint did not import candidate result helpers")
	}
	if !bytes.Contains(body, []byte(`from "./series-results.js"`)) {
		t.Fatalf("module entrypoint did not import series result helpers")
	}
	if !bytes.Contains(downloadBody, []byte("downloadTextFile")) ||
		!bytes.Contains(downloadBody, []byte("safeFileName")) ||
		!bytes.Contains(downloadBody, []byte("csvCell")) ||
		!bytes.Contains(downloadBody, []byte("markdownTable")) {
		t.Fatalf("download helper module did not expose export helpers")
	}
	if !bytes.Contains(connectionsBody, []byte("connectionMediumStateForNodes")) ||
		!bytes.Contains(connectionsBody, []byte("connectionUnitStateForNodes")) ||
		!bytes.Contains(connectionsBody, []byte("connectionContractLabels")) ||
		!bytes.Contains(connectionsBody, []byte("value_type")) ||
		!bytes.Contains(connectionsBody, []byte("unit mismatch")) ||
		!bytes.Contains(connectionsBody, []byte("medium mismatch")) {
		t.Fatalf("connection helper module did not expose canvas and Inspector contract labels")
	}
	if !bytes.Contains(body, []byte("renderDiagnostics")) || !bytes.Contains(body, []byte("diagnosticsPanel")) {
		t.Fatalf("module entrypoint did not keep raw result JSON in diagnostics")
	}
	if !bytes.Contains(resultUIBody, []byte("result-help-button")) {
		t.Fatalf("module entrypoint did not render result help links")
	}
	if !bytes.Contains(resultUIBody, []byte("Raw JSON / Diagnostics")) {
		t.Fatalf("module entrypoint did not keep raw JSON behind diagnostics")
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
	if !bytes.Contains(body, []byte("replaceSelectedComponent")) || !bytes.Contains(body, []byte("/api/project/components/replace")) || !bytes.Contains(body, []byte("replaceComponentButton")) {
		t.Fatalf("module entrypoint did not expose model replacement workflow")
	}
	if !bytes.Contains(body, []byte("replacementPreviewBlock")) ||
		!bytes.Contains(body, []byte("replacementMapParameters")) ||
		!bytes.Contains(body, []byte("Replace And Validate")) ||
		!bytes.Contains(replacementPreviewBody, []byte("replacementPreview")) ||
		!bytes.Contains(replacementPreviewBody, []byte("replacementContractDiff")) ||
		!bytes.Contains(replacementPreviewBody, []byte("replacementNodeMappings")) ||
		!bytes.Contains(replacementPreviewBody, []byte("replacementParameterMappings")) {
		t.Fatalf("module entrypoint did not expose replacement mapping preview")
	}
	if !bytes.Contains(body, []byte("createMLComponent")) ||
		!bytes.Contains(body, []byte("newMLComponentButton")) ||
		!bytes.Contains(body, []byte(`createComponent("ml_inference")`)) {
		t.Fatalf("module entrypoint did not expose ML component quick creation")
	}
	if !bytes.Contains(candidateResultsBody, []byte("parameterChangeRows")) {
		t.Fatalf("module entrypoint did not render calibration parameter change rows")
	}
	if !bytes.Contains(validationPlotsBody, []byte("Objective History")) || !bytes.Contains(validationPlotsBody, []byte("candidateObjectiveHistory")) {
		t.Fatalf("module entrypoint did not render candidate objective history")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Apply Parameter Set")) {
		t.Fatalf("module entrypoint did not expose saved calibration parameter-set apply action")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Use for Runs")) || !bytes.Contains(candidateResultsBody, []byte("Revert Active")) {
		t.Fatalf("module entrypoint did not expose calibration parameter-set apply/revert flow")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Validation Before/After")) || !bytes.Contains(validationResultsBody, []byte("calibrationValidationComparisonSection")) {
		t.Fatalf("module entrypoint did not expose calibration before/after validation plots")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Best Candidate")) || !bytes.Contains(candidateResultsBody, []byte("bestCandidateRows")) {
		t.Fatalf("module entrypoint did not render calibration best candidate details")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Compare Existing Set")) || !bytes.Contains(body, []byte("runCalibrationParameterSetComparison")) {
		t.Fatalf("module entrypoint did not expose calibration parameter-set comparison")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Export CSV")) || !bytes.Contains(candidateResultsBody, []byte("downloadCandidateCSV")) {
		t.Fatalf("module entrypoint did not expose candidate CSV export")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Export Report")) || !bytes.Contains(candidateResultsBody, []byte("downloadCandidateReport")) {
		t.Fatalf("module entrypoint did not expose candidate report export")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Best Decision Variables")) || !bytes.Contains(candidateResultsBody, []byte("Output Comparison")) || !bytes.Contains(candidateResultsBody, []byte("Constraint Status")) {
		t.Fatalf("module entrypoint did not render optimization result comparison")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Open Saved Scenario")) ||
		!bytes.Contains(candidateResultsBody, []byte("scenarioIDFromPath")) ||
		!bytes.Contains(body, []byte("loadScenario")) {
		t.Fatalf("module entrypoint did not expose saved optimization scenario action")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Use Saved Parameter Set")) ||
		!bytes.Contains(candidateResultsBody, []byte("Apply Saved Parameter Set")) {
		t.Fatalf("module entrypoint did not expose saved optimization parameter-set actions")
	}
	if !bytes.Contains(candidateResultsBody, []byte("Export SDK Script")) || !bytes.Contains(candidateResultsBody, []byte("downloadOptimizationSDKScript")) || !bytes.Contains(candidateResultsBody, []byte("run_optimization")) {
		t.Fatalf("module entrypoint did not expose optimization SDK script export")
	}
	if !bytes.Contains(body, []byte("exportIncludeRecordsInput")) ||
		!bytes.Contains(body, []byte("include_datasets: includeDatasets")) ||
		!bytes.Contains(body, []byte("include_calibration_setups: includeCalibration")) ||
		!bytes.Contains(body, []byte("include_optimization_setups: includeOptimization")) ||
		!bytes.Contains(body, []byte("include_ml_assets: includeMLAssets")) ||
		!bytes.Contains(body, []byte("include_sdk_examples: includeSDKExamples")) ||
		!bytes.Contains(body, []byte("include_records: includeRecords")) {
		t.Fatalf("module entrypoint did not send runtime export record selection")
	}
	if !bytes.Contains(validationResultsBody, []byte("validationPlotSection")) ||
		!bytes.Contains(validationPlotsBody, []byte("Measured vs Simulated")) ||
		!bytes.Contains(validationPlotsBody, []byte("Residual Histogram")) {
		t.Fatalf("module entrypoint did not render validation plots")
	}
	if !bytes.Contains(artifactResultsBody, []byte("datasetMappingEditorSection")) ||
		!bytes.Contains(artifactResultsBody, []byte("suggested_time_column")) ||
		!bytes.Contains(body, []byte("collectValidationColumnMap")) ||
		!bytes.Contains(body, []byte("unit_hints")) ||
		!bytes.Contains(datasetMappingBody, []byte("datasetTimeColumnSelect")) ||
		!bytes.Contains(datasetMappingBody, []byte("datasetSampleRowPreview")) ||
		!bytes.Contains(datasetMappingBody, []byte("Sample Row Preview")) ||
		!bytes.Contains(datasetMappingBody, []byte("datasetSampleEvaluationPayload")) ||
		!bytes.Contains(datasetMappingBody, []byte("Evaluate Sample")) ||
		!bytes.Contains(datasetMappingBody, []byte("Sample Output Comparison")) ||
		!bytes.Contains(datasetMappingBody, []byte("updateMappingEditorStatus")) ||
		!bytes.Contains(datasetMappingBody, []byte("unit mismatch")) ||
		!bytes.Contains(stylesBody, []byte("mapping-warning")) {
		t.Fatalf("module entrypoint did not expose dataset mapping editor")
	}
	if !bytes.Contains(artifactResultsBody, []byte("datasetResultSection")) ||
		!bytes.Contains(artifactResultsBody, []byte("validationMappingArtifactSection")) ||
		!bytes.Contains(artifactResultsBody, []byte("parameterSetResultSection")) ||
		!bytes.Contains(artifactResultsBody, []byte("Create Mapping")) ||
		!bytes.Contains(artifactResultsBody, []byte("Save Name")) ||
		!bytes.Contains(artifactResultsBody, []byte("Apply to Graph")) {
		t.Fatalf("module entrypoint did not expose structured artifact result renderers")
	}
	if !bytes.Contains(body, []byte("validationComparisonBaseline")) || !bytes.Contains(validationResultsBody, []byte("Parameter Set Comparison")) || !bytes.Contains(body, []byte("compareValidationParameterSet")) {
		t.Fatalf("module entrypoint did not render validation parameter-set comparisons")
	}
	if !bytes.Contains(validationResultsBody, []byte("validationResultActions")) || !bytes.Contains(validationResultsBody, []byte("Create Calibration Setup")) {
		t.Fatalf("module entrypoint did not connect validation results to calibration setup")
	}
	if !bytes.Contains(validationResultsBody, []byte("Compare Parameter Set")) {
		t.Fatalf("module entrypoint did not expose validation parameter-set comparison action")
	}
	if !bytes.Contains(body, []byte("openHighErrorInspection")) || !bytes.Contains(validationResultsBody, []byte("highErrorInspectionSection")) {
		t.Fatalf("module entrypoint did not expose high-error timestep inspection")
	}
	if !bytes.Contains(seriesResultsBody, []byte("downloadSeriesCSV")) ||
		!bytes.Contains(seriesResultsBody, []byte("Export Series JSON")) ||
		!bytes.Contains(seriesResultsBody, []byte("Time Indexed Steps")) {
		t.Fatalf("module entrypoint did not expose series export and step inspection")
	}
	if !bytes.Contains(body, []byte("seriesInputField")) ||
		!bytes.Contains(body, []byte("activeSeriesInputPath")) ||
		!bytes.Contains(body, []byte("input_path")) ||
		!bytes.Contains(body, []byte("runSeriesInputSelect")) {
		t.Fatalf("module entrypoint did not expose series input file selection")
	}
	if !bytes.Contains(body, []byte("runTimeoutField")) {
		t.Fatalf("module entrypoint did not include run timeout control rendering")
	}
	if !bytes.Contains(body, []byte("timeout_ms")) {
		t.Fatalf("module entrypoint did not send run timeout requests")
	}
	if !bytes.Contains(body, []byte("AbortController")) {
		t.Fatalf("module entrypoint did not create abort controllers for runtime requests")
	}
	if !bytes.Contains(body, []byte("cancelActiveRun")) {
		t.Fatalf("module entrypoint did not bind runtime cancellation")
	}
	if !bytes.Contains(body, []byte("retryLastRuntimeAction")) || !bytes.Contains(body, []byte("lastRuntimeAction")) {
		t.Fatalf("module entrypoint did not bind runtime retry")
	}
	if !bytes.Contains(body, []byte("activeRunAbortController")) {
		t.Fatalf("module entrypoint did not track active runtime requests")
	}
	if !bytes.Contains(body, []byte("in progress")) ||
		(!bytes.Contains(body, []byte("Runtime ready")) && !bytes.Contains(startBody, []byte("Runtime ready"))) {
		t.Fatalf("module entrypoint did not expose runtime progress status")
	}
	if !bytes.Contains(logsBody, []byte("logSeverityFilter")) ||
		!bytes.Contains(logsBody, []byte("downloadLogBundle")) ||
		!bytes.Contains(logsBody, []byte("exportLogBundleButton")) ||
		!bytes.Contains(logsBody, []byte("logSourceLocation")) {
		t.Fatalf("module entrypoint did not expose log filtering and export")
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
	if !bytes.Contains(body, []byte("connectionUnitConversionEditor")) ||
		!bytes.Contains(body, []byte("UNIT_CONVERSION_PRESETS")) ||
		!bytes.Contains(body, []byte("/api/project/connections/update")) ||
		!bytes.Contains(body, []byte("connectionUnitConversionPreview")) ||
		!bytes.Contains(connectionsBody, []byte("connectionUnitConversionPresetID")) ||
		!bytes.Contains(connectionsBody, []byte("unitConversionPresetDefinition")) ||
		!bytes.Contains(connectionsBody, []byte("unitConversionInitialNumber")) {
		t.Fatalf("module entrypoint did not expose connection unit conversion editing")
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
	if !bytes.Contains(body, []byte("connectionMediumBadge")) || !bytes.Contains(body, []byte("medium mismatch")) {
		t.Fatalf("module entrypoint did not mirror canvas medium status in the Inspector")
	}
	if !bytes.Contains(body, []byte("connectionContractBadge")) ||
		!bytes.Contains(body, []byte("contract-state")) ||
		!bytes.Contains(body, []byte("value_type")) {
		t.Fatalf("module entrypoint did not mirror connection unit and value_type status in the Inspector")
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
	if !bytes.Contains(body, []byte("newStateUnit")) || !bytes.Contains(body, []byte("newStateDescription")) {
		t.Fatalf("module entrypoint did not include state unit/description creation fields")
	}
	if !bytes.Contains(body, []byte("/api/project/component-contract")) {
		t.Fatalf("module entrypoint did not call component contract endpoint")
	}
	if !bytes.Contains(body, []byte("syncParameterInputs")) {
		t.Fatalf("module entrypoint did not include synchronized parameter input editing")
	}
	if !bytes.Contains(body, []byte("newParameterRole")) ||
		!bytes.Contains(body, []byte("newParameterMin")) ||
		!bytes.Contains(body, []byte("PARAMETER_ROLES")) {
		t.Fatalf("module entrypoint did not include parameter role and bounds creation fields")
	}
	if !bytes.Contains(body, []byte("editableNodeRow")) {
		t.Fatalf("module entrypoint did not include editable node rows")
	}
	if !bytes.Contains(body, []byte("updateNodeFromInspector")) {
		t.Fatalf("module entrypoint did not include node metadata editing")
	}
	if !bytes.Contains(body, []byte(`data-node-field`)) ||
		!bytes.Contains(body, []byte("confirmNodeRename")) ||
		!bytes.Contains(body, []byte("nodeRenameSourceDetails")) ||
		!bytes.Contains(contractAuthoringBody, []byte("new_id")) ||
		!bytes.Contains(nodeImpactBody, []byte("nodeRenameImpact")) ||
		!bytes.Contains(nodeImpactBody, []byte("nodeRenameImpactConfirmText")) {
		t.Fatalf("module entrypoint did not include node rename impact confirmation")
	}
	if !bytes.Contains(body, []byte("node-impact")) ||
		!bytes.Contains(body, []byte("nodeDeleteImpactConfirmText")) ||
		!bytes.Contains(nodeImpactBody, []byte("nodeDeleteImpact")) ||
		!bytes.Contains(nodeImpactBody, []byte("nodeDeleteImpactSummary")) ||
		!bytes.Contains(nodeImpactBody, []byte("Restores")) {
		t.Fatalf("module entrypoint did not include node delete impact preview")
	}
	if !bytes.Contains(body, []byte("newNodeName")) {
		t.Fatalf("module entrypoint did not include detailed node creation fields")
	}
	if !bytes.Contains(body, []byte("newNodePreset")) || !bytes.Contains(body, []byte("NODE_PRESETS")) {
		t.Fatalf("module entrypoint did not include node preset creation fields")
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
	if !bytes.Contains(body, []byte("filteredComponentTemplates")) {
		t.Fatalf("module entrypoint did not include component category/mode filtering")
	}
	if !bytes.Contains(body, []byte("include_in_system")) {
		t.Fatalf("module entrypoint did not send component system inclusion preference")
	}
	if !bytes.Contains(body, []byte("defaultProjectName")) {
		t.Fatalf("module entrypoint did not include in-app project naming")
	}
	if !bytes.Contains(body, []byte("defaultComponentName")) {
		t.Fatalf("module entrypoint did not include in-app component naming")
	}
	if !bytes.Contains(body, []byte("renderStartWorkspace")) {
		t.Fatalf("module entrypoint did not include the Start workspace renderer")
	}
	if !bytes.Contains(startBody, []byte("renderStartWorkflowRows")) ||
		!bytes.Contains(startBody, []byte("workflowReadinessRows")) ||
		!bytes.Contains(startBody, []byte("readinessRow")) {
		t.Fatalf("module entrypoint did not include Start workflow readiness rendering")
	}
	if bytes.Contains(body, []byte("currentProjectSummary")) {
		t.Fatalf("module entrypoint should not reference removed project summary helper")
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
	if !bytes.Contains(body, []byte("/api/project/datasets/import")) {
		t.Fatalf("module entrypoint did not import datasets")
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
	if !bytes.Contains(body, []byte("evaluateDatasetSample")) ||
		!bytes.Contains(body, []byte(`api("/api/run"`)) {
		t.Fatalf("module entrypoint did not evaluate dataset sample rows through the runner")
	}
	if !bytes.Contains(body, []byte("/api/project/validation-mapping/update")) {
		t.Fatalf("module entrypoint did not update validation mappings")
	}
	if !bytes.Contains(body, []byte("copyValidationMapping")) {
		t.Fatalf("module entrypoint did not copy validation mappings")
	}
	if !bytes.Contains(body, []byte("deleteValidationMapping")) {
		t.Fatalf("module entrypoint did not delete validation mappings")
	}
	if !bytes.Contains(validationResultsBody, []byte("validationResultSection")) ||
		!bytes.Contains(validationResultsBody, []byte("validationPlotSection")) ||
		!bytes.Contains(validationPlotsBody, []byte("Measured vs Simulated")) ||
		!bytes.Contains(validationPlotsBody, []byte("Residual Histogram")) ||
		!bytes.Contains(validationResultsBody, []byte("highErrorRows")) ||
		!bytes.Contains(body, []byte("openHighErrorInspection")) ||
		!bytes.Contains(validationResultsBody, []byte("Component Inputs")) ||
		!bytes.Contains(validationResultsBody, []byte("Create Calibration Setup")) {
		t.Fatalf("module entrypoint did not expose validation result plots, high-error inspection, and calibration handoff")
	}
	if !bytes.Contains(body, []byte("/api/project/calibration-setup")) {
		t.Fatalf("module entrypoint did not create calibration setups")
	}
	if !bytes.Contains(calibrationSetupEditorBody, []byte("calibrationSetupEditorSection")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Candidate Parameters")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Differential Evolution")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("differential_evolution")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Least Squares")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("least_squares")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Expected Runs")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("calibration-editor-warning")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Fix invalid parameter bounds")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("Ready to create setup")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("stopping_rules")) ||
		!bytes.Contains(calibrationSetupEditorBody, []byte("objective_outputs")) {
		t.Fatalf("module entrypoint did not expose calibration setup editor")
	}
	if !bytes.Contains(setupEditorUIBody, []byte("defaultCalibrationGridStep")) ||
		!bytes.Contains(setupEditorUIBody, []byte("formatExpectedRunCount")) ||
		!bytes.Contains(setupEditorUIBody, []byte("labeledEditorControl")) {
		t.Fatalf("module entrypoint did not expose shared setup editor UI helpers")
	}
	if !bytes.Contains(workflowCandidatesBody, []byte("calibrationParameterCandidates")) ||
		!bytes.Contains(workflowCandidatesBody, []byte("optimizationDecisionCandidates")) ||
		!bytes.Contains(workflowCandidatesBody, []byte("optimizationPublicOutputs")) ||
		!bytes.Contains(workflowCandidatesBody, []byte(`role === "calibration_target"`)) ||
		!bytes.Contains(workflowCandidatesBody, []byte(`definition.role === "optimization_variable"`)) ||
		!bytes.Contains(workflowCandidatesBody, []byte("system_parameter")) {
		t.Fatalf("module entrypoint did not expose role-aware workflow candidate helpers")
	}
	if !bytes.Contains(body, []byte("/api/project/optimization-setup")) {
		t.Fatalf("module entrypoint did not create optimization setups")
	}
	if !bytes.Contains(optimizationSetupEditorBody, []byte("optimizationSetupEditorSection")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Decision Variables")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Base Input/Scenario")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Differential Evolution")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("differential_evolution")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Custom SDK Script")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("custom_sdk_script")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Estimated Runs")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("optimization-editor-warning")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Fix invalid decision bounds")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Fix invalid constraints")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("Select at least one decision variable")) ||
		!bytes.Contains(optimizationSetupEditorBody, []byte("constraints")) {
		t.Fatalf("module entrypoint did not expose optimization setup editor")
	}
	if !bytes.Contains(body, []byte("/api/project/parameter-set/apply")) {
		t.Fatalf("module entrypoint did not apply parameter sets")
	}
	if !bytes.Contains(resultUIBody, []byte("Raw JSON")) {
		t.Fatalf("module entrypoint did not keep raw workflow JSON available")
	}
	if !bytes.Contains(body, []byte("runCalibrationSetup")) {
		t.Fatalf("module entrypoint did not include calibration setup execution")
	}
	if !bytes.Contains(body, []byte("runOptimizationSetup")) {
		t.Fatalf("module entrypoint did not include optimization setup execution")
	}
	if !bytes.Contains(body, []byte("mlMetadataBlock")) {
		t.Fatalf("module entrypoint did not include ML metadata inspector rendering")
	}
	if !bytes.Contains(body, []byte("mlValidationReportBlock")) ||
		!bytes.Contains(mlInspectorBody, []byte("ML Validation")) ||
		!bytes.Contains(mlInspectorBody, []byte("model_asset_checksum")) {
		t.Fatalf("module entrypoint did not include ML validation report rendering")
	}
	if !bytes.Contains(body, []byte("featurePreviewBlock")) ||
		!bytes.Contains(mlInspectorBody, []byte("Feature Preview")) ||
		!bytes.Contains(mlInspectorBody, []byte("Received Features")) {
		t.Fatalf("module entrypoint did not include feature preview rendering")
	}
	if !bytes.Contains(body, []byte("featureMappingSuggestionBlock")) ||
		!bytes.Contains(mlInspectorBody, []byte("Feature Mapping Suggestion")) ||
		!bytes.Contains(mlInspectorBody, []byte("Connect Feature Mapper")) {
		t.Fatalf("module entrypoint did not include feature mapping suggestions")
	}
	if !bytes.Contains(body, []byte("mlAssetEditorBlock")) ||
		!bytes.Contains(body, []byte("/api/project/components/ml-assets")) ||
		!bytes.Contains(body, []byte("/api/project/components/ml-schema-nodes")) ||
		!bytes.Contains(mlInspectorBody, []byte("Apply Schema Nodes")) ||
		!bytes.Contains(mlInspectorBody, []byte(`mlMetadataField = "required_packages"`)) ||
		!bytes.Contains(mlInspectorBody, []byte(`mlMetadataField = "valid_time_resolution"`)) ||
		!bytes.Contains(mlInspectorBody, []byte(`mlMetadataField = "valid_input_ranges"`)) ||
		!bytes.Contains(mlInspectorBody, []byte("parseValidInputRanges")) ||
		!bytes.Contains(mlInspectorBody, []byte("Input Ranges")) ||
		!bytes.Contains(configBody, []byte("input_scaler_file")) ||
		!bytes.Contains(body, []byte("fileToBase64")) {
		t.Fatalf("module entrypoint did not include ML asset import editing")
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
	if !bytes.Contains(startBody, []byte("Baseline graph parameters")) {
		t.Fatalf("module entrypoint did not expose baseline run parameter state")
	}
	if bytes.Contains(body, []byte("serveButton")) {
		t.Fatalf("module entrypoint should not reserve disabled Serve command state")
	}
	if bytes.Contains(body, []byte(`window.prompt("Project name"`)) || bytes.Contains(body, []byte(`window.prompt("Copy project as"`)) {
		t.Fatalf("module entrypoint should not use prompts for project creation or copy")
	}
	if bytes.Contains(body, []byte(`window.prompt("Component name"`)) {
		t.Fatalf("module entrypoint should not use a prompt for component creation")
	}
	if bytes.Contains(body, []byte(`"planned"`)) {
		t.Fatalf("module entrypoint should not expose planned project types")
	}
	if bytes.Contains(body, []byte(">empty<")) || bytes.Contains(body, []byte(">Empty<")) {
		t.Fatalf("module entrypoint should not render generic empty states")
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
	if len(template.Inputs) != 1 || template.Inputs[0].ID != "value" ||
		len(template.Outputs) != 1 || template.Outputs[0].ID != "result" ||
		template.Parameters["gain"] == nil {
		t.Fatalf("template contract = %#v", template)
	}
	if !hasComponentTemplate(body.Templates, "controller") ||
		!hasComponentTemplate(body.Templates, "stateful") ||
		!hasComponentTemplate(body.Templates, "data_source") ||
		!hasComponentTemplate(body.Templates, "data_sink") ||
		!hasComponentTemplate(body.Templates, "feature_mapper") ||
		!hasComponentTemplate(body.Templates, "ml_inference") ||
		!hasComponentTemplate(body.Templates, "utility") ||
		!hasComponentTemplate(body.Templates, "external_executable") ||
		!hasComponentTemplate(body.Templates, "vectorized") ||
		!hasComponentTemplate(body.Templates, "solver_boundary") ||
		!hasComponentTemplate(body.Templates, "zone_load_ann") {
		t.Fatalf("expected beta component templates, got %#v", body.Templates)
	}
}

func TestFeatureMapperTemplatePreservesFeatureOrder(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	helperBytes, err := os.ReadFile(filepath.Join(repoRoot, "templates", "components", "feature_mapper", "helpers.py"))
	if err != nil {
		t.Fatal(err)
	}
	helper := string(helperBytes)
	last := -1
	for _, feature := range []string{
		"outdoor_temperature_c",
		"return_air_temperature_c",
		"chw_setpoint_c",
		"fan_speed_fraction",
	} {
		index := strings.Index(helper, `"`+feature+`"`)
		if index <= last {
			t.Fatalf("feature order not preserved for %s in:\n%s", feature, helper)
		}
		last = index
	}
	for _, token := range []string{"missing feature input", `"scale"`, `"offset"`, `"min"`, `"max"`} {
		if !strings.Contains(helper, token) {
			t.Fatalf("feature mapper helper did not include %s:\n%s", token, helper)
		}
	}
}

func TestProjectDetailIncludesMLValidationReports(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(repoRoot, "examples", "014_ahu_state_ann", "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	detail := projectDetail(loaded)
	report, ok := detail.MLValidationReports["ahu_state_ann"]
	if !ok {
		t.Fatalf("ML validation reports = %#v", detail.MLValidationReports)
	}
	if report.Dataset != "synthetic_ahu_state_reference" ||
		report.ReportPath != "assets/ahu_state_ann/validation_report.json" ||
		report.FeatureSchemaVersion != "1.0" ||
		report.TimeResolution != "step" {
		t.Fatalf("ML validation report = %#v", report)
	}
	if len(report.ModelAssetChecksum) != 64 {
		t.Fatalf("model checksum = %q", report.ModelAssetChecksum)
	}
	if report.Metrics["supply_air_temperature_c"]["rmse"] == nil || report.Metrics["cooling_power_kw"]["r2"] == nil {
		t.Fatalf("metrics = %#v", report.Metrics)
	}

	composition, err := project.Load(filepath.Join(repoRoot, "examples", "015_rc_ahu_ann_composition", "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	compositionDetail := projectDetail(composition)
	compositionReport, ok := compositionDetail.MLValidationReports["ahu_state_ann"]
	if !ok {
		t.Fatalf("composition ML validation reports = %#v", compositionDetail.MLValidationReports)
	}
	if compositionReport.Dataset != "synthetic commissioning set" ||
		compositionReport.FeatureSchemaVersion != "1.0" ||
		compositionReport.TrainingPeriod != "synthetic commissioning baseline" ||
		compositionReport.ValidationPeriod != "synthetic commissioning set" ||
		compositionReport.TimeResolution != "step" {
		t.Fatalf("composition ML validation report = %#v", compositionReport)
	}
	if len(compositionReport.ModelAssetChecksum) != 64 {
		t.Fatalf("composition model checksum = %q", compositionReport.ModelAssetChecksum)
	}
	if compositionReport.Metrics["supply_air_temperature_c"]["rmse"] == nil || compositionReport.Metrics["coil_load_kw"]["mae"] == nil {
		t.Fatalf("composition metrics = %#v", compositionReport.Metrics)
	}
}

func TestExampleMLComponentMetadataMirrorsGraphContract(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(repoRoot, "examples", "014_ahu_state_ann", "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	graphComponent, found := findComponent(loaded.Graph, "ahu_state_ann")
	if !found || graphComponent.MLMetadata == nil {
		t.Fatalf("graph ML component = %#v found=%v", graphComponent, found)
	}

	metadataBytes, err := os.ReadFile(filepath.Join(repoRoot, "examples", "014_ahu_state_ann", "components", "ahu_state_ann", "component.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadataComponent model.Component
	if err := json.Unmarshal(metadataBytes, &metadataComponent); err != nil {
		t.Fatal(err)
	}
	if metadataComponent.MLMetadata == nil {
		t.Fatalf("component metadata missing ML metadata:\n%s", string(metadataBytes))
	}
	if metadataComponent.MLMetadata.ValidTimeResolution != graphComponent.MLMetadata.ValidTimeResolution {
		t.Fatalf("metadata valid time resolution = %q graph = %q", metadataComponent.MLMetadata.ValidTimeResolution, graphComponent.MLMetadata.ValidTimeResolution)
	}
	for _, feature := range []string{"outdoor_temperature_c", "return_air_temperature_c", "chw_setpoint_c", "fan_speed_fraction"} {
		metadataBounds, ok := metadataComponent.MLMetadata.ValidInputRanges[feature]
		graphBounds, graphOK := graphComponent.MLMetadata.ValidInputRanges[feature]
		if !ok || !graphOK || metadataBounds.Min != graphBounds.Min || metadataBounds.Max != graphBounds.Max {
			t.Fatalf("metadata range for %s = %#v graph = %#v", feature, metadataBounds, graphBounds)
		}
	}
}

func TestExampleComponentMetadataAndGeneratedWrappersMirrorGraphContracts(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectFiles, err := filepath.Glob(filepath.Join(repoRoot, "examples", "*", "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	if len(projectFiles) == 0 {
		t.Fatal("no example projects found")
	}
	for _, projectFile := range projectFiles {
		loaded, err := project.Load(projectFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, component := range loaded.Graph.Components {
			if strings.TrimSpace(component.Source.Metadata) == "" {
				continue
			}
			metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
			if err != nil {
				t.Fatalf("%s %s metadata path: %v", projectFile, component.ID, err)
			}
			actualMetadata, err := os.ReadFile(metadataPath)
			if err != nil {
				t.Fatalf("%s %s metadata: %v", projectFile, component.ID, err)
			}
			expectedPath := filepath.Join(t.TempDir(), "component.json")
			if err := writeComponentMetadataFile(expectedPath, component, classNameFromPath(component.Class)); err != nil {
				t.Fatalf("%s %s expected metadata: %v", projectFile, component.ID, err)
			}
			expectedMetadata, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatal(err)
			}
			if normalizeJSONForComparison(t, actualMetadata) != normalizeJSONForComparison(t, expectedMetadata) {
				t.Fatalf("%s %s component metadata is not synced with graph contract\nactual:\n%s\nexpected:\n%s", projectFile, component.ID, string(actualMetadata), string(expectedMetadata))
			}

			if componentUsesGeneratedPythonWrapper(component) {
				wrapperPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Wrapper)
				if err != nil {
					t.Fatalf("%s %s wrapper path: %v", projectFile, component.ID, err)
				}
				actualWrapper, err := os.ReadFile(wrapperPath)
				if err != nil {
					t.Fatalf("%s %s wrapper: %v", projectFile, component.ID, err)
				}
				expectedWrapper := generatedWrapperContent(component)
				if string(actualWrapper) != expectedWrapper {
					t.Fatalf("%s %s generated wrapper is not synced with graph contract\nactual:\n%s\nexpected:\n%s", projectFile, component.ID, string(actualWrapper), expectedWrapper)
				}
			}
		}
	}
}

func normalizeJSONForComparison(t *testing.T, data []byte) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(normalized)
}

func TestProjectDetailIncludesSeriesInputs(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(repoRoot, "examples", "004_stateful_controller", "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	detail := projectDetail(loaded)
	if len(detail.SeriesInputs) != 1 {
		t.Fatalf("series inputs = %#v", detail.SeriesInputs)
	}
	series := detail.SeriesInputs[0]
	if series.RelativePath != "inputs/series01.json" || series.StepCount != 3 || series.TimeKey != "context.time" {
		t.Fatalf("series input summary = %#v", series)
	}
	if !containsString(series.BaseContextKeys, "dt") || !containsString(series.StepContextKeys, "time") {
		t.Fatalf("series context keys = base %#v step %#v", series.BaseContextKeys, series.StepContextKeys)
	}
}

func TestStaticExportWorkspaceModuleServes(t *testing.T) {
	server := newTestServer(t)
	body := getRouteBody(t, server, "/js/export-workspace.js")
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
	if !bytes.Contains(body, []byte("Include datasets")) ||
		!bytes.Contains(body, []byte("Include SDK examples")) ||
		!bytes.Contains(body, []byte("Include records")) ||
		!bytes.Contains(body, []byte("exportIncludeRecordsInput")) {
		t.Fatalf("export workspace module did not render record selection summary")
	}
	if !bytes.Contains(body, []byte("Records")) {
		t.Fatalf("export workspace module did not render record count")
	}
}

func TestStaticRunOutputModuleServes(t *testing.T) {
	server := newTestServer(t)
	body := getRouteBody(t, server, "/js/run-output.js")
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
	if !bytes.Contains(body, []byte("downloadRunResultCSV")) ||
		!bytes.Contains(body, []byte("Export Result JSON")) ||
		!bytes.Contains(body, []byte("hvac-studio.run-result-export.v1")) {
		t.Fatalf("run output module did not expose run result CSV/JSON exports")
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
	if !bytes.Contains(body, []byte("logSourceLocation")) || !bytes.Contains(body, []byte("log.time")) {
		t.Fatalf("run output module did not render component log time/source context")
	}
	if !bytes.Contains(body, []byte("failureSummaryRows")) {
		t.Fatalf("run output module did not render failed run summaries")
	}
	if !bytes.Contains(body, []byte("pendingRunSummaryRows")) {
		t.Fatalf("run output module did not render pending run context")
	}
	if !bytes.Contains(body, []byte("Runtime request")) {
		t.Fatalf("run output module did not render runtime progress context")
	}
	if !bytes.Contains(body, []byte("Baseline graph parameters")) {
		t.Fatalf("run output module did not expose baseline parameter context")
	}
	if !bytes.Contains(body, []byte("Save target")) {
		t.Fatalf("run output module did not expose run write target")
	}
	if !bytes.Contains(body, []byte("Output path")) {
		t.Fatalf("run output module did not expose saved run output path")
	}
	if !bytes.Contains(body, []byte("Series time key")) ||
		!bytes.Contains(body, []byte("activeSeriesInputSummary")) {
		t.Fatalf("run output module did not expose series input summary context")
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
	if !bytes.Contains(body, []byte("source_value")) || !bytes.Contains(body, []byte("converted_value")) {
		t.Fatalf("run output module did not render converted connection trace values")
	}
}
