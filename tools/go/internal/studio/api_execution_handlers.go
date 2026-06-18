package studio

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		writeErrorWithProblems(w, apperror.Wrap(apperror.CodeValidation, err), inferProblems(loaded, err))
		return
	}
	problems := compilerDiagnosticsProblems(plan.Diagnostics)
	sourceCheckCount, sourceProblems := checkProjectSources(r.Context(), loaded)
	if hasErrorProblems(sourceProblems) {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), sourceProblems)
		return
	}
	problems = append(problems, sourceProblems...)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"validation": map[string]any{
			"project_name":     loaded.Project.ProjectName,
			"entry_system":     loaded.Project.EntrySystem,
			"component_count":  len(plan.System.Components),
			"connection_count": len(plan.System.Connections),
			"execution_order":  plan.Order,
			"source_checks":    sourceCheckCount,
			"problems":         problems,
		},
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}

	input := runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context}
	if input.Inputs == nil {
		input, err = runtimecore.LoadInput(resolveProjectFile(loaded.Root, loaded.Project.DefaultInput))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeInput, err))
			return
		}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, 30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.Run(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "run", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "result": result}
	if req.Save {
		runRecord, err := writeRunRecord(loaded, input, result, filepath.ToSlash(req.ParameterSetPath))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
			return
		}
		response["run_record"] = runRecord
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRunSeries(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSeriesRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	input := runtimecore.SeriesInput{
		SchemaVersion: req.SchemaVersion,
		Context:       req.Context,
		Steps:         req.Steps,
	}
	if req.InputPath != "" {
		input, err = runtimecore.LoadSeriesInput(resolveProjectFile(loaded.Root, req.InputPath))
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if len(input.Steps) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeInput, "series input requires steps"))
		return
	}

	timeout, err := requestTimeout(apiRequest{TimeoutMS: req.TimeoutMS}, time.Duration(len(input.Steps))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.RunSeries(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "series", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	scenarios := loadScenarioSummaries(loaded.Root)
	if len(scenarios) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "batch requires at least one saved scenario"))
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, time.Duration(len(scenarios))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	cases := make([]BatchCaseRecord, 0, len(scenarios))
	for index := len(scenarios) - 1; index >= 0; index-- {
		scenario, err := loadScenarioRecord(loaded.Root, scenarios[index].ID)
		if err != nil {
			writeError(w, err)
			return
		}
		input := runtimecore.RunInput{Inputs: scenario.Inputs, Context: scenario.Context}
		if input.Context == nil {
			input.Context = map[string]any{}
		}
		caseRecord := BatchCaseRecord{
			ScenarioID:   scenario.ID,
			ScenarioName: scenario.Name,
			Inputs:       input.Inputs,
			Context:      input.Context,
		}
		result, err := runtimecore.Run(ctx, loaded, input)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeTimeoutError(w, "batch", timeout)
				return
			}
			caseRecord.Error = err.Error()
			caseRecord.Problems = inferProblems(loaded, err)
		} else {
			if req.ParameterSetPath != "" {
				result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
			}
			caseRecord.OK = true
			caseRecord.Result = result
		}
		cases = append(cases, caseRecord)
	}
	summary, record, err := writeBatchRecord(loaded, cases, filepath.ToSlash(req.ParameterSetPath))
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "batch": record})
}

func (s *Server) handleDataValidation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeValidationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}
	if req.MappingPath == "" {
		mappings := loadValidationMappingSummaries(loaded.Root)
		if len(mappings) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "data validation requires a saved mapping"))
			return
		}
		req.MappingPath = mappings[0].RelativePath
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, req.MappingPath)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: req.HighErrorRows})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "validation_result": result}
	if req.Save {
		summary, err := modelvalidation.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["validation_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCalibrationRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCalibrationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadCalibrationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "calibration requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := calibration.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveParameterSet == "" {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_calibrated.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := calibration.Run(ctx, loaded.Path, setup, calibration.Options{SaveParameterSet: req.SaveParameterSet})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "calibration_result": result}
	if req.Save {
		summary, err := calibration.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["calibration_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOptimizationRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeOptimizationRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadOptimizationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "optimization requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := optimization.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveScenario == "" {
		req.SaveScenario = filepath.ToSlash(filepath.Join("scenarios", setup.ID+"_optimized.json"))
	}
	if req.Save && req.SaveParameterSet == "" && optimizationSetupHasParameterVariables(setup) {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_optimized.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := optimization.Run(ctx, loaded.Path, setup, optimization.Options{
		SaveScenario:     req.SaveScenario,
		SaveParameterSet: req.SaveParameterSet,
	})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "optimization_result": result}
	if req.Save {
		summary, err := optimization.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["optimization_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schema": schema})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	req, err := decodeExportRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	summary, manifest, err := writeExportManifest(loaded, req.Profile, exportOptionsFromRequest(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "export": manifest})
}
