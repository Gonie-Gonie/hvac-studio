package studio

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type ValidationMappingSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	RelativePath       string `json:"relative_path"`
	Dataset            string `json:"dataset"`
	DatasetChecksum    string `json:"dataset_checksum,omitempty"`
	InputCount         int    `json:"input_count"`
	OutputCount        int    `json:"output_count"`
	MissingValuePolicy string `json:"missing_value_policy,omitempty"`
}

type CalibrationSetupSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RelativePath   string `json:"relative_path"`
	Algorithm      string `json:"algorithm"`
	Mapping        string `json:"mapping"`
	ParameterCount int    `json:"parameter_count"`
}

type OptimizationSetupSummary struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	RelativePath     string                 `json:"relative_path"`
	Algorithm        string                 `json:"algorithm"`
	BaseParameterSet string                 `json:"base_parameter_set,omitempty"`
	Objective        optimization.Objective `json:"objective"`
	VariableCount    int                    `json:"variable_count"`
}

func createValidationMapping(loaded *project.LoadedProject, req createValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	if strings.TrimSpace(req.DatasetPath) == "" {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "dataset_path is required")
	}
	preview, err := datasetPreview(loaded, req.DatasetPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	inputColumns := nonEmptyColumns(req.InputColumns)
	if len(inputColumns) == 0 {
		inputColumns = suggestionsToColumns(preview.SuggestedInputs)
	}
	outputColumns := nonEmptyColumns(req.ObservedOutputColumns)
	if len(outputColumns) == 0 {
		outputColumns = suggestionsToColumns(preview.SuggestedOutputs)
	}
	unitHints := datasetUnitHints(req.UnitHints, preview.Columns)
	if len(inputColumns) == 0 {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping requires at least one input column")
	}
	if len(outputColumns) == 0 {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping requires at least one observed output column")
	}

	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueValidationMappingID(loaded.Root, strings.ReplaceAll(slugify(preview.Summary.ID+"_mapping"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	policy := strings.TrimSpace(req.MissingValuePolicy)
	policy, err = modelvalidation.NormalizeMissingValuePolicy(policy)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}

	mapping := modelvalidation.Mapping{
		ID:                    id,
		Name:                  name,
		Dataset:               preview.Summary.RelativePath,
		DatasetChecksum:       preview.Summary.SHA256,
		TimeColumn:            firstMatchingColumn(preview.Columns, req.TimeColumn, "time", "timestamp"),
		InputColumns:          inputColumns,
		ObservedOutputColumns: outputColumns,
		UnitHints:             unitHints,
		MissingValuePolicy:    policy,
	}
	mappingPath := filepath.Join(loaded.Root, "validation", "mappings", id+".json")
	if _, err := os.Stat(mappingPath); err == nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(mappingPath), 0o755); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, mappingPath)
	summary := ValidationMappingSummary{
		ID:                 mapping.ID,
		Name:               mapping.Name,
		RelativePath:       filepath.ToSlash(rel),
		Dataset:            mapping.Dataset,
		DatasetChecksum:    mapping.DatasetChecksum,
		InputCount:         len(mapping.InputColumns),
		OutputCount:        len(mapping.ObservedOutputColumns),
		MissingValuePolicy: mapping.MissingValuePolicy,
	}
	mapping.Path = summary.RelativePath
	return summary, mapping, nil
}

func updateValidationMapping(loaded *project.LoadedProject, req updateValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	mappingPath, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping name is required")
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, relativePath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	mapping.Name = name
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	mapping.Path = relativePath
	return validationMappingSummaryFromMapping(mapping), mapping, nil
}

func copyValidationMapping(loaded *project.LoadedProject, req copyValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	_, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, relativePath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(mapping.Name)
		if name == "" {
			name = displayNameFromID(mapping.ID)
		}
		name += " Copy"
	}
	id := uniqueValidationMappingID(loaded.Root, strings.ReplaceAll(slugify(name), "-", "_"))
	mapping.ID = id
	mapping.Name = name
	mapping.Path = ""
	mappingPath := filepath.Join(loaded.Root, "validation", "mappings", id+".json")
	if _, err := os.Stat(mappingPath); err == nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(mappingPath), 0o755); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, mappingPath)
	mapping.Path = filepath.ToSlash(rel)
	return validationMappingSummaryFromMapping(mapping), mapping, nil
}

