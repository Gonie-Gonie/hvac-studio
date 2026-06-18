package studio

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

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
	var req createCalibrationSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
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
	var req createOptimizationSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
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
	var req validationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
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
	var req calibrationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
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
	var req optimizationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
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

func (s *Server) loadProject(projectPath string) (*project.LoadedProject, error) {
	resolved, err := s.resolveProjectPath(projectPath)
	if err != nil {
		return nil, err
	}
	loaded, err := project.Load(resolved)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	return loaded, nil
}

func projectDetail(loaded *project.LoadedProject) ProjectDetail {
	detail := ProjectDetail{
		Project:             loaded.Project,
		Graph:               loaded.Graph,
		ProjectPath:         loaded.Path,
		GraphPath:           loaded.GraphPath,
		Layout:              loadStudioLayout(loaded.Root),
		Root:                loaded.Root,
		Runs:                loadRunSummaries(loaded.Root),
		Batches:             loadBatchSummaries(loaded.Root),
		Exports:             loadExportSummaries(loaded.Root),
		Scenarios:           loadScenarioSummaries(loaded.Root),
		Datasets:            loadDatasetSummaries(loaded.Root),
		SeriesInputs:        loadSeriesInputSummaries(loaded.Root),
		ParameterSets:       loadParameterSetSummaries(loaded.Root),
		ValidationMappings:  loadValidationMappingSummaries(loaded.Root),
		CalibrationSetups:   loadCalibrationSetupSummaries(loaded.Root),
		OptimizationSetups:  loadOptimizationSetupSummaries(loaded.Root),
		ValidationRuns:      modelvalidation.LoadRecordSummaries(loaded.Root),
		CalibrationResults:  calibration.LoadRecordSummaries(loaded.Root),
		OptimizationResults: optimization.LoadRecordSummaries(loaded.Root),
		MLValidationReports: mlValidationSummaryMap(mlValidationSummaries(loaded, true, false)),
	}
	if inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput); err == nil {
		detail.DefaultInputPath = inputPath
		if input, err := runtimecore.LoadInput(inputPath); err == nil {
			detail.DefaultRunInput = &input
		}
	}
	return detail
}

func loadStudioLayout(projectRoot string) StudioLayout {
	layout := StudioLayout{Components: map[string]CanvasPosition{}}
	path := filepath.Join(projectRoot, "studio", "layout.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return layout
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		return StudioLayout{Components: map[string]CanvasPosition{}}
	}
	if layout.Components == nil {
		layout.Components = map[string]CanvasPosition{}
	}
	return layout
}

func writeStudioLayout(loaded *project.LoadedProject, positions map[string]CanvasPosition) error {
	componentIDs := map[string]bool{}
	for _, component := range loaded.Graph.Components {
		componentIDs[component.ID] = true
	}
	layout := StudioLayout{Components: map[string]CanvasPosition{}}
	for componentID, position := range positions {
		if !componentIDs[componentID] {
			continue
		}
		if position.X < 0 {
			position.X = 0
		}
		if position.Y < 0 {
			position.Y = 0
		}
		layout.Components[componentID] = position
	}
	path := filepath.Join(loaded.Root, "studio", "layout.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(path, layout); err != nil {
		return err
	}
	return nil
}

func (s *Server) findProjectSummaries(root string, source string) []ProjectSummary {
	projects := []ProjectSummary{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "project.bcsproj" {
			return nil
		}
		loaded, err := project.Load(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.repoRoot, path)
		projects = append(projects, ProjectSummary{
			Name:         loaded.Project.ProjectName,
			ProjectPath:  path,
			RelativePath: filepath.ToSlash(rel),
			Source:       source,
		})
		return nil
	})
	return projects
}

