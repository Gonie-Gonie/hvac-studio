package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type RunInput struct {
	Inputs  map[string]any `json:"inputs"`
	Context map[string]any `json:"context"`
}

type RunResult struct {
	OK               bool                      `json:"ok"`
	ParameterSet     string                    `json:"parameter_set,omitempty"`
	Outputs          map[string]any            `json:"outputs"`
	ComponentInputs  map[string]map[string]any `json:"component_inputs"`
	ComponentOutputs map[string]map[string]any `json:"component_outputs"`
	NodeValues       []NodeValueTrace          `json:"node_values"`
	ConnectionValues []ConnectionValueTrace    `json:"connection_values"`
	States           map[string]map[string]any `json:"states"`
	Context          map[string]any            `json:"context"`
	ExecutionOrder   []string                  `json:"execution_order"`
	ComponentTimings []ComponentTiming         `json:"component_timings,omitempty"`
	ComponentLogs    []ComponentLog            `json:"component_logs,omitempty"`
	DurationMS       float64                   `json:"duration_ms,omitempty"`
}

type NodeValueTrace struct {
	Component string `json:"component"`
	Node      string `json:"node"`
	Direction string `json:"direction"`
	Medium    string `json:"medium,omitempty"`
	ValueType string `json:"value_type,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Value     any    `json:"value"`
}

type ConnectionValueTrace struct {
	ID           string         `json:"id"`
	From         model.Endpoint `json:"from"`
	To           model.Endpoint `json:"to"`
	SourceMedium string         `json:"source_medium,omitempty"`
	TargetMedium string         `json:"target_medium,omitempty"`
	ValueType    string         `json:"value_type,omitempty"`
	Unit         string         `json:"unit,omitempty"`
	Value        any            `json:"value"`
}

type ComponentTiming struct {
	Component  string  `json:"component"`
	Stage      string  `json:"stage"`
	DurationMS float64 `json:"duration_ms"`
}

type ComponentLog struct {
	Component string `json:"component"`
	Stage     string `json:"stage"`
	Stream    string `json:"stream,omitempty"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

func Run(ctx context.Context, loaded *project.LoadedProject, input RunInput) (*RunResult, error) {
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	session, err := newSession(ctx, loaded, input.Context)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Evaluate(input)
}

func nodeValueTraces(componentID string, direction string, nodes []model.Node, values map[string]any) []NodeValueTrace {
	traces := []NodeValueTrace{}
	for _, node := range nodes {
		value, exists := values[node.ID]
		if !exists {
			continue
		}
		traces = append(traces, NodeValueTrace{
			Component: componentID,
			Node:      node.ID,
			Direction: direction,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Value:     value,
		})
	}
	return traces
}

func connectionValueTraces(plan *compiler.Plan, componentOutputs map[string]map[string]any) []ConnectionValueTrace {
	traces := []ConnectionValueTrace{}
	for _, connectionID := range plan.System.Connections {
		connection := plan.Index.Connections[connectionID]
		componentValues := componentOutputs[connection.From.Component]
		value, exists := componentValues[connection.From.Node]
		if !exists {
			continue
		}
		sourceNode, _ := plan.Index.OutputNode(connection.From.Component, connection.From.Node)
		targetNode, _ := plan.Index.InputNode(connection.To.Component, connection.To.Node)
		traceValue := value
		traceValueType := sourceNode.ValueType
		traceUnit := sourceNode.Unit
		if connection.UnitConversion != nil {
			if converted, err := applyConnectionUnitConversion(connection, value); err == nil {
				traceValue = converted
				traceValueType = targetNode.ValueType
				traceUnit = targetNode.Unit
			}
		}
		traces = append(traces, ConnectionValueTrace{
			ID:           connection.ID,
			From:         connection.From,
			To:           connection.To,
			SourceMedium: sourceNode.Medium,
			TargetMedium: targetNode.Medium,
			ValueType:    traceValueType,
			Unit:         traceUnit,
			Value:        traceValue,
		})
	}
	return traces
}

func LoadInput(inputPath string) (RunInput, error) {
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return RunInput{}, apperror.Wrap(apperror.CodeInput, err)
	}
	var structured RunInput
	if err := json.Unmarshal(inputBytes, &structured); err != nil {
		return RunInput{}, apperror.Wrap(apperror.CodeInput, err)
	}
	if structured.Inputs != nil {
		if structured.Context == nil {
			structured.Context = map[string]any{}
		}
		return structured, nil
	}

	var plain map[string]any
	if err := json.Unmarshal(inputBytes, &plain); err != nil {
		return RunInput{}, apperror.Wrap(apperror.CodeInput, err)
	}
	return RunInput{Inputs: plain, Context: map[string]any{}}, nil
}