func deleteValidationMapping(loaded *project.LoadedProject, req deleteValidationMappingRequest) (string, error) {
	mappingPath, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return "", err
	}
	if reference := calibrationSetupReferencingValidationMapping(loaded.Root, relativePath); reference != "" {
		return "", apperror.Errorf(apperror.CodeValidation, "validation mapping is used by calibration setup: %s", reference)
	}
	if err := os.Remove(mappingPath); err != nil {
		return "", apperror.Wrap(apperror.CodeRuntime, err)
	}
	return relativePath, nil
}

func resolveValidationMappingFile(projectRoot string, mappingPath string) (string, string, error) {
	absPath, err := resolveProjectOwnedFile(projectRoot, mappingPath)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "validation/mappings/") || strings.ToLower(filepath.Ext(rel)) != ".json" {
		return "", "", apperror.Errorf(apperror.CodeValidation, "validation mapping path must be under validation/mappings: %s", mappingPath)
	}
	return absPath, rel, nil
}

func calibrationSetupReferencingValidationMapping(projectRoot string, mappingPath string) string {
	target := filepath.ToSlash(mappingPath)
	for _, summary := range loadCalibrationSetupSummaries(projectRoot) {
		if filepath.ToSlash(summary.Mapping) == target {
			return summary.RelativePath
		}
	}
	return ""
}

func validationMappingSummaryFromMapping(mapping modelvalidation.Mapping) ValidationMappingSummary {
	name := strings.TrimSpace(mapping.Name)
	if name == "" {
		name = displayNameFromID(mapping.ID)
	}
	policy, err := modelvalidation.NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
	if err != nil {
		policy = modelvalidation.MissingPolicyError
	}
	return ValidationMappingSummary{
		ID:                 mapping.ID,
		Name:               name,
		RelativePath:       filepath.ToSlash(mapping.Path),
		Dataset:            filepath.ToSlash(mapping.Dataset),
		DatasetChecksum:    mapping.DatasetChecksum,
		InputCount:         len(mapping.InputColumns),
		OutputCount:        len(mapping.ObservedOutputColumns),
		MissingValuePolicy: policy,
	}
}

