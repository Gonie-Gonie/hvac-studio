package studio

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
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

type SeriesInputSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RelativePath    string   `json:"relative_path"`
	StepCount       int      `json:"step_count"`
	TimeKey         string   `json:"time_key"`
	BaseContextKeys []string `json:"base_context_keys,omitempty"`
	StepContextKeys []string `json:"step_context_keys,omitempty"`
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

type ParameterSetSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RelativePath   string `json:"relative_path"`
	CreatedAtUTC   string `json:"created_at_utc"`
	ComponentCount int    `json:"component_count"`
	ParameterCount int    `json:"parameter_count"`
}

type ParameterSetDetail struct {
	Summary     ParameterSetSummary `json:"summary"`
	Set         parameterset.Set    `json:"set"`
	Differences []ParameterDiff     `json:"differences"`
}

type ParameterDiff struct {
	Component string `json:"component"`
	Parameter string `json:"parameter"`
	Baseline  any    `json:"baseline,omitempty"`
	Value     any    `json:"value"`
	Exists    bool   `json:"exists"`
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

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects := []ProjectSummary{}
	projects = append(projects, s.findProjectSummaries(filepath.Join(s.repoRoot, "projects"), "workspace")...)
	projects = append(projects, s.findProjectSummaries(filepath.Join(s.repoRoot, "examples"), "example")...)
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Source != projects[j].Source {
			return projects[i].Source == "workspace"
		}
		return projects[i].RelativePath < projects[j].RelativePath
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "projects": projects})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateProjectRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	summary, err := s.createProject(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "project": summary})
}

func (s *Server) handleCopyProject(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCopyProjectRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	summary, err := s.copyProject(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "project": summary})
}

func (s *Server) handleRunRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadRunRecord(loaded.Root, r.URL.Query().Get("run_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "run_record": record})
}

func (s *Server) handleBatchRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadBatchRecord(loaded.Root, r.URL.Query().Get("batch_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "batch_record": record})
}

func (s *Server) handleScenarioRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadScenarioRecord(loaded.Root, r.URL.Query().Get("scenario_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "scenario": record})
}

func (s *Server) handleExportRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	summary, manifest, err := loadExportManifest(loaded.Root, r.URL.Query().Get("profile"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "export": manifest})
}

func (s *Server) handleDatasetPreview(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	preview, err := datasetPreview(loaded, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dataset": preview})
}

func (s *Server) handleImportDataset(w http.ResponseWriter, r *http.Request) {
	req, err := decodeImportDatasetRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	preview, err := importDataset(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "dataset": preview, "summary": preview.Summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleParameterSetDetail(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	detail, err := parameterSetDetail(loaded, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parameter_set": detail})
}

func (s *Server) handleValidationMappingDetail(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping": mapping})
}

func (s *Server) handleCreateValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := createValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := updateValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleCopyValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCopyValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := copyValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	deletedPath, err := deleteValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping_path": deletedPath, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateCalibrationSetup(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateCalibrationSetupRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, setup, err := createCalibrationSetup(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "setup": setup, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateOptimizationSetup(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateOptimizationSetupRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, setup, err := createOptimizationSetup(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "setup": setup, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleValidationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := modelvalidation.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "validation_record": record})
}

func (s *Server) handleCalibrationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := calibration.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "calibration_record": record})
}

func (s *Server) handleOptimizationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := optimization.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "optimization_record": record})
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	source, err := loadComponentSource(loaded, r.URL.Query().Get("component_id"), s.ensureWorkspaceProject(loaded.Root) != nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
}

func (s *Server) handleComponentTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := listComponentTemplates(s.repoRoot)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "templates": templates})
}

func (s *Server) handleCheckSource(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSourceCheckRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	check, err := checkComponentSource(r.Context(), loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "check": check})
}

