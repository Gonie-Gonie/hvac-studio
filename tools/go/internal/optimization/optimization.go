package optimization

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
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type Setup struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Path              string             `json:"-"`
	Algorithm         string             `json:"algorithm"`
	BaseInputs        map[string]any     `json:"base_inputs"`
	Context           map[string]any     `json:"context"`
	Objective         Objective          `json:"objective"`
	DecisionVariables []DecisionVariable `json:"decision_variables"`
}

type Objective struct {
	Output string `json:"output"`
	Sense  string `json:"sense"`
}

type DecisionVariable struct {
	Kind string  `json:"kind"`
	Name string  `json:"name"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step"`
}

type Options struct {
	SaveScenario string
}

type Result struct {
	OK            bool               `json:"ok"`
	SetupID       string             `json:"setup_id"`
	SetupName     string             `json:"setup_name,omitempty"`
	Setup         string             `json:"setup,omitempty"`
	Algorithm     string             `json:"algorithm"`
	Objective     Objective          `json:"objective"`
	SavedRecord   string             `json:"saved_record,omitempty"`
	BestObjective float64            `json:"best_objective"`
	BestInputs    map[string]any     `json:"best_inputs"`
	BestOutputs   map[string]any     `json:"best_outputs"`
	SavedScenario string             `json:"saved_scenario,omitempty"`
	Candidates    []CandidateSummary `json:"candidates"`
}

