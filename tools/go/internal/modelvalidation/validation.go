package modelvalidation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type Mapping struct {
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	Dataset               string            `json:"dataset"`
	TimeColumn            string            `json:"time_column,omitempty"`
	InputColumns          map[string]string `json:"input_columns"`
	ObservedOutputColumns map[string]string `json:"observed_output_columns"`
}

type Options struct {
	HighErrorRows int
}

type Result struct {
	OK                    bool                     `json:"ok"`
	MappingID             string                   `json:"mapping_id"`
	MappingName           string                   `json:"mapping_name,omitempty"`
	Dataset               string                   `json:"dataset"`
	RowCount              int                      `json:"row_count"`
	InputColumns          map[string]string        `json:"input_columns"`
	ObservedOutputColumns map[string]string        `json:"observed_output_columns"`
	Metrics               map[string]MetricSummary `json:"metrics"`
	Rows                  []RowSummary             `json:"rows"`
}

type RowSummary struct {
	RowIndex  int                `json:"row_index"`
	Time      any                `json:"time,omitempty"`
	Inputs    map[string]any     `json:"inputs"`
	Observed  map[string]float64 `json:"observed"`
	Simulated map[string]float64 `json:"simulated"`
	Errors    map[string]float64 `json:"errors"`
	OK        bool               `json:"ok"`
	Error     string             `json:"error,omitempty"`
}

type MetricSummary struct {
	Count         int            `json:"count"`
	RMSE          float64        `json:"rmse"`
	MAE           float64        `json:"mae"`
	MBE           float64        `json:"mbe"`
	CVRMSE        float64        `json:"cvrmse"`
	R2            float64        `json:"r2"`
	HighErrorRows []HighErrorRow `json:"high_error_rows"`
}

type HighErrorRow struct {
	RowIndex   int        `json:"row_index"`
	Time       any        `json:"time,omitempty"`
	Observed   float64    `json:"observed"`
	Simulated  float64    `json:"simulated"`
	Error      float64    `json:"error"`
	AbsError   float64    `json:"abs_error"`
	Inspection Inspection `json:"inspection"`
}

type Inspection struct {
	ComponentInputs  map[string]map[string]any          `json:"component_inputs"`
	ComponentOutputs map[string]map[string]any          `json:"component_outputs"`
	NodeValues       []runtimecore.NodeValueTrace       `json:"node_values"`
	ConnectionValues []runtimecore.ConnectionValueTrace `json:"connection_values"`
	States           map[string]map[string]any          `json:"states"`
}

func LoadMapping(projectRoot string, mappingPath string) (Mapping, error) {
	if strings.TrimSpace(mappingPath) == "" {
		return Mapping{}, apperror.Errorf(apperror.CodeValidation, "--mapping is required")
	}
	resolved, err := resolveProjectOwnedFile(projectRoot, mappingPath)
	if err != nil {
		return Mapping{}, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Mapping{}, apperror.Wrap(apperror.CodeInput, err)
	}
	var mapping Mapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return Mapping{}, apperror.Wrap(apperror.CodeInput, err)
	}
	if mapping.ID == "" {
		mapping.ID = strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
	}
	if mapping.Dataset == "" {
		return Mapping{}, apperror.Errorf(apperror.CodeInput, "validation mapping dataset is required")
	}
	if len(mapping.InputColumns) == 0 {
		return Mapping{}, apperror.Errorf(apperror.CodeInput, "validation mapping input_columns is required")
	}
	if len(mapping.ObservedOutputColumns) == 0 {
		return Mapping{}, apperror.Errorf(apperror.CodeInput, "validation mapping observed_output_columns is required")
	}
	return mapping, nil
}

