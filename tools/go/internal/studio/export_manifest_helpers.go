package studio

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

func exportArtifactPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("project", path))
}

func writeExportManifest(loaded *project.LoadedProject, profile string, options exportOptions) (ExportSummary, ExportManifest, error) {
	if profile == "" {
		profile = "runtime_package"
	}
	if profile != "runtime_package" && profile != "research_project" {
		return ExportSummary{}, ExportManifest{}, apperror.Errorf(apperror.CodeValidation, "unsupported export profile: %s", profile)
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	now := time.Now().UTC()
	projectPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.Path)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	graphPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.GraphPath)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	defaultInputPath := ""
	if loaded.Project.DefaultInput != "" {
		defaultInputPath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.DefaultInput)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	environmentLockfilePath := ""
	if loaded.Project.Environment.Lockfile != "" {
		environmentLockfilePath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.Environment.Lockfile)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	exportRoot := filepath.Join(loaded.Root, "exports", profile)
	if err := resetGeneratedDir(filepath.Join(loaded.Root, "exports"), exportRoot); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files, err := writeRuntimeExportProject(loaded, filepath.Join(exportRoot, "project"), options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	supportFiles, err := writeRuntimeExportSupportFiles(loaded.Root, exportRoot, options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, supportFiles...)
	interfaceSchemaPath := "schema/public-io.json"
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := schemaexport.Write(filepath.Join(exportRoot, filepath.FromSlash(interfaceSchemaPath)), schema); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, interfaceSchemaPath)
	entrypoints := runtimeExportEntrypoints(files, plan, exportArtifactPath(projectPath), exportArtifactPath(defaultInputPath), exportArtifactPath(environmentLockfilePath), options)
	entrypointFiles, err := writeRuntimeExportEntrypoints(exportRoot, entrypoints)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, entrypointFiles...)
	sort.Strings(files)
	checksums, err := exportFileChecksums(exportRoot, files)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	commands := []string{}
	for _, entrypoint := range entrypoints {
		if strings.HasSuffix(entrypoint.Rel, ".ps1") {
			commands = append(commands, entrypoint.Rel)
		}
	}
	manifest := ExportManifest{
		Profile:             profile,
		CreatedAtUTC:        now.Format(time.RFC3339Nano),
		ProjectName:         loaded.Project.ProjectName,
		ProjectRoot:         "project",
		ProjectPath:         exportArtifactPath(projectPath),
		GraphPath:           exportArtifactPath(graphPath),
		DefaultInput:        exportArtifactPath(defaultInputPath),
		EnvironmentLockfile: exportArtifactPath(environmentLockfilePath),
		InterfaceSchema:     interfaceSchemaPath,
		Runner:              "bin/bcs-runner.exe",
		RuntimePython:       "runtime/python/python.exe",
		Files:               files,
		Components:          append([]string{}, plan.System.Components...),
		ModelAssets:         modelAssetExportPaths(loaded.Graph, options.IncludeMLAssets),
		MLValidationReports: mlValidationSummaries(loaded, options.IncludeMLAssets, true),
		Checksums:           checksums,
		PublicInputs:        append([]model.PublicNodeRef{}, plan.System.PublicInputs...),
		PublicOutputs:       append([]model.PublicNodeRef{}, plan.System.PublicOutputs...),
		ExecutionOrder:      append([]string{}, plan.Order...),
		ParameterSets:       exportFilesWithPrefix(files, "project/parameter_sets/"),
		Datasets:            exportFilesWithPrefix(files, "project/datasets/"),
		ValidationMappings:  exportFilesWithPrefix(files, "project/validation/mappings/"),
		CalibrationSetups:   exportFilesWithPrefix(files, "project/calibration/setups/"),
		OptimizationSetups:  exportFilesWithPrefix(files, "project/optimization/setups/"),
		RunRecords:          exportFilesWithPrefix(files, "project/runs/"),
		BatchRecords:        exportFilesWithPrefix(files, "project/batches/"),
		ValidationRecords:   exportFilesWithPrefix(files, "project/validation/runs/"),
		CalibrationRecords:  exportFilesWithPrefix(files, "project/calibration/results/"),
		OptimizationRecords: exportFilesWithPrefix(files, "project/optimization/results/"),
		Commands:            commands,
		IncludeDatasets:     options.IncludeDatasets,
		IncludeCalibration:  options.IncludeCalibrationSetups,
		IncludeOptimization: options.IncludeOptimizationSetups,
		IncludeMLAssets:     options.IncludeMLAssets,
		IncludeSDKExamples:  options.IncludeSDKExamples,
		IncludeRecords:      options.IncludeRecords,
	}
	exportPath := filepath.Join(exportRoot, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	if err := writeJSONFile(exportPath, manifest); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, exportPath)
	return ExportSummary{
		Profile:      profile,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: manifest.CreatedAtUTC,
	}, manifest, nil
}

func modelAssetExportPaths(graph *model.Graph, includeMLAssets bool) []string {
	if graph == nil || !includeMLAssets {
		return nil
	}
	paths := []string{}
	seen := map[string]bool{}
	for _, component := range graph.Components {
		if component.MLMetadata == nil {
			continue
		}
		for _, asset := range component.MLMetadata.AssetPaths() {
			assetPath := strings.TrimSpace(asset.Path)
			if assetPath == "" {
				continue
			}
			exportPath := exportArtifactPath(assetPath)
			if seen[exportPath] {
				continue
			}
			seen[exportPath] = true
			paths = append(paths, exportPath)
		}
	}
	sort.Strings(paths)
	return paths
}