func (s *Server) createProject(req createProjectRequest) (ProjectSummary, error) {
	projectName := strings.TrimSpace(req.Name)
	if projectName == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	if template != "scalar" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "unsupported project template: %s", template)
	}

	slug := slugify(projectName)
	if slug == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name must contain letters or numbers")
	}
	projectRoot := filepath.Join(s.repoRoot, "projects", slug)
	if _, err := os.Stat(projectRoot); err == nil {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project already exists: projects/%s", slug)
	}

	templateRoot := filepath.Join(s.repoRoot, "templates", "projects", template)
	if _, err := os.Stat(templateRoot); err != nil {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project template is missing: templates/projects/%s", template)
	}
	if err := copyProjectTree(templateRoot, projectRoot); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "runs"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "scenarios"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "exports"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	projBytes, err := os.ReadFile(projectPath)
	if err != nil {
		return ProjectSummary{}, err
	}
	var proj model.Project
	if err := json.Unmarshal(projBytes, &proj); err != nil {
		return ProjectSummary{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	proj.ProjectName = projectName
	if err := writeJSONFile(projectPath, proj); err != nil {
		return ProjectSummary{}, err
	}

	rel, _ := filepath.Rel(s.repoRoot, projectPath)
	return ProjectSummary{
		Name:         projectName,
		ProjectPath:  projectPath,
		RelativePath: filepath.ToSlash(rel),
		Source:       "workspace",
	}, nil
}

func (s *Server) copyProject(req copyProjectRequest) (ProjectSummary, error) {
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		return ProjectSummary{}, err
	}
	projectName := strings.TrimSpace(req.Name)
	if projectName == "" {
		projectName = loaded.Project.ProjectName + " Copy"
	}
	slugBase := slugify(projectName)
	if slugBase == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name must contain letters or numbers")
	}

	workspaceRoot := filepath.Join(s.repoRoot, "projects")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return ProjectSummary{}, err
	}
	slug := slugBase
	targetName := projectName
	targetRoot := filepath.Join(workspaceRoot, slug)
	for index := 2; ; index++ {
		if _, err := os.Stat(targetRoot); os.IsNotExist(err) {
			break
		} else if err != nil {
			return ProjectSummary{}, err
		}
		slug = fmt.Sprintf("%s-%d", slugBase, index)
		targetName = fmt.Sprintf("%s %d", projectName, index)
		targetRoot = filepath.Join(workspaceRoot, slug)
	}

	if err := copyProjectTree(loaded.Root, targetRoot); err != nil {
		return ProjectSummary{}, err
	}
	projectPath := filepath.Join(targetRoot, "project.bcsproj")
	copied, err := project.Load(projectPath)
	if err != nil {
		return ProjectSummary{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	copied.Project.ProjectName = targetName
	if err := writeJSONFile(projectPath, copied.Project); err != nil {
		return ProjectSummary{}, err
	}
	rel, _ := filepath.Rel(s.repoRoot, projectPath)
	return ProjectSummary{
		Name:         targetName,
		ProjectPath:  projectPath,
		RelativePath: filepath.ToSlash(rel),
		Source:       "workspace",
	}, nil
}

func createComponent(loaded *project.LoadedProject, req createComponentRequest, repoRoot string) (model.Component, error) {
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	templateManifest, templateFiles, err := loadComponentTemplate(repoRoot, template)
	if err != nil {
		return model.Component{}, err
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name must contain letters or numbers")
	}
	className := pythonClassName(componentID)
	component := componentFromTemplateManifest(componentID, componentName, templateManifest)

	componentsRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(componentsRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	initPath := filepath.Join(componentsRoot, "__init__.py")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		if err := os.WriteFile(initPath, []byte(""), 0o644); err != nil {
			return model.Component{}, err
		}
	}
	if err := writeComponentTemplateFiles(loaded.Root, component, templateFiles, templateManifest.ClassName, className); err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return component, nil
}

func componentFromTemplateManifest(componentID string, componentName string, manifest componentTemplateManifest) model.Component {
	className := pythonClassName(componentID)
	componentSource := componentSourceForTemplate(manifest.Source, componentID)
	return model.Component{
		ID:            componentID,
		Name:          componentName,
		Kind:          defaultString(manifest.Kind, "user_python"),
		Category:      defaultString(manifest.Category, "utility"),
		ExecutionMode: defaultString(manifest.ExecutionMode, "step"),
		Class:         classPathForComponentSource(componentID, componentSource, className),
		Source:        componentSource,
		Nodes: model.NodeSet{
			Inputs:  componentTemplateNodes(manifest.Inputs, "inlet"),
			Outputs: componentTemplateNodes(manifest.Outputs, "outlet"),
		},
		Parameters:           cloneMap(manifest.Parameters),
		ParameterDefinitions: cloneParameterDefinitions(manifest.ParameterDefinitions),
		StateDefinitions:     cloneStateDefinitions(manifest.StateDefinitions),
		SolverBoundary:       cloneSolverBoundary(manifest.SolverBoundary),
		MLMetadata:           componentMLMetadataForTemplate(manifest.MLMetadata, componentID),
	}
}

func duplicateComponent(loaded *project.LoadedProject, req duplicateComponentRequest) (model.Component, error) {
	sourceID := strings.TrimSpace(req.SourceComponentID)
	if sourceID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "source_component_id is required")
	}
	source, found := findComponent(loaded.Graph, sourceID)
	if !found {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", sourceID)
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		componentName = strings.TrimSpace(source.Name) + " Copy"
	}
	if strings.TrimSpace(componentName) == "" {
		componentName = sourceID + " Copy"
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	classParts := strings.Split(source.Class, ".")
	if len(classParts) < 1 || classParts[len(classParts)-1] == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component %s class is invalid: %s", sourceID, source.Class)
	}
	className := classParts[len(classParts)-1]
	component := source
	component.ID = componentID
	component.Name = componentName
	component.Class = "components." + componentID + "." + className
	component.Nodes.Inputs = append([]model.Node(nil), source.Nodes.Inputs...)
	component.Nodes.Outputs = append([]model.Node(nil), source.Nodes.Outputs...)
	component.Parameters = map[string]any{}
	for name, value := range source.Parameters {
		component.Parameters[name] = value
	}
	component.ParameterDefinitions = cloneParameterDefinitions(source.ParameterDefinitions)
	component.StateDefinitions = cloneStateDefinitions(source.StateDefinitions)
	component.SolverBoundary = cloneSolverBoundary(source.SolverBoundary)
	component.MLMetadata = cloneMLMetadata(source.MLMetadata)
	component.Source = cloneComponentSource(source.Source)
	if component.Source.Layout == "" || component.Source.Layout == "single_file_class" {
		component.Source.Layout = "single_file_class"
		component.Source.Step = filepath.ToSlash(filepath.Join("components", componentID+".py"))
		component.Class = "components." + componentID + "." + className
	} else if component.Source.Layout == "generated_wrapper" {
		sourceDirRel, err := generatedComponentSourceDir(component.Source)
		if err != nil {
			return model.Component{}, err
		}
		targetDirRel := filepath.ToSlash(filepath.Join("components", componentID))
		component.Source = rebaseComponentSource(component.Source, sourceDirRel, targetDirRel)
		component.Class = classPathForComponentSource(componentID, component.Source, className)
	}

	sourceRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	copiedPath, err := copyComponentSourceArtifact(loaded, source, component)
	if err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		_ = removeComponentSourceArtifact(copiedPath, component.Source.Layout)
		return model.Component{}, err
	}
	return component, nil
}

