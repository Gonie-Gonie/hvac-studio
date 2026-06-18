package studio

import (
	"net/http"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

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