func mlValidationSummaries(loaded *project.LoadedProject, includeMLAssets bool, exportPaths bool) []MLValidationSummary {
	if loaded == nil || loaded.Graph == nil || !includeMLAssets {
		return nil
	}
	summaries := []MLValidationSummary{}
	for _, component := range loaded.Graph.Components {
		if component.MLMetadata == nil || strings.TrimSpace(component.MLMetadata.ValidationReportFile) == "" {
			continue
		}
		reportRel := strings.TrimSpace(component.MLMetadata.ValidationReportFile)
		reportPath, err := resolveProjectOwnedFile(loaded.Root, reportRel)
		if err != nil {
			continue
		}
		reportBytes, err := os.ReadFile(reportPath)
		if err != nil {
			continue
		}
		var report struct {
			Dataset              string                    `json:"dataset"`
			Metrics              map[string]map[string]any `json:"metrics"`
			FeatureSchemaVersion string                    `json:"feature_schema_version"`
			ModelAssetChecksum   string                    `json:"model_asset_checksum"`
			TrainingPeriod       string                    `json:"training_period"`
			ValidationPeriod     string                    `json:"validation_period"`
			TimeResolution       string                    `json:"time_resolution"`
		}
		if err := json.Unmarshal(reportBytes, &report); err != nil {
			continue
		}
		modelChecksum := strings.TrimSpace(report.ModelAssetChecksum)
		if modelChecksum == "" && strings.TrimSpace(component.MLMetadata.ModelFile) != "" {
			if modelPath, err := resolveProjectOwnedFile(loaded.Root, component.MLMetadata.ModelFile); err == nil {
				modelChecksum, _ = datasetChecksum(modelPath)
			}
		}
		path := filepath.ToSlash(reportRel)
		if exportPaths {
			path = exportArtifactPath(path)
		}
		summaries = append(summaries, MLValidationSummary{
			ComponentID:          component.ID,
			ReportPath:           path,
			Dataset:              report.Dataset,
			Metrics:              report.Metrics,
			FeatureSchemaVersion: report.FeatureSchemaVersion,
			ModelAssetChecksum:   modelChecksum,
			TrainingPeriod:       report.TrainingPeriod,
			ValidationPeriod:     report.ValidationPeriod,
			TimeResolution:       report.TimeResolution,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ComponentID < summaries[j].ComponentID
	})
	return summaries
}

func mlValidationSummaryMap(values []MLValidationSummary) map[string]MLValidationSummary {
	if len(values) == 0 {
		return nil
	}
	out := map[string]MLValidationSummary{}
	for _, value := range values {
		out[value.ComponentID] = value
	}
	return out
}

func exportFileChecksums(exportRoot string, files []string) (map[string]string, error) {
	checksums := map[string]string{}
	for _, rel := range files {
		path := filepath.Join(exportRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		checksums[rel] = fmt.Sprintf("%x", sum)
	}
	return checksums, nil
}

func exportFilesWithPrefix(files []string, prefix string) []string {
	matches := []string{}
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			matches = append(matches, file)
		}
	}
	sort.Strings(matches)
	return matches
}

func firstProjectRelativeExport(files []string, prefix string) string {
	matches := exportFilesWithPrefix(files, prefix)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimPrefix(matches[0], "project/")
}

func projectOwnedRelativePath(projectRoot string, path string) (string, string, error) {
	resolved, err := resolveProjectOwnedFile(projectRoot, path)
	if err != nil {
		return "", "", err
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	return rel, resolved, nil
}

func resetGeneratedDir(ownerRoot string, targetPath string) error {
	ownerRoot, err := filepath.Abs(ownerRoot)
	if err != nil {
		return err
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(ownerRoot, targetPath)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return apperror.Errorf(apperror.CodeValidation, "generated export path must stay inside export root: %s", targetPath)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}
	return os.MkdirAll(targetPath, 0o755)
}

func loadExportSummaries(projectRoot string) []ExportSummary {
	manifestFiles, err := filepath.Glob(filepath.Join(projectRoot, "exports", "*", "manifest.json"))
	if err != nil {
		return []ExportSummary{}
	}
	summaries := []ExportSummary{}
	for _, manifestPath := range manifestFiles {
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest ExportManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, manifestPath)
		profile := manifest.Profile
		if profile == "" {
			profile = filepath.Base(filepath.Dir(manifestPath))
		}
		summaries = append(summaries, ExportSummary{
			Profile:      profile,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: manifest.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadExportManifest(projectRoot string, profile string) (ExportSummary, ExportManifest, error) {
	if profile == "" {
		profile = "runtime_package"
	}
	if filepath.Base(profile) != profile || strings.ContainsAny(profile, `/\`) {
		return ExportSummary{}, ExportManifest{}, apperror.Errorf(apperror.CodeValidation, "profile must be an export profile id")
	}
	manifestPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("exports", profile, "manifest.json"))
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var manifest ExportManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if manifest.Profile == "" {
		manifest.Profile = profile
	}
	rel, _ := filepath.Rel(projectRoot, manifestPath)
	return ExportSummary{
		Profile:      manifest.Profile,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: manifest.CreatedAtUTC,
	}, manifest, nil
}