func createCalibrationSetup(loaded *project.LoadedProject, req createCalibrationSetupRequest) (CalibrationSetupSummary, calibration.Setup, error) {
	mappingPath := strings.TrimSpace(req.MappingPath)
	if mappingPath == "" {
		mappings := loadValidationMappingSummaries(loaded.Root)
		if len(mappings) == 0 {
			return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup requires a validation mapping")
		}
		mappingPath = mappings[0].RelativePath
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, mappingPath)
	if err != nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, err
	}
	if strings.TrimSpace(req.BaseParameterSet) != "" {
		if _, err := resolveProjectOwnedFile(loaded.Root, req.BaseParameterSet); err != nil {
			return CalibrationSetupSummary{}, calibration.Setup{}, err
		}
	}
	objectiveOutputs := req.ObjectiveOutputs
	if len(objectiveOutputs) == 0 {
		objectiveOutputs = map[string]float64{}
		for outputID := range mapping.ObservedOutputColumns {
			objectiveOutputs[outputID] = 1.0
		}
	}
	parameters := req.Parameters
	if len(parameters) == 0 {
		parameters = defaultCalibrationParameters(loaded.Graph)
	}
	if len(parameters) == 0 {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup requires at least one calibration_target parameter with numeric bounds")
	}
	algorithm := strings.TrimSpace(req.Algorithm)
	if algorithm == "" {
		algorithm = "grid"
	}
	if algorithm != "grid" && algorithm != "least_squares" && algorithm != "differential_evolution" {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported calibration algorithm: %s", algorithm)
	}
	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueCalibrationSetupID(loaded.Root, strings.ReplaceAll(slugify(mapping.ID+"_calibration"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	setup := calibration.Setup{
		ID:               id,
		Name:             name,
		Algorithm:        algorithm,
		Mapping:          filepath.ToSlash(mapping.Path),
		BaseParameterSet: filepath.ToSlash(strings.TrimSpace(req.BaseParameterSet)),
		Objective: calibration.Objective{
			Metric:  "rmse",
			Outputs: objectiveOutputs,
		},
		Parameters:    parameters,
		StoppingRules: req.StoppingRules,
	}
	setupPath := filepath.Join(loaded.Root, "calibration", "setups", id+".json")
	if _, err := os.Stat(setupPath); err == nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(setupPath, setup); err != nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, setupPath)
	setup.Path = filepath.ToSlash(rel)
	summary := CalibrationSetupSummary{
		ID:             setup.ID,
		Name:           setup.Name,
		RelativePath:   setup.Path,
		Algorithm:      setup.Algorithm,
		Mapping:        filepath.ToSlash(setup.Mapping),
		ParameterCount: len(setup.Parameters),
	}
	return summary, setup, nil
}

func createOptimizationSetup(loaded *project.LoadedProject, req createOptimizationSetupRequest) (OptimizationSetupSummary, optimization.Setup, error) {
	baseInputs := req.BaseInputs
	if baseInputs == nil {
		baseInputs = map[string]any{}
	}
	contextValues := req.Context
	if contextValues == nil {
		contextValues = map[string]any{"time": 0.0, "dt": 60.0}
	}
	objective := req.Objective
	if strings.TrimSpace(objective.Output) == "" {
		outputID := defaultOptimizationObjectiveOutput(loaded.Graph, loaded.Project.EntrySystem)
		if outputID == "" {
			return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup requires a numeric public output objective")
		}
		objective = optimization.Objective{Output: outputID, Sense: "min"}
	}
	if objective.Sense == "" {
		objective.Sense = "min"
	}
	algorithm := strings.TrimSpace(req.Algorithm)
	if algorithm == "" {
		algorithm = "grid"
	}
	if algorithm != "grid" && algorithm != "differential_evolution" && algorithm != "custom_sdk_script" {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization algorithm: %s", algorithm)
	}
	baseParameterSet := strings.TrimSpace(req.BaseParameterSet)
	if baseParameterSet != "" {
		if _, err := resolveProjectOwnedFile(loaded.Root, baseParameterSet); err != nil {
			return OptimizationSetupSummary{}, optimization.Setup{}, err
		}
	}
	variables := req.DecisionVariables
	if len(variables) == 0 {
		variable, ok := defaultOptimizationDecisionVariable(loaded.Graph, loaded.Project.EntrySystem, baseInputs)
		if !ok {
			return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup requires a numeric public input or optimization_variable parameter")
		}
		variables = []optimization.DecisionVariable{variable}
	}
	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueOptimizationSetupID(loaded.Root, strings.ReplaceAll(slugify(objective.Output+"_optimization"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	setup := optimization.Setup{
		ID:                id,
		Name:              name,
		Algorithm:         algorithm,
		BaseInputs:        baseInputs,
		Context:           contextValues,
		BaseParameterSet:  filepath.ToSlash(baseParameterSet),
		Objective:         objective,
		DecisionVariables: variables,
		Constraints:       req.Constraints,
	}
	setupPath := filepath.Join(loaded.Root, "optimization", "setups", id+".json")
	if _, err := os.Stat(setupPath); err == nil {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(setupPath, setup); err != nil {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, setupPath)
	setup.Path = filepath.ToSlash(rel)
	summary := OptimizationSetupSummary{
		ID:               setup.ID,
		Name:             setup.Name,
		RelativePath:     setup.Path,
		Algorithm:        setup.Algorithm,
		BaseParameterSet: filepath.ToSlash(setup.BaseParameterSet),
		Objective:        setup.Objective,
		VariableCount:    len(setup.DecisionVariables),
	}
	return summary, setup, nil
}

func optimizationSetupHasParameterVariables(setup optimization.Setup) bool {
	for _, variable := range setup.DecisionVariables {
		if variable.Kind == "component_parameter" || variable.Kind == "system_parameter" {
			return true
		}
	}
	return false
}

func defaultCalibrationParameters(graph *model.Graph) []calibration.ParameterSpec {
	if graph == nil {
		return nil
	}
	parameters := []calibration.ParameterSpec{}
	for _, component := range graph.Components {
		names := sortedMapKeys(component.ParameterDefinitions)
		for _, name := range names {
			definition := component.ParameterDefinitions[name]
			if definition.Role != "calibration_target" || definition.Bounds == nil {
				continue
			}
			if _, ok := studioNumberValue(component.Parameters[name]); !ok {
				continue
			}
			minValue, minOK := studioNumberValue(definition.Bounds.Min)
			maxValue, maxOK := studioNumberValue(definition.Bounds.Max)
			if !minOK || !maxOK || maxValue < minValue {
				continue
			}
			parameters = append(parameters, calibration.ParameterSpec{
				Component: component.ID,
				Name:      name,
				Min:       minValue,
				Max:       maxValue,
				Step:      defaultGridStep(minValue, maxValue),
			})
		}
	}
	return parameters
}

func defaultOptimizationObjectiveOutput(graph *model.Graph, systemID string) string {
	system, ok := findSystem(graph, systemID)
	if !ok {
		return ""
	}
	for _, output := range system.PublicOutputs {
		if isNumericValueType(output.ValueType) {
			return output.ID
		}
	}
	return ""
}

func defaultOptimizationDecisionVariable(graph *model.Graph, systemID string, baseInputs map[string]any) (optimization.DecisionVariable, bool) {
	system, ok := findSystem(graph, systemID)
	if !ok {
		return optimization.DecisionVariable{}, false
	}
	candidates := []model.PublicNodeRef{}
	for _, input := range system.PublicInputs {
		if !isNumericValueType(input.ValueType) {
			continue
		}
		if _, ok := studioNumberValue(baseInputs[input.ID]); !ok {
			continue
		}
		candidates = append(candidates, input)
	}
	if len(candidates) == 0 {
		return defaultOptimizationParameterDecisionVariable(graph)
	}
	chosen := candidates[0]
	for _, candidate := range candidates {
		id := strings.ToLower(candidate.ID)
		if strings.Contains(id, "setpoint") || strings.Contains(id, "speed") || strings.Contains(id, "fraction") {
			chosen = candidate
			break
		}
	}
	value, _ := studioNumberValue(baseInputs[chosen.ID])
	minValue, maxValue := defaultDecisionBounds(value)
	return optimization.DecisionVariable{
		Kind: "public_input",
		Name: chosen.ID,
		Min:  minValue,
		Max:  maxValue,
		Step: defaultGridStep(minValue, maxValue),
	}, true
}

func defaultOptimizationParameterDecisionVariable(graph *model.Graph) (optimization.DecisionVariable, bool) {
	if graph == nil {
		return optimization.DecisionVariable{}, false
	}
	for _, component := range graph.Components {
		names := sortedMapKeys(component.ParameterDefinitions)
		for _, name := range names {
			definition := component.ParameterDefinitions[name]
			if definition.Role != "optimization_variable" {
				continue
			}
			value, ok := studioNumberValue(component.Parameters[name])
			if !ok {
				continue
			}
			minValue, maxValue := defaultDecisionBounds(value)
			if definition.Bounds != nil {
				if minBound, ok := studioNumberValue(definition.Bounds.Min); ok {
					minValue = minBound
				}
				if maxBound, ok := studioNumberValue(definition.Bounds.Max); ok {
					maxValue = maxBound
				}
			}
			return optimization.DecisionVariable{
				Kind:      "component_parameter",
				Component: component.ID,
				Name:      name,
				Min:       minValue,
				Max:       maxValue,
				Step:      defaultGridStep(minValue, maxValue),
			}, true
		}
	}
	return optimization.DecisionVariable{}, false
}

func defaultDecisionBounds(value float64) (float64, float64) {
	if value == 0 {
		return 0, 1
	}
	delta := math.Abs(value) * 0.2
	if delta == 0 {
		delta = 1
	}
	return value - delta, value + delta
}

func defaultGridStep(minValue float64, maxValue float64) float64 {
	step := (maxValue - minValue) / 4.0
	if step <= 0 {
		return 1
	}
	return math.Round(step*1e9) / 1e9
}

func isNumericValueType(valueType string) bool {
	switch strings.ToLower(strings.TrimSpace(valueType)) {
	case "float", "int", "integer", "number":
		return true
	default:
		return false
	}
}

func nonEmptyColumns(values map[string]string) map[string]string {
	result := map[string]string{}
	for id, column := range values {
		id = strings.TrimSpace(id)
		column = strings.TrimSpace(column)
		if id != "" && column != "" {
			result[id] = column
		}
	}
	return result
}

func datasetUnitHints(values map[string]string, columns []string) map[string]string {
	allowed := map[string]bool{}
	for _, column := range columns {
		allowed[column] = true
	}
	result := map[string]string{}
	for column, unit := range values {
		column = strings.TrimSpace(column)
		unit = strings.TrimSpace(unit)
		if column != "" && unit != "" && allowed[column] {
			result[column] = unit
		}
	}
	return result
}

func suggestionsToColumns(suggestions []ColumnSuggestion) map[string]string {
	values := map[string]string{}
	for _, suggestion := range suggestions {
		if suggestion.PublicID != "" && suggestion.Column != "" {
			values[suggestion.PublicID] = suggestion.Column
		}
	}
	return values
}

func firstMatchingColumn(columns []string, preferred string, fallbacks ...string) string {
	candidates := append([]string{preferred}, fallbacks...)
	for _, candidate := range candidates {
		normalized := normalizeColumnName(candidate)
		if normalized == "" {
			continue
		}
		for _, column := range columns {
			if normalizeColumnName(column) == normalized {
				return column
			}
		}
	}
	return ""
}

func uniqueValidationMappingID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "mapping"
	}
	exists := map[string]bool{}
	for _, summary := range loadValidationMappingSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueCalibrationSetupID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "calibration_setup"
	}
	exists := map[string]bool{}
	for _, summary := range loadCalibrationSetupSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueOptimizationSetupID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "optimization_setup"
	}
	exists := map[string]bool{}
	for _, summary := range loadOptimizationSetupSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func loadValidationMappingSummaries(projectRoot string) []ValidationMappingSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "validation", "mappings"), []string{"*.json"})
	summaries := []ValidationMappingSummary{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var mapping modelvalidation.Mapping
		if err := json.Unmarshal(data, &mapping); err != nil {
			continue
		}
		id := mapping.ID
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		name := mapping.Name
		if name == "" {
			name = displayNameFromID(id)
		}
		policy, err := modelvalidation.NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
		if err != nil {
			policy = modelvalidation.MissingPolicyError
		}
		rel, _ := filepath.Rel(projectRoot, path)
		summaries = append(summaries, ValidationMappingSummary{
			ID:                 id,
			Name:               name,
			RelativePath:       filepath.ToSlash(rel),
			Dataset:            filepath.ToSlash(mapping.Dataset),
			DatasetChecksum:    mapping.DatasetChecksum,
			InputCount:         len(mapping.InputColumns),
			OutputCount:        len(mapping.ObservedOutputColumns),
			MissingValuePolicy: policy,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadCalibrationSetupSummaries(projectRoot string) []CalibrationSetupSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "calibration", "setups"), []string{"*.json"})
	summaries := []CalibrationSetupSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		setup, err := calibration.LoadSetup(projectRoot, rel)
		if err != nil {
			continue
		}
		name := setup.Name
		if name == "" {
			name = displayNameFromID(setup.ID)
		}
		summaries = append(summaries, CalibrationSetupSummary{
			ID:             setup.ID,
			Name:           name,
			RelativePath:   filepath.ToSlash(rel),
			Algorithm:      setup.Algorithm,
			Mapping:        filepath.ToSlash(setup.Mapping),
			ParameterCount: len(setup.Parameters),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadOptimizationSetupSummaries(projectRoot string) []OptimizationSetupSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "optimization", "setups"), []string{"*.json"})
	summaries := []OptimizationSetupSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		setup, err := optimization.LoadSetup(projectRoot, rel)
		if err != nil {
			continue
		}
		name := setup.Name
		if name == "" {
			name = displayNameFromID(setup.ID)
		}
		summaries = append(summaries, OptimizationSetupSummary{
			ID:               setup.ID,
			Name:             name,
			RelativePath:     filepath.ToSlash(rel),
			Algorithm:        setup.Algorithm,
			BaseParameterSet: filepath.ToSlash(setup.BaseParameterSet),
			Objective:        setup.Objective,
			VariableCount:    len(setup.DecisionVariables),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}
