package studio

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
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
	Project          *model.Project        `json:"project"`
	Graph            *model.Graph          `json:"graph"`
	ProjectPath      string                `json:"project_path"`
	GraphPath        string                `json:"graph_path"`
	DefaultInputPath string                `json:"default_input_path"`
	DefaultRunInput  *runtimecore.RunInput `json:"default_run_input"`
	Root             string                `json:"root"`
	Runs             []RunSummary          `json:"runs"`
	Batches          []BatchSummary        `json:"batches"`
	Exports          []ExportSummary       `json:"exports"`
	Scenarios        []ScenarioSummary     `json:"scenarios"`
}

type apiRequest struct {
	ProjectPath string         `json:"project_path"`
	Inputs      map[string]any `json:"inputs"`
	Context     map[string]any `json:"context"`
	Save        bool           `json:"save"`
}

type createProjectRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type copyProjectRequest struct {
	ProjectPath string `json:"project_path"`
	Name        string `json:"name"`
}

type createComponentRequest struct {
	ProjectPath string `json:"project_path"`
	Name        string `json:"name"`
	Template    string `json:"template"`
}

type duplicateComponentRequest struct {
	ProjectPath       string `json:"project_path"`
	SourceComponentID string `json:"source_component_id"`
	Name              string `json:"name"`
}

type updateComponentRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
}

type deleteComponentRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
}

type includeComponentRequest struct {
	ProjectPath string `json:"project_path"`
	SystemID    string `json:"system_id"`
	ComponentID string `json:"component_id"`
}

type createNodeRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Direction   string `json:"direction"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Medium      string `json:"medium"`
	ValueType   string `json:"value_type"`
	Unit        string `json:"unit"`
	Required    *bool  `json:"required"`
	Default     any    `json:"default"`
}

type deleteNodeRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	NodeID      string `json:"node_id"`
}

type createConnectionRequest struct {
	ProjectPath   string `json:"project_path"`
	SystemID      string `json:"system_id"`
	FromComponent string `json:"from_component"`
	FromNode      string `json:"from_node"`
	ToComponent   string `json:"to_component"`
	ToNode        string `json:"to_node"`
}

type deleteConnectionRequest struct {
	ProjectPath  string `json:"project_path"`
	SystemID     string `json:"system_id"`
	ConnectionID string `json:"connection_id"`
}

type exportRequest struct {
	ProjectPath string `json:"project_path"`
	Profile     string `json:"profile"`
}

type sourceRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Content     string `json:"content"`
}

type sourceCheckRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Content     string `json:"content"`
}

type createScenarioRequest struct {
	ProjectPath string         `json:"project_path"`
	Name        string         `json:"name"`
	Inputs      map[string]any `json:"inputs"`
	Context     map[string]any `json:"context"`
}

type updateParametersRequest struct {
	ProjectPath string                    `json:"project_path"`
	Parameters  map[string]map[string]any `json:"parameters"`
}

type deleteParameterRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
}

type updateInputRequest struct {
	ProjectPath string         `json:"project_path"`
	Inputs      map[string]any `json:"inputs"`
	Context     map[string]any `json:"context"`
}

type RunSummary struct {
	ID           string         `json:"id"`
	RelativePath string         `json:"relative_path"`
	CreatedAtUTC string         `json:"created_at_utc"`
	Outputs      map[string]any `json:"outputs"`
}

type RunRecord struct {
	ID           string                 `json:"id"`
	ProjectName  string                 `json:"project_name"`
	CreatedAtUTC string                 `json:"created_at_utc"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result"`
}

type BatchSummary struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
	CaseCount    int    `json:"case_count"`
	OKCount      int    `json:"ok_count"`
}