func (s *Server) handleCreateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := createComponent(loaded, req, s.repoRoot)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.IncludeInSystem {
		if err := includeComponentInSystem(loaded, includeComponentRequest{
			ProjectPath: req.ProjectPath,
			SystemID:    loaded.Project.EntrySystem,
			ComponentID: component.ID,
		}); err != nil {
			writeError(w, err)
			return
		}
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleDuplicateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDuplicateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := duplicateComponent(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleReplaceComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeReplaceComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, summary, problems, err := replaceComponent(loaded, req, s.repoRoot)
	if err != nil {
		writeErrorWithProblems(w, err, problems)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "replacement": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := updateComponent(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponentMLAssets(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentMLAssetsRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, importedFiles, err := updateComponentMLAssets(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "imported_files": importedFiles, "project": projectDetail(reloaded)})
}

func (s *Server) handleApplyComponentMLSchemaNodes(w http.ResponseWriter, r *http.Request) {
	req, err := decodeApplyComponentMLSchemaNodesRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, summary, err := applyComponentMLSchemaNodes(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := deleteComponent(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleIncludeComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeIncludeComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := includeComponentInSystem(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleRemoveComponentFromSystem(w http.ResponseWriter, r *http.Request) {
	req, err := decodeIncludeComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := removeComponentFromSystem(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := createNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := updateNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := deleteNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := createConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := updateConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := deleteConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateLayout(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateLayoutRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := writeStudioLayout(loaded, req.Components); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"project": projectDetail(loaded),
	})
}

func (s *Server) handleUpdateParameters(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateParametersRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if len(req.Parameters) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "parameters are required"))
		return
	}

	for componentID, parameters := range req.Parameters {
		if strings.TrimSpace(componentID) == "" {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "component id is required"))
			return
		}
		found := false
		for componentIndex := range loaded.Graph.Components {
			component := &loaded.Graph.Components[componentIndex]
			if component.ID != componentID {
				continue
			}
			found = true
			if component.Parameters == nil {
				component.Parameters = map[string]any{}
			}
			for name, value := range parameters {
				if strings.TrimSpace(name) == "" {
					writeError(w, apperror.Errorf(apperror.CodeValidation, "parameter name is required"))
					return
				}
				component.Parameters[name] = value
				if component.ParameterDefinitions != nil {
					if definition, exists := component.ParameterDefinitions[name]; exists {
						definition.Current = value
						component.ParameterDefinitions[name] = definition
					}
				}
			}
			if err := syncComponentMetadataFile(loaded, *component); err != nil {
				writeError(w, err)
				return
			}
			break
		}
		if !found {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID))
			return
		}
	}

	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponentContract(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentContractRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := updateComponentContract(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded), "component": component})
}

func (s *Server) handleApplyParameterSet(w http.ResponseWriter, r *http.Request) {
	req, err := decodeApplyParameterSetRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	set, err := parameterset.ApplyFile(loaded, req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded), "parameter_set": set})
}