func updateComponent(loaded *project.LoadedProject, req updateComponentRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID != componentID {
			continue
		}
		loaded.Graph.Components[index].Name = componentName
		if err := syncComponentMetadataFile(loaded, loaded.Graph.Components[index]); err != nil {
			return model.Component{}, err
		}
		if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
			return model.Component{}, err
		}
		return loaded.Graph.Components[index], nil
	}
	return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func deleteComponent(loaded *project.LoadedProject, req deleteComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	deletedComponent := loaded.Graph.Components[componentIndex]
	for _, system := range loaded.Graph.Systems {
		if containsString(system.Components, componentID) {
			return apperror.Errorf(apperror.CodeValidation, "component is still used by system %s; remove it from the system first", system.ID)
		}
	}
	for _, connection := range loaded.Graph.Connections {
		if connection.From.Component == componentID || connection.To.Component == componentID {
			return apperror.Errorf(apperror.CodeValidation, "component still has connection reference: %s", connection.ID)
		}
	}

	sourceArtifactPath, err := componentSourceArtifactPath(loaded, deletedComponent)
	if err != nil {
		return err
	}
	sourceShared := false
	for index := range loaded.Graph.Components {
		if index == componentIndex {
			continue
		}
		otherPath, err := componentSourceArtifactPath(loaded, loaded.Graph.Components[index])
		if err == nil && sameFilesystemPath(otherPath, sourceArtifactPath) {
			sourceShared = true
			break
		}
	}

	loaded.Graph.Components = append(loaded.Graph.Components[:componentIndex], loaded.Graph.Components[componentIndex+1:]...)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	if !sourceShared {
		if err := removeComponentSourceArtifact(sourceArtifactPath, deletedComponent.Source.Layout); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func includeComponentInSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	component, foundComponent := findComponent(loaded.Graph, componentID)
	if !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if containsString(system.Components, componentID) {
		return nil
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}
	system.Components = append(system.Components, componentID)
	for _, node := range component.Nodes.Inputs {
		if hasPublicInputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+node.ID)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}
	for _, node := range component.Nodes.Outputs {
		if hasPublicOutputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+node.ID)
		system.PublicOutputs = append(system.PublicOutputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Default:   node.Default,
		})
	}

	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}

func removeComponentFromSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	if _, foundComponent := findComponent(loaded.Graph, componentID); !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, componentID) {
		return nil
	}
	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}

	system.Components = removeString(system.Components, componentID)
	removedPublicInputs := removePublicInputsForComponent(system, componentID)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	removePublicOutputsForComponent(system, componentID)

	removedConnections := []model.Connection{}
	keptConnectionIDs := system.Connections[:0]
	for _, connectionID := range system.Connections {
		connection, found := findConnection(loaded.Graph, connectionID)
		if !found {
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
			continue
		}
		if connection.From.Component == componentID || connection.To.Component == componentID {
			removedConnections = append(removedConnections, connection)
			continue
		}
		keptConnectionIDs = append(keptConnectionIDs, connectionID)
	}
	system.Connections = keptConnectionIDs

	for _, connection := range removedConnections {
		if connection.To.Component == componentID || !containsString(system.Components, connection.To.Component) {
			continue
		}
		if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
			continue
		}
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
		}
		publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: connection.To.Component,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}

