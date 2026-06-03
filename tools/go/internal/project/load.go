package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

type LoadedProject struct {
	Project   *model.Project
	Graph     *model.Graph
	Root      string
	Path      string
	GraphPath string
}

func Load(projectPath string) (*LoadedProject, error) {
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}

	projectBytes, err := os.ReadFile(absProjectPath)
	if err != nil {
		return nil, fmt.Errorf("read project: %w", err)
	}

	var proj model.Project
	if err := decodeStrict(projectBytes, &proj); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}
	if err := validateProject(proj); err != nil {
		return nil, err
	}
	if proj.Graph == "" {
		return nil, fmt.Errorf("project graph path is required")
	}
	if proj.EntrySystem == "" {
		return nil, fmt.Errorf("project entry_system is required")
	}

	root := filepath.Dir(absProjectPath)
	if err := validateEnvironmentLockfile(root, proj.Environment.Lockfile); err != nil {
		return nil, err
	}

	graphPath := proj.Graph
	if !filepath.IsAbs(graphPath) {
		graphPath = filepath.Join(root, graphPath)
	}
	graphPath, err = filepath.Abs(graphPath)
	if err != nil {
		return nil, err
	}

	graphBytes, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, fmt.Errorf("read graph: %w", err)
	}

	var graph model.Graph
	if err := decodeStrict(graphBytes, &graph); err != nil {
		return nil, fmt.Errorf("decode graph: %w", err)
	}

	return &LoadedProject{
		Project:   &proj,
		Graph:     &graph,
		Root:      root,
		Path:      absProjectPath,
		GraphPath: graphPath,
	}, nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON value")
	}
	return nil
}

func validateProject(proj model.Project) error {
	if strings.TrimSpace(proj.ProjectName) == "" {
		return fmt.Errorf("project project_name is required")
	}
	if strings.TrimSpace(proj.SchemaVersion) == "" {
		return fmt.Errorf("project schema_version is required")
	}
	if strings.TrimSpace(proj.EntrySystem) == "" {
		return fmt.Errorf("project entry_system is required")
	}
	if strings.TrimSpace(proj.Graph) == "" {
		return fmt.Errorf("project graph path is required")
	}
	if proj.Environment.Mode != "" {
		switch proj.Environment.Mode {
		case "builtin", "project", "external":
		default:
			return fmt.Errorf("project environment mode is invalid: %s", proj.Environment.Mode)
		}
	}
	return nil
}

func validateEnvironmentLockfile(projectRoot string, lockfile string) error {
	lockfile = strings.TrimSpace(lockfile)
	if lockfile == "" {
		return nil
	}
	if filepath.IsAbs(lockfile) {
		return fmt.Errorf("project environment lockfile must be relative to project root: %s", lockfile)
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	lockPath, err := filepath.Abs(filepath.Join(absRoot, lockfile))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, lockPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("project environment lockfile must stay inside project root: %s", lockfile)
	}
	info, err := os.Stat(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("project environment lockfile not found: %s", lockfile)
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("project environment lockfile is a directory: %s", lockfile)
	}
	return nil
}
