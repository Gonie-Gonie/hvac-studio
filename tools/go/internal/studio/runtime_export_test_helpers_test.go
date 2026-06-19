package studio

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func assertRuntimeExportCompiles(t *testing.T, exportRoot string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(exportRoot, "manifest.json")); err != nil {
		t.Fatalf("relocated manifest: %v", err)
	}
	loaded, err := project.Load(filepath.Join(exportRoot, "project", "project.bcsproj"))
	if err != nil {
		t.Fatalf("load relocated export: %v", err)
	}
	if _, err := compiler.Compile(loaded); err != nil {
		t.Fatalf("compile relocated export: %v", err)
	}
}

func assertRuntimeExportHasNoSourceCheckoutPaths(t *testing.T, exportRoot string, forbiddenPaths ...string) {
	t.Helper()
	forbidden := []string{}
	for _, path := range forbiddenPaths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		cleanPath := filepath.Clean(path)
		forbidden = append(forbidden, cleanPath, filepath.ToSlash(cleanPath))
	}

	textExtensions := map[string]bool{
		".bcsproj": true,
		".cfg":     true,
		".csv":     true,
		".json":    true,
		".jsonl":   true,
		".lock":    true,
		".md":      true,
		".ps1":     true,
		".py":      true,
		".txt":     true,
	}
	if err := filepath.WalkDir(exportRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !textExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		for _, forbiddenPath := range forbidden {
			if strings.Contains(text, forbiddenPath) {
				t.Fatalf("exported text file %s references source checkout path %s", path, forbiddenPath)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("scan exported text files for source checkout paths: %v", err)
	}
}

func assertRuntimeExportWorkflowsRun(t *testing.T, loaded *project.LoadedProject) {
	t.Helper()
	loaded.Project.Environment.Python = testPythonExecutable(t)
	if err := writeJSONFile(loaded.Path, loaded.Project); err != nil {
		t.Fatalf("write exported project python override: %v", err)
	}

	ctx := context.Background()
	mapping, err := modelvalidation.LoadMapping(loaded.Root, filepath.Join("validation", "mappings", "scalar_validation.json"))
	if err != nil {
		t.Fatalf("load exported validation mapping: %v", err)
	}
	validationResult, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: 1})
	if err != nil {
		t.Fatalf("run exported validation: %v", err)
	}
	if !validationResult.OK || validationResult.RowCount != 1 || validationResult.Metrics["result"].Count != 1 {
		t.Fatalf("exported validation result = %#v", validationResult)
	}

	calibrationSetup, err := calibration.LoadSetup(loaded.Root, filepath.Join("calibration", "setups", "scalar_gain.json"))
	if err != nil {
		t.Fatalf("load exported calibration setup: %v", err)
	}
	calibrationResult, err := calibration.Run(ctx, loaded.Path, calibrationSetup, calibration.Options{})
	if err != nil {
		t.Fatalf("run exported calibration: %v", err)
	}
	if !calibrationResult.OK || len(calibrationResult.Candidates) != 3 {
		t.Fatalf("exported calibration result = %#v", calibrationResult)
	}

	optimizationSetup, err := optimization.LoadSetup(loaded.Root, filepath.Join("optimization", "setups", "scalar_grid.json"))
	if err != nil {
		t.Fatalf("load exported optimization setup: %v", err)
	}
	optimizationResult, err := optimization.Run(ctx, loaded.Path, optimizationSetup, optimization.Options{})
	if err != nil {
		t.Fatalf("run exported optimization: %v", err)
	}
	if !optimizationResult.OK || len(optimizationResult.Candidates) != 3 || optimizationResult.BestOutputs["result"] == nil {
		t.Fatalf("exported optimization result = %#v", optimizationResult)
	}
}

