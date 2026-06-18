package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type DatasetSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	Format       string `json:"format"`
	RowCount     int    `json:"row_count"`
	ColumnCount  int    `json:"column_count"`
	SHA256       string `json:"sha256,omitempty"`
}

type DatasetPreview struct {
	Summary             DatasetSummary      `json:"summary"`
	Columns             []string            `json:"columns"`
	SuggestedTimeColumn string              `json:"suggested_time_column,omitempty"`
	ColumnProfiles      []ColumnProfile     `json:"column_profiles,omitempty"`
	PreviewRows         []map[string]string `json:"preview_rows"`
	SuggestedInputs     []ColumnSuggestion  `json:"suggested_inputs"`
	SuggestedOutputs    []ColumnSuggestion  `json:"suggested_outputs"`
}

type ColumnProfile struct {
	Column       string   `json:"column"`
	ValueType    string   `json:"value_type"`
	MissingCount int      `json:"missing_count"`
	Samples      []string `json:"samples,omitempty"`
}

type ColumnSuggestion struct {
	PublicID  string `json:"public_id"`
	Name      string `json:"name"`
	Column    string `json:"column,omitempty"`
	Unit      string `json:"unit,omitempty"`
	ValueType string `json:"value_type,omitempty"`
	Required  bool   `json:"required"`
}

func loadDatasetSummaries(projectRoot string) []DatasetSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "datasets"), []string{"*.csv", "*.json"})
	summaries := []DatasetSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		rowCount, columnCount := datasetShape(path)
		checksum, _ := datasetChecksum(path)
		summaries = append(summaries, DatasetSummary{
			ID:           id,
			Name:         displayNameFromID(id),
			RelativePath: filepath.ToSlash(rel),
			Format:       strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
			RowCount:     rowCount,
			ColumnCount:  columnCount,
			SHA256:       checksum,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func importDataset(loaded *project.LoadedProject, req importDatasetRequest) (DatasetPreview, error) {
	sourcePath := strings.TrimSpace(req.SourcePath)
	if sourcePath == "" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeValidation, "source_path is required")
	}
	if encoding := strings.TrimSpace(strings.ToLower(req.Encoding)); encoding != "" && encoding != "utf-8" && encoding != "utf8" && encoding != "utf-8-bom" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeValidation, "unsupported dataset encoding: %s", req.Encoding)
	}
	if !filepath.IsAbs(sourcePath) {
		abs, err := filepath.Abs(sourcePath)
		if err != nil {
			return DatasetPreview{}, apperror.Wrap(apperror.CodeValidation, err)
		}
		sourcePath = abs
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return DatasetPreview{}, apperror.Wrap(apperror.CodeInput, fmt.Errorf("dataset source not found: %s", sourcePath))
	}
	if info.IsDir() {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source must be a CSV file: %s", sourcePath)
	}
	if strings.ToLower(filepath.Ext(sourcePath)) != ".csv" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset import supports CSV files: %s", sourcePath)
	}

	delimiter, err := requestedDatasetDelimiter(req.Delimiter)
	if err != nil {
		return DatasetPreview{}, err
	}
	records, _, err := readCSVRecords(sourcePath, delimiter)
	if err != nil {
		return DatasetPreview{}, err
	}
	if len(records) == 0 {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source has no header row: %s", sourcePath)
	}
	if !hasNonEmptyHeader(records[0]) {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source header row is empty: %s", sourcePath)
	}

	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = strings.ReplaceAll(slugify(req.Name), "-", "_")
	}
	if id == "" {
		id = strings.ReplaceAll(slugify(strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))), "-", "_")
	}
	id = uniqueDatasetID(loaded.Root, id)
	targetRel := filepath.Join("datasets", id+".csv")
	targetPath := filepath.Join(loaded.Root, targetRel)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return DatasetPreview{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeNormalizedCSV(targetPath, records); err != nil {
		return DatasetPreview{}, err
	}
	preview, err := datasetPreview(loaded, targetRel)
	if err != nil {
		return DatasetPreview{}, err
	}
	if strings.TrimSpace(req.Name) != "" {
		preview.Summary.Name = strings.TrimSpace(req.Name)
	}
	return preview, nil
}

func datasetPreview(loaded *project.LoadedProject, relativePath string) (DatasetPreview, error) {
	resolved, err := resolveProjectOwnedFile(loaded.Root, relativePath)
	if err != nil {
		return DatasetPreview{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, resolved)
	rows, columns, err := readDatasetPreviewRows(resolved, 8)
	if err != nil {
		return DatasetPreview{}, err
	}
	rowCount, columnCount := datasetShape(resolved)
	checksum, _ := datasetChecksum(resolved)
	summary := DatasetSummary{
		ID:           strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved)),
		Name:         displayNameFromID(strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))),
		RelativePath: filepath.ToSlash(rel),
		Format:       strings.TrimPrefix(strings.ToLower(filepath.Ext(resolved)), "."),
		RowCount:     rowCount,
		ColumnCount:  columnCount,
		SHA256:       checksum,
	}
	system := entrySystem(loaded)
	return DatasetPreview{
		Summary:             summary,
		Columns:             columns,
		SuggestedTimeColumn: suggestedTimeColumn(columns),
		ColumnProfiles:      inferColumnProfiles(columns, rows),
		PreviewRows:         rows,
		SuggestedInputs:     columnSuggestions(system.PublicInputs, columns),
		SuggestedOutputs:    columnSuggestions(system.PublicOutputs, columns),
	}, nil
}

