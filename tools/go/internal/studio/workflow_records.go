package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type RunSummary struct {
	ID           string         `json:"id"`
	RelativePath string         `json:"relative_path"`
	CreatedAtUTC string         `json:"created_at_utc"`
	ParameterSet string         `json:"parameter_set,omitempty"`
	Outputs      map[string]any `json:"outputs"`
}

type RunRecord struct {
	ID           string                 `json:"id"`
	ProjectName  string                 `json:"project_name"`
	CreatedAtUTC string                 `json:"created_at_utc"`
	ParameterSet string                 `json:"parameter_set,omitempty"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result"`
}

type BatchSummary struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
	ParameterSet string `json:"parameter_set,omitempty"`
	CaseCount    int    `json:"case_count"`
	OKCount      int    `json:"ok_count"`
}

type BatchCaseRecord struct {
	ScenarioID   string                 `json:"scenario_id"`
	ScenarioName string                 `json:"scenario_name"`
	OK           bool                   `json:"ok"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Problems     []Problem              `json:"problems,omitempty"`
}

type BatchRecord struct {
	ID           string            `json:"id"`
	ProjectName  string            `json:"project_name"`
	CreatedAtUTC string            `json:"created_at_utc"`
	ParameterSet string            `json:"parameter_set,omitempty"`
	Cases        []BatchCaseRecord `json:"cases"`
}

type ScenarioSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type ScenarioRecord struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	ProjectName  string         `json:"project_name"`
	CreatedAtUTC string         `json:"created_at_utc"`
	Inputs       map[string]any `json:"inputs"`
	Context      map[string]any `json:"context"`
}

func writeRunRecord(loaded *project.LoadedProject, input runtimecore.RunInput, result *runtimecore.RunResult, parameterSet string) (RunSummary, error) {
	now := time.Now().UTC()
	runID := "run-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return RunSummary{}, err
	}
	runPath := filepath.Join(runsRoot, runID+".json")
	record := RunRecord{
		ID:           runID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		ParameterSet: parameterSet,
		Inputs:       input.Inputs,
		Context:      input.Context,
		Result:       result,
	}
	if err := writeJSONFile(runPath, record); err != nil {
		return RunSummary{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, runPath)
	return RunSummary{
		ID:           runID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		ParameterSet: record.ParameterSet,
		Outputs:      result.Outputs,
	}, nil
}

func loadRunSummaries(projectRoot string) []RunSummary {
	runFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "run-*.json"))
	if err != nil {
		return []RunSummary{}
	}
	summaries := []RunSummary{}
	for _, runPath := range runFiles {
		runBytes, err := os.ReadFile(runPath)
		if err != nil {
			continue
		}
		var record RunRecord
		if err := json.Unmarshal(runBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, runPath)
		outputs := map[string]any{}
		if record.Result != nil {
			outputs = record.Result.Outputs
		}
		summaries = append(summaries, RunSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			ParameterSet: record.ParameterSet,
			Outputs:      outputs,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadRunRecord(projectRoot string, runID string) (RunRecord, error) {
	if runID == "" {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id is required")
	}
	if filepath.Base(runID) != runID || strings.ContainsAny(runID, `/\`) {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id must be a run record id")
	}
	runPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", runID+".json"))
	if err != nil {
		return RunRecord{}, err
	}
	runBytes, err := os.ReadFile(runPath)
	if err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record RunRecord
	if err := json.Unmarshal(runBytes, &record); err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func writeBatchRecord(loaded *project.LoadedProject, cases []BatchCaseRecord, parameterSet string) (BatchSummary, BatchRecord, error) {
	now := time.Now().UTC()
	batchID := "batch-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	batchPath := filepath.Join(runsRoot, batchID+".json")
	record := BatchRecord{
		ID:           batchID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		ParameterSet: parameterSet,
		Cases:        cases,
	}
	if err := writeJSONFile(batchPath, record); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, batchPath)
	return BatchSummary{
		ID:           batchID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		ParameterSet: record.ParameterSet,
		CaseCount:    len(cases),
		OKCount:      batchOKCount(cases),
	}, record, nil
}

func loadBatchSummaries(projectRoot string) []BatchSummary {
	batchFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "batch-*.json"))
	if err != nil {
		return []BatchSummary{}
	}
	summaries := []BatchSummary{}
	for _, batchPath := range batchFiles {
		batchBytes, err := os.ReadFile(batchPath)
		if err != nil {
			continue
		}
		var record BatchRecord
		if err := json.Unmarshal(batchBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, batchPath)
		summaries = append(summaries, BatchSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			ParameterSet: record.ParameterSet,
			CaseCount:    len(record.Cases),
			OKCount:      batchOKCount(record.Cases),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadBatchRecord(projectRoot string, batchID string) (BatchRecord, error) {
	if batchID == "" {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id is required")
	}
	if filepath.Base(batchID) != batchID || strings.ContainsAny(batchID, `/\`) {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id must be a batch record id")
	}
	batchPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", batchID+".json"))
	if err != nil {
		return BatchRecord{}, err
	}
	batchBytes, err := os.ReadFile(batchPath)
	if err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record BatchRecord
	if err := json.Unmarshal(batchBytes, &record); err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func batchOKCount(cases []BatchCaseRecord) int {
	count := 0
	for _, item := range cases {
		if item.OK {
			count++
		}
	}
	return count
}

func writeScenarioRecord(loaded *project.LoadedProject, req createScenarioRequest) (ScenarioSummary, ScenarioRecord, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name is required")
	}
	scenarioID := slugify(name)
	if scenarioID == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name must contain letters or numbers")
	}
	if req.Inputs == nil {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario inputs are required")
	}
	context := req.Context
	if context == nil {
		context = map[string]any{}
	}
	now := time.Now().UTC()
	scenariosRoot := filepath.Join(loaded.Root, "scenarios")
	if err := os.MkdirAll(scenariosRoot, 0o755); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	scenarioPath := filepath.Join(scenariosRoot, scenarioID+".json")
	if _, err := os.Stat(scenarioPath); err == nil {
		scenarioID = scenarioID + "-" + now.Format("20060102-150405")
		scenarioPath = filepath.Join(scenariosRoot, scenarioID+".json")
	}
	record := ScenarioRecord{
		ID:           scenarioID,
		Name:         name,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		Inputs:       req.Inputs,
		Context:      context,
	}
	if err := writeJSONFile(scenarioPath, record); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, scenarioPath)
	return ScenarioSummary{
		ID:           record.ID,
		Name:         record.Name,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
	}, record, nil
}

func loadScenarioSummaries(projectRoot string) []ScenarioSummary {
	scenarioFiles, err := filepath.Glob(filepath.Join(projectRoot, "scenarios", "*.json"))
	if err != nil {
		return []ScenarioSummary{}
	}
	summaries := []ScenarioSummary{}
	for _, scenarioPath := range scenarioFiles {
		scenarioBytes, err := os.ReadFile(scenarioPath)
		if err != nil {
			continue
		}
		var record ScenarioRecord
		if err := json.Unmarshal(scenarioBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, scenarioPath)
		summaries = append(summaries, ScenarioSummary{
			ID:           record.ID,
			Name:         record.Name,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadScenarioRecord(projectRoot string, scenarioID string) (ScenarioRecord, error) {
	if scenarioID == "" {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id is required")
	}
	if filepath.Base(scenarioID) != scenarioID || strings.ContainsAny(scenarioID, `/\`) {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id must be a scenario id")
	}
	scenarioPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("scenarios", scenarioID+".json"))
	if err != nil {
		return ScenarioRecord{}, err
	}
	scenarioBytes, err := os.ReadFile(scenarioPath)
	if err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record ScenarioRecord
	if err := json.Unmarshal(scenarioBytes, &record); err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}
