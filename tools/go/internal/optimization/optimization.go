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
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

const failedCandidatePenalty = 1e99

type Setup struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Path              string             `json:"-"`
	Algorithm         string             `json:"algorithm"`
	BaseInputs        map[string]any     `json:"base_inputs"`
	Context           map[string]any     `json:"context"`
	BaseParameterSet  string             `json:"base_parameter_set,omitempty"`
	Objective         Objective          `json:"objective"`
	DecisionVariables []DecisionVariable `json:"decision_variables"`
	Constraints       []Constraint       `json:"constraints,omitempty"`
}

type Objective struct {
	Output string `json:"output"`
	Sense  string `json:"sense"`
}

type DecisionVariable struct {
	Kind      string  `json:"kind"`
	Component string  `json:"component,omitempty"`
	Name      string  `json:"name"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Step      float64 `json:"step"`
}

type Constraint struct {
	Output    string  `json:"output"`
	Operator  string  `json:"operator"`
	Value     float64 `json:"value"`
	Tolerance float64 `json:"tolerance,omitempty"`
	Penalty   float64 `json:"penalty,omitempty"`
}

type Options struct {
	SaveScenario     string
	SaveParameterSet string
}

type Result struct {
	OK                bool                      `json:"ok"`
	SetupID           string                    `json:"setup_id"`
	SetupName         string                    `json:"setup_name,omitempty"`
	Setup             string                    `json:"setup,omitempty"`
	Algorithm         string                    `json:"algorithm"`
	BaseParameterSet  string                    `json:"base_parameter_set,omitempty"`
	Objective         Objective                 `json:"objective"`
	SavedRecord       string                    `json:"saved_record,omitempty"`
	BestObjective     float64                   `json:"best_objective"`
	BestInputs        map[string]any            `json:"best_inputs"`
	BestParameters    map[string]map[string]any `json:"best_parameters,omitempty"`
	BestOutputs       map[string]any            `json:"best_outputs"`
	SavedScenario     string                    `json:"saved_scenario,omitempty"`
	SavedParameterSet string                    `json:"saved_parameter_set,omitempty"`
	Candidates        []CandidateSummary        `json:"candidates"`
}

type RecordSummary struct {
	ID                string    `json:"id"`
	RelativePath      string    `json:"relative_path"`
	CreatedAtUTC      string    `json:"created_at_utc"`
	SetupID           string    `json:"setup_id"`
	SetupName         string    `json:"setup_name,omitempty"`
	Algorithm         string    `json:"algorithm"`
	BaseParameterSet  string    `json:"base_parameter_set,omitempty"`
	Objective         Objective `json:"objective"`
	SavedScenario     string    `json:"saved_scenario,omitempty"`
	SavedParameterSet string    `json:"saved_parameter_set,omitempty"`
	BestObjective     float64   `json:"best_objective"`
	CandidateCount    int       `json:"candidate_count"`
	OK                bool      `json:"ok"`
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
	Index                int                       `json:"index"`
	Inputs               map[string]any            `json:"inputs"`
	Parameters           map[string]map[string]any `json:"parameters,omitempty"`
	Objective            float64                   `json:"objective"`
	Outputs              map[string]any            `json:"outputs"`
	Feasible             bool                      `json:"feasible"`
	ConstraintPenalty    float64                   `json:"constraint_penalty,omitempty"`
	ConstraintViolations []ConstraintViolation     `json:"constraint_violations,omitempty"`
	Error                string                    `json:"error,omitempty"`
}

type ConstraintViolation struct {
	Output    string  `json:"output"`
	Operator  string  `json:"operator"`
	Value     float64 `json:"value"`
	Actual    any     `json:"actual,omitempty"`
	Violation float64 `json:"violation"`
	Message   string  `json:"message,omitempty"`
}

type candidateValues struct {
	Inputs     map[string]any
	Parameters map[string]map[string]any
}

func WriteRecord(loaded *project.LoadedProject, result *Result) (RecordSummary, error) {
	if result == nil {
		return RecordSummary{}, apperror.Errorf(apperror.CodeRuntime, "optimization result is required")
	}
	provenance, err := artifactmeta.Build(loaded, []artifactmeta.Reference{
		{Role: "optimization_setup", Path: result.Setup},
		{Role: "base_parameter_set", Path: result.BaseParameterSet},
		{Role: "saved_scenario", Path: result.SavedScenario},
		{Role: "saved_parameter_set", Path: result.SavedParameterSet},
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
		summary.BaseParameterSet = record.Result.BaseParameterSet
		summary.SavedScenario = record.Result.SavedScenario
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
		switch variable.Kind {
		case "public_input", "component_parameter", "system_parameter":
		default:
			return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization variable kind: %s", variable.Kind)
		}
	}
	for _, constraint := range setup.Constraints {
		if strings.TrimSpace(constraint.Output) == "" {
			return Setup{}, apperror.Errorf(apperror.CodeInput, "optimization constraint output is required")
		}
		if normalizedConstraintOperator(constraint.Operator) == "" {
			return Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization constraint operator: %s", constraint.Operator)
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
	if setup.BaseParameterSet != "" {
		if _, err := parameterset.ApplyFile(loaded, setup.BaseParameterSet); err != nil {
			return nil, err
		}
	}
	if err := validateDecisionVariables(loaded, setup.DecisionVariables); err != nil {
		return nil, err
	}
	candidates := candidateValuesForAlgorithm(setup)
	if len(candidates) == 0 {
		return nil, apperror.Errorf(apperror.CodeInput, "optimization %s produced no candidates", setup.Algorithm)
	}

	var best CandidateSummary
	hasBest := false
	summaries := []CandidateSummary{}
	for index, candidate := range candidates {
		candidateLoaded, err := cloneLoadedProject(loaded)
		if err != nil {
			return nil, err
		}
		if err := applyCandidateParameters(candidateLoaded, candidate.Parameters); err != nil {
			return nil, err
		}
		summary := CandidateSummary{
			Index:      index,
			Inputs:     cloneMap(candidate.Inputs),
			Parameters: cloneNestedMap(candidate.Parameters),
			Feasible:   true,
		}
		runResult, err := runtimecore.Run(ctx, candidateLoaded, runtimecore.RunInput{
			Inputs:  candidate.Inputs,
			Context: cloneMap(setup.Context),
		})
		if err != nil {
			summary.Feasible = false
			summary.ConstraintPenalty = failedCandidatePenalty
			summary.Error = err.Error()
		} else {
			summary.Outputs = runResult.Outputs
			objective, err := objectiveValue(setup.Objective, runResult.Outputs)
			if err != nil {
				summary.Feasible = false
				summary.ConstraintPenalty = failedCandidatePenalty
				summary.Error = err.Error()
			} else {
				summary.Objective = objective
				violations, penalty := constraintViolations(setup.Constraints, runResult.Outputs)
				summary.ConstraintViolations = violations
				summary.ConstraintPenalty = penalty
				summary.Feasible = len(violations) == 0
			}
		}
		summaries = append(summaries, summary)
		if !hasBest || betterCandidate(setup.Objective.Sense, summary, best) {
			best = summary
			hasBest = true
		}
	}
	if !hasBest {
		return nil, apperror.Errorf(apperror.CodeRuntime, "optimization produced no evaluated candidates")
	}

	result := &Result{
		OK:               true,
		SetupID:          setup.ID,
		SetupName:        setup.Name,
		Setup:            setup.Path,
		Algorithm:        setup.Algorithm,
		BaseParameterSet: filepath.ToSlash(setup.BaseParameterSet),
		Objective:        setup.Objective,
		BestObjective:    best.Objective,
		BestInputs:       best.Inputs,
		BestParameters:   best.Parameters,
		BestOutputs:      best.Outputs,
		Candidates:       summaries,
	}
	if !best.Feasible || best.Error != "" {
		result.OK = false
	}
	if options.SaveScenario != "" {
		if err := writeScenario(loaded.Root, options.SaveScenario, setup, best); err != nil {
			return nil, err
		}
		result.SavedScenario = filepath.ToSlash(options.SaveScenario)
	}
	if options.SaveParameterSet != "" && len(best.Parameters) > 0 {
		if err := writeParameterSet(loaded.Root, options.SaveParameterSet, setup, best); err != nil {
			return nil, err
		}
		result.SavedParameterSet = filepath.ToSlash(options.SaveParameterSet)
	}
	return result, nil
}

func gridCandidates(setup Setup) []candidateValues {
	candidates := []candidateValues{{Inputs: cloneMap(setup.BaseInputs), Parameters: map[string]map[string]any{}}}
	for _, variable := range setup.DecisionVariables {
		values := gridValues(variable)
		next := []candidateValues{}
		for _, candidate := range candidates {
			for _, value := range values {
				cloned := candidateValues{
					Inputs:     cloneMap(candidate.Inputs),
					Parameters: cloneNestedMap(candidate.Parameters),
				}
				if variable.Kind == "component_parameter" || variable.Kind == "system_parameter" {
					componentID, name := componentParameterRef(variable)
					if cloned.Parameters[componentID] == nil {
						cloned.Parameters[componentID] = map[string]any{}
					}
					cloned.Parameters[componentID][name] = value
				} else {
					cloned.Inputs[variable.Name] = value
				}
				next = append(next, cloned)
			}
		}
		candidates = next
	}
	return candidates
}

func isSupportedAlgorithm(algorithm string) bool {
	switch algorithm {
	case "grid", "differential_evolution", "custom_sdk_script":
		return true
	default:
		return false
	}
}

func candidateValuesForAlgorithm(setup Setup) []candidateValues {
	switch setup.Algorithm {
	case "differential_evolution", "custom_sdk_script":
		return gridCandidates(setup)
	default:
		return gridCandidates(setup)
	}
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

func validateDecisionVariables(loaded *project.LoadedProject, variables []DecisionVariable) error {
	for _, variable := range variables {
		switch variable.Kind {
		case "public_input":
			if strings.TrimSpace(variable.Name) == "" {
				return apperror.Errorf(apperror.CodeInput, "public input decision variable name is required")
			}
		case "component_parameter", "system_parameter":
			componentID, name := componentParameterRef(variable)
			if componentID == "" || name == "" {
				return apperror.Errorf(apperror.CodeInput, "component parameter decision variable requires component and name")
			}
			component, ok := findComponent(loaded.Graph, componentID)
			if !ok {
				return apperror.Errorf(apperror.CodeValidation, "optimization parameter component is not in graph: %s", componentID)
			}
			if _, exists := component.Parameters[name]; !exists {
				return apperror.Errorf(apperror.CodeValidation, "optimization parameter is not in graph: %s.%s", componentID, name)
			}
			if _, ok := numberValue(component.Parameters[name]); !ok {
				return apperror.Errorf(apperror.CodeValidation, "optimization parameter must be numeric: %s.%s", componentID, name)
			}
		}
	}
	return nil
}

func componentParameterRef(variable DecisionVariable) (string, string) {
	componentID := strings.TrimSpace(variable.Component)
	name := strings.TrimSpace(variable.Name)
	if componentID == "" && strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		componentID = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
	}
	return componentID, name
}

func findComponent(graph *model.Graph, componentID string) (model.Component, bool) {
	if graph == nil {
		return model.Component{}, false
	}
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
}

func cloneLoadedProject(loaded *project.LoadedProject) (*project.LoadedProject, error) {
	graphBytes, err := json.Marshal(loaded.Graph)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeRuntime, err)
	}
	var graph model.Graph
	if err := json.Unmarshal(graphBytes, &graph); err != nil {
		return nil, apperror.Wrap(apperror.CodeRuntime, err)
	}
	projectCopy := *loaded.Project
	return &project.LoadedProject{
		Project:   &projectCopy,
		Graph:     &graph,
		Root:      loaded.Root,
		Path:      loaded.Path,
		GraphPath: loaded.GraphPath,
	}, nil
}

func applyCandidateParameters(loaded *project.LoadedProject, parameters map[string]map[string]any) error {
	if len(parameters) == 0 {
		return nil
	}
	return parameterset.Apply(loaded.Graph, parameterset.Set{Components: parameters})
}

func constraintViolations(constraints []Constraint, outputs map[string]any) ([]ConstraintViolation, float64) {
	violations := []ConstraintViolation{}
	totalPenalty := 0.0
	for _, constraint := range constraints {
		operator := normalizedConstraintOperator(constraint.Operator)
		actual, ok := outputs[constraint.Output]
		if !ok {
			violation := ConstraintViolation{
				Output:   constraint.Output,
				Operator: operator,
				Value:    constraint.Value,
				Message:  "constraint output is missing",
			}
			violations = append(violations, violation)
			totalPenalty += constraintPenalty(constraint, 1)
			continue
		}
		number, ok := numberValue(actual)
		if !ok {
			violation := ConstraintViolation{
				Output:   constraint.Output,
				Operator: operator,
				Value:    constraint.Value,
				Actual:   actual,
				Message:  "constraint output must be numeric",
			}
			violations = append(violations, violation)
			totalPenalty += constraintPenalty(constraint, 1)
			continue
		}
		if violation := constraintViolation(constraint, operator, number); violation > 0 {
			violations = append(violations, ConstraintViolation{
				Output:    constraint.Output,
				Operator:  operator,
				Value:     constraint.Value,
				Actual:    number,
				Violation: violation,
			})
			totalPenalty += constraintPenalty(constraint, violation)
		}
	}
	return violations, totalPenalty
}

func normalizedConstraintOperator(operator string) string {
	switch strings.TrimSpace(strings.ToLower(operator)) {
	case "<=", "lte", "le", "max":
		return "<="
	case ">=", "gte", "ge", "min":
		return ">="
	case "=", "==", "eq":
		return "=="
	default:
		return ""
	}
}

func constraintViolation(constraint Constraint, operator string, actual float64) float64 {
	tolerance := math.Max(0, constraint.Tolerance)
	switch operator {
	case "<=":
		return math.Max(0, actual-constraint.Value-tolerance)
	case ">=":
		return math.Max(0, constraint.Value-actual-tolerance)
	case "==":
		return math.Max(0, math.Abs(actual-constraint.Value)-tolerance)
	default:
		return 0
	}
}

func constraintPenalty(constraint Constraint, violation float64) float64 {
	weight := constraint.Penalty
	if weight <= 0 {
		weight = 1
	}
	return math.Max(0, violation) * weight
}

func betterCandidate(sense string, candidate CandidateSummary, incumbent CandidateSummary) bool {
	if candidate.Feasible != incumbent.Feasible {
		return candidate.Feasible
	}
	if candidate.Error != "" && incumbent.Error == "" {
		return false
	}
	if candidate.Error == "" && incumbent.Error != "" {
		return true
	}
	if !candidate.Feasible {
		if candidate.ConstraintPenalty != incumbent.ConstraintPenalty {
			return candidate.ConstraintPenalty < incumbent.ConstraintPenalty
		}
	}
	return better(sense, candidate.Objective, incumbent.Objective)
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

func writeParameterSet(projectRoot string, relativePath string, setup Setup, best CandidateSummary) error {
	set := parameterset.Set{
		ID:           strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath)),
		Name:         setup.Name + " Optimized Parameters",
		CreatedAtUTC: time.Now().UTC().Format(time.RFC3339Nano),
		Components:   cloneNestedMap(best.Parameters),
	}
	return parameterset.Write(projectRoot, relativePath, set)
}

func cloneMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneNestedMap(values map[string]map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for componentID, params := range values {
		out[componentID] = cloneMap(params)
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