func (s *Server) handleDeleteParameter(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteParameterRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := deleteParameter(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateInput(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateInputRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if req.Inputs == nil {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "inputs are required"))
		return
	}
	inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput)
	if err != nil {
		writeError(w, err)
		return
	}
	input := runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSourceRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	sourcePath, err := componentSourcePath(loaded, req.ComponentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := os.WriteFile(sourcePath, []byte(req.Content), 0o644); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	source, err := loadComponentSource(loaded, req.ComponentID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	check, err := checkComponentSource(r.Context(), loaded, sourceCheckRequest{
		ProjectPath: req.ProjectPath,
		ComponentID: req.ComponentID,
		Content:     req.Content,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source, "check": check})
}

func (s *Server) handleCreateScenario(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateScenarioRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, record, err := writeScenarioRecord(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "summary": summary, "scenario": record})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		writeErrorWithProblems(w, apperror.Wrap(apperror.CodeValidation, err), inferProblems(loaded, err))
		return
	}
	problems := compilerDiagnosticsProblems(plan.Diagnostics)
	sourceCheckCount, sourceProblems := checkProjectSources(r.Context(), loaded)
	if hasErrorProblems(sourceProblems) {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), sourceProblems)
		return
	}
	problems = append(problems, sourceProblems...)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"validation": map[string]any{
			"project_name":     loaded.Project.ProjectName,
			"entry_system":     loaded.Project.EntrySystem,
			"component_count":  len(plan.System.Components),
			"connection_count": len(plan.System.Connections),
			"execution_order":  plan.Order,
			"source_checks":    sourceCheckCount,
			"problems":         problems,
		},
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}

	input := runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context}
	if input.Inputs == nil {
		input, err = runtimecore.LoadInput(resolveProjectFile(loaded.Root, loaded.Project.DefaultInput))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeInput, err))
			return
		}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, 30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.Run(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "run", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "result": result}
	if req.Save {
		runRecord, err := writeRunRecord(loaded, input, result, filepath.ToSlash(req.ParameterSetPath))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
			return
		}
		response["run_record"] = runRecord
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRunSeries(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSeriesRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	input := runtimecore.SeriesInput{
		SchemaVersion: req.SchemaVersion,
		Context:       req.Context,
		Steps:         req.Steps,
	}
	if req.InputPath != "" {
		input, err = runtimecore.LoadSeriesInput(resolveProjectFile(loaded.Root, req.InputPath))
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if len(input.Steps) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeInput, "series input requires steps"))
		return
	}

	timeout, err := requestTimeout(apiRequest{TimeoutMS: req.TimeoutMS}, time.Duration(len(input.Steps))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.RunSeries(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "series", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	scenarios := loadScenarioSummaries(loaded.Root)
	if len(scenarios) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "batch requires at least one saved scenario"))
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, time.Duration(len(scenarios))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	cases := make([]BatchCaseRecord, 0, len(scenarios))
	for index := len(scenarios) - 1; index >= 0; index-- {
		scenario, err := loadScenarioRecord(loaded.Root, scenarios[index].ID)
		if err != nil {
			writeError(w, err)
			return
		}
		input := runtimecore.RunInput{Inputs: scenario.Inputs, Context: scenario.Context}
		if input.Context == nil {
			input.Context = map[string]any{}
		}
		caseRecord := BatchCaseRecord{
			ScenarioID:   scenario.ID,
			ScenarioName: scenario.Name,
			Inputs:       input.Inputs,
			Context:      input.Context,
		}
		result, err := runtimecore.Run(ctx, loaded, input)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeTimeoutError(w, "batch", timeout)
				return
			}
			caseRecord.Error = err.Error()
			caseRecord.Problems = inferProblems(loaded, err)
		} else {
			if req.ParameterSetPath != "" {
				result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
			}
			caseRecord.OK = true
			caseRecord.Result = result
		}
		cases = append(cases, caseRecord)
	}
	summary, record, err := writeBatchRecord(loaded, cases, filepath.ToSlash(req.ParameterSetPath))
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "batch": record})
}

