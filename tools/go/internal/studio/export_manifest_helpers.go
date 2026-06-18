package studio

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
