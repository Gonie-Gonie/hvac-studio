package modelvalidation

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/artifactmeta"
	"github.com/goniegonie/hvac-studio/tools/go/internal/jsonfile"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/projectpath"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type Mapping struct {
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	Dataset               string            `json:"dataset"`
	DatasetChecksum       string            `json:"dataset_checksum,omitempty"`
	Path                  string            `json:"-"`
	TimeColumn            string            `json:"time_column,omitempty"`
	InputColumns          map[string]string `json:"input_columns"`
	ObservedOutputColumns map[string]string `json:"observed_output_columns"`
	UnitHints             map[string]string `json:"unit_hints,omitempty"`
	MissingValuePolicy    string            `json:"missing_value_policy,omitempty"`
}

type Options struct {
	HighErrorRows int
}

const (
	MissingPolicyError            = "error"
	MissingPolicyDrop             = "drop"
	MissingPolicyFill             = "fill"
	MissingPolicyIgnoreOutputRows = "ignore_output_rows"
)

type Result struct {
	OK                    bool                     `json:"ok"`
	MappingID             string                   `json:"mapping_id"`
	MappingName           string                   `json:"mapping_name,omitempty"`
	Mapping               string                   `json:"mapping,omitempty"`
	ParameterSet          string                   `json:"parameter_set,omitempty"`
	SavedRecord           string                   `json:"saved_record,omitempty"`
	Dataset               string                   `json:"dataset"`
	DatasetChecksum       string                   `json:"dataset_checksum,omitempty"`
	RowCount              int                      `json:"row_count"`
	InputRowCount         int                      `json:"input_row_count,omitempty"`
	SkippedRowCount       int                      `json:"skipped_row_count,omitempty"`
	FilledValueCount      int                      `json:"filled_value_count,omitempty"`
	InputColumns          map[string]string        `json:"input_columns"`
	ObservedOutputColumns map[string]string        `json:"observed_output_columns"`
	MissingValuePolicy    string                   `json:"missing_value_policy,omitempty"`
	Warnings              []string                 `json:"warnings,omitempty"`
	Metrics               map[string]MetricSummary `json:"metrics"`
	Rows                  []RowSummary             `json:"rows"`
}

type RecordSummary struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
	MappingID    string `json:"mapping_id"`
	MappingName  string `json:"mapping_name,omitempty"`
	ParameterSet string `json:"parameter_set,omitempty"`
	Dataset      string `json:"dataset"`
	RowCount     int    `json:"row_count"`
	OK           bool   `json:"ok"`
}

type Record struct {
	ID           string                  `json:"id"`
	ProjectName  string                  `json:"project_name"`
	CreatedAtUTC string                  `json:"created_at_utc"`
	MappingID    string                  `json:"mapping_id"`
	MappingName  string                  `json:"mapping_name,omitempty"`
	ParameterSet string                  `json:"parameter_set,omitempty"`
	Dataset      string                  `json:"dataset"`
	Provenance   artifactmeta.Provenance `json:"provenance,omitempty"`
	Result       *Result                 `json:"result"`
}