func WriteResult(outputPath string, result *RunResult) error {
	output, err := json.MarshalIndent(result, "", "  ")
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

func collectInputs(component model.Component, plan *compiler.Plan, publicInputs map[string]any, componentOutputs map[string]map[string]any) (map[string]any, error) {
	inputs := map[string]any{}
	publicByEndpoint := map[string]model.PublicNodeRef{}
	for _, input := range plan.System.PublicInputs {
		publicByEndpoint[compiler.EndpointKey(input.Component, input.Node)] = input
	}

	for _, node := range component.Nodes.Inputs {
		key := compiler.EndpointKey(component.ID, node.ID)
		if connection, ok := plan.Incoming[key]; ok {
			sourceOutputs := componentOutputs[connection.From.Component]
			value, exists := sourceOutputs[connection.From.Node]
			if !exists {
				return nil, apperror.Errorf(apperror.CodeRuntime, "connection %s source output is missing: %s.%s", connection.ID, connection.From.Component, connection.From.Node)
			}
			value, err := applyConnectionUnitConversion(connection, value)
			if err != nil {
				return nil, err
			}
			if err := validateInputValue(component.ID, node, value, apperror.CodeRuntime); err != nil {
				return nil, err
			}
			inputs[node.ID] = value
			continue
		}

		if publicInput, ok := publicByEndpoint[key]; ok {
			value, exists := publicInputs[publicInput.ID]
			if exists {
				if err := validateInputValue(component.ID, node, value, apperror.CodeInput); err != nil {
					return nil, err
				}
				inputs[node.ID] = value
				continue
			}
			if publicInput.Default != nil {
				if err := validateInputValue(component.ID, node, publicInput.Default, apperror.CodeValidation); err != nil {
					return nil, err
				}
				inputs[node.ID] = publicInput.Default
				continue
			}
			if publicInput.IsRequired() {
				return nil, apperror.Errorf(apperror.CodeInput, "missing required public input: %s", publicInput.ID)
			}
			continue
		}

		if node.Default != nil {
			if err := validateInputValue(component.ID, node, node.Default, apperror.CodeValidation); err != nil {
				return nil, err
			}
			inputs[node.ID] = node.Default
			continue
		}
		if node.IsRequired() {
			return nil, apperror.Errorf(apperror.CodeValidation, "component %s missing required input node: %s", component.ID, node.ID)
		}
	}

	return inputs, nil
}

func validateOutputs(component model.Component, outputs map[string]any) error {
	code := apperror.CodePythonWorker
	if component.Kind == "external_exe" || component.Kind == "composite" {
		code = apperror.CodeRuntime
	}
	declared := map[string]bool{}
	for _, node := range component.Nodes.Outputs {
		declared[node.ID] = true
		value, ok := outputs[node.ID]
		if !ok {
			return apperror.Errorf(code, "component %s did not return declared output node: %s", component.ID, node.ID)
		}
		if err := validateOutputValue(component, node, value); err != nil {
			return err
		}
	}
	for name := range outputs {
		if !declared[name] {
			return apperror.Errorf(code, "component %s returned undeclared output node: %s", component.ID, name)
		}
	}
	return nil
}

func resolvePython(projectRoot string, env model.EnvironmentConfig) string {
	if env.Python == "" {
		env.Python = "python"
	}
	if filepath.IsAbs(env.Python) {
		return env.Python
	}
	projectPython := filepath.Join(projectRoot, env.Python)
	if _, err := os.Stat(projectPython); err == nil {
		return projectPython
	}
	if platform.IsDefaultPythonName(env.Python) {
		if packagedPython := platform.FindNearestRuntimePython(projectRoot); packagedPython != "" {
			return packagedPython
		}
	}
	return env.Python
}
