package studio

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func exportArtifactPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("project", path))
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