func assertRuntimeExportCompositionWorkflowsRun(t *testing.T, loaded *project.LoadedProject) {
	t.Helper()
	loaded.Project.Environment.Python = testPythonExecutable(t)
	if err := writeJSONFile(loaded.Path, loaded.Project); err != nil {
		t.Fatalf("write exported composition project python override: %v", err)
	}

	ctx := context.Background()
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, filepath.FromSlash(loaded.Project.DefaultInput)))
	if err != nil {
		t.Fatalf("load exported composition input: %v", err)
	}
	runResult, err := runtimecore.Run(ctx, loaded, input)
	if err != nil {
		t.Fatalf("run exported composition project: %v", err)
	}
	if !runResult.OK || runResult.Outputs["total_power_kw"] == nil || runResult.Outputs["zone_temperature_c"] == nil {
		t.Fatalf("exported composition run result = %#v", runResult)
	}

	seriesInput, err := runtimecore.LoadSeriesInput(filepath.Join(loaded.Root, "inputs", "series01.json"))
	if err != nil {
		t.Fatalf("load exported composition series input: %v", err)
	}
	seriesResult, err := runtimecore.RunSeries(ctx, loaded, seriesInput)
	if err != nil {
		t.Fatalf("run exported composition series: %v", err)
	}
	zoneState := seriesResult.FinalStates["zone_rc"]
	if !seriesResult.OK || seriesResult.StepCount != 3 || zoneState["zone_temperature_c"] == nil {
		t.Fatalf("exported composition series result = %#v", seriesResult)
	}

	mapping, err := modelvalidation.LoadMapping(loaded.Root, filepath.Join("validation", "mappings", "rc_ahu_validation.json"))
	if err != nil {
		t.Fatalf("load exported composition validation mapping: %v", err)
	}
	validationResult, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: 1})
	if err != nil {
		t.Fatalf("run exported composition validation: %v", err)
	}
	if !validationResult.OK || validationResult.RowCount != 3 || validationResult.Metrics["total_power_kw"].Count != 3 {
		t.Fatalf("exported composition validation result = %#v", validationResult)
	}

	calibrationSetup, err := calibration.LoadSetup(loaded.Root, filepath.Join("calibration", "setups", "chiller_cop_grid.json"))
	if err != nil {
		t.Fatalf("load exported composition calibration setup: %v", err)
	}
	calibrationResult, err := calibration.Run(ctx, loaded.Path, calibrationSetup, calibration.Options{})
	if err != nil {
		t.Fatalf("run exported composition calibration: %v", err)
	}
	if !calibrationResult.OK || len(calibrationResult.Candidates) != 3 || calibrationResult.BestParameterSet.Components["chiller"]["cop"] == nil {
		t.Fatalf("exported composition calibration result = %#v", calibrationResult)
	}

	optimizationSetup, err := optimization.LoadSetup(loaded.Root, filepath.Join("optimization", "setups", "chw_pump_grid.json"))
	if err != nil {
		t.Fatalf("load exported composition optimization setup: %v", err)
	}
	optimizationResult, err := optimization.Run(ctx, loaded.Path, optimizationSetup, optimization.Options{})
	if err != nil {
		t.Fatalf("run exported composition optimization: %v", err)
	}
	if !optimizationResult.OK || len(optimizationResult.Candidates) != 9 || optimizationResult.BestInputs["pump_speed_fraction"] == nil {
		t.Fatalf("exported composition optimization result = %#v", optimizationResult)
	}
}

func zipDirectory(sourceRoot string, archivePath string) error {
	archive, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	writer := zip.NewWriter(archive)
	defer writer.Close()
	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		target, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(target, source)
		return err
	})
}

func unzipArchive(archivePath string, targetRoot string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	cleanRoot := filepath.Clean(targetRoot)
	cleanRootWithSeparator := cleanRoot + string(os.PathSeparator)
	for _, file := range reader.File {
		targetPath := filepath.Join(targetRoot, filepath.FromSlash(file.Name))
		cleanTarget := filepath.Clean(targetPath)
		if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRootWithSeparator) {
			return &os.PathError{Op: "unzip", Path: file.Name, Err: os.ErrInvalid}
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		target, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = source.Close()
			return err
		}
		_, copyErr := io.Copy(target, source)
		closeErr := target.Close()
		sourceErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if sourceErr != nil {
			return sourceErr
		}
	}
	return nil
}
