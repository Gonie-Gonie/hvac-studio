package studio

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

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

func componentSourcePath(loaded *project.LoadedProject, componentID string) (string, error) {
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return "", apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	if sourceRel := editableComponentSource(component); sourceRel != "" {
		return resolveProjectOwnedFile(loaded.Root, sourceRel)
	}
	parts := strings.Split(component.Class, ".")
	if len(parts) < 3 || parts[0] != "components" {
		return "", apperror.Errorf(apperror.CodeValidation, "component %s class does not map to a project source file: %s", componentID, component.Class)
	}
	modulePath := filepath.Join(parts[:len(parts)-1]...) + ".py"
	return resolveProjectOwnedFile(loaded.Root, modulePath)
}

func editableComponentSource(component model.Component) string {
	source := component.Source
	switch source.Layout {
	case "generated_wrapper":
		return strings.TrimSpace(source.Step)
	case "single_file_class", "":
		if strings.TrimSpace(source.Step) != "" {
			return strings.TrimSpace(source.Step)
		}
	}
	return ""
}

func componentSourceArtifactPath(loaded *project.LoadedProject, component model.Component) (string, error) {
	if component.Source.Layout == "generated_wrapper" {
		sourceDir, err := generatedComponentSourceDir(component.Source)
		if err != nil {
			return "", err
		}
		return resolveProjectOwnedFile(loaded.Root, sourceDir)
	}
	if sourceRel := editableComponentSource(component); sourceRel != "" {
		return resolveProjectOwnedFile(loaded.Root, sourceRel)
	}
	parts := strings.Split(component.Class, ".")
	if len(parts) < 3 || parts[0] != "components" {
		return "", apperror.Errorf(apperror.CodeValidation, "component %s class does not map to a project source file: %s", component.ID, component.Class)
	}
	modulePath := filepath.Join(parts[:len(parts)-1]...) + ".py"
	return resolveProjectOwnedFile(loaded.Root, modulePath)
}

func generatedComponentSourceDir(source model.ComponentSource) (string, error) {
	paths := []string{source.Metadata, source.Init, source.Step, source.Helpers, source.Wrapper}
	sourceDir := ""
	for _, sourcePath := range paths {
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			continue
		}
		clean, err := cleanRelativePath(sourcePath)
		if err != nil {
			return "", err
		}
		dir := filepath.ToSlash(filepath.Dir(clean))
		if dir == "." || dir == "" {
			return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source files must live in a component directory")
		}
		if sourceDir == "" {
			sourceDir = dir
			continue
		}
		if dir != sourceDir {
			return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source files must share one directory")
		}
	}
	if sourceDir == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source directory is missing")
	}
	return sourceDir, nil
}

func rebaseComponentSource(source model.ComponentSource, oldDir string, newDir string) model.ComponentSource {
	return model.ComponentSource{
		Layout:   source.Layout,
		Metadata: rebaseComponentSourcePath(source.Metadata, oldDir, newDir),
		Init:     rebaseComponentSourcePath(source.Init, oldDir, newDir),
		Step:     rebaseComponentSourcePath(source.Step, oldDir, newDir),
		Helpers:  rebaseComponentSourcePath(source.Helpers, oldDir, newDir),
		Wrapper:  rebaseComponentSourcePath(source.Wrapper, oldDir, newDir),
	}
}

func rebaseComponentSourcePath(sourcePath string, oldDir string, newDir string) string {
	sourcePath = strings.TrimSpace(filepath.ToSlash(sourcePath))
	if sourcePath == "" {
		return ""
	}
	oldDir = strings.TrimSuffix(strings.TrimSpace(filepath.ToSlash(oldDir)), "/")
	newDir = strings.TrimSuffix(strings.TrimSpace(filepath.ToSlash(newDir)), "/")
	if sourcePath == oldDir {
		return newDir
	}
	prefix := oldDir + "/"
	if strings.HasPrefix(sourcePath, prefix) {
		return newDir + "/" + strings.TrimPrefix(sourcePath, prefix)
	}
	return filepath.ToSlash(filepath.Join(newDir, filepath.Base(sourcePath)))
}

func copyComponentSourceArtifact(loaded *project.LoadedProject, source model.Component, target model.Component) (string, error) {
	sourcePath, err := componentSourceArtifactPath(loaded, source)
	if err != nil {
		return "", err
	}
	targetPath, err := componentSourceArtifactPath(loaded, target)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(targetPath); err == nil {
		rel, _ := filepath.Rel(loaded.Root, targetPath)
		return "", apperror.Errorf(apperror.CodeValidation, "component source already exists: %s", filepath.ToSlash(rel))
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if target.Source.Layout == "generated_wrapper" {
		if err := copyProjectTree(sourcePath, targetPath); err != nil {
			return "", err
		}
		return targetPath, nil
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, sourceBytes, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func removeComponentSourceArtifact(path string, layout string) error {
	if layout == "generated_wrapper" {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