func Run(ctx context.Context, loaded *project.LoadedProject, mapping Mapping, options Options) (*Result, error) {
	if options.HighErrorRows <= 0 {
		options.HighErrorRows = 3
	}
	datasetPath, err := resolveProjectOwnedFile(loaded.Root, mapping.Dataset)
	if err != nil {
		return nil, err
	}
	rows, err := readCSVRows(datasetPath)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apperror.Errorf(apperror.CodeInput, "validation dataset has no data rows: %s", mapping.Dataset)
	}

	session, err := runtimecore.NewSession(ctx, loaded)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	accumulators := map[string]*metricAccumulator{}
	for outputID := range mapping.ObservedOutputColumns {
		accumulators[outputID] = &metricAccumulator{}
	}

	result := &Result{
		OK:                    true,
		MappingID:             mapping.ID,
		MappingName:           mapping.Name,
		Dataset:               filepath.ToSlash(mapping.Dataset),
		RowCount:              len(rows),
		InputColumns:          mapping.InputColumns,
		ObservedOutputColumns: mapping.ObservedOutputColumns,
		Metrics:               map[string]MetricSummary{},
		Rows:                  []RowSummary{},
	}

	for rowIndex, row := range rows {
		inputs, err := rowInputs(row, mapping.InputColumns)
		if err != nil {
			return nil, err
		}
		observed, err := rowObserved(row, mapping.ObservedOutputColumns)
		if err != nil {
			return nil, err
		}
		contextValues := map[string]any{
			"row_index": rowIndex,
			"dataset":   filepath.ToSlash(mapping.Dataset),
		}
		timeValue := any(nil)
		if mapping.TimeColumn != "" {
			timeValue, err = rowValue(row, mapping.TimeColumn)
			if err != nil {
				return nil, err
			}
			contextValues["time"] = timeValue
		}

		runResult, err := session.Evaluate(runtimecore.RunInput{Inputs: inputs, Context: contextValues})
		rowSummary := RowSummary{
			RowIndex:  rowIndex,
			Time:      timeValue,
			Inputs:    inputs,
			Observed:  observed,
			Simulated: map[string]float64{},
			Errors:    map[string]float64{},
			OK:        err == nil,
		}
		if err != nil {
			rowSummary.Error = err.Error()
			result.OK = false
			result.Rows = append(result.Rows, rowSummary)
			continue
		}

		inspection := Inspection{
			ComponentInputs:  runResult.ComponentInputs,
			ComponentOutputs: runResult.ComponentOutputs,
			NodeValues:       runResult.NodeValues,
			ConnectionValues: runResult.ConnectionValues,
			States:           runResult.States,
		}
		for outputID, observedValue := range observed {
			simulatedValue, err := outputValue(runResult.Outputs, outputID)
			if err != nil {
				return nil, err
			}
			errorValue := simulatedValue - observedValue
			rowSummary.Simulated[outputID] = simulatedValue
			rowSummary.Errors[outputID] = errorValue
			accumulators[outputID].add(observedValue, simulatedValue, HighErrorRow{
				RowIndex:   rowIndex,
				Time:       timeValue,
				Observed:   observedValue,
				Simulated:  simulatedValue,
				Error:      errorValue,
				AbsError:   math.Abs(errorValue),
				Inspection: inspection,
			})
		}
		result.Rows = append(result.Rows, rowSummary)
	}

	for outputID, accumulator := range accumulators {
		result.Metrics[outputID] = accumulator.summary(options.HighErrorRows)
	}
	return result, nil
}

