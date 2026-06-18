package studio

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

//go:embed static
var staticFS embed.FS

func StaticAssets() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}

type Server struct {
	repoRoot string
	mux      *http.ServeMux
}

type ProjectSummary struct {
	Name         string `json:"name"`
	ProjectPath  string `json:"project_path"`
	RelativePath string `json:"relative_path"`
	Source       string `json:"source"`
}

type ProjectDetail struct {
	Project             *model.Project                  `json:"project"`
	Graph               *model.Graph                    `json:"graph"`
	ProjectPath         string                          `json:"project_path"`
	GraphPath           string                          `json:"graph_path"`
	DefaultInputPath    string                          `json:"default_input_path"`
	DefaultRunInput     *runtimecore.RunInput           `json:"default_run_input"`
	Layout              StudioLayout                    `json:"layout"`
	Root                string                          `json:"root"`
	Runs                []RunSummary                    `json:"runs"`
	Batches             []BatchSummary                  `json:"batches"`
	Exports             []ExportSummary                 `json:"exports"`
	Scenarios           []ScenarioSummary               `json:"scenarios"`
	Datasets            []DatasetSummary                `json:"datasets"`
	SeriesInputs        []SeriesInputSummary            `json:"series_inputs"`
	ParameterSets       []ParameterSetSummary           `json:"parameter_sets"`
	ValidationMappings  []ValidationMappingSummary      `json:"validation_mappings"`
	CalibrationSetups   []CalibrationSetupSummary       `json:"calibration_setups"`
	OptimizationSetups  []OptimizationSetupSummary      `json:"optimization_setups"`
	ValidationRuns      []modelvalidation.RecordSummary `json:"validation_runs"`
	CalibrationResults  []calibration.RecordSummary     `json:"calibration_results"`
	OptimizationResults []optimization.RecordSummary    `json:"optimization_results"`
	MLValidationReports map[string]MLValidationSummary  `json:"ml_validation_reports,omitempty"`
}

type MLValidationSummary struct {
	ComponentID          string                    `json:"component_id"`
	ReportPath           string                    `json:"report_path"`
	Dataset              string                    `json:"dataset,omitempty"`
	Metrics              map[string]map[string]any `json:"metrics,omitempty"`
	FeatureSchemaVersion string                    `json:"feature_schema_version,omitempty"`
	ModelAssetChecksum   string                    `json:"model_asset_checksum,omitempty"`
	TrainingPeriod       string                    `json:"training_period,omitempty"`
	ValidationPeriod     string                    `json:"validation_period,omitempty"`
	TimeResolution       string                    `json:"time_resolution,omitempty"`
}

type StudioLayout struct {
	Components map[string]CanvasPosition `json:"components"`
}

type CanvasPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ExportSummary struct {
	Profile      string `json:"profile"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type ExportManifest struct {
	Profile             string                `json:"profile"`
	CreatedAtUTC        string                `json:"created_at_utc"`
	ProjectName         string                `json:"project_name"`
	ProjectRoot         string                `json:"project_root"`
	ProjectPath         string                `json:"project_path"`
	GraphPath           string                `json:"graph_path"`
	DefaultInput        string                `json:"default_input"`
	EnvironmentLockfile string                `json:"environment_lockfile"`
	InterfaceSchema     string                `json:"interface_schema"`
	Runner              string                `json:"runner"`
	RuntimePython       string                `json:"runtime_python"`
	Files               []string              `json:"files"`
	Components          []string              `json:"components"`
	ModelAssets         []string              `json:"model_assets,omitempty"`
	MLValidationReports []MLValidationSummary `json:"ml_validation_reports,omitempty"`
	Checksums           map[string]string     `json:"checksums,omitempty"`
	PublicInputs        []model.PublicNodeRef `json:"public_inputs"`
	PublicOutputs       []model.PublicNodeRef `json:"public_outputs"`
	ExecutionOrder      []string              `json:"execution_order"`
	ParameterSets       []string              `json:"parameter_sets,omitempty"`
	Datasets            []string              `json:"datasets,omitempty"`
	ValidationMappings  []string              `json:"validation_mappings,omitempty"`
	CalibrationSetups   []string              `json:"calibration_setups,omitempty"`
	OptimizationSetups  []string              `json:"optimization_setups,omitempty"`
	RunRecords          []string              `json:"run_records,omitempty"`
	BatchRecords        []string              `json:"batch_records,omitempty"`
	ValidationRecords   []string              `json:"validation_records,omitempty"`
	CalibrationRecords  []string              `json:"calibration_records,omitempty"`
	OptimizationRecords []string              `json:"optimization_records,omitempty"`
	Commands            []string              `json:"commands,omitempty"`
	IncludeDatasets     bool                  `json:"include_datasets"`
	IncludeCalibration  bool                  `json:"include_calibration_setups"`
	IncludeOptimization bool                  `json:"include_optimization_setups"`
	IncludeMLAssets     bool                  `json:"include_ml_assets"`
	IncludeSDKExamples  bool                  `json:"include_sdk_examples"`
	IncludeRecords      bool                  `json:"include_records"`
}

type SourceDetail struct {
	ComponentID  string `json:"component_id"`
	RelativePath string `json:"relative_path"`
	Layout       string `json:"layout"`
	EditableRole string `json:"editable_role"`
	Content      string `json:"content"`
	ReadOnly     bool   `json:"read_only"`
}

type SourceCheck struct {
	OK               bool      `json:"ok"`
	ComponentID      string    `json:"component_id"`
	RelativePath     string    `json:"relative_path"`
	ExpectedClass    string    `json:"expected_class"`
	ExpectedFunction string    `json:"expected_function,omitempty"`
	LineCount        int       `json:"line_count"`
	Problems         []Problem `json:"problems"`
}

type Problem struct {
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	ComponentID string `json:"component_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Source      string `json:"source,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

type apiError struct {
	OK       bool             `json:"ok"`
	Error    apperror.Payload `json:"error"`
	Code     int              `json:"code"`
	Kind     string           `json:"kind"`
	Message  string           `json:"message"`
	Problems []Problem        `json:"problems,omitempty"`
}

func New(repoRoot string) (*Server, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	assets, err := StaticAssets()
	if err != nil {
		return nil, err
	}

	server := &Server{
		repoRoot: absRoot,
		mux:      http.NewServeMux(),
	}
	server.routes(http.FileServer(http.FS(assets)))
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes(staticHandler http.Handler) {
	s.mux.HandleFunc("GET /api/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	s.mux.HandleFunc("POST /api/projects/copy", s.handleCopyProject)
	s.mux.HandleFunc("GET /api/project", s.handleProject)
	s.mux.HandleFunc("GET /api/project/run", s.handleRunRecord)
	s.mux.HandleFunc("GET /api/project/batch", s.handleBatchRecord)
	s.mux.HandleFunc("GET /api/project/scenario", s.handleScenarioRecord)
	s.mux.HandleFunc("GET /api/project/export", s.handleExportRecord)
	s.mux.HandleFunc("GET /api/project/dataset", s.handleDatasetPreview)
	s.mux.HandleFunc("GET /api/project/parameter-set", s.handleParameterSetDetail)
	s.mux.HandleFunc("GET /api/project/validation-record", s.handleValidationRecord)
	s.mux.HandleFunc("GET /api/project/calibration-record", s.handleCalibrationRecord)
	s.mux.HandleFunc("GET /api/project/optimization-record", s.handleOptimizationRecord)
	s.mux.HandleFunc("GET /api/project/source", s.handleSource)
	s.mux.HandleFunc("GET /api/component-templates", s.handleComponentTemplates)
	s.mux.HandleFunc("POST /api/project/source/check", s.handleCheckSource)
	s.mux.HandleFunc("POST /api/project/datasets/import", s.handleImportDataset)
	s.mux.HandleFunc("POST /api/project/components", s.handleCreateComponent)
	s.mux.HandleFunc("POST /api/project/components/duplicate", s.handleDuplicateComponent)
	s.mux.HandleFunc("POST /api/project/components/replace", s.handleReplaceComponent)
	s.mux.HandleFunc("POST /api/project/components/update", s.handleUpdateComponent)
	s.mux.HandleFunc("POST /api/project/components/ml-assets", s.handleUpdateComponentMLAssets)
	s.mux.HandleFunc("POST /api/project/components/ml-schema-nodes", s.handleApplyComponentMLSchemaNodes)
	s.mux.HandleFunc("POST /api/project/components/delete", s.handleDeleteComponent)
	s.mux.HandleFunc("POST /api/project/system/components", s.handleIncludeComponent)
	s.mux.HandleFunc("POST /api/project/system/components/remove", s.handleRemoveComponentFromSystem)
	s.mux.HandleFunc("POST /api/project/nodes", s.handleCreateNode)
	s.mux.HandleFunc("POST /api/project/nodes/update", s.handleUpdateNode)
	s.mux.HandleFunc("POST /api/project/nodes/delete", s.handleDeleteNode)
	s.mux.HandleFunc("POST /api/project/connections", s.handleCreateConnection)
	s.mux.HandleFunc("POST /api/project/connections/update", s.handleUpdateConnection)
	s.mux.HandleFunc("POST /api/project/connections/delete", s.handleDeleteConnection)
	s.mux.HandleFunc("POST /api/project/layout", s.handleUpdateLayout)
	s.mux.HandleFunc("POST /api/project/input", s.handleUpdateInput)
	s.mux.HandleFunc("POST /api/project/parameters", s.handleUpdateParameters)
	s.mux.HandleFunc("POST /api/project/component-contract", s.handleUpdateComponentContract)
	s.mux.HandleFunc("POST /api/project/parameter-set/apply", s.handleApplyParameterSet)
	s.mux.HandleFunc("POST /api/project/parameters/delete", s.handleDeleteParameter)
	s.mux.HandleFunc("POST /api/project/source", s.handleUpdateSource)
	s.mux.HandleFunc("GET /api/project/validation-mapping", s.handleValidationMappingDetail)
	s.mux.HandleFunc("POST /api/project/validation-mapping", s.handleCreateValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/update", s.handleUpdateValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/copy", s.handleCopyValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/delete", s.handleDeleteValidationMapping)
	s.mux.HandleFunc("POST /api/project/calibration-setup", s.handleCreateCalibrationSetup)
	s.mux.HandleFunc("POST /api/project/optimization-setup", s.handleCreateOptimizationSetup)
	s.mux.HandleFunc("POST /api/project/scenarios", s.handleCreateScenario)
	s.mux.HandleFunc("POST /api/validate", s.handleValidate)
	s.mux.HandleFunc("POST /api/run", s.handleRun)
	s.mux.HandleFunc("POST /api/run-series", s.handleRunSeries)
	s.mux.HandleFunc("POST /api/batch", s.handleBatch)
	s.mux.HandleFunc("POST /api/validation/run", s.handleDataValidation)
	s.mux.HandleFunc("POST /api/calibration/run", s.handleCalibrationRun)
	s.mux.HandleFunc("POST /api/optimization/run", s.handleOptimizationRun)
	s.mux.HandleFunc("POST /api/schema", s.handleSchema)
	s.mux.HandleFunc("POST /api/export", s.handleExport)
	s.mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir(filepath.Join(s.repoRoot, "docs")))))
	s.mux.Handle("/", staticHandler)
}
