package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/artifactschema"
)

type Report struct {
	OK             bool             `json:"ok"`
	ProjectPath    string           `json:"project_path"`
	CurrentVersion string           `json:"current_version"`
	WriteRequested bool             `json:"write_requested"`
	Artifacts      []ArtifactReport `json:"artifacts"`
	Actions        []Action         `json:"actions"`
	Problems       []Problem        `json:"problems,omitempty"`
}

type ArtifactReport struct {
	Kind           string `json:"kind"`
	Path           string `json:"path"`
	Version        string `json:"version"`
	CurrentVersion string `json:"current_version"`
	Compatible     bool   `json:"compatible"`
	NeedsMigration bool   `json:"needs_migration"`
	Policy         string `json:"policy"`
	Problem        string `json:"problem,omitempty"`
}

type Action struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type Problem struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

func InspectProject(projectPath string, writeRequested bool) (Report, error) {
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		OK:             true,
		ProjectPath:    absProjectPath,
		CurrentVersion: artifactschema.CurrentVersion,
		WriteRequested: writeRequested,
	}

	projectDoc, err := readJSONObject(absProjectPath)
	if err != nil {
		return report, fmt.Errorf("read project: %w", err)
	}
	projectVersion, err := stringField(projectDoc, "schema_version")
	if err != nil {
		projectVersion = ""
	}
	addArtifactReport(&report, "project", absProjectPath, projectVersion)

	graphRel, err := stringField(projectDoc, "graph")
	if err != nil {
		addProblem(&report, "graph", "", err.Error())
	} else {
		graphPath := graphRel
		if !filepath.IsAbs(graphPath) {
			graphPath = filepath.Join(filepath.Dir(absProjectPath), graphPath)
		}
		graphPath, err = filepath.Abs(graphPath)
		if err != nil {
			addProblem(&report, "graph", graphRel, err.Error())
		} else {
			graphDoc, err := readJSONObject(graphPath)
			if err != nil {
				addProblem(&report, "graph", graphPath, err.Error())
			} else {
				graphVersion, err := stringField(graphDoc, "schema_version")
				if err != nil {
					graphVersion = ""
				}
				addArtifactReport(&report, "graph", graphPath, graphVersion)
			}
		}
	}

	finishReport(&report)
	return report, nil
}

func readJSONObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var doc map[string]json.RawMessage
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected trailing JSON value")
	}
	return doc, nil
}

func stringField(doc map[string]json.RawMessage, name string) (string, error) {
	raw, ok := doc[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func addArtifactReport(report *Report, kind string, path string, version string) {
	compatibility, err := artifactschema.Report(kind, version)
	artifact := ArtifactReport{
		Kind:           compatibility.Kind,
		Path:           path,
		Version:        compatibility.Version,
		CurrentVersion: compatibility.CurrentVersion,
		Compatible:     compatibility.Compatible,
		NeedsMigration: compatibility.NeedsMigration,
		Policy:         compatibility.Policy,
	}
	if err != nil {
		artifact.Problem = err.Error()
		artifact.NeedsMigration = true
		addProblem(report, kind, path, err.Error())
	}
	report.Artifacts = append(report.Artifacts, artifact)
}

func addProblem(report *Report, kind string, path string, message string) {
	report.Problems = append(report.Problems, Problem{Kind: kind, Path: path, Message: message})
}

func finishReport(report *Report) {
	report.OK = len(report.Problems) == 0
	for _, artifact := range report.Artifacts {
		if !artifact.Compatible || artifact.NeedsMigration {
			report.OK = false
			report.Actions = append(report.Actions, Action{
				Kind:    "manual_migration_required",
				Path:    artifact.Path,
				Message: fmt.Sprintf("%s schema_version %s is outside the %s compatibility line", artifact.Kind, artifact.Version, artifact.CurrentVersion),
			})
		}
	}
	if report.OK {
		message := "project and graph are compatible; no migration is needed"
		if report.WriteRequested {
			message = "project and graph are compatible; no files were changed"
		}
		report.Actions = append(report.Actions, Action{Kind: "no_migration_needed", Message: message})
	}
}