func createNode(loaded *project.LoadedProject, req createNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	nodeID := strings.TrimSpace(req.ID)
	if !isIdentifierLike(nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node id must start with a letter or underscore and contain only letters, numbers, and underscores")
	}
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	isInput := false
	nodeDirection := ""
	switch direction {
	case "input", "in", "inlet":
		isInput = true
		nodeDirection = "inlet"
	case "output", "out", "outlet":
		nodeDirection = "outlet"
	default:
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node direction must be input or output")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	if componentHasNode(*component, nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component already has node: %s.%s", componentID, nodeID)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node := model.Node{
		ID:        nodeID,
		Name:      name,
		Preset:    strings.TrimSpace(req.Preset),
		Direction: nodeDirection,
		Medium:    medium,
		ValueType: valueType,
		Unit:      strings.TrimSpace(req.Unit),
		Required:  req.Required,
		Default:   req.Default,
	}
	if isInput {
		component.Nodes.Inputs = append(component.Nodes.Inputs, node)
	} else {
		component.Nodes.Outputs = append(component.Nodes.Outputs, node)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for index := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[index]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			if hasPublicInputFor(*system, componentID, nodeID) {
				continue
			}
			publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+nodeID)
			system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
				ID:        publicID,
				Name:      node.Name,
				Component: componentID,
				Node:      node.ID,
				Medium:    node.Medium,
				ValueType: node.ValueType,
				Unit:      node.Unit,
				Required:  node.Required,
				Default:   node.Default,
			})
			if _, exists := input.Inputs[publicID]; !exists {
				input.Inputs[publicID] = defaultValueForNode(node)
			}
			continue
		}
		if hasPublicOutputFor(*system, componentID, nodeID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+nodeID)
		system.PublicOutputs = append(system.PublicOutputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Default:   node.Default,
		})
	}
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func updateNode(loaded *project.LoadedProject, req updateNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	component := &loaded.Graph.Components[componentIndex]
	nodeIndex := -1
	isInput := true
	for index := range component.Nodes.Inputs {
		if component.Nodes.Inputs[index].ID == nodeID {
			nodeIndex = index
			break
		}
	}
	if nodeIndex < 0 {
		isInput = false
		for index := range component.Nodes.Outputs {
			if component.Nodes.Outputs[index].ID == nodeID {
				nodeIndex = index
				break
			}
		}
	}
	if nodeIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	var node *model.Node
	if isInput {
		node = &component.Nodes.Inputs[nodeIndex]
	} else {
		node = &component.Nodes.Outputs[nodeIndex]
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node.Name = name
	node.Medium = medium
	node.ValueType = valueType
	node.Unit = strings.TrimSpace(req.Unit)
	node.Required = req.Required
	if req.DefaultProvided {
		node.Default = req.Default
	}
	updatedNode := *node

	inputPath := ""
	var input runtimecore.RunInput
	inputDirty := false
	if isInput {
		var err error
		inputPath, input, err = loadEditableDefaultInput(loaded)
		if err != nil {
			return model.Node{}, err
		}
	}

	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for refIndex := range system.PublicInputs {
				ref := &system.PublicInputs[refIndex]
				if ref.Component != componentID || ref.Node != nodeID {
					continue
				}
				updatePublicNodeRef(ref, updatedNode)
				if req.DefaultProvided {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
					continue
				}
				if _, exists := input.Inputs[ref.ID]; !exists {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
				}
			}
			continue
		}
		for refIndex := range system.PublicOutputs {
			ref := &system.PublicOutputs[refIndex]
			if ref.Component != componentID || ref.Node != nodeID {
				continue
			}
			updatePublicNodeRef(ref, updatedNode)
		}
	}

	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if inputDirty {
		if err := writeJSONFile(inputPath, input); err != nil {
			return model.Node{}, err
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return updatedNode, nil
}

func deleteNode(loaded *project.LoadedProject, req deleteNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	node, isInput, foundNode := removeNodeFromComponent(component, nodeID)
	if !foundNode {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for _, inputID := range removePublicInputsFor(system, componentID, nodeID) {
				delete(input.Inputs, inputID)
			}
		} else {
			removePublicOutputsFor(system, componentID, nodeID)
		}

		removedConnections := []model.Connection{}
		keptConnectionIDs := system.Connections[:0]
		for _, connectionID := range system.Connections {
			connection, found := findConnection(loaded.Graph, connectionID)
			if !found {
				keptConnectionIDs = append(keptConnectionIDs, connectionID)
				continue
			}
			if endpointMatches(connection.From, componentID, nodeID) || endpointMatches(connection.To, componentID, nodeID) {
				removedConnections = append(removedConnections, connection)
				continue
			}
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
		}
		system.Connections = keptConnectionIDs

		for _, connection := range removedConnections {
			if endpointMatches(connection.To, componentID, nodeID) || !containsString(system.Components, connection.To.Component) {
				continue
			}
			if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
				continue
			}
			targetComponent, foundComponent := findComponent(loaded.Graph, connection.To.Component)
			if !foundComponent {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
			}
			targetNode, foundTargetNode := findInputNode(targetComponent, connection.To.Node)
			if !foundTargetNode {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
			}
			publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
			system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
				ID:        publicID,
				Name:      targetNode.Name,
				Component: connection.To.Component,
				Node:      targetNode.ID,
				Medium:    targetNode.Medium,
				ValueType: targetNode.ValueType,
				Unit:      targetNode.Unit,
				Required:  targetNode.Required,
				Default:   targetNode.Default,
			})
			if _, exists := input.Inputs[publicID]; !exists {
				input.Inputs[publicID] = defaultValueForNode(targetNode)
			}
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func deleteParameter(loaded *project.LoadedProject, req deleteParameterRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	name := strings.TrimSpace(req.Name)
	if componentID == "" || name == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id and name are required")
	}
	for index := range loaded.Graph.Components {
		component := &loaded.Graph.Components[index]
		if component.ID != componentID {
			continue
		}
		if component.Parameters == nil {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		if _, found := component.Parameters[name]; !found {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		delete(component.Parameters, name)
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
		if err := syncComponentMetadataFile(loaded, *component); err != nil {
			return err
		}
		return writeJSONFile(loaded.GraphPath, loaded.Graph)
	}
	return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func updateComponentContract(loaded *project.LoadedProject, req updateComponentContractRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]

	if req.Parameters != nil && component.Parameters == nil {
		component.Parameters = map[string]any{}
	}
	for name, value := range req.Parameters {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.Parameters[name] = value
	}

	if len(req.ParameterDefinitions) > 0 && component.ParameterDefinitions == nil {
		component.ParameterDefinitions = map[string]model.ParameterDefinition{}
	}
	for name, definition := range req.ParameterDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		current, hasCurrent := component.Parameters[name]
		definition = normalizeParameterDefinition(name, definition, current, hasCurrent)
		if err := validateParameterDefinition(component.ID, name, definition); err != nil {
			return model.Component{}, err
		}
		component.ParameterDefinitions[name] = definition
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		if _, exists := component.Parameters[name]; !exists {
			if definition.Current != nil {
				component.Parameters[name] = definition.Current
			} else if definition.Default != nil {
				component.Parameters[name] = definition.Default
			}
		}
	}
	for _, name := range req.DeleteParameterDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
	}

	if len(req.StateDefinitions) > 0 && component.StateDefinitions == nil {
		component.StateDefinitions = map[string]model.StateDefinition{}
	}
	for name, definition := range req.StateDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "state definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.StateDefinitions[name] = normalizeStateDefinition(name, definition)
	}
	for _, name := range req.DeleteStateDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.StateDefinitions != nil {
			delete(component.StateDefinitions, name)
		}
	}

	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Component{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return *component, nil
}

