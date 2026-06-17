package calibration

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/artifactmeta"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type Setup struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Path             string          `json:"-"`
	Algorithm        string          `json:"algorithm"`
	Mapping          string          `json:"mapping"`
	BaseParameterSet string          `json:"base_parameter_set,omitempty"`
	Objective        Objective       `json:"objective"`
	Parameters       []ParameterSpec `json:"parameters"`
}

type Objective struct {
	Metric  string             `json:"metric"`
	Outputs map[string]float64 `json:"outputs"`
}

type ParameterSpec struct {
	Component string  `json:"component"`
	Name      string  `json:"name"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Step      float64 `json:"step"`
}

type Options struct {
	SaveParameterSet string
}

type Result struct {
	OK                bool                         `json:"ok"`
	SetupID           string                       `json:"setup_id"`
	SetupName         string                       `json:"setup_name,omitempty"`
	Setup             string                       `json:"setup,omitempty"`
	Algorithm         string                       `json:"algorithm"`
	Mapping           string                       `json:"mapping"`
	BaseParameterSet  string                       `json:"base_parameter_set,omitempty"`
	SavedRecord       string                       `json:"saved_record,omitempty"`
	InitialObjective  float64                      `json:"initial_objective"`
	BestObjective     float64                      `json:"best_objective"`
	ChangedParameters map[string]map[string]Change `json:"changed_parameters"`
	BestParameterSet  parameterset.Set             `json:"best_parameter_set"`
	SavedParameterSet string                       `json:"saved_parameter_set,omitempty"`
	Candidates        []CandidateSummary           `json:"candidates"`
}

type RecordSummary struct {
	ID                string  `json:"id"`
	RelativePath      string  `json:"relative_path"`
	CreatedAtUTC      string  `json:"created_at_utc"`
	SetupID           string  `json:"setup_id"`
	SetupName         string  `json:"setup_name,omitempty"`
	Algorithm         string  `json:"algorithm"`
	BaseParameterSet  string  `json:"base_parameter_set,omitempty"`
	SavedParameterSet string  `json:"saved_parameter_set,omitempty"`
	BestObjective     float64 `json:"best_objective"`
	CandidateCount    int     `json:"candidate_count"`
	OK                bool    `json:"ok"`
}

type Record struct {
	ID           string                  `json:"id"`
	ProjectName  string                  `json:"project_name"`
	CreatedAtUTC string                  `json:"created_at_utc"`
	SetupID      string                  `json:"setup_id"`
	SetupName    string                  `json:"setup_name,omitempty"`
	Algorithm    string                  `json:"algorithm"`
	Provenance   artifactmeta.Provenance `json:"provenance,omitempty"`
	Result       *Result                 `json:"result"`
}

type Change struct {
	Initial any `json:"initial"`
	Best    any `json:"best"`
}

type CandidateSummary struct {
	Index      int                           `json:"index"`
	Parameters map[string]map[string]float64 `json:"parameters"`
	Objective  float64                       `json:"objective"`
	Metrics    map[string]float64            `json:"metrics"`
}

func WriteRecord(loaded *project.LoadedProject, result *Result) (RecordSummary, error) {
	if result == nil {
		return RecordSummary{}, apperror.Errorf(apperror.CodeRuntime, "calibration result is required")
	}
	provenance, err := artifactmeta.Build(loaded, []artifactmeta.Reference{
		{Role: "calibration_setup", Path: result.Setup},
		{Role: "validation_mapping", Path: result.Mapping},
		{Role: "base_parameter_set", Path: result.BaseParameterSet},
		{Role: "saved_parameter_set", Path: result.SavedParameterSet},
	})
	if err != nil {
		return RecordSummary{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	now := time.Now().UTC()
	recordID := "calibration-" + now.Format("20060102-150405.000000000")
	recordPath := filepath.Join(loaded.Root, "calibration", "results", recordID+".json")
	record := Record{
		ID:           recordID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		SetupID:      result.SetupID,
		SetupName:    result.SetupName,
		Algorithm:    result.Algorithm,
		Provenance:   provenance,
		Result:       result,
	}
	rel, _ := filepath.Rel(loaded.Root, recordPath)
	result.SavedRecord = filepath.ToSlash(rel)
	if err := writeJSONFile(recordPath, record); err != nil {
		result.SavedRecord = ""
		return RecordSummary{}, err
	}
	return summarizeRecord(loaded.Root, recordPath, record), nil
}

func LoadRecord(projectRoot string, recordID string) (Record, error) {
	if recordID == "" {
		return Record{}, apperror.Errorf(apperror.CodeValidation, "calibration_record_id is required")
	}
	if filepath.Base(recordID) != recordID || strings.ContainsAny(recordID, `/\`) {
		return Record{}, apperror.Errorf(apperror.CodeValidation, "calibration_record_id must be a calibration record id")
	}
	recordPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("calibration", "results", recordID+".json"))
	if err != nil {
		return Record{}, err
	}
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		return Record{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record Record
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		return Record{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func LoadRecordSummaries(projectRoot string) []RecordSummary {
	recordFiles, err := filepath.Glob(filepath.Join(projectRoot, "calibration", "results", "calibration-*.json"))
	if err != nil {
		return []RecordSummary{}
	}
	summaries := []RecordSummary{}
	for _, recordPath := range recordFiles {
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			continue
		}
		var record Record
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			continue
		}
		summaries = append(summaries, summarizeRecord(projectRoot, recordPath, record))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func summarizeRecord(projectRoot string, recordPath string, record Record) RecordSummary {
	rel, _ := filepath.Rel(projectRoot, recordPath)
	summary := RecordSummary{
		ID:           record.ID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		SetupID:      record.SetupID,
		SetupName:    record.SetupName,
		Algorithm:    record.Algorithm,
	}
	if record.Result != nil {
		summary.OK = record.Result.OK
		summary.BestObjective = record.Result.BestObjective
		summary.BaseParameterSet = record.Result.BaseParameterSet
		summary.SavedParameterSet = record.Result.SavedParameterSet
		summary.CandidateCount = len(record.Result.Candidates)
		if summary.SetupID == "" {
			summary.SetupID = record.Result.SetupID
		}
		if summary.SetupName == "" {
			summary.SetupName = record.Result.SetupName
		}
		if summary.Algorithm == "" {
			summary.Algorithm = record.Result.Algorithm
		}
	}
	return summary
}

func LoadSetup(projectRoot string, relativePath string) (Setup, error) {
	resolved, err := resolveProjectOwnedFile(projectRoot, relativePath)
	if err != nil {
		return Setup{}, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Setup{}, apperror.Wrap(apperror.CodeInput, err)
	}
	var setup Setup
	if err := json.Unmarshal(data, &setup); err != nil {
		return Setup{}, apperror.Wrap(apperror.CodeInput, err)
	}
	if setup.ID == "" {
		setup.ID = strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
	}
	if rel, err := filepath.Rel(projectRoot, resolved); err == nil {
		setup.Path = filepath.ToSlash(rel)
	}
	if setup.Algorithm == "" {
		setup.Algorithm = "grid"
	}
	if !isSupportedAlgorithm(setup.Algorithm) {
		return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported calibration algorithm: %s", setup.Algorithm)
	}
	if setup.Mapping == "" {
		return Setup{}, apperror.Errorf(apperror.CodeInput, "calibration setup mapping is required")
	}
	if len(setup.Parameters) == 0 {
		return Setup{}, apperror.Errorf(apperror.CodeInput, "calibration setup parameters is required")
	}
	if setup.Objective.Metric == "" {
		setup.Objective.Metric = "rmse"
	}
	if setup.Objective.Metric != "rmse" {
		return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported calibration objective metric: %s", setup.Objective.Metric)
	}
	return setup, nil
}

func Run(ctx context.Context, projectPath string, setup Setup, options Options) (*Result, error) {
	initialLoaded, err := project.Load(projectPath)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	if setup.BaseParameterSet != "" {
		if _, err := parameterset.ApplyFile(initialLoaded, setup.BaseParameterSet); err != nil {
			return nil, err
		}
	}
	initialValues, err := currentParameterValues(initialLoaded, setup.Parameters)
	if err != nil {
		return nil, err
	}

	candidates := candidateParameters(setup.Algorithm, setup.Parameters)
	if len(candidates) == 0 {
		return nil, apperror.Errorf(apperror.CodeInput, "calibration %s produced no candidates", setup.Algorithm)
	}

	var best CandidateSummary
	var initialObjective float64
	candidateSummaries := []CandidateSummary{}
	for index, candidate := range candidates {
		summary, err := evaluateCandidate(ctx, projectPath, setup, index, candidate)
		if err != nil {
			return nil, err
		}
		candidateSummaries = append(candidateSummaries, summary)
		if sameParameterValues(candidate, initialValues) {
			initialObjective = summary.Objective
		}
		if index == 0 || summary.Objective < best.Objective {
			best = summary
		}
	}
	if initialObjective == 0 {
		initialSummary, err := evaluateCandidate(ctx, projectPath, setup, -1, initialValues)
		if err != nil {
			return nil, err
		}
		initialObjective = initialSummary.Objective
	}

	bestSet := parameterset.Set{
		ID:         setup.ID + "_result",
		Name:       setup.Name + " Result",
		Components: floatParametersToAny(best.Parameters),
	}
	result := &Result{
		OK:                true,
		SetupID:           setup.ID,
		SetupName:         setup.Name,
		Setup:             setup.Path,
		Algorithm:         setup.Algorithm,
		Mapping:           filepath.ToSlash(setup.Mapping),
		BaseParameterSet:  filepath.ToSlash(setup.BaseParameterSet),
		InitialObjective:  initialObjective,
		BestObjective:     best.Objective,
		ChangedParameters: changedParameters(initialValues, best.Parameters),
		BestParameterSet:  bestSet,
		Candidates:        candidateSummaries,
	}
	if options.SaveParameterSet != "" {
		if err := parameterset.Write(initialLoaded.Root, options.SaveParameterSet, bestSet); err != nil {
			return nil, err
		}
		result.SavedParameterSet = filepath.ToSlash(options.SaveParameterSet)
	}
	return result, nil
}

func evaluateCandidate(ctx context.Context, projectPath string, setup Setup, index int, values map[string]map[string]float64) (CandidateSummary, error) {
	loaded, err := project.Load(projectPath)
	if err != nil {
		return CandidateSummary{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if setup.BaseParameterSet != "" {
		if _, err := parameterset.ApplyFile(loaded, setup.BaseParameterSet); err != nil {
			return CandidateSummary{}, err
		}
	}
	if err := parameterset.Apply(loaded.Graph, parameterset.Set{Components: floatParametersToAny(values)}); err != nil {
		return CandidateSummary{}, err
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, setup.Mapping)
	if err != nil {
		return CandidateSummary{}, err
	}
	validation, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: 1})
	if err != nil {
		return CandidateSummary{}, err
	}
	objective, metrics := objectiveValue(setup.Objective, validation)
	return CandidateSummary{
		Index:      index,
		Parameters: cloneFloatParameters(values),
		Objective:  objective,
		Metrics:    metrics,
	}, nil
}

func isSupportedAlgorithm(algorithm string) bool {
	switch algorithm {
	case "grid", "least_squares":
		return true
	default:
		return false
	}
}

func candidateParameters(algorithm string, specs []ParameterSpec) []map[string]map[string]float64 {
	switch algorithm {
	case "least_squares":
		return gridCandidates(specs)
	default:
		return gridCandidates(specs)
	}
}

func objectiveValue(objective Objective, validation *modelvalidation.Result) (float64, map[string]float64) {
	weights := objective.Outputs
	if len(weights) == 0 {
		weights = map[string]float64{}
		for outputID := range validation.Metrics {
			weights[outputID] = 1.0
		}
	}
	metrics := map[string]float64{}
	total := 0.0
	for outputID, weight := range weights {
		value := validation.Metrics[outputID].RMSE
		metrics[outputID] = value
		total += weight * value
	}
	return total, metrics
}

func currentParameterValues(loaded *project.LoadedProject, specs []ParameterSpec) (map[string]map[string]float64, error) {
	values := map[string]map[string]float64{}
	components := map[string]map[string]any{}
	for _, component := range loaded.Graph.Components {
		components[component.ID] = component.Parameters
	}
	for _, spec := range specs {
		componentParams, ok := components[spec.Component]
		if !ok {
			return nil, apperror.Errorf(apperror.CodeValidation, "calibration component is not in graph: %s", spec.Component)
		}
		raw, ok := componentParams[spec.Name]
		if !ok {
			return nil, apperror.Errorf(apperror.CodeValidation, "calibration parameter is not in graph: %s.%s", spec.Component, spec.Name)
		}
		value, ok := numberValue(raw)
		if !ok {
			return nil, apperror.Errorf(apperror.CodeValidation, "calibration parameter must be numeric: %s.%s", spec.Component, spec.Name)
		}
		if values[spec.Component] == nil {
			values[spec.Component] = map[string]float64{}
		}
		values[spec.Component][spec.Name] = value
	}
	return values, nil
}

func gridCandidates(specs []ParameterSpec) []map[string]map[string]float64 {
	candidates := []map[string]map[string]float64{{}}
	for _, spec := range specs {
		values := gridValues(spec)
		next := []map[string]map[string]float64{}
		for _, candidate := range candidates {
			for _, value := range values {
				cloned := cloneFloatParameters(candidate)
				if cloned[spec.Component] == nil {
					cloned[spec.Component] = map[string]float64{}
				}
				cloned[spec.Component][spec.Name] = value
				next = append(next, cloned)
			}
		}
		candidates = next
	}
	return candidates
}

func gridValues(spec ParameterSpec) []float64 {
	if spec.Step <= 0 {
		return []float64{spec.Min}
	}
	values := []float64{}
	for value := spec.Min; value <= spec.Max+spec.Step/1000.0; value += spec.Step {
		values = append(values, math.Round(value*1e9)/1e9)
	}
	return values
}

func changedParameters(initial, best map[string]map[string]float64) map[string]map[string]Change {
	changes := map[string]map[string]Change{}
	for componentID, params := range best {
		for name, bestValue := range params {
			initialValue := initial[componentID][name]
			if initialValue == bestValue {
				continue
			}
			if changes[componentID] == nil {
				changes[componentID] = map[string]Change{}
			}
			changes[componentID][name] = Change{Initial: initialValue, Best: bestValue}
		}
	}
	return changes
}

func sameParameterValues(candidate, initial map[string]map[string]float64) bool {
	for componentID, params := range initial {
		for name, value := range params {
			if candidate[componentID][name] != value {
				return false
			}
		}
	}
	return true
}

func floatParametersToAny(values map[string]map[string]float64) map[string]map[string]any {
	out := map[string]map[string]any{}
	for componentID, params := range values {
		out[componentID] = map[string]any{}
		for name, value := range params {
			out[componentID][name] = value
		}
	}
	return out
}

func cloneFloatParameters(values map[string]map[string]float64) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for componentID, params := range values {
		out[componentID] = map[string]float64{}
		for name, value := range params {
			out[componentID][name] = value
		}
	}
	return out
}

func numberValue(value any) (float64, bool) {
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

func resolveProjectOwnedFile(projectRoot string, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", apperror.Errorf(apperror.CodeInput, "project path must be relative: %s", relativePath)
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
		return "", apperror.Errorf(apperror.CodeInput, "project path escapes project root: %s", relativePath)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", apperror.Wrap(apperror.CodeInput, err)
	}
	return resolved, nil
}

func writeJSONFile(path string, value any) error {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	return apperror.Wrap(apperror.CodeRuntime, os.WriteFile(path, append(output, '\n'), 0o644))
}

func sortedCandidateSummaries(candidates []CandidateSummary) []CandidateSummary {
	out := append([]CandidateSummary(nil), candidates...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Objective < out[j].Objective
	})
	return out
}
