package studio

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

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
