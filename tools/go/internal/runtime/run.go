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
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/pythonworker"
)

type RunInput struct {
	Inputs  map[string]any `json:"inputs"`
	Context map[string]any `json:"context"`
}

type RunResult struct {
	OK               bool                      `json:"ok"`
	Outputs          map[string]any            `json:"outputs"`
	ComponentInputs  map[string]map[string]any `json:"component_inputs"`
	ComponentOutputs map[string]map[string]any `json:"component_outputs"`
	States           map[string]map[string]any `json:"states"`
	Context          map[string]any            `json:"context"`
	ExecutionOrder   []string                  `json:"execution_order"`
}

func Run(ctx context.Context, loaded *project.LoadedProject, input RunInput) (*RunResult, error) {
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}

	pythonExe := resolvePython(loaded.Root, loaded.Project.Environment)
	client, err := pythonworker.Start(ctx, pythonExe, loaded.Root)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePythonWorker, err)
	}
	defer client.Close()

	states := map[string]map[string]any{}
	componentInputsByID := map[string]map[string]any{}
	componentOutputs := map[string]map[string]any{}

	for _, componentID := range plan.Order {
		component := plan.Index.Components[componentID]
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		if component.Kind != "user_python" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component %s kind %q is not supported by the MVP runner", component.ID, component.Kind)
		}
		if component.Class == "" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component %s kind user_python requires class", component.ID)
		}
		if err := client.LoadComponent(component.ID, component.Class, loaded.Root); err != nil {
			return nil, apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("load component %s: %w", component.ID, err))
		}
		state, err := client.InitializeComponent(component.ID, component.Parameters, input.Context)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("initialize component %s: %w", component.ID, err))
		}
		if state == nil {
			state = map[string]any{}
		}
		states[component.ID] = state
	}

	for _, componentID := range plan.Order {
		component := plan.Index.Components[componentID]
		componentInputs, err := collectInputs(component, plan, input.Inputs, componentOutputs)
		if err != nil {
			return nil, err
		}
		componentInputsByID[component.ID] = componentInputs

		outputs, nextState, err := client.EvaluateComponent(
			component.ID,
			componentInputs,
			states[component.ID],
			component.Parameters,
			input.Context,
		)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("evaluate component %s: %w", component.ID, err))
		}
		if outputs == nil {
			outputs = map[string]any{}
		}
		if nextState == nil {
			nextState = map[string]any{}
		}
		if err := validateOutputs(component, outputs); err != nil {
			return nil, err
		}
		componentOutputs[component.ID] = outputs
		states[component.ID] = nextState
	}

	publicOutputs := map[string]any{}
	for _, output := range plan.System.PublicOutputs {
		componentValues := componentOutputs[output.Component]
		value, ok := componentValues[output.Node]
		if !ok {
			return nil, apperror.Errorf(apperror.CodeRuntime, "public output %s could not read %s.%s", output.ID, output.Component, output.Node)
		}
		publicOutputs[output.ID] = value
	}

	return &RunResult{
		OK:               true,
		Outputs:          publicOutputs,
		ComponentInputs:  componentInputsByID,
		ComponentOutputs: componentOutputs,
		States:           states,
		Context:          input.Context,
		ExecutionOrder:   plan.Order,
	}, nil
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
			inputs[node.ID] = value
			continue
		}

		if publicInput, ok := publicByEndpoint[key]; ok {
			value, exists := publicInputs[publicInput.ID]
			if exists {
				inputs[node.ID] = value
				continue
			}
			if publicInput.Default != nil {
				inputs[node.ID] = publicInput.Default
				continue
			}
			if publicInput.IsRequired() {
				return nil, apperror.Errorf(apperror.CodeInput, "missing required public input: %s", publicInput.ID)
			}
			continue
		}

		if node.Default != nil {
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
	for _, node := range component.Nodes.Outputs {
		if _, ok := outputs[node.ID]; !ok {
			return apperror.Errorf(apperror.CodePythonWorker, "component %s did not return declared output node: %s", component.ID, node.ID)
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
	if isDefaultPythonName(env.Python) {
		if packagedPython := findPackagedPython(projectRoot); packagedPython != "" {
			return packagedPython
		}
	}
	return env.Python
}

func isDefaultPythonName(path string) bool {
	name := filepath.Base(path)
	return name == "python" || name == "python.exe" || name == "python3" || name == "python3.exe"
}

func findPackagedPython(start string) string {
	if start == "" {
		return ""
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		candidates := []string{
			filepath.Join(absStart, "runtime", "python", "python.exe"),
			filepath.Join(absStart, "runtime", "python", "python"),
			filepath.Join(absStart, "runtime", "python", "bin", "python"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(absStart)
		if parent == absStart {
			return ""
		}
		absStart = parent
	}
}
