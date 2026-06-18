package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type SeriesInputSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RelativePath    string   `json:"relative_path"`
	StepCount       int      `json:"step_count"`
	TimeKey         string   `json:"time_key"`
	BaseContextKeys []string `json:"base_context_keys,omitempty"`
	StepContextKeys []string `json:"step_context_keys,omitempty"`
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