type BatchCaseRecord struct {
	ScenarioID   string                 `json:"scenario_id"`
	ScenarioName string                 `json:"scenario_name"`
	OK           bool                   `json:"ok"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

type BatchRecord struct {
	ID           string            `json:"id"`
	ProjectName  string            `json:"project_name"`
	CreatedAtUTC string            `json:"created_at_utc"`
	Cases        []BatchCaseRecord `json:"cases"`
}

type ScenarioSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type ScenarioRecord struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	ProjectName  string         `json:"project_name"`
	CreatedAtUTC string         `json:"created_at_utc"`
	Inputs       map[string]any `json:"inputs"`
	Context      map[string]any `json:"context"`
}

type ExportSummary struct {
	Profile      string `json:"profile"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type ExportManifest struct {
	Profile        string                `json:"profile"`
	CreatedAtUTC   string                `json:"created_at_utc"`
	ProjectName    string                `json:"project_name"`
	ProjectPath    string                `json:"project_path"`
	GraphPath      string                `json:"graph_path"`
	DefaultInput   string                `json:"default_input"`
	Runner         string                `json:"runner"`
	RuntimePython  string                `json:"runtime_python"`
	Components     []string              `json:"components"`
	PublicInputs   []model.PublicNodeRef `json:"public_inputs"`
	PublicOutputs  []model.PublicNodeRef `json:"public_outputs"`
	ExecutionOrder []string              `json:"execution_order"`
}

type SourceDetail struct {
	ComponentID  string `json:"component_id"`
	RelativePath string `json:"relative_path"`
	Content      string `json:"content"`
	ReadOnly     bool   `json:"read_only"`
}

type SourceCheck struct {
	OK            bool      `json:"ok"`
	ComponentID   string    `json:"component_id"`
	RelativePath  string    `json:"relative_path"`
	ExpectedClass string    `json:"expected_class"`
	LineCount     int       `json:"line_count"`
	Problems      []Problem `json:"problems"`
}

type Problem struct {
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	ComponentID string `json:"component_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

type apiError struct {
	OK       bool      `json:"ok"`
	Code     int       `json:"code"`
	Kind     string    `json:"kind"`
	Message  string    `json:"message"`
	Problems []Problem `json:"problems,omitempty"`
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
	s.mux.HandleFunc("GET /api/project/source", s.handleSource)
	s.mux.HandleFunc("POST /api/project/source/check", s.handleCheckSource)
	s.mux.HandleFunc("POST /api/project/components", s.handleCreateComponent)
	s.mux.HandleFunc("POST /api/project/components/duplicate", s.handleDuplicateComponent)
	s.mux.HandleFunc("POST /api/project/components/update", s.handleUpdateComponent)
	s.mux.HandleFunc("POST /api/project/components/delete", s.handleDeleteComponent)
	s.mux.HandleFunc("POST /api/project/system/components", s.handleIncludeComponent)
	s.mux.HandleFunc("POST /api/project/system/components/remove", s.handleRemoveComponentFromSystem)
	s.mux.HandleFunc("POST /api/project/nodes", s.handleCreateNode)
	s.mux.HandleFunc("POST /api/project/nodes/delete", s.handleDeleteNode)
	s.mux.HandleFunc("POST /api/project/connections", s.handleCreateConnection)
	s.mux.HandleFunc("POST /api/project/connections/delete", s.handleDeleteConnection)
	s.mux.HandleFunc("POST /api/project/input", s.handleUpdateInput)
	s.mux.HandleFunc("POST /api/project/parameters", s.handleUpdateParameters)
	s.mux.HandleFunc("POST /api/project/parameters/delete", s.handleDeleteParameter)
	s.mux.HandleFunc("POST /api/project/source", s.handleUpdateSource)
	s.mux.HandleFunc("POST /api/project/scenarios", s.handleCreateScenario)
	s.mux.HandleFunc("POST /api/validate", s.handleValidate)
	s.mux.HandleFunc("POST /api/run", s.handleRun)
	s.mux.HandleFunc("POST /api/batch", s.handleBatch)
	s.mux.HandleFunc("POST /api/schema", s.handleSchema)
	s.mux.HandleFunc("POST /api/export", s.handleExport)
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
	component, err := createComponent(loaded, req)
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
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
		writeErrorWithProblems(w, apperror.Wrap(apperror.CodeValidation, err), inferProblems(loaded.Graph, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"validation": map[string]any{
			"project_name":     loaded.Project.ProjectName,
			"entry_system":     loaded.Project.EntrySystem,
			"component_count":  len(plan.System.Components),
			"connection_count": len(plan.System.Connections),
			"execution_order":  plan.Order,
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

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := runtimecore.Run(ctx, loaded, input)
	if err != nil {
		writeError(w, err)
		return
	}
	response := map[string]any{"ok": true, "result": result}
	if req.Save {
		runRecord, err := writeRunRecord(loaded, input, result)
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
			return
		}
		response["run_record"] = runRecord
	}
	writeJSON(w, http.StatusOK, response)
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

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(len(scenarios))*30*time.Second)
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
			caseRecord.Error = err.Error()
		} else {
			caseRecord.OK = true
			caseRecord.Result = result
		}
		cases = append(cases, caseRecord)
	}
	summary, record, err := writeBatchRecord(loaded, cases)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "batch": record})
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
	summary, manifest, err := writeExportManifest(loaded, req.Profile)
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
		Project:     loaded.Project,
		Graph:       loaded.Graph,
		ProjectPath: loaded.Path,
		GraphPath:   loaded.GraphPath,
		Root:        loaded.Root,
		Runs:        loadRunSummaries(loaded.Root),
		Batches:     loadBatchSummaries(loaded.Root),
		Exports:     loadExportSummaries(loaded.Root),
		Scenarios:   loadScenarioSummaries(loaded.Root),
	}
	if inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput); err == nil {
		detail.DefaultInputPath = inputPath
		if input, err := runtimecore.LoadInput(inputPath); err == nil {
			detail.DefaultRunInput = &input
		}
	}
	return detail
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
	if err := os.MkdirAll(filepath.Join(projectRoot, "components"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "inputs"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "runs"), 0o755); err != nil {
		return ProjectSummary{}, err
	}

	proj := model.Project{
		ProjectName:   projectName,
		SchemaVersion: "0.1.0",
		EngineVersion: "0.1.0",
		EntrySystem:   "MainSystem",
		Graph:         "graph.json",
		Environment: model.EnvironmentConfig{
			Mode:   "project",
			Python: "python",
		},
		DefaultInput:  "inputs/case01.json",
		DefaultOutput: "runs/latest.json",
	}
	required := true
	graph := model.Graph{
		SchemaVersion: "0.1.0",
		Systems: []model.System{
			{
				ID:          "MainSystem",
				Name:        "Main System",
				Components:  []string{"scalar"},
				Connections: []string{},
				PublicInputs: []model.PublicNodeRef{
					{ID: "value", Name: "Value", Component: "scalar", Node: "value", Medium: "signal", ValueType: "float", Unit: "", Required: &required},
				},
				PublicOutputs: []model.PublicNodeRef{
					{ID: "result", Name: "Result", Component: "scalar", Node: "result", Medium: "signal", ValueType: "float", Unit: ""},
				},
			},
		},
		Components: []model.Component{
			{
				ID:    "scalar",
				Name:  "Scalar Component",
				Kind:  "user_python",
				Class: "components.scalar.ScalarComponent",
				Nodes: model.NodeSet{
					Inputs: []model.Node{
						{ID: "value", Name: "Value", Direction: "inlet", Medium: "signal", ValueType: "float", Unit: "", Required: &required},
					},
					Outputs: []model.Node{
						{ID: "result", Name: "Result", Direction: "outlet", Medium: "signal", ValueType: "float", Unit: ""},
					},
				},
				Parameters: map[string]any{"gain": 2.0},
			},
		},
		Connections: []model.Connection{},
	}
	input := runtimecore.RunInput{
		Inputs:  map[string]any{"value": 4.0},
		Context: map[string]any{"time": 0.0, "dt": 60.0},
	}

	if err := writeJSONFile(filepath.Join(projectRoot, "project.bcsproj"), proj); err != nil {
		return ProjectSummary{}, err
	}
	if err := writeJSONFile(filepath.Join(projectRoot, "graph.json"), graph); err != nil {
		return ProjectSummary{}, err
	}
	if err := writeJSONFile(filepath.Join(projectRoot, "inputs", "case01.json"), input); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "components", "__init__.py"), []byte(""), 0o644); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "components", "scalar.py"), []byte(scalarComponentSource), 0o644); err != nil {
		return ProjectSummary{}, err
	}

	projectPath := filepath.Join(projectRoot, "project.bcsproj")
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

func createComponent(loaded *project.LoadedProject, req createComponentRequest) (model.Component, error) {
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	if template != "scalar" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "unsupported component template: %s", template)
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name must contain letters or numbers")
	}
	className := pythonClassName(componentID)
	required := true
	component := model.Component{
		ID:    componentID,
		Name:  componentName,
		Kind:  "user_python",
		Class: "components." + componentID + "." + className,
		Nodes: model.NodeSet{
			Inputs: []model.Node{
				{ID: "value", Name: "Value", Direction: "inlet", Medium: "signal", ValueType: "float", Unit: "", Required: &required},
			},
			Outputs: []model.Node{
				{ID: "result", Name: "Result", Direction: "outlet", Medium: "signal", ValueType: "float", Unit: ""},
			},
		},
		Parameters: map[string]any{"gain": 1.0},
	}

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
	sourcePath := filepath.Join(componentsRoot, componentID+".py")
	if _, err := os.Stat(sourcePath); err == nil {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component source already exists: components/%s.py", componentID)
	}
	if err := os.WriteFile(sourcePath, []byte(componentSource(className)), 0o644); err != nil {
		return model.Component{}, err
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return component, nil
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

	sourcePath, err := componentSourcePath(loaded, sourceID)
	if err != nil {
		return model.Component{}, err
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return model.Component{}, apperror.Wrap(apperror.CodeValidation, err)
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

	sourceRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	targetPath := filepath.Join(sourceRoot, componentID+".py")
	if _, err := os.Stat(targetPath); err == nil {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component source already exists: components/%s.py", componentID)
	}
	if err := os.WriteFile(targetPath, sourceBytes, 0o644); err != nil {
		return model.Component{}, err
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		_ = os.Remove(targetPath)
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

	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return err
	}
	sourceShared := false
	for index := range loaded.Graph.Components {
		if index == componentIndex {
			continue
		}
		otherPath, err := componentSourcePath(loaded, loaded.Graph.Components[index].ID)
		if err == nil && otherPath == sourcePath {
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
		if err := os.Remove(sourcePath); err != nil && !os.IsNotExist(err) {
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
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
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
		return writeJSONFile(loaded.GraphPath, loaded.Graph)
	}
	return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
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

func loadComponentSource(loaded *project.LoadedProject, componentID string, readOnly bool) (SourceDetail, error) {
	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return SourceDetail{}, err
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return SourceDetail{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, _ := filepath.Rel(loaded.Root, sourcePath)
	return SourceDetail{
		ComponentID:  componentID,
		RelativePath: filepath.ToSlash(rel),
		Content:      string(sourceBytes),
		ReadOnly:     readOnly,
	}, nil
}

func checkComponentSource(ctx context.Context, loaded *project.LoadedProject, req sourceCheckRequest) (SourceCheck, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return SourceCheck{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return SourceCheck{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, sourcePath)
	expectedClass := classNameFromPath(component.Class)
	check := SourceCheck{
		OK:            true,
		ComponentID:   componentID,
		RelativePath:  filepath.ToSlash(rel),
		ExpectedClass: expectedClass,
		LineCount:     countLines(req.Content),
		Problems:      []Problem{},
	}
	if strings.TrimSpace(req.Content) == "" {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: "source is empty", ComponentID: componentID})
	}
	if expectedClass == "" {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: "component class path is invalid", ComponentID: componentID})
	} else if line := findPythonClassLine(req.Content, expectedClass); line == 0 {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: fmt.Sprintf("expected class is missing: %s", expectedClass), ComponentID: componentID})
	}
	if line := findPythonMethodLine(req.Content, "evaluate"); line == 0 {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: "evaluate method is missing", ComponentID: componentID})
	}
	if !strings.Contains(req.Content, "return") {
		check.Problems = append(check.Problems, Problem{Severity: "warning", Message: "source has no return statement", ComponentID: componentID})
	}
	check.Problems = append(check.Problems, pythonSyntaxProblems(ctx, loaded, componentID, filepath.ToSlash(rel), req.Content)...)
	check.OK = !hasErrorProblems(check.Problems)
	return check, nil
}

func componentSourcePath(loaded *project.LoadedProject, componentID string) (string, error) {
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return "", apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	parts := strings.Split(component.Class, ".")
	if len(parts) < 3 || parts[0] != "components" {
		return "", apperror.Errorf(apperror.CodeValidation, "component %s class does not map to a project source file: %s", componentID, component.Class)
	}
	modulePath := filepath.Join(parts[:len(parts)-1]...) + ".py"
	return resolveProjectOwnedFile(loaded.Root, modulePath)
}

func classNameFromPath(classPath string) string {
	classPath = strings.TrimSpace(classPath)
	if classPath == "" {
		return ""
	}
	parts := strings.Split(classPath, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func findPythonClassLine(content string, className string) int {
	pattern := regexp.MustCompile(`(?m)^class\s+` + regexp.QuoteMeta(className) + `\b`)
	return regexpLine(content, pattern.FindStringIndex(content))
}

func findPythonMethodLine(content string, methodName string) int {
	pattern := regexp.MustCompile(`(?m)^\s+def\s+` + regexp.QuoteMeta(methodName) + `\s*\(`)
	return regexpLine(content, pattern.FindStringIndex(content))
}

func regexpLine(content string, match []int) int {
	if len(match) != 2 {
		return 0
	}
	return strings.Count(content[:match[0]], "\n") + 1
}

func pythonSyntaxProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, pythonExe, "-c", "import sys\ncompile(sys.stdin.read(), sys.argv[1], 'exec')", relativePath)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return []Problem{{Severity: "warning", Message: "python syntax check timed out", ComponentID: componentID}}
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			return []Problem{{Severity: "warning", Message: "python syntax check unavailable: " + err.Error(), ComponentID: componentID}}
		}
		return []Problem{syntaxProblemFromStderr(componentID, stderrText)}
	}
	return []Problem{}
}

func syntaxProblemFromStderr(componentID string, stderrText string) Problem {
	line := 0
	linePattern := regexp.MustCompile(`(?m)File ".*", line ([0-9]+)`)
	if match := linePattern.FindStringSubmatch(stderrText); len(match) == 2 {
		fmt.Sscanf(match[1], "%d", &line)
	}
	lines := strings.Split(stderrText, "\n")
	message := strings.TrimSpace(lines[len(lines)-1])
	if message == "" {
		message = "python syntax error"
	}
	return Problem{Severity: "error", Message: message, ComponentID: componentID, Line: line}
}

func resolveStudioPython(projectRoot string, env model.EnvironmentConfig) string {
	if env.Python == "" {
		env.Python = "python"
	}
	if filepath.IsAbs(env.Python) {
		return env.Python
	}
	projectPython := filepath.Join(projectRoot, env.Python)
	if _, err := os.Stat(projectPython); err == nil {
		return projectPython
	}
	if isDefaultPythonName(env.Python) {
		if packagedPython := findPackagedPython(projectRoot); packagedPython != "" {
			return packagedPython
		}
	}
	return env.Python
}

func isDefaultPythonName(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "python" || name == "python.exe" || name == "python3" || name == "python3.exe"
}

func findPackagedPython(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		candidates := []string{
			filepath.Join(absStart, "runtime", "python", "python.exe"),
			filepath.Join(absStart, "runtime", "python", "python"),
			filepath.Join(absStart, "runtime", "python", "bin", "python"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(absStart)
		if parent == absStart {
			return ""
		}
		absStart = parent
	}
}

func hasErrorProblems(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Severity == "error" {
			return true
		}
	}
	return false
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

func decodeDeleteConnectionRequest(r *http.Request) (deleteConnectionRequest, error) {
	defer r.Body.Close()
	var req deleteConnectionRequest
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

func decodeDeleteParameterRequest(r *http.Request) (deleteParameterRequest, error) {
	defer r.Body.Close()
	var req deleteParameterRequest
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeErrorWithProblems(w, err, nil)
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
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		err = appErr.Unwrap()
	}
	writeJSON(w, status, apiError{
		OK:       false,
		Code:     int(code),
		Kind:     apperror.CodeName(code),
		Message:  fmt.Sprint(err),
		Problems: problems,
	})
}

func inferProblems(graph *model.Graph, err error) []Problem {
	message := fmt.Sprint(err)
	problem := Problem{Severity: "error", Message: message}
	for _, component := range graph.Components {
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
	return []Problem{problem}
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

func writeRunRecord(loaded *project.LoadedProject, input runtimecore.RunInput, result *runtimecore.RunResult) (RunSummary, error) {
	now := time.Now().UTC()
	runID := "run-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return RunSummary{}, err
	}
	runPath := filepath.Join(runsRoot, runID+".json")
	record := RunRecord{
		ID:           runID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		Inputs:       input.Inputs,
		Context:      input.Context,
		Result:       result,
	}
	if err := writeJSONFile(runPath, record); err != nil {
		return RunSummary{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, runPath)
	return RunSummary{
		ID:           runID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		Outputs:      result.Outputs,
	}, nil
}

func loadRunSummaries(projectRoot string) []RunSummary {
	runFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "run-*.json"))
	if err != nil {
		return []RunSummary{}
	}
	summaries := []RunSummary{}
	for _, runPath := range runFiles {
		runBytes, err := os.ReadFile(runPath)
		if err != nil {
			continue
		}
		var record RunRecord
		if err := json.Unmarshal(runBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, runPath)
		outputs := map[string]any{}
		if record.Result != nil {
			outputs = record.Result.Outputs
		}
		summaries = append(summaries, RunSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			Outputs:      outputs,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadRunRecord(projectRoot string, runID string) (RunRecord, error) {
	if runID == "" {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id is required")
	}
	if filepath.Base(runID) != runID || strings.ContainsAny(runID, `/\`) {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id must be a run record id")
	}
	runPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", runID+".json"))
	if err != nil {
		return RunRecord{}, err
	}
	runBytes, err := os.ReadFile(runPath)
	if err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record RunRecord
	if err := json.Unmarshal(runBytes, &record); err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func writeBatchRecord(loaded *project.LoadedProject, cases []BatchCaseRecord) (BatchSummary, BatchRecord, error) {
	now := time.Now().UTC()
	batchID := "batch-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	batchPath := filepath.Join(runsRoot, batchID+".json")
	record := BatchRecord{
		ID:           batchID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		Cases:        cases,
	}
	if err := writeJSONFile(batchPath, record); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, batchPath)
	return BatchSummary{
		ID:           batchID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		CaseCount:    len(cases),
		OKCount:      batchOKCount(cases),
	}, record, nil
}

func loadBatchSummaries(projectRoot string) []BatchSummary {
	batchFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "batch-*.json"))
	if err != nil {
		return []BatchSummary{}
	}
	summaries := []BatchSummary{}
	for _, batchPath := range batchFiles {
		batchBytes, err := os.ReadFile(batchPath)
		if err != nil {
			continue
		}
		var record BatchRecord
		if err := json.Unmarshal(batchBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, batchPath)
		summaries = append(summaries, BatchSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			CaseCount:    len(record.Cases),
			OKCount:      batchOKCount(record.Cases),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadBatchRecord(projectRoot string, batchID string) (BatchRecord, error) {
	if batchID == "" {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id is required")
	}
	if filepath.Base(batchID) != batchID || strings.ContainsAny(batchID, `/\`) {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id must be a batch record id")
	}
	batchPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", batchID+".json"))
	if err != nil {
		return BatchRecord{}, err
	}
	batchBytes, err := os.ReadFile(batchPath)
	if err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record BatchRecord
	if err := json.Unmarshal(batchBytes, &record); err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func batchOKCount(cases []BatchCaseRecord) int {
	count := 0
	for _, item := range cases {
		if item.OK {
			count++
		}
	}
	return count
}

func writeScenarioRecord(loaded *project.LoadedProject, req createScenarioRequest) (ScenarioSummary, ScenarioRecord, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name is required")
	}
	scenarioID := slugify(name)
	if scenarioID == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name must contain letters or numbers")
	}
	if req.Inputs == nil {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario inputs are required")
	}
	context := req.Context
	if context == nil {
		context = map[string]any{}
	}
	now := time.Now().UTC()
	scenariosRoot := filepath.Join(loaded.Root, "scenarios")
	if err := os.MkdirAll(scenariosRoot, 0o755); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	scenarioPath := filepath.Join(scenariosRoot, scenarioID+".json")
	if _, err := os.Stat(scenarioPath); err == nil {
		scenarioID = scenarioID + "-" + now.Format("20060102-150405")
		scenarioPath = filepath.Join(scenariosRoot, scenarioID+".json")
	}
	record := ScenarioRecord{
		ID:           scenarioID,
		Name:         name,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		Inputs:       req.Inputs,
		Context:      context,
	}
	if err := writeJSONFile(scenarioPath, record); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, scenarioPath)
	return ScenarioSummary{
		ID:           record.ID,
		Name:         record.Name,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
	}, record, nil
}

func loadScenarioSummaries(projectRoot string) []ScenarioSummary {
	scenarioFiles, err := filepath.Glob(filepath.Join(projectRoot, "scenarios", "*.json"))
	if err != nil {
		return []ScenarioSummary{}
	}
	summaries := []ScenarioSummary{}
	for _, scenarioPath := range scenarioFiles {
		scenarioBytes, err := os.ReadFile(scenarioPath)
		if err != nil {
			continue
		}
		var record ScenarioRecord
		if err := json.Unmarshal(scenarioBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, scenarioPath)
		summaries = append(summaries, ScenarioSummary{
			ID:           record.ID,
			Name:         record.Name,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadScenarioRecord(projectRoot string, scenarioID string) (ScenarioRecord, error) {
	if scenarioID == "" {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id is required")
	}
	if filepath.Base(scenarioID) != scenarioID || strings.ContainsAny(scenarioID, `/\`) {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id must be a scenario id")
	}
	scenarioPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("scenarios", scenarioID+".json"))
	if err != nil {
		return ScenarioRecord{}, err
	}
	scenarioBytes, err := os.ReadFile(scenarioPath)
	if err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record ScenarioRecord
	if err := json.Unmarshal(scenarioBytes, &record); err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func writeExportManifest(loaded *project.LoadedProject, profile string) (ExportSummary, ExportManifest, error) {
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
	projectPath, _ := filepath.Rel(loaded.Root, loaded.Path)
	graphPath, _ := filepath.Rel(loaded.Root, loaded.GraphPath)
	manifest := ExportManifest{
		Profile:        profile,
		CreatedAtUTC:   now.Format(time.RFC3339Nano),
		ProjectName:    loaded.Project.ProjectName,
		ProjectPath:    filepath.ToSlash(projectPath),
		GraphPath:      filepath.ToSlash(graphPath),
		DefaultInput:   filepath.ToSlash(loaded.Project.DefaultInput),
		Runner:         "bin/bcs-runner.exe",
		RuntimePython:  "runtime/python/python.exe",
		Components:     append([]string{}, plan.System.Components...),
		PublicInputs:   append([]model.PublicNodeRef{}, plan.System.PublicInputs...),
		PublicOutputs:  append([]model.PublicNodeRef{}, plan.System.PublicOutputs...),
		ExecutionOrder: append([]string{}, plan.Order...),
	}
	exportPath := filepath.Join(loaded.Root, "exports", profile, "manifest.json")
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

func loadExportSummaries(projectRoot string) []ExportSummary {
	manifestFiles, err := filepath.Glob(filepath.Join(projectRoot, "exports", "*", "manifest.json"))
	if err != nil {
		return []ExportSummary{}
	}
	summaries := []ExportSummary{}
	for _, manifestPath := range manifestFiles {
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest ExportManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, manifestPath)
		profile := manifest.Profile
		if profile == "" {
			profile = filepath.Base(filepath.Dir(manifestPath))
		}
		summaries = append(summaries, ExportSummary{
			Profile:      profile,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: manifest.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
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
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
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

func componentSource(className string) string {
	return fmt.Sprintf(`class %s:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        gain = float(params.get("gain", 1.0))
        return {"result": value * gain}, state
`, className)
}

const scalarComponentSource = `class ScalarComponent:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        gain = float(params.get("gain", 2.0))
        return {"result": value * gain}, state
`
