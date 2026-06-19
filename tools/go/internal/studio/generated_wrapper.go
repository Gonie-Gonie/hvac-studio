package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func writeGeneratedWrapperFile(projectRoot string, component model.Component) error {
	wrapperRel := strings.TrimSpace(component.Source.Wrapper)
	if !componentUsesGeneratedPythonWrapper(component) || wrapperRel == "" {
		return nil
	}
	wrapperPath, err := resolveProjectOwnedFile(projectRoot, wrapperRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(wrapperPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(wrapperPath, []byte(generatedWrapperContent(component)), 0o644)
}

func componentUsesGeneratedPythonWrapper(component model.Component) bool {
	return component.Kind == "user_python" && component.Source.Layout == "generated_wrapper"
}

func generatedWrapperContent(component model.Component) string {
	className := classNameFromPath(component.Class)
	if strings.TrimSpace(className) == "" {
		className = pythonClassName(component.ID)
	}
	return "import json\n" +
		"from . import user_init, user_step\n\n\n" +
		"class " + className + ":\n" +
		generatedWrapperDocstring(component) + "\n" +
		"    input_nodes = json.loads(" + pythonStringLiteralForWrapper(componentNodeContractMap(component.Nodes.Inputs)) + ")\n" +
		"    output_nodes = json.loads(" + pythonStringLiteralForWrapper(componentNodeContractMap(component.Nodes.Outputs)) + ")\n" +
		"    parameter_schema = json.loads(" + pythonStringLiteralForWrapper(component.ParameterDefinitions) + ")\n" +
		"    state_schema = json.loads(" + pythonStringLiteralForWrapper(component.StateDefinitions) + ")\n\n" +
		"    def initialize(self, params, context):\n" +
		"        state = user_init.initialize(params, context)\n" +
		"        if state is None:\n" +
		"            return {}\n" +
		"        return state\n\n" +
		"    def evaluate(self, inputs, state, params, context):\n" +
		"        return user_step.step(inputs, state, params, context)\n"
}

func generatedWrapperDocstring(component model.Component) string {
	return strings.Join([]string{
		"    \"\"\"Studio-owned runtime wrapper.",
		"",
		"    Component contract metadata is regenerated from graph.json/component.json.",
		"    Edit user_step.py for model logic.",
		"",
		"    Inputs: " + generatedWrapperNodeIDs(component.Nodes.Inputs),
		"    Outputs: " + generatedWrapperNodeIDs(component.Nodes.Outputs),
		"    Parameters: " + generatedWrapperParameterNames(component),
		"    State: " + generatedWrapperStateNames(component),
		"    \"\"\"",
		"",
	}, "\n")
}

func generatedWrapperNodeIDs(nodes []model.Node) string {
	ids := []string{}
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) != "" {
			ids = append(ids, node.ID)
		}
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

func generatedWrapperParameterNames(component model.Component) string {
	names := map[string]bool{}
	for name := range component.Parameters {
		names[name] = true
	}
	for name := range component.ParameterDefinitions {
		names[name] = true
	}
	return generatedWrapperSortedNames(names)
}

func generatedWrapperStateNames(component model.Component) string {
	names := map[string]bool{}
	for name := range component.StateDefinitions {
		names[name] = true
	}
	return generatedWrapperSortedNames(names)
}

func generatedWrapperSortedNames(names map[string]bool) string {
	sorted := sortedMapKeys(names)
	if len(sorted) == 0 {
		return "none"
	}
	return strings.Join(sorted, ", ")
}

func componentNodeContractMap(nodes []model.Node) map[string]model.Node {
	out := map[string]model.Node{}
	for _, node := range nodes {
		out[node.ID] = node
	}
	return out
}

func pythonStringLiteralForWrapper(value any) string {
	if value == nil {
		return strconv.Quote("{}")
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "null" {
		return strconv.Quote("{}")
	}
	return strconv.Quote(string(data))
}
