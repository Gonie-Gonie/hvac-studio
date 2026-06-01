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
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

//go:embed static
var staticFS embed.FS

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

type createComponentRequest struct {
	ProjectPath string `json:"project_path"`
	Name        string `json:"name"`
	Template    string `json:"template"`
}

type includeComponentRequest struct {
	ProjectPath string `json:"project_path"`
	SystemID    string `json:"system_id"`
	ComponentID string `json:"component_id"`
}

type updateParametersRequest struct {
	ProjectPath string                    `json:"project_path"`
	Parameters  map[string]map[string]any `json:"parameters"`
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

type apiError struct {
	OK      bool   `json:"ok"`
	Code    int    `json:"code"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func New(repoRoot string) (*Server, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFS, "static")
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
	s.mux.HandleFunc("GET /api/project", s.handleProject)
	s.mux.HandleFunc("GET /api/project/run", s.handleRunRecord)
	s.mux.HandleFunc("POST /api/project/components", s.handleCreateComponent)
	s.mux.HandleFunc("POST /api/project/system/components", s.handleIncludeComponent)
	s.mux.HandleFunc("POST /api/project/input", s.handleUpdateInput)
	s.mux.HandleFunc("POST /api/project/parameters", s.handleUpdateParameters)
	s.mux.HandleFunc("POST /api/validate", s.handleValidate)
	s.mux.HandleFunc("POST /api/run", s.handleRun)
	s.mux.HandleFunc("POST /api/schema", s.handleSchema)
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
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
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

func decodeCreateComponentRequest(r *http.Request) (createComponentRequest, error) {
	defer r.Body.Close()
	var req createComponentRequest
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

func decodeUpdateParametersRequest(r *http.Request) (updateParametersRequest, error) {
	defer r.Body.Close()
	var req updateParametersRequest
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
		OK:      false,
		Code:    int(code),
		Kind:    apperror.CodeName(code),
		Message: fmt.Sprint(err),
	})
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

func writeJSONFile(path string, value any) error {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(output, '\n'), 0o644)
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

func findComponent(graph *model.Graph, componentID string) (model.Component, bool) {
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
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
