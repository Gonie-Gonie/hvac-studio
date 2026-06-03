package parameterset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type Set struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	CreatedAtUTC string                    `json:"created_at_utc,omitempty"`
	Components   map[string]map[string]any `json:"components"`
}

func ApplyFile(loaded *project.LoadedProject, relativePath string) (Set, error) {
	set, err := Load(loaded.Root, relativePath)
	if err != nil {
		return Set{}, err
	}
	if err := Apply(loaded.Graph, set); err != nil {
		return Set{}, err
	}
	return set, nil
}

func Load(projectRoot string, relativePath string) (Set, error) {
	if strings.TrimSpace(relativePath) == "" {
		return Set{}, apperror.Errorf(apperror.CodeValidation, "parameter set path is required")
	}
	resolved, err := resolveProjectOwnedFile(projectRoot, relativePath)
	if err != nil {
		return Set{}, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Set{}, apperror.Wrap(apperror.CodeInput, err)
	}
	var set Set
	if err := json.Unmarshal(data, &set); err != nil {
		return Set{}, apperror.Wrap(apperror.CodeInput, err)
	}
	if set.ID == "" {
		set.ID = strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
	}
	if set.Components == nil {
		return Set{}, apperror.Errorf(apperror.CodeInput, "parameter set components is required")
	}
	return set, nil
}

func Apply(graph *model.Graph, set Set) error {
	components := map[string]*model.Component{}
	for index := range graph.Components {
		components[graph.Components[index].ID] = &graph.Components[index]
	}
	for componentID, values := range set.Components {
		component, ok := components[componentID]
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter set component is not in graph: %s", componentID)
		}
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		for name, value := range values {
			if _, exists := component.Parameters[name]; !exists {
				return apperror.Errorf(apperror.CodeValidation, "parameter set parameter is not in graph: %s.%s", componentID, name)
			}
			component.Parameters[name] = value
		}
	}
	return nil
}

func resolveProjectOwnedFile(projectRoot string, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", apperror.Errorf(apperror.CodeInput, "parameter set path must be project-relative: %s", relativePath)
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeRuntime, err)
	}
	resolved, err := filepath.Abs(filepath.Join(absRoot, relativePath))
	if err != nil {
		return "", apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeRuntime, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", apperror.Errorf(apperror.CodeInput, "parameter set path escapes project root: %s", relativePath)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", apperror.Wrap(apperror.CodeInput, err)
	}
	return resolved, nil
}
