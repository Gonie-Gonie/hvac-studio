package studio

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

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