type RecordSummary struct {
	ID             string    `json:"id"`
	RelativePath   string    `json:"relative_path"`
	CreatedAtUTC   string    `json:"created_at_utc"`
	SetupID        string    `json:"setup_id"`
	SetupName      string    `json:"setup_name,omitempty"`
	Algorithm      string    `json:"algorithm"`
	Objective      Objective `json:"objective"`
	SavedScenario  string    `json:"saved_scenario,omitempty"`
	BestObjective  float64   `json:"best_objective"`
	CandidateCount int       `json:"candidate_count"`
	OK             bool      `json:"ok"`
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

type CandidateSummary struct {
	Index     int            `json:"index"`
	Inputs    map[string]any `json:"inputs"`
	Objective float64        `json:"objective"`
	Outputs   map[string]any `json:"outputs"`
}

func WriteRecord(loaded *project.LoadedProject, result *Result) (RecordSummary, error) {
	if result == nil {
		return RecordSummary{}, apperror.Errorf(apperror.CodeRuntime, "optimization result is required")
	}
	provenance, err := artifactmeta.Build(loaded, []artifactmeta.Reference{
		{Role: "optimization_setup", Path: result.Setup},
		{Role: "saved_scenario", Path: result.SavedScenario},
	})
	if err != nil {
		return RecordSummary{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	now := time.Now().UTC()
	recordID := "optimization-" + now.Format("20060102-150405.000000000")
	recordPath := filepath.Join(loaded.Root, "optimization", "results", recordID+".json")
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
		return Record{}, apperror.Errorf(apperror.CodeValidation, "optimization_record_id is required")
	}
	if filepath.Base(recordID) != recordID || strings.ContainsAny(recordID, `/\`) {
		return Record{}, apperror.Errorf(apperror.CodeValidation, "optimization_record_id must be an optimization record id")
	}
	recordPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("optimization", "results", recordID+".json"))
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
	recordFiles, err := filepath.Glob(filepath.Join(projectRoot, "optimization", "results", "optimization-*.json"))
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
		summary.Objective = record.Result.Objective
		summary.BestObjective = record.Result.BestObjective
		summary.SavedScenario = record.Result.SavedScenario
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
	if setup.Algorithm != "grid" {
		return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization algorithm: %s", setup.Algorithm)
	}
	if setup.Objective.Output == "" {
		return Setup{}, apperror.Errorf(apperror.CodeInput, "optimization objective output is required")
	}
	if setup.Objective.Sense == "" {
		setup.Objective.Sense = "min"
	}
	if setup.Objective.Sense != "min" && setup.Objective.Sense != "max" {
		return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization objective sense: %s", setup.Objective.Sense)
	}
	if len(setup.DecisionVariables) == 0 {
		return Setup{}, apperror.Errorf(apperror.CodeInput, "optimization decision_variables is required")
	}
	for _, variable := range setup.DecisionVariables {
		if variable.Kind != "public_input" {
			return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization variable kind: %s", variable.Kind)
		}
	}
	if setup.BaseInputs == nil {
		setup.BaseInputs = map[string]any{}
	}
	if setup.Context == nil {
		setup.Context = map[string]any{}
	}
	return setup, nil
}

func Run(ctx context.Context, projectPath string, setup Setup, options Options) (*Result, error) {
	loaded, err := project.Load(projectPath)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	candidates := gridCandidates(setup)
	if len(candidates) == 0 {
		return nil, apperror.Errorf(apperror.CodeInput, "optimization grid produced no candidates")
	}

	var best CandidateSummary
	summaries := []CandidateSummary{}
	for index, inputs := range candidates {
		result, err := runtimecore.Run(ctx, loaded, runtimecore.RunInput{
			Inputs:  inputs,
			Context: cloneMap(setup.Context),
		})
		if err != nil {
			return nil, err
		}
		objective, err := objectiveValue(setup.Objective, result.Outputs)
		if err != nil {
			return nil, err
		}
		summary := CandidateSummary{
			Index:     index,
			Inputs:    cloneMap(inputs),
			Objective: objective,
			Outputs:   result.Outputs,
		}
		summaries = append(summaries, summary)
		if index == 0 || better(setup.Objective.Sense, summary.Objective, best.Objective) {
			best = summary
		}
	}

	result := &Result{
		OK:            true,
		SetupID:       setup.ID,
		SetupName:     setup.Name,
		Setup:         setup.Path,
		Algorithm:     setup.Algorithm,
		Objective:     setup.Objective,
		BestObjective: best.Objective,
		BestInputs:    best.Inputs,
		BestOutputs:   best.Outputs,
		Candidates:    summaries,
	}
	if options.SaveScenario != "" {
		if err := writeScenario(loaded.Root, options.SaveScenario, setup, best); err != nil {
			return nil, err
		}
		result.SavedScenario = filepath.ToSlash(options.SaveScenario)
	}
	return result, nil
}

func gridCandidates(setup Setup) []map[string]any {
	candidates := []map[string]any{cloneMap(setup.BaseInputs)}
	for _, variable := range setup.DecisionVariables {
		values := gridValues(variable)
		next := []map[string]any{}
		for _, candidate := range candidates {
			for _, value := range values {
				cloned := cloneMap(candidate)
				cloned[variable.Name] = value
				next = append(next, cloned)
			}
		}
		candidates = next
	}
	return candidates
}

func gridValues(variable DecisionVariable) []float64 {
	if variable.Step <= 0 {
		return []float64{variable.Min}
	}
	values := []float64{}
	for value := variable.Min; value <= variable.Max+variable.Step/1000.0; value += variable.Step {
		values = append(values, math.Round(value*1e9)/1e9)
	}
	return values
}

func objectiveValue(objective Objective, outputs map[string]any) (float64, error) {
	value, ok := outputs[objective.Output]
	if !ok {
		return 0, apperror.Errorf(apperror.CodeRuntime, "optimization objective output is missing: %s", objective.Output)
	}
	number, ok := numberValue(value)
	if !ok {
		return 0, apperror.Errorf(apperror.CodeRuntime, "optimization objective output must be numeric: %s", objective.Output)
	}
	return number, nil
}

func better(sense string, value float64, incumbent float64) bool {
	if sense == "max" {
		return value > incumbent
	}
	return value < incumbent
}

func writeScenario(projectRoot string, relativePath string, setup Setup, best CandidateSummary) error {
	resolved, err := resolveProjectOutputFile(projectRoot, relativePath)
	if err != nil {
		return err
	}
	record := map[string]any{
		"id":             strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved)),
		"name":           setup.Name + " Result",
		"optimization":   setup.ID,
		"inputs":         best.Inputs,
		"context":        setup.Context,
		"best_objective": best.Objective,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	return apperror.Wrap(apperror.CodeRuntime, os.WriteFile(resolved, append(data, '\n'), 0o644))
}

func cloneMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
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

func resolveProjectOutputFile(projectRoot string, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", apperror.Errorf(apperror.CodeInput, "project output path must be relative: %s", relativePath)
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
		return "", apperror.Errorf(apperror.CodeInput, "project output path escapes project root: %s", relativePath)
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