func (s *Server) handleDataValidation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeValidationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}
	if req.MappingPath == "" {
		mappings := loadValidationMappingSummaries(loaded.Root)
		if len(mappings) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "data validation requires a saved mapping"))
			return
		}
		req.MappingPath = mappings[0].RelativePath
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, req.MappingPath)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: req.HighErrorRows})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "validation_result": result}
	if req.Save {
		summary, err := modelvalidation.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["validation_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCalibrationRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCalibrationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadCalibrationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "calibration requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := calibration.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveParameterSet == "" {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_calibrated.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := calibration.Run(ctx, loaded.Path, setup, calibration.Options{SaveParameterSet: req.SaveParameterSet})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "calibration_result": result}
	if req.Save {
		summary, err := calibration.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["calibration_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOptimizationRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeOptimizationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadOptimizationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "optimization requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := optimization.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveScenario == "" {
		req.SaveScenario = filepath.ToSlash(filepath.Join("scenarios", setup.ID+"_optimized.json"))
	}
	if req.Save && req.SaveParameterSet == "" && optimizationSetupHasParameterVariables(setup) {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_optimized.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := optimization.Run(ctx, loaded.Path, setup, optimization.Options{
		SaveScenario:     req.SaveScenario,
		SaveParameterSet: req.SaveParameterSet,
	})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "optimization_result": result}
	if req.Save {
		summary, err := optimization.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["optimization_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schema": schema})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	req, err := decodeExportRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	summary, manifest, err := writeExportManifest(loaded, req.Profile, exportOptionsFromRequest(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "export": manifest})
}

func studioNumberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func loadParameterSetSummaries(projectRoot string) []ParameterSetSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "parameter_sets"), []string{"*.json"})
	summaries := []ParameterSetSummary{}
	for _, path := range files {
		summary, ok := parameterSetSummary(projectRoot, path)
		if ok {
			summaries = append(summaries, summary)
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadSeriesInputSummaries(projectRoot string) []SeriesInputSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "inputs"), []string{"*.json"})
	summaries := []SeriesInputSummary{}
	for _, path := range files {
		input, err := runtimecore.LoadSeriesInput(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, path)
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		summaries = append(summaries, SeriesInputSummary{
			ID:              id,
			Name:            displayNameFromID(id),
			RelativePath:    filepath.ToSlash(rel),
			StepCount:       len(input.Steps),
			TimeKey:         seriesInputTimeKey(input),
			BaseContextKeys: sortedMapKeys(input.Context),
			StepContextKeys: seriesStepContextKeys(input.Steps),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func seriesInputTimeKey(input runtimecore.SeriesInput) string {
	if _, ok := input.Context["time"]; ok {
		return "context.time"
	}
	for _, step := range input.Steps {
		if _, ok := step.Context["time"]; ok {
			return "context.time"
		}
	}
	return "step index"
}

func seriesStepContextKeys(steps []runtimecore.SeriesStep) []string {
	keys := map[string]bool{}
	for _, step := range steps {
		for key := range step.Context {
			keys[key] = true
		}
	}
	return sortedMapKeys(keys)
}

func appendMatchingFiles(root string, patterns []string) []string {
	files := []string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files
}

func parameterSetSummary(projectRoot string, path string) (ParameterSetSummary, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParameterSetSummary{}, false
	}
	var record struct {
		ID           string                    `json:"id"`
		Name         string                    `json:"name"`
		CreatedAtUTC string                    `json:"created_at_utc"`
		Components   map[string]map[string]any `json:"components"`
		Parameters   map[string]any            `json:"parameters"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return ParameterSetSummary{}, false
	}
	id := record.ID
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	name := record.Name
	if name == "" {
		name = displayNameFromID(id)
	}
	parameterCount := 0
	for _, params := range record.Components {
		parameterCount += len(params)
	}
	if parameterCount == 0 && record.Parameters != nil {
		parameterCount = len(record.Parameters)
	}
	rel, _ := filepath.Rel(projectRoot, path)
	return ParameterSetSummary{
		ID:             id,
		Name:           name,
		RelativePath:   filepath.ToSlash(rel),
		CreatedAtUTC:   record.CreatedAtUTC,
		ComponentCount: len(record.Components),
		ParameterCount: parameterCount,
	}, true
}

func parameterSetDetail(loaded *project.LoadedProject, relativePath string) (ParameterSetDetail, error) {
	set, err := parameterset.Load(loaded.Root, relativePath)
	if err != nil {
		return ParameterSetDetail{}, err
	}
	resolved, err := resolveProjectOwnedFile(loaded.Root, relativePath)
	if err != nil {
		return ParameterSetDetail{}, err
	}
	summary, ok := parameterSetSummary(loaded.Root, resolved)
	if !ok {
		return ParameterSetDetail{}, apperror.Errorf(apperror.CodeInput, "parameter set could not be summarized: %s", relativePath)
	}
	return ParameterSetDetail{
		Summary:     summary,
		Set:         set,
		Differences: parameterSetDifferences(loaded.Graph, set),
	}, nil
}

func parameterSetDifferences(graph *model.Graph, set parameterset.Set) []ParameterDiff {
	components := map[string]model.Component{}
	for _, component := range graph.Components {
		components[component.ID] = component
	}
	diffs := []ParameterDiff{}
	for componentID, values := range set.Components {
		component, componentExists := components[componentID]
		for name, value := range values {
			baseline, exists := component.Parameters[name]
			diffs = append(diffs, ParameterDiff{
				Component: componentID,
				Parameter: name,
				Baseline:  baseline,
				Value:     value,
				Exists:    componentExists && exists,
			})
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Component != diffs[j].Component {
			return diffs[i].Component < diffs[j].Component
		}
		return diffs[i].Parameter < diffs[j].Parameter
	})
	return diffs
}

func writeExportManifest(loaded *project.LoadedProject, profile string, options exportOptions) (ExportSummary, ExportManifest, error) {
	if profile == "" {
		profile = "runtime_package"
	}
	if profile != "runtime_package" && profile != "research_project" {
		return ExportSummary{}, ExportManifest{}, apperror.Errorf(apperror.CodeValidation, "unsupported export profile: %s", profile)
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	now := time.Now().UTC()
	projectPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.Path)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	graphPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.GraphPath)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	defaultInputPath := ""
	if loaded.Project.DefaultInput != "" {
		defaultInputPath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.DefaultInput)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	environmentLockfilePath := ""
	if loaded.Project.Environment.Lockfile != "" {
		environmentLockfilePath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.Environment.Lockfile)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	exportRoot := filepath.Join(loaded.Root, "exports", profile)
	if err := resetGeneratedDir(filepath.Join(loaded.Root, "exports"), exportRoot); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files, err := writeRuntimeExportProject(loaded, filepath.Join(exportRoot, "project"), options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	supportFiles, err := writeRuntimeExportSupportFiles(loaded.Root, exportRoot, options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, supportFiles...)
	interfaceSchemaPath := "schema/public-io.json"
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := schemaexport.Write(filepath.Join(exportRoot, filepath.FromSlash(interfaceSchemaPath)), schema); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, interfaceSchemaPath)
	entrypoints := runtimeExportEntrypoints(files, plan, exportArtifactPath(projectPath), exportArtifactPath(defaultInputPath), exportArtifactPath(environmentLockfilePath), options)
	entrypointFiles, err := writeRuntimeExportEntrypoints(exportRoot, entrypoints)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, entrypointFiles...)
	sort.Strings(files)
	checksums, err := exportFileChecksums(exportRoot, files)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	commands := []string{}
	for _, entrypoint := range entrypoints {
		if strings.HasSuffix(entrypoint.Rel, ".ps1") {
			commands = append(commands, entrypoint.Rel)
		}
	}
	manifest := ExportManifest{
		Profile:             profile,
		CreatedAtUTC:        now.Format(time.RFC3339Nano),
		ProjectName:         loaded.Project.ProjectName,
		ProjectRoot:         "project",
		ProjectPath:         exportArtifactPath(projectPath),
		GraphPath:           exportArtifactPath(graphPath),
		DefaultInput:        exportArtifactPath(defaultInputPath),
		EnvironmentLockfile: exportArtifactPath(environmentLockfilePath),
		InterfaceSchema:     interfaceSchemaPath,
		Runner:              "bin/bcs-runner.exe",
		RuntimePython:       "runtime/python/python.exe",
		Files:               files,
		Components:          append([]string{}, plan.System.Components...),
		ModelAssets:         modelAssetExportPaths(loaded.Graph, options.IncludeMLAssets),
		MLValidationReports: mlValidationSummaries(loaded, options.IncludeMLAssets, true),
		Checksums:           checksums,
		PublicInputs:        append([]model.PublicNodeRef{}, plan.System.PublicInputs...),
		PublicOutputs:       append([]model.PublicNodeRef{}, plan.System.PublicOutputs...),
		ExecutionOrder:      append([]string{}, plan.Order...),
		ParameterSets:       exportFilesWithPrefix(files, "project/parameter_sets/"),
		Datasets:            exportFilesWithPrefix(files, "project/datasets/"),
		ValidationMappings:  exportFilesWithPrefix(files, "project/validation/mappings/"),
		CalibrationSetups:   exportFilesWithPrefix(files, "project/calibration/setups/"),
		OptimizationSetups:  exportFilesWithPrefix(files, "project/optimization/setups/"),
		RunRecords:          exportFilesWithPrefix(files, "project/runs/"),
		BatchRecords:        exportFilesWithPrefix(files, "project/batches/"),
		ValidationRecords:   exportFilesWithPrefix(files, "project/validation/runs/"),
		CalibrationRecords:  exportFilesWithPrefix(files, "project/calibration/results/"),
		OptimizationRecords: exportFilesWithPrefix(files, "project/optimization/results/"),
		Commands:            commands,
		IncludeDatasets:     options.IncludeDatasets,
		IncludeCalibration:  options.IncludeCalibrationSetups,
		IncludeOptimization: options.IncludeOptimizationSetups,
		IncludeMLAssets:     options.IncludeMLAssets,
		IncludeSDKExamples:  options.IncludeSDKExamples,
		IncludeRecords:      options.IncludeRecords,
	}
	exportPath := filepath.Join(exportRoot, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	if err := writeJSONFile(exportPath, manifest); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, exportPath)
	return ExportSummary{
		Profile:      profile,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: manifest.CreatedAtUTC,
	}, manifest, nil
}