func normalizeParameterDefinition(name string, definition model.ParameterDefinition, current any, hasCurrent bool) model.ParameterDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	if definition.Current == nil && hasCurrent {
		definition.Current = current
	}
	if definition.Default == nil && hasCurrent {
		definition.Default = current
	}
	return definition
}

func validateParameterDefinition(componentID string, name string, definition model.ParameterDefinition) error {
	if definition.Bounds == nil {
		return nil
	}
	hasMin := definition.Bounds.Min != nil
	hasMax := definition.Bounds.Max != nil
	var minValue, maxValue float64
	if hasMin {
		var ok bool
		minValue, ok = studioNumberValue(definition.Bounds.Min)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMax {
		var ok bool
		maxValue, ok = studioNumberValue(definition.Bounds.Max)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds max must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMin && hasMax && minValue > maxValue {
		return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be <= max: %s.%s", componentID, name)
	}
	return nil
}

func normalizeStateDefinition(name string, definition model.StateDefinition) model.StateDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	return definition
}

func createConnection(loaded *project.LoadedProject, req createConnectionRequest) (model.Connection, error) {
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}
	fromComponent := strings.TrimSpace(req.FromComponent)
	fromNode := strings.TrimSpace(req.FromNode)
	toComponent := strings.TrimSpace(req.ToComponent)
	toNode := strings.TrimSpace(req.ToNode)
	if fromComponent == "" || fromNode == "" || toComponent == "" || toNode == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection endpoints are required")
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, fromComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection source component is not in system: %s", fromComponent)
	}
	if !containsString(system.Components, toComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component is not in system: %s", toComponent)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	connection := model.Connection{
		ID: uniqueConnectionID(
			loaded.Graph,
			fmt.Sprintf("%s_%s_to_%s_%s", fromComponent, fromNode, toComponent, toNode),
		),
		From: model.Endpoint{Component: fromComponent, Node: fromNode},
		To:   model.Endpoint{Component: toComponent, Node: toNode},
	}
	removedPublicInputs := removePublicInputsFor(system, toComponent, toNode)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	loaded.Graph.Connections = append(loaded.Graph.Connections, connection)
	system.Connections = append(system.Connections, connection.ID)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

func updateConnection(loaded *project.LoadedProject, req updateConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
	}
	if !req.UnitConversionWasPresent {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "unit_conversion is required")
	}
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}
	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection is not in system %s: %s", systemID, connectionID)
	}

	loaded.Graph.Connections[connectionIndex].UnitConversion = normalizeUnitConversion(req.UnitConversion)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return loaded.Graph.Connections[connectionIndex], nil
}

func normalizeUnitConversion(conversion *model.UnitConversion) *model.UnitConversion {
	if conversion == nil {
		return nil
	}
	normalized := *conversion
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	if normalized.Mode == "" {
		normalized.Mode = "linear"
	}
	normalized.Description = strings.TrimSpace(normalized.Description)
	return &normalized
}

func deleteConnection(loaded *project.LoadedProject, req deleteConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
	}
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	var connection model.Connection
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			connection = item
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system %s does not contain connection: %s", systemID, connectionID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	system.Connections = removeString(system.Connections, connectionID)
	if !graphReferencesConnection(loaded.Graph.Systems, connectionID) {
		loaded.Graph.Connections = append(loaded.Graph.Connections[:connectionIndex], loaded.Graph.Connections[connectionIndex+1:]...)
	}
	if !systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) && !hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
		}
		publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: connection.To.Component,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

func sameFilesystemPath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func cleanRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "relative path is required")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", apperror.Errorf(apperror.CodeValidation, "relative path escapes project: %s", rel)
	}
	return clean, nil
}

func classNameFromPath(classPath string) string {
	classPath = strings.TrimSpace(classPath)
	if classPath == "" {
		return ""
	}
	parts := strings.Split(classPath, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

func (s *Server) resolveProjectPath(projectPath string) (string, error) {
	if projectPath == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "project_path is required")
	}
	if !filepath.IsAbs(projectPath) {
		projectPath = filepath.Join(s.repoRoot, projectPath)
	}
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(s.repoRoot, absProjectPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", apperror.Errorf(apperror.CodeValidation, "project_path must stay inside repository: %s", projectPath)
	}
	if _, err := os.Stat(absProjectPath); err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	return absProjectPath, nil
}

