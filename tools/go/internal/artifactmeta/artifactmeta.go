package artifactmeta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

const schema = "hvac-studio.workflow-provenance.v1"

type Provenance struct {
	Schema               string         `json:"schema"`
	RunnerVersion        string         `json:"runner_version"`
	PackageVersion       string         `json:"package_version,omitempty"`
	ProjectSchemaVersion string         `json:"project_schema_version,omitempty"`
	GraphSchemaVersion   string         `json:"graph_schema_version,omitempty"`
	EngineVersion        string         `json:"engine_version,omitempty"`
	Project              FileChecksum   `json:"project"`
	Graph                FileChecksum   `json:"graph"`
	Artifacts            []FileChecksum `json:"artifacts,omitempty"`
}

type FileChecksum struct {
	Role        string `json:"role"`
	ComponentID string `json:"component_id,omitempty"`
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
}

type Reference struct {
	Role string
	Path string
}

func Build(loaded *project.LoadedProject, refs []Reference) (Provenance, error) {
	projectFile, err := checksumLoadedFile(loaded, "project", loaded.Path, "")
	if err != nil {
		return Provenance{}, err
	}
	graphFile, err := checksumLoadedFile(loaded, "graph", loaded.GraphPath, "")
	if err != nil {
		return Provenance{}, err
	}

	artifacts := []FileChecksum{}
	for _, ref := range refs {
		if strings.TrimSpace(ref.Path) == "" {
			continue
		}
		file, err := checksumProjectFile(loaded.Root, ref.Role, ref.Path, "")
		if err != nil {
			return Provenance{}, err
		}
		artifacts = append(artifacts, file)
	}
	for _, component := range loaded.Graph.Components {
		for _, rel := range componentSourceFiles(component) {
			file, err := checksumProjectFile(loaded.Root, "component_source", rel, component.ID)
			if err != nil {
				return Provenance{}, err
			}
			artifacts = append(artifacts, file)
		}
	}

	artifacts = uniqueSorted(artifacts)
	return Provenance{
		Schema:               schema,
		RunnerVersion:        RunnerVersion(),
		PackageVersion:       PackageVersion(loaded.Root),
		ProjectSchemaVersion: loaded.Project.SchemaVersion,
		GraphSchemaVersion:   loaded.Graph.SchemaVersion,
		EngineVersion:        loaded.Project.EngineVersion,
		Project:              projectFile,
		Graph:                graphFile,
		Artifacts:            artifacts,
	}, nil
}

func RunnerVersion() string {
	if version := strings.TrimSpace(os.Getenv("HVAC_STUDIO_RUNNER_VERSION")); version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "0.1.0-dev"
}

func PackageVersion(projectRoot string) string {
	if version := strings.TrimSpace(os.Getenv("HVAC_STUDIO_PACKAGE_VERSION")); version != "" {
		return version
	}
	manifestPath := findUpward(projectRoot, "release-manifest.json")
	if manifestPath == "" {
		return ""
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	return strings.TrimSpace(manifest.Version)
}

func checksumLoadedFile(loaded *project.LoadedProject, role string, path string, componentID string) (FileChecksum, error) {
	rel, err := filepath.Rel(loaded.Root, path)
	if err != nil {
		return FileChecksum{}, err
	}
	return checksumProjectFile(loaded.Root, role, rel, componentID)
}

func checksumProjectFile(projectRoot string, role string, rel string, componentID string) (FileChecksum, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if cleanRel == "." || filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || cleanRel == ".." {
		return FileChecksum{}, os.ErrInvalid
	}
	path := filepath.Join(projectRoot, cleanRel)
	sum, err := sha256File(path)
	if err != nil {
		return FileChecksum{}, err
	}
	return FileChecksum{
		Role:        strings.TrimSpace(role),
		ComponentID: strings.TrimSpace(componentID),
		Path:        filepath.ToSlash(cleanRel),
		SHA256:      sum,
	}, nil
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func componentSourceFiles(component model.Component) []string {
	paths := []string{}
	source := component.Source
	for _, rel := range []string{source.Metadata, source.Init, source.Step, source.Helpers, source.Wrapper} {
		rel = strings.TrimSpace(rel)
		if rel != "" {
			paths = append(paths, rel)
		}
	}
	if len(paths) == 0 {
		if rel := classPathSource(component.Class); rel != "" {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	return compactStrings(paths)
}

func classPathSource(classPath string) string {
	parts := strings.Split(strings.TrimSpace(classPath), ".")
	if len(parts) < 3 || parts[0] != "components" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(parts[:len(parts)-1]...) + ".py")
}

func uniqueSorted(files []FileChecksum) []FileChecksum {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		if files[i].Role != files[j].Role {
			return files[i].Role < files[j].Role
		}
		return files[i].ComponentID < files[j].ComponentID
	})
	out := []FileChecksum{}
	seen := map[string]bool{}
	for _, file := range files {
		key := file.Role + "\x00" + file.ComponentID + "\x00" + file.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, file)
	}
	return out
}

func compactStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
		if value == "." || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func findUpward(start string, name string) string {
	if start == "" {
		return ""
	}
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(current, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