type RowSummary struct {
	RowIndex  int                `json:"row_index"`
	Time      any                `json:"time,omitempty"`
	Inputs    map[string]any     `json:"inputs"`
	Observed  map[string]float64 `json:"observed"`
	Simulated map[string]float64 `json:"simulated"`
	Errors    map[string]float64 `json:"errors"`
	OK        bool               `json:"ok"`
	Skipped   bool               `json:"skipped,omitempty"`
	Error     string             `json:"error,omitempty"`
	Filled    []string           `json:"filled_columns,omitempty"`
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

func WriteRecord(loaded *project.LoadedProject, result *Result) (RecordSummary, error) {
	if result == nil {
		return RecordSummary{}, apperror.Errorf(apperror.CodeRuntime, "validation result is required")
	}
	provenance, err := artifactmeta.Build(loaded, []artifactmeta.Reference{
		{Role: "validation_mapping", Path: result.Mapping},
		{Role: "dataset", Path: result.Dataset},
		{Role: "parameter_set", Path: result.ParameterSet},
	})
	if err != nil {
		return RecordSummary{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	now := time.Now().UTC()
	recordID := "validation-" + now.Format("20060102-150405.000000000")
	recordPath := filepath.Join(loaded.Root, "validation", "runs", recordID+".json")
	record := Record{
		ID:           recordID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		MappingID:    result.MappingID,
		MappingName:  result.MappingName,
		ParameterSet: result.ParameterSet,
		Dataset:      result.Dataset,
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
		return Record{}, apperror.Errorf(apperror.CodeValidation, "validation_record_id is required")
	}
	if filepath.Base(recordID) != recordID || strings.ContainsAny(recordID, `/\`) {
		return Record{}, apperror.Errorf(apperror.CodeValidation, "validation_record_id must be a validation record id")
	}
	recordPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("validation", "runs", recordID+".json"))
	if err != nil {
		return Record{}, err
	}
	recordBytes, err := jsonfile.Read(recordPath)
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
	recordFiles, err := filepath.Glob(filepath.Join(projectRoot, "validation", "runs", "validation-*.json"))
	if err != nil {
		return []RecordSummary{}
	}
	summaries := []RecordSummary{}
	for _, recordPath := range recordFiles {
		recordBytes, err := jsonfile.Read(recordPath)
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
		MappingID:    record.MappingID,
		MappingName:  record.MappingName,
		ParameterSet: record.ParameterSet,
		Dataset:      record.Dataset,
	}
	if record.Result != nil {
		summary.RowCount = record.Result.RowCount
		summary.OK = record.Result.OK
		if summary.MappingID == "" {
			summary.MappingID = record.Result.MappingID
		}
		if summary.MappingName == "" {
			summary.MappingName = record.Result.MappingName
		}
		if summary.ParameterSet == "" {
			summary.ParameterSet = record.Result.ParameterSet
		}
		if summary.Dataset == "" {
			summary.Dataset = record.Result.Dataset
		}
	}
	return summary
}

func LoadMapping(projectRoot string, mappingPath string) (Mapping, error) {
	if strings.TrimSpace(mappingPath) == "" {
		return Mapping{}, apperror.Errorf(apperror.CodeValidation, "--mapping is required")
	}
	resolved, err := resolveProjectOwnedFile(projectRoot, mappingPath)
	if err != nil {
		return Mapping{}, err
	}
	data, err := jsonfile.Read(resolved)
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
	if rel, err := filepath.Rel(projectRoot, resolved); err == nil {
		mapping.Path = filepath.ToSlash(rel)
	}
	policy, err := NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
	if err != nil {
		return Mapping{}, err
	}
	mapping.MissingValuePolicy = policy
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
	policy, err := NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
	if err != nil {
		return nil, err
	}
	mapping.MissingValuePolicy = policy
	datasetPath, err := resolveProjectOwnedFile(loaded.Root, mapping.Dataset)
	if err != nil {
		return nil, err
	}
	datasetChecksum := strings.TrimSpace(mapping.DatasetChecksum)
	if datasetChecksum == "" {
		datasetChecksum, _ = fileChecksum(datasetPath)
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
		Mapping:               mapping.Path,
		Dataset:               filepath.ToSlash(mapping.Dataset),
		DatasetChecksum:       datasetChecksum,
		InputRowCount:         len(rows),
		InputColumns:          mapping.InputColumns,
		ObservedOutputColumns: mapping.ObservedOutputColumns,
		MissingValuePolicy:    mapping.MissingValuePolicy,
		Metrics:               map[string]MetricSummary{},
		Rows:                  []RowSummary{},
	}
	resolver := newMissingValueResolver(mapping.MissingValuePolicy)

	for rowIndex, row := range rows {
		prepared, skipReason, err := prepareValidationRow(row, mapping, resolver)
		if err != nil {
			return nil, apperror.Errorf(apperror.CodeInput, "dataset row %d: %s", rowIndex, err.Error())
		}
		if skipReason != "" {
			result.SkippedRowCount++
			result.Rows = append(result.Rows, RowSummary{
				RowIndex:  rowIndex,
				Time:      prepared.Time,
				Inputs:    prepared.Inputs,
				Observed:  prepared.Observed,
				Simulated: map[string]float64{},
				Errors:    map[string]float64{},
				OK:        false,
				Skipped:   true,
				Error:     skipReason,
				Filled:    prepared.FilledColumns,
			})
			continue
		}
		result.RowCount++
		result.FilledValueCount += len(prepared.FilledColumns)
		contextValues := map[string]any{
			"row_index": rowIndex,
			"dataset":   filepath.ToSlash(mapping.Dataset),
		}
		if mapping.TimeColumn != "" {
			contextValues["time"] = prepared.Time
		}

		runResult, err := session.Evaluate(runtimecore.RunInput{Inputs: prepared.Inputs, Context: contextValues})
		rowSummary := RowSummary{
			RowIndex:  rowIndex,
			Time:      prepared.Time,
			Inputs:    prepared.Inputs,
			Observed:  prepared.Observed,
			Simulated: map[string]float64{},
			Errors:    map[string]float64{},
			OK:        err == nil,
			Filled:    prepared.FilledColumns,
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
		for outputID, observedValue := range prepared.Observed {
			simulatedValue, err := outputValue(runResult.Outputs, outputID)
			if err != nil {
				return nil, err
			}
			errorValue := simulatedValue - observedValue
			rowSummary.Simulated[outputID] = simulatedValue
			rowSummary.Errors[outputID] = errorValue
			accumulators[outputID].add(observedValue, simulatedValue, HighErrorRow{
				RowIndex:   rowIndex,
				Time:       prepared.Time,
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
	if result.SkippedRowCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d dataset row(s) were skipped by missing value policy %q", result.SkippedRowCount, result.MissingValuePolicy))
	}
	if result.RowCount == 0 {
		result.OK = false
		result.Warnings = append(result.Warnings, "validation did not evaluate any dataset rows")
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
	for _, record := range records[1:] {
		row := map[string]string{}
		for index, header := range headers {
			if strings.TrimSpace(header) == "" {
				continue
			}
			if index >= len(record) {
				row[header] = ""
				continue
			}
			row[header] = record[index]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func NormalizeMissingValuePolicy(policy string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	if normalized == "" || normalized == "fail_fast" {
		return MissingPolicyError, nil
	}
	switch normalized {
	case MissingPolicyError, MissingPolicyDrop, MissingPolicyFill, MissingPolicyIgnoreOutputRows:
		return normalized, nil
	default:
		return "", apperror.Errorf(apperror.CodeValidation, "unsupported missing value policy: %s", policy)
	}
}

type preparedValidationRow struct {
	Inputs        map[string]any
	Observed      map[string]float64
	Time          any
	FilledColumns []string
}

func prepareValidationRow(row map[string]string, mapping Mapping, resolver *missingValueResolver) (preparedValidationRow, string, error) {
	prepared := preparedValidationRow{
		Inputs:   map[string]any{},
		Observed: map[string]float64{},
	}
	filled := map[string]bool{}
	if mapping.TimeColumn != "" {
		raw, wasFilled, skipReason, err := resolver.value(row, mapping.TimeColumn, false)
		if err != nil || skipReason != "" {
			return prepared, skipReason, err
		}
		if wasFilled {
			filled[mapping.TimeColumn] = true
		}
		value, err := parseRowValue(mapping.TimeColumn, raw)
		if err != nil {
			return prepared, "", err
		}
		prepared.Time = value
	}
	for publicInput, column := range mapping.InputColumns {
		raw, wasFilled, skipReason, err := resolver.value(row, column, false)
		if err != nil || skipReason != "" {
			return prepared, skipReason, err
		}
		if wasFilled {
			filled[column] = true
		}
		value, err := parseRowValue(column, raw)
		if err != nil {
			return prepared, "", err
		}
		prepared.Inputs[publicInput] = value
	}
	for publicOutput, column := range mapping.ObservedOutputColumns {
		raw, wasFilled, skipReason, err := resolver.value(row, column, true)
		if err != nil || skipReason != "" {
			return prepared, skipReason, err
		}
		if wasFilled {
			filled[column] = true
		}
		value, err := parseRowFloat(column, raw)
		if err != nil {
			return prepared, "", err
		}
		prepared.Observed[publicOutput] = value
	}
	prepared.FilledColumns = make([]string, 0, len(filled))
	for column := range filled {
		prepared.FilledColumns = append(prepared.FilledColumns, column)
	}
	sort.Strings(prepared.FilledColumns)
	return prepared, "", nil
}

type missingValueResolver struct {
	policy     string
	lastValues map[string]string
}

func newMissingValueResolver(policy string) *missingValueResolver {
	return &missingValueResolver{
		policy:     policy,
		lastValues: map[string]string{},
	}
}

func (r *missingValueResolver) value(row map[string]string, column string, observedOutput bool) (string, bool, string, error) {
	raw, ok := row[column]
	if ok && !isMissingValue(raw) {
		r.lastValues[column] = raw
		return raw, false, "", nil
	}
	switch r.policy {
	case MissingPolicyFill:
		fillValue := r.lastValues[column]
		if fillValue == "" {
			fillValue = "0"
		}
		r.lastValues[column] = fillValue
		return fillValue, true, "", nil
	case MissingPolicyDrop:
		return "", false, fmt.Sprintf("skipped because dataset column %s has a missing value", column), nil
	case MissingPolicyIgnoreOutputRows:
		if observedOutput {
			return "", false, fmt.Sprintf("skipped because observed output column %s has a missing value", column), nil
		}
	}
	return "", false, "", apperror.Errorf(apperror.CodeInput, "dataset column %s has a missing value", column)
}

func isMissingValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "na", "n/a", "nan", "null", "none":
		return true
	default:
		return false
	}
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
	if !ok || isMissingValue(raw) {
		return nil, apperror.Errorf(apperror.CodeInput, "dataset column %s has a missing value", column)
	}
	return parseRowValue(column, raw)
}

func parseRowValue(column string, raw string) (any, error) {
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
	return numericRowValue(column, value)
}

func parseRowFloat(column string, raw string) (float64, error) {
	value, err := parseRowValue(column, raw)
	if err != nil {
		return 0, err
	}
	return numericRowValue(column, value)
}

func numericRowValue(column string, value any) (float64, error) {
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

func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
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
	resolved, err := projectpath.ResolveInside(projectRoot, relativePath)
	if err != nil {
		return "", apperror.Errorf(apperror.CodeInput, "project-relative path escapes project root: %s", relativePath)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", apperror.Wrap(apperror.CodeInput, fmt.Errorf("project artifact not found: %s", relativePath))
	}
	return resolved, nil
}
