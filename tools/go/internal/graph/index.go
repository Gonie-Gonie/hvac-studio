package graph

import (
	"fmt"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/artifactschema"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

type Index struct {
	Systems     map[string]model.System
	Components  map[string]model.Component
	Connections map[string]model.Connection
}

func NewIndex(g *model.Graph) (*Index, error) {
	if strings.TrimSpace(g.SchemaVersion) == "" {
		return nil, fmt.Errorf("graph schema_version is required")
	}
	if err := artifactschema.Check("graph", g.SchemaVersion); err != nil {
		return nil, err
	}

	idx := &Index{
		Systems:     map[string]model.System{},
		Components:  map[string]model.Component{},
		Connections: map[string]model.Connection{},
	}

	for _, system := range g.Systems {
		if strings.TrimSpace(system.ID) == "" {
			return nil, fmt.Errorf("system id is required")
		}
		if _, exists := idx.Systems[system.ID]; exists {
			return nil, fmt.Errorf("duplicate system id: %s", system.ID)
		}
		if err := validateSystem(system); err != nil {
			return nil, err
		}
		idx.Systems[system.ID] = system
	}

	for _, component := range g.Components {
		if strings.TrimSpace(component.ID) == "" {
			return nil, fmt.Errorf("component id is required")
		}
		if _, exists := idx.Components[component.ID]; exists {
			return nil, fmt.Errorf("duplicate component id: %s", component.ID)
		}
		if err := validateComponent(component); err != nil {
			return nil, err
		}
		idx.Components[component.ID] = component
	}

	for _, connection := range g.Connections {
		if strings.TrimSpace(connection.ID) == "" {
			return nil, fmt.Errorf("connection id is required")
		}
		if _, exists := idx.Connections[connection.ID]; exists {
			return nil, fmt.Errorf("duplicate connection id: %s", connection.ID)
		}
		if err := validateConnectionShape(connection); err != nil {
			return nil, err
		}
		idx.Connections[connection.ID] = connection
	}

	return idx, nil
}

func validateSystem(system model.System) error {
	if err := validateUniqueIDs("system "+system.ID+" component reference", system.Components); err != nil {
		return err
	}
	if err := validateUniqueIDs("system "+system.ID+" connection reference", system.Connections); err != nil {
		return err
	}
	if err := validatePublicNodeRefs("system "+system.ID+" public input", system.PublicInputs); err != nil {
		return err
	}
	if err := validatePublicNodeRefs("system "+system.ID+" public output", system.PublicOutputs); err != nil {
		return err
	}
	return nil
}

func validateComponent(component model.Component) error {
	switch component.Kind {
	case "user_python", "builtin_go", "external_exe", "future_compiled", "composite":
	default:
		if strings.TrimSpace(component.Kind) == "" {
			return fmt.Errorf("component %s kind is required", component.ID)
		}
		return fmt.Errorf("component %s kind is invalid: %s", component.ID, component.Kind)
	}
	if err := validateOptionalEnum(
		"component "+component.ID+" category",
		component.Category,
		"physical_component",
		"controller",
		"data_source",
		"data_sink",
		"utility",
		"solver",
		"composite_wrapper",
	); err != nil {
		return err
	}
	if err := validateOptionalEnum(
		"component "+component.ID+" execution_mode",
		component.ExecutionMode,
		"step",
		"vectorized",
		"initialization_only",
		"external_executable",
	); err != nil {
		return err
	}
	if err := validateComponentSource(component.ID, component.Source); err != nil {
		return err
	}
	if err := validateNodes(component.ID, "input", component.Nodes.Inputs); err != nil {
		return err
	}
	if err := validateNodes(component.ID, "output", component.Nodes.Outputs); err != nil {
		return err
	}
	if err := validateParameterDefinitions(component.ID, component.ParameterDefinitions); err != nil {
		return err
	}
	if err := validateStateDefinitions(component.ID, component.StateDefinitions); err != nil {
		return err
	}
	if err := validateSolverBoundary(component.ID, component.SolverBoundary); err != nil {
		return err
	}
	if err := validateCompositeReference(component.ID, component.Kind, component.Composite); err != nil {
		return err
	}
	return nil
}

func validateComponentSource(componentID string, source model.ComponentSource) error {
	if err := validateOptionalEnum(
		"component "+componentID+" source layout",
		source.Layout,
		"single_file_class",
		"generated_wrapper",
	); err != nil {
		return err
	}
	if source.Layout == "generated_wrapper" {
		if strings.TrimSpace(source.Step) == "" {
			return fmt.Errorf("component %s generated_wrapper source step is required", componentID)
		}
		if strings.TrimSpace(source.Wrapper) == "" {
			return fmt.Errorf("component %s generated_wrapper source wrapper is required", componentID)
		}
	}
	return nil
}

func validateNodes(componentID string, direction string, nodes []model.Node) error {
	seen := map[string]bool{}
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("component %s %s node id is required", componentID, direction)
		}
		if seen[node.ID] {
			return fmt.Errorf("component %s duplicate %s node id: %s", componentID, direction, node.ID)
		}
		seen[node.ID] = true
		if strings.TrimSpace(node.Medium) == "" {
			return fmt.Errorf("component %s %s node %s medium is required", componentID, direction, node.ID)
		}
		if strings.TrimSpace(node.ValueType) == "" {
			return fmt.Errorf("component %s %s node %s value_type is required", componentID, direction, node.ID)
		}
		if err := validateOptionalEnum(
			"component "+componentID+" "+direction+" node "+node.ID+" preset",
			node.Preset,
			"water_inlet",
			"water_outlet",
			"air_inlet",
			"air_outlet",
			"control_signal_input",
			"electric_power_output",
			"scalar_input",
			"scalar_output",
			"time_series_input",
		); err != nil {
			return err
		}
	}
	return nil
}