func readCSVRows(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInput, err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInput, err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	headers := records[0]
	rows := []map[string]string{}
	for rowIndex, record := range records[1:] {
		row := map[string]string{}
		for index, header := range headers {
			if strings.TrimSpace(header) == "" {
				continue
			}
			if index >= len(record) {
				return nil, apperror.Errorf(apperror.CodeInput, "dataset row %d is missing column %s", rowIndex, header)
			}
			row[header] = record[index]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func rowInputs(row map[string]string, mapping map[string]string) (map[string]any, error) {
	inputs := map[string]any{}
	for publicInput, column := range mapping {
		value, err := rowValue(row, column)
		if err != nil {
			return nil, err
		}
		inputs[publicInput] = value
	}
	return inputs, nil
}

func rowObserved(row map[string]string, mapping map[string]string) (map[string]float64, error) {
	observed := map[string]float64{}
	for publicOutput, column := range mapping {
		value, err := rowFloat(row, column)
		if err != nil {
			return nil, err
		}
		observed[publicOutput] = value
	}
	return observed, nil
}

func rowValue(row map[string]string, column string) (any, error) {
	raw, ok := row[column]
	if !ok {
		return nil, apperror.Errorf(apperror.CodeInput, "dataset column is missing: %s", column)
	}
	if value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		return value, nil
	}
	return raw, nil
}

func rowFloat(row map[string]string, column string) (float64, error) {
	value, err := rowValue(row, column)
	if err != nil {
		return 0, err
	}
	number, ok := value.(float64)
	if !ok {
		return 0, apperror.Errorf(apperror.CodeInput, "dataset column %s must be numeric", column)
	}
	return number, nil
}

func outputValue(outputs map[string]any, outputID string) (float64, error) {
	value, ok := outputs[outputID]
	if !ok {
		return 0, apperror.Errorf(apperror.CodeRuntime, "validation output is missing from run result: %s", outputID)
	}
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return 0, apperror.Wrap(apperror.CodeRuntime, err)
		}
		return number, nil
	default:
		return 0, apperror.Errorf(apperror.CodeRuntime, "validation output %s must be numeric", outputID)
	}
}

type metricAccumulator struct {
	observed  []float64
	simulated []float64
	errors    []float64
	highRows  []HighErrorRow
}

func (a *metricAccumulator) add(observed float64, simulated float64, row HighErrorRow) {
	a.observed = append(a.observed, observed)
	a.simulated = append(a.simulated, simulated)
	a.errors = append(a.errors, simulated-observed)
	a.highRows = append(a.highRows, row)
}

func (a *metricAccumulator) summary(highErrorRows int) MetricSummary {
	count := len(a.errors)
	if count == 0 {
		return MetricSummary{}
	}
	var sumObserved, sumSquaredError, sumAbsError, sumError float64
	for index, observed := range a.observed {
		err := a.errors[index]
		sumObserved += observed
		sumSquaredError += err * err
		sumAbsError += math.Abs(err)
		sumError += err
	}
	meanObserved := sumObserved / float64(count)
	rmse := math.Sqrt(sumSquaredError / float64(count))
	cvrmse := 0.0
	if meanObserved != 0 {
		cvrmse = rmse / math.Abs(meanObserved) * 100.0
	}

	var ssTotal, ssResidual float64
	for index, observed := range a.observed {
		diff := observed - meanObserved
		ssTotal += diff * diff
		err := a.errors[index]
		ssResidual += err * err
	}
	r2 := 0.0
	if ssTotal != 0 {
		r2 = 1.0 - ssResidual/ssTotal
	}

	rows := append([]HighErrorRow(nil), a.highRows...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].AbsError > rows[j].AbsError
	})
	if highErrorRows < len(rows) {
		rows = rows[:highErrorRows]
	}

	return MetricSummary{
		Count:         count,
		RMSE:          rmse,
		MAE:           sumAbsError / float64(count),
		MBE:           sumError / float64(count),
		CVRMSE:        cvrmse,
		R2:            r2,
		HighErrorRows: rows,
	}
}

func resolveProjectOwnedFile(projectRoot string, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", apperror.Errorf(apperror.CodeInput, "project-relative path is required")
	}
	if filepath.IsAbs(relativePath) {
		return "", apperror.Errorf(apperror.CodeInput, "project-relative path must not be absolute: %s", relativePath)
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
		return "", apperror.Errorf(apperror.CodeInput, "project-relative path escapes project root: %s", relativePath)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", apperror.Wrap(apperror.CodeInput, fmt.Errorf("project artifact not found: %s", relativePath))
	}
	return resolved, nil
}
