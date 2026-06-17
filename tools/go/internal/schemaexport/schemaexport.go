package schemaexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type InterfaceSchema struct {
	SchemaVersion string           `json:"schema_version"`
	ProjectName   string           `json:"project_name"`
	System        string           `json:"system"`
	Inputs        []PublicNodeInfo `json:"inputs"`
	Outputs       []PublicNodeInfo `json:"outputs"`
	ModelAssets   []ModelAssetInfo `json:"model_assets,omitempty"`
}

type PublicNodeInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Component string `json:"component"`
	Node      string `json:"node"`
	Medium    string `json:"medium,omitempty"`
	ValueType string `json:"value_type,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Required  bool   `json:"required,omitempty"`
	Default   any    `json:"default,omitempty"`
}

type ModelAssetInfo struct {
	Component        string   `json:"component"`
	ModelFormat      string   `json:"model_format,omitempty"`
	Field            string   `json:"field"`
	Path             string   `json:"path"`
	RequiredPackages []string `json:"required_packages,omitempty"`
}

func Export(loaded *project.LoadedProject) (*InterfaceSchema, error) {
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return nil, err
	}

	schema := &InterfaceSchema{
		SchemaVersion: loaded.Project.SchemaVersion,
		ProjectName:   loaded.Project.ProjectName,
		System:        loaded.Project.EntrySystem,
		Inputs:        make([]PublicNodeInfo, 0, len(plan.System.PublicInputs)),
		Outputs:       make([]PublicNodeInfo, 0, len(plan.System.PublicOutputs)),
	}

	for _, input := range plan.System.PublicInputs {
		node, _ := plan.Index.InputNode(input.Component, input.Node)
		schema.Inputs = append(schema.Inputs, publicNodeInfo(input, node, true))
	}
	for _, output := range plan.System.PublicOutputs {
		node, _ := plan.Index.OutputNode(output.Component, output.Node)
		schema.Outputs = append(schema.Outputs, publicNodeInfo(output, node, false))
	}
	for _, componentID := range plan.System.Components {
		component := plan.Index.Components[componentID]
		if component.MLMetadata == nil {
			continue
		}
		for _, asset := range component.MLMetadata.AssetPaths() {
			if asset.Path == "" {
				continue
			}
			schema.ModelAssets = append(schema.ModelAssets, ModelAssetInfo{
				Component:        component.ID,
				ModelFormat:      component.MLMetadata.ModelFormat,
				Field:            asset.Field,
				Path:             asset.Path,
				RequiredPackages: append([]string{}, component.MLMetadata.RequiredPackages...),
			})
		}
	}

	return schema, nil
}

func Write(outputPath string, schema *InterfaceSchema) error {
	output, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	if outputPath == "" {
		fmt.Println(string(output))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(output, '\n'), 0o644)
}

func publicNodeInfo(public model.PublicNodeRef, node model.Node, input bool) PublicNodeInfo {
	info := PublicNodeInfo{
		ID:        public.ID,
		Name:      firstNonEmpty(public.Name, node.Name),
		Component: public.Component,
		Node:      public.Node,
		Medium:    firstNonEmpty(public.Medium, node.Medium),
		ValueType: firstNonEmpty(public.ValueType, node.ValueType),
		Unit:      firstNonEmpty(public.Unit, node.Unit),
	}
	if input {
		info.Required = public.IsRequired()
		if public.Default != nil {
			info.Default = public.Default
		}
	}
	return info
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