func validateParameterDefinitions(componentID string, definitions map[string]model.ParameterDefinition) error {
	for name, definition := range definitions {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("component %s parameter_defs key is required", componentID)
		}
		if err := validateOptionalEnum(
			"component "+componentID+" parameter "+name+" role",
			definition.Role,
			"fixed",
			"scenario_input",
			"calibration_target",
			"optimization_variable",
			"derived",
		); err != nil {
			return err
		}
	}
	return nil
}

func validateStateDefinitions(componentID string, definitions map[string]model.StateDefinition) error {
	for name := range definitions {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("component %s state_defs key is required", componentID)
		}
	}
	return nil
}

func validateSolverBoundary(componentID string, boundary *model.SolverBoundary) error {
	if boundary == nil {
		return nil
	}
	if strings.TrimSpace(boundary.Method) == "" {
		return fmt.Errorf("component %s solver_boundary method is required", componentID)
	}
	if boundary.MaxIterations < 0 {
		return fmt.Errorf("component %s solver_boundary max_iterations must be non-negative", componentID)
	}
	if boundary.Tolerance < 0 {
		return fmt.Errorf("component %s solver_boundary tolerance must be non-negative", componentID)
	}
	return nil
}

func validateCompositeReference(componentID string, kind string, composite *model.CompositeReference) error {
	if kind != "composite" {
		return nil
	}
	if composite == nil || strings.TrimSpace(composite.System) == "" {
		return fmt.Errorf("component %s kind composite requires composite.system", componentID)
	}
	return nil
}

func validateConnectionShape(connection model.Connection) error {
	if strings.TrimSpace(connection.From.Component) == "" {
		return fmt.Errorf("connection %s source component is required", connection.ID)
	}
	if strings.TrimSpace(connection.From.Node) == "" {
		return fmt.Errorf("connection %s source node is required", connection.ID)
	}
	if strings.TrimSpace(connection.To.Component) == "" {
		return fmt.Errorf("connection %s target component is required", connection.ID)
	}
	if strings.TrimSpace(connection.To.Node) == "" {
		return fmt.Errorf("connection %s target node is required", connection.ID)
	}
	if conversion := connection.UnitConversion; conversion != nil {
		if strings.TrimSpace(conversion.Mode) == "" {
			conversion.Mode = "linear"
		}
		if conversion.Mode != "linear" {
			return fmt.Errorf("connection %s unit_conversion mode is unsupported: %s", connection.ID, conversion.Mode)
		}
	}
	return nil
}

func validatePublicNodeRefs(label string, refs []model.PublicNodeRef) error {
	seen := map[string]bool{}
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) == "" {
			return fmt.Errorf("%s id is required", label)
		}
		if seen[ref.ID] {
			return fmt.Errorf("duplicate %s id: %s", label, ref.ID)
		}
		seen[ref.ID] = true
		if strings.TrimSpace(ref.Component) == "" {
			return fmt.Errorf("%s %s component is required", label, ref.ID)
		}
		if strings.TrimSpace(ref.Node) == "" {
			return fmt.Errorf("%s %s node is required", label, ref.ID)
		}
	}
	return nil
}

func validateUniqueIDs(label string, ids []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s id is required", label)
		}
		if seen[id] {
			return fmt.Errorf("duplicate %s id: %s", label, id)
		}
		seen[id] = true
	}
	return nil
}

func validateOptionalEnum(label string, value string, allowed ...string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("%s is invalid: %s", label, value)
}

func (i *Index) InputNode(componentID string, nodeID string) (model.Node, bool) {
	component, ok := i.Components[componentID]
	if !ok {
		return model.Node{}, false
	}
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}

func (i *Index) OutputNode(componentID string, nodeID string) (model.Node, bool) {
	component, ok := i.Components[componentID]
	if !ok {
		return model.Node{}, false
	}
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}