func (s *Server) ensureWorkspaceProject(projectRoot string) error {
	workspaceRoot, err := filepath.Abs(filepath.Join(s.repoRoot, "projects"))
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(workspaceRoot, absProjectRoot)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return apperror.Errorf(apperror.CodeValidation, "only workspace projects under projects/ can be edited")
	}
	return nil
}

func decodeCreateProjectRequest(r *http.Request) (createProjectRequest, error) {
	defer r.Body.Close()
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCopyProjectRequest(r *http.Request) (copyProjectRequest, error) {
	defer r.Body.Close()
	var req copyProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateComponentRequest(r *http.Request) (createComponentRequest, error) {
	defer r.Body.Close()
	var req createComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDuplicateComponentRequest(r *http.Request) (duplicateComponentRequest, error) {
	defer r.Body.Close()
	var req duplicateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeReplaceComponentRequest(r *http.Request) (replaceComponentRequest, error) {
	defer r.Body.Close()
	var req replaceComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateComponentRequest(r *http.Request) (updateComponentRequest, error) {
	defer r.Body.Close()
	var req updateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteComponentRequest(r *http.Request) (deleteComponentRequest, error) {
	defer r.Body.Close()
	var req deleteComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeIncludeComponentRequest(r *http.Request) (includeComponentRequest, error) {
	defer r.Body.Close()
	var req includeComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateNodeRequest(r *http.Request) (createNodeRequest, error) {
	defer r.Body.Close()
	var req createNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateNodeRequest(r *http.Request) (updateNodeRequest, error) {
	defer r.Body.Close()
	var req updateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteNodeRequest(r *http.Request) (deleteNodeRequest, error) {
	defer r.Body.Close()
	var req deleteNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateConnectionRequest(r *http.Request) (createConnectionRequest, error) {
	defer r.Body.Close()
	var req createConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateConnectionRequest(r *http.Request) (updateConnectionRequest, error) {
	defer r.Body.Close()
	var req updateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteConnectionRequest(r *http.Request) (deleteConnectionRequest, error) {
	defer r.Body.Close()
	var req deleteConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateLayoutRequest(r *http.Request) (updateLayoutRequest, error) {
	defer r.Body.Close()
	var req updateLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeExportRequest(r *http.Request) (exportRequest, error) {
	defer r.Body.Close()
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSourceRequest(r *http.Request) (sourceRequest, error) {
	defer r.Body.Close()
	var req sourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSourceCheckRequest(r *http.Request) (sourceCheckRequest, error) {
	defer r.Body.Close()
	var req sourceCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateScenarioRequest(r *http.Request) (createScenarioRequest, error) {
	defer r.Body.Close()
	var req createScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateParametersRequest(r *http.Request) (updateParametersRequest, error) {
	defer r.Body.Close()
	var req updateParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateComponentContractRequest(r *http.Request) (updateComponentContractRequest, error) {
	defer r.Body.Close()
	var req updateComponentContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeApplyParameterSetRequest(r *http.Request) (applyParameterSetRequest, error) {
	defer r.Body.Close()
	var req applyParameterSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteParameterRequest(r *http.Request) (deleteParameterRequest, error) {
	defer r.Body.Close()
	var req deleteParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeImportDatasetRequest(r *http.Request) (importDatasetRequest, error) {
	defer r.Body.Close()
	var req importDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateValidationMappingRequest(r *http.Request) (createValidationMappingRequest, error) {
	defer r.Body.Close()
	var req createValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateValidationMappingRequest(r *http.Request) (updateValidationMappingRequest, error) {
	defer r.Body.Close()
	var req updateValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCopyValidationMappingRequest(r *http.Request) (copyValidationMappingRequest, error) {
	defer r.Body.Close()
	var req copyValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteValidationMappingRequest(r *http.Request) (deleteValidationMappingRequest, error) {
	defer r.Body.Close()
	var req deleteValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateInputRequest(r *http.Request) (updateInputRequest, error) {
	defer r.Body.Close()
	var req updateInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeRequest(r *http.Request) (apiRequest, error) {
	defer r.Body.Close()
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSeriesRunRequest(r *http.Request) (seriesRunRequest, error) {
	defer r.Body.Close()
	var req seriesRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func requestTimeout(req apiRequest, fallback time.Duration) (time.Duration, error) {
	if req.TimeoutMS == 0 {
		return fallback, nil
	}
	if req.TimeoutMS < 100 {
		return 0, apperror.Errorf(apperror.CodeInput, "timeout_ms must be at least 100")
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout > 30*time.Minute {
		return 0, apperror.Errorf(apperror.CodeInput, "timeout_ms must be at most 1800000")
	}
	return timeout, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeErrorWithProblems(w, err, nil)
}

func writeTimeoutError(w http.ResponseWriter, workflow string, timeout time.Duration) {
	err := apperror.Errorf(apperror.CodeRuntime, "%s timed out after %s", workflow, formatTimeoutDuration(timeout))
	payload := apperror.PayloadFor(err, nil)
	writeJSON(w, http.StatusGatewayTimeout, apiError{
		OK:      false,
		Error:   payload,
		Code:    payload.Code,
		Kind:    payload.Kind,
		Message: payload.Message,
	})
}

func writeErrorWithProblems(w http.ResponseWriter, err error, problems []Problem) {
	code := apperror.ErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case apperror.CodeValidation:
		status = http.StatusBadRequest
	case apperror.CodeInput:
		status = http.StatusUnprocessableEntity
	case apperror.CodePythonWorker:
		status = http.StatusBadGateway
	}
	payload := apperror.PayloadFor(err, toAppProblems(problems))
	writeJSON(w, status, apiError{
		OK:       false,
		Error:    payload,
		Code:     payload.Code,
		Kind:     payload.Kind,
		Message:  payload.Message,
		Problems: problems,
	})
}

func formatTimeoutDuration(timeout time.Duration) string {
	if timeout%time.Second == 0 {
		return fmt.Sprintf("%.0f seconds", timeout.Seconds())
	}
	return timeout.String()
}

func toAppProblems(problems []Problem) []apperror.Problem {
	if len(problems) == 0 {
		return nil
	}
	out := make([]apperror.Problem, 0, len(problems))
	for _, problem := range problems {
		out = append(out, apperror.Problem{
			Severity:    problem.Severity,
			Message:     problem.Message,
			ComponentID: problem.ComponentID,
			NodeID:      problem.NodeID,
			Source:      problem.Source,
			Line:        problem.Line,
			Column:      problem.Column,
		})
	}
	return out
}

func inferProblems(loaded *project.LoadedProject, err error) []Problem {
	message := fmt.Sprint(err)
	problem := Problem{Severity: "error", Message: message}
	if loaded == nil || loaded.Graph == nil {
		return []Problem{problem}
	}
	for _, component := range loaded.Graph.Components {
		if strings.Contains(message, component.ID) {
			problem.ComponentID = component.ID
			for _, node := range component.Nodes.Inputs {
				if strings.Contains(message, component.ID+"."+node.ID) || strings.Contains(message, " "+node.ID) {
					problem.NodeID = node.ID
					break
				}
			}
			if problem.NodeID == "" {
				for _, node := range component.Nodes.Outputs {
					if strings.Contains(message, component.ID+"."+node.ID) || strings.Contains(message, " "+node.ID) {
						problem.NodeID = node.ID
						break
					}
				}
			}
			break
		}
	}
	if location, ok := tracebackSourceLocation(loaded, message, problem.ComponentID); ok {
		problem.ComponentID = location.ComponentID
		problem.Source = location.Source
		problem.Line = location.Line
	}
	return []Problem{problem}
}

func compilerDiagnosticsProblems(diagnostics []compiler.Diagnostic) []Problem {
	problems := make([]Problem, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		problem := Problem{
			Severity:    defaultString(diagnostic.Severity, "warning"),
			Message:     diagnostic.Message,
			ComponentID: diagnostic.To.Component,
			NodeID:      diagnostic.To.Node,
		}
		if problem.ComponentID == "" {
			problem.ComponentID = diagnostic.From.Component
			problem.NodeID = diagnostic.From.Node
		}
		problems = append(problems, problem)
	}
	return problems
}

func resolveProjectFile(projectRoot string, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectRoot, path)
}

func resolveProjectOwnedFile(projectRoot string, path string) (string, error) {
	if path == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "project file path is required")
	}
	resolved := resolveProjectFile(projectRoot, path)
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", apperror.Errorf(apperror.CodeValidation, "project file path must stay inside project root: %s", path)
	}
	return absPath, nil
}

func loadEditableDefaultInput(loaded *project.LoadedProject) (string, runtimecore.RunInput, error) {
	inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput)
	if err != nil {
		return "", runtimecore.RunInput{}, err
	}
	input, err := runtimecore.LoadInput(inputPath)
	if err != nil {
		return "", runtimecore.RunInput{}, err
	}
	if input.Inputs == nil {
		input.Inputs = map[string]any{}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	return inputPath, input, nil
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

func writeJSONFile(path string, value any) error {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(output, '\n'), 0o644)
}

func copyProjectTree(sourceRoot string, targetRoot string) error {
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return err
	}
	targetRoot, err = filepath.Abs(targetRoot)
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetRoot); err == nil {
		return apperror.Errorf(apperror.CodeValidation, "target project already exists: %s", targetRoot)
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo")) {
			return nil
		}
		targetPath := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, bytes, info.Mode().Perm())
	})
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueComponentID(graph *model.Graph, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		return ""
	}
	exists := map[string]bool{}
	for _, component := range graph.Components {
		exists[component.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueConnectionID(graph *model.Graph, base string) string {
	base = strings.ReplaceAll(slugify(base), "-", "_")
	if base == "" {
		base = "connection"
	}
	exists := map[string]bool{}
	for _, connection := range graph.Connections {
		exists[connection.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func findComponent(graph *model.Graph, componentID string) (model.Component, bool) {
	if graph == nil {
		return model.Component{}, false
	}
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
}

func findSystem(graph *model.Graph, systemID string) (model.System, bool) {
	if graph == nil {
		return model.System{}, false
	}
	for _, system := range graph.Systems {
		if system.ID == systemID {
			return system, true
		}
	}
	return model.System{}, false
}

func findConnection(graph *model.Graph, connectionID string) (model.Connection, bool) {
	for _, connection := range graph.Connections {
		if connection.ID == connectionID {
			return connection, true
		}
	}
	return model.Connection{}, false
}

func findInputNode(component model.Component, nodeID string) (model.Node, bool) {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}

func removeNodeFromComponent(component *model.Component, nodeID string) (model.Node, bool, bool) {
	for index, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			component.Nodes.Inputs = append(component.Nodes.Inputs[:index], component.Nodes.Inputs[index+1:]...)
			return node, true, true
		}
	}
	for index, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			component.Nodes.Outputs = append(component.Nodes.Outputs[:index], component.Nodes.Outputs[index+1:]...)
			return node, false, true
		}
	}
	return model.Node{}, false, false
}

func componentHasNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasInputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasOutputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func isIdentifierLike(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	kept := values[:0]
	for _, value := range values {
		if value == target {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

func endpointMatches(endpoint model.Endpoint, componentID string, nodeID string) bool {
	return endpoint.Component == componentID && endpoint.Node == nodeID
}

func graphReferencesConnection(systems []model.System, connectionID string) bool {
	for _, system := range systems {
		if containsString(system.Connections, connectionID) {
			return true
		}
	}
	return false
}

func systemHasIncomingConnection(system model.System, graph *model.Graph, componentID string, nodeID string) bool {
	for _, connectionID := range system.Connections {
		for _, connection := range graph.Connections {
			if connection.ID == connectionID && connection.To.Component == componentID && connection.To.Node == nodeID {
				return true
			}
		}
	}
	return false
}

func entrySystem(loaded *project.LoadedProject) model.System {
	index := entrySystemIndex(loaded)
	if index < 0 {
		return model.System{}
	}
	return loaded.Graph.Systems[index]
}

func entrySystemIndex(loaded *project.LoadedProject) int {
	if loaded == nil || loaded.Graph == nil || loaded.Project == nil {
		return -1
	}
	for index, system := range loaded.Graph.Systems {
		if system.ID == loaded.Project.EntrySystem {
			return index
		}
	}
	return -1
}

func hasPublicInputFor(system model.System, componentID string, nodeID string) bool {
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			return true
		}
	}
	return false
}

func hasPublicOutputFor(system model.System, componentID string, nodeID string) bool {
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			return true
		}
	}
	return false
}

func removePublicOutputsFor(system *model.System, componentID string, nodeID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removePublicInputsFor(system *model.System, componentID string, nodeID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicInputsForComponent(system *model.System, componentID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicOutputsForComponent(system *model.System, componentID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removeUnreferencedConnections(connections []model.Connection, systems []model.System) []model.Connection {
	kept := connections[:0]
	for _, connection := range connections {
		if graphReferencesConnection(systems, connection.ID) {
			kept = append(kept, connection)
		}
	}
	return kept
}

func uniquePublicNodeID(refs []model.PublicNodeRef, base string) string {
	exists := map[string]bool{}
	for _, ref := range refs {
		exists[ref.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func normalizeColumnName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func updatePublicNodeRef(ref *model.PublicNodeRef, node model.Node) {
	ref.Name = node.Name
	ref.Medium = node.Medium
	ref.ValueType = node.ValueType
	ref.Unit = node.Unit
	ref.Required = node.Required
	ref.Default = node.Default
}

func defaultValueForNode(node model.Node) any {
	if node.Default != nil {
		return node.Default
	}
	switch node.ValueType {
	case "int", "integer":
		return 0
	case "bool", "boolean":
		return false
	case "string":
		return ""
	default:
		return 0.0
	}
}

func pythonClassName(componentID string) string {
	var b strings.Builder
	capitalizeNext := true
	for _, r := range componentID {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if b.Len() == 0 && unicode.IsDigit(r) {
				b.WriteRune('C')
			}
			if capitalizeNext {
				b.WriteRune(unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				b.WriteRune(r)
			}
			continue
		}
		capitalizeNext = true
	}
	if b.Len() == 0 {
		return "UserComponent"
	}
	b.WriteString("Component")
	return b.String()
}

func displayNameFromID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[index] = string(runes)
	}
	name := strings.Join(parts, " ")
	if name == "" {
		return id
	}
	return name
}

func cloneMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneComponentSource(value model.ComponentSource) model.ComponentSource {
	return model.ComponentSource{
		Layout:   value.Layout,
		Metadata: value.Metadata,
		Init:     value.Init,
		Step:     value.Step,
		Helpers:  value.Helpers,
		Wrapper:  value.Wrapper,
	}
}

func cloneParameterDefinitions(values map[string]model.ParameterDefinition) map[string]model.ParameterDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.ParameterDefinition{}
	for key, value := range values {
		if value.Bounds != nil {
			bounds := *value.Bounds
			value.Bounds = &bounds
		}
		if value.Visible != nil {
			visible := *value.Visible
			value.Visible = &visible
		}
		out[key] = value
	}
	return out
}

func cloneStateDefinitions(values map[string]model.StateDefinition) map[string]model.StateDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.StateDefinition{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneSolverBoundary(value *model.SolverBoundary) *model.SolverBoundary {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneMLMetadata(value *model.MLMetadata) *model.MLMetadata {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.RequiredPackages = append([]string{}, value.RequiredPackages...)
	if value.ValidInputRanges != nil {
		cloned.ValidInputRanges = map[string]model.ValueBounds{}
		for key, bounds := range value.ValidInputRanges {
			cloned.ValidInputRanges[key] = bounds
		}
	}
	return &cloned
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