func readDatasetPreviewRows(path string, limit int) ([]map[string]string, []string, error) {
	if strings.ToLower(filepath.Ext(path)) != ".csv" {
		return []map[string]string{}, []string{}, nil
	}
	records, _, err := readCSVRecords(path, 0)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return []map[string]string{}, []string{}, nil
	}
	columns := append([]string(nil), records[0]...)
	rows := []map[string]string{}
	for _, record := range records[1:] {
		if len(rows) >= limit {
			break
		}
		row := map[string]string{}
		for index, column := range columns {
			if index < len(record) {
				row[column] = record[index]
			} else {
				row[column] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, columns, nil
}

func readCSVRecords(path string, delimiter rune) ([][]string, rune, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInput, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if delimiter == 0 {
		delimiter = detectCSVDelimiter(string(data))
	}
	if delimiter == 0 {
		delimiter = ','
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	reader.Comma = delimiter
	records, err := reader.ReadAll()
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInput, err)
	}
	return records, delimiter, nil
}

func requestedDatasetDelimiter(value string) (rune, error) {
	raw := strings.ToLower(value)
	if raw == "\t" {
		return '\t', nil
	}
	value = strings.TrimSpace(raw)
	switch value {
	case "", "auto":
		return 0, nil
	case ",", "comma":
		return ',', nil
	case ";", "semicolon", "semi-colon":
		return ';', nil
	case "\\t", "tab":
		return '\t', nil
	case "|", "pipe":
		return '|', nil
	}
	runes := []rune(value)
	if len(runes) == 1 && runes[0] != '"' && runes[0] != '\r' && runes[0] != '\n' {
		return runes[0], nil
	}
	return 0, apperror.Errorf(apperror.CodeValidation, "unsupported dataset delimiter: %s", value)
}

func detectCSVDelimiter(data string) rune {
	firstLine := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			firstLine = line
			break
		}
	}
	if firstLine == "" {
		return ','
	}
	type candidate struct {
		delimiter rune
		count     int
	}
	candidates := []candidate{
		{delimiter: ',', count: strings.Count(firstLine, ",")},
		{delimiter: ';', count: strings.Count(firstLine, ";")},
		{delimiter: '\t', count: strings.Count(firstLine, "\t")},
		{delimiter: '|', count: strings.Count(firstLine, "|")},
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})
	if candidates[0].count == 0 {
		return ','
	}
	return candidates[0].delimiter
}

func writeNormalizedCSV(path string, records [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.WriteAll(records); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	return nil
}

func hasNonEmptyHeader(columns []string) bool {
	for _, column := range columns {
		if strings.TrimSpace(column) != "" {
			return true
		}
	}
	return false
}

func inferColumnProfiles(columns []string, rows []map[string]string) []ColumnProfile {
	profiles := []ColumnProfile{}
	for _, column := range columns {
		profile := ColumnProfile{Column: column, ValueType: "number"}
		samples := []string{}
		seen := map[string]bool{}
		for _, row := range rows {
			raw := strings.TrimSpace(row[column])
			if raw == "" {
				profile.MissingCount++
				continue
			}
			if _, err := strconv.ParseFloat(raw, 64); err != nil {
				profile.ValueType = "string"
			}
			if !seen[raw] && len(samples) < 3 {
				seen[raw] = true
				samples = append(samples, raw)
			}
		}
		if len(rows) == 0 {
			profile.ValueType = "unknown"
		}
		profile.Samples = samples
		profiles = append(profiles, profile)
	}
	return profiles
}

func datasetChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}

func columnSuggestions(refs []model.PublicNodeRef, columns []string) []ColumnSuggestion {
	suggestions := []ColumnSuggestion{}
	for _, ref := range refs {
		suggestions = append(suggestions, ColumnSuggestion{
			PublicID:  ref.ID,
			Name:      ref.Name,
			Column:    matchColumn(ref, columns),
			Unit:      ref.Unit,
			ValueType: ref.ValueType,
			Required:  ref.IsRequired(),
		})
	}
	return suggestions
}

func suggestedTimeColumn(columns []string) string {
	for _, candidate := range []string{"time", "timestamp", "datetime", "date", "timestep", "step"} {
		normalizedCandidate := normalizeColumnName(candidate)
		for _, column := range columns {
			if normalizeColumnName(column) == normalizedCandidate {
				return column
			}
		}
	}
	for _, column := range columns {
		normalized := normalizeColumnName(column)
		if strings.Contains(normalized, "timestamp") || strings.Contains(normalized, "datetime") || strings.Contains(normalized, "time") {
			return column
		}
	}
	return ""
}

func matchColumn(ref model.PublicNodeRef, columns []string) string {
	targets := []string{ref.ID, ref.Name}
	for _, target := range targets {
		normalizedTarget := normalizeColumnName(target)
		if normalizedTarget == "" {
			continue
		}
		for _, column := range columns {
			if normalizeColumnName(column) == normalizedTarget {
				return column
			}
		}
	}
	for _, target := range targets {
		normalizedTarget := normalizeColumnName(target)
		if normalizedTarget == "" {
			continue
		}
		for _, column := range columns {
			normalizedColumn := normalizeColumnName(column)
			if strings.HasSuffix(normalizedColumn, normalizedTarget) || strings.Contains(normalizedColumn, normalizedTarget) {
				return column
			}
		}
	}
	return ""
}

func uniqueDatasetID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "dataset"
	}
	exists := map[string]bool{}
	for _, summary := range loadDatasetSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func datasetShape(path string) (int, int) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		records, _, err := readCSVRecords(path, 0)
		if err != nil || len(records) == 0 {
			return 0, 0
		}
		return len(records) - 1, len(records[0])
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, 0
		}
		var rows []map[string]any
		if err := json.Unmarshal(data, &rows); err == nil {
			if len(rows) == 0 {
				return 0, 0
			}
			return len(rows), len(rows[0])
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err == nil {
			return 1, len(object)
		}
	}
	return 0, 0
}
