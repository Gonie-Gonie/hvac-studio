package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	if err := json.Unmarshal(projectBytes, &proj); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}
	if proj.Graph == "" {
		return nil, fmt.Errorf("project graph path is required")
	}
	if proj.EntrySystem == "" {
		return nil, fmt.Errorf("project entry_system is required")
	}

	root := filepath.Dir(absProjectPath)
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
	if err := json.Unmarshal(graphBytes, &graph); err != nil {
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
