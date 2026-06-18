package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func checkComponentSource(ctx context.Context, loaded *project.LoadedProject, req sourceCheckRequest) (SourceCheck, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return SourceCheck{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return SourceCheck{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, sourcePath)
	expectedClass := classNameFromPath(component.Class)
	expectedFunction := ""
	if component.Source.Layout == "generated_wrapper" {
		expectedFunction = "step"
	}
	check := SourceCheck{
		OK:               true,
		ComponentID:      componentID,
		RelativePath:     filepath.ToSlash(rel),
		ExpectedClass:    expectedClass,
		ExpectedFunction: expectedFunction,
		LineCount:        countLines(req.Content),
		Problems:         []Problem{},
	}
	if strings.TrimSpace(req.Content) == "" {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: "source is empty", ComponentID: componentID})
	}
	if component.Source.Layout == "generated_wrapper" {
		check.Problems = append(check.Problems, generatedWrapperStepProblems(componentID, req.Content)...)
	} else {
		check.Problems = append(check.Problems, singleFileClassProblems(componentID, req.Content, expectedClass)...)
	}
	if !strings.Contains(req.Content, "return") {
		check.Problems = append(check.Problems, Problem{Severity: "warning", Message: "source has no return statement", ComponentID: componentID})
	}
	check.Problems = append(check.Problems, sourceContractReferenceProblems(component, req.Content, filepath.ToSlash(rel))...)
	syntaxProblems := pythonSyntaxProblems(ctx, loaded, componentID, filepath.ToSlash(rel), req.Content)
	check.Problems = append(check.Problems, syntaxProblems...)
	if !hasErrorProblems(syntaxProblems) {
		check.Problems = append(check.Problems, pythonUndefinedNameProblems(ctx, loaded, componentID, filepath.ToSlash(rel), req.Content)...)
	}
	if component.Source.Layout != "generated_wrapper" && !hasErrorProblems(syntaxProblems) && expectedClass != "" {
		check.Problems = append(check.Problems, pythonLoadProblems(ctx, loaded, componentID, filepath.ToSlash(rel), expectedClass, req.Content)...)
	}
	check.OK = !hasErrorProblems(check.Problems)
	return check, nil
}

func checkProjectSources(ctx context.Context, loaded *project.LoadedProject) (int, []Problem) {
	problems := []Problem{}
	count := 0
	for _, component := range loaded.Graph.Components {
		if component.Kind != "user_python" {
			continue
		}
		count++
		source, err := loadComponentSource(loaded, component.ID, false)
		if err != nil {
			problems = append(problems, Problem{
				Severity:    "error",
				Message:     fmt.Sprintf("source check failed: %s", err),
				ComponentID: component.ID,
			})
			continue
		}
		check, err := checkComponentSource(ctx, loaded, sourceCheckRequest{
			ComponentID: component.ID,
			Content:     source.Content,
		})
		if err != nil {
			problems = append(problems, Problem{
				Severity:    "error",
				Message:     fmt.Sprintf("source check failed: %s", err),
				ComponentID: component.ID,
			})
			continue
		}
		problems = append(problems, check.Problems...)
	}
	return count, problems
}

func projectSourceErrorProblems(ctx context.Context, loaded *project.LoadedProject) []Problem {
	_, problems := checkProjectSources(ctx, loaded)
	if hasErrorProblems(problems) {
		return problems
	}
	return []Problem{}
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func findPythonClassLine(content string, className string) int {
	pattern := regexp.MustCompile(`(?m)^class\s+` + regexp.QuoteMeta(className) + `\b`)
	return regexpLine(content, pattern.FindStringIndex(content))
}

func singleFileClassProblems(componentID string, content string, expectedClass string) []Problem {
	problems := []Problem{}
	if expectedClass == "" {
		problems = append(problems, Problem{Severity: "error", Message: "component class path is invalid", ComponentID: componentID})
	} else if line := findPythonClassLine(content, expectedClass); line == 0 {
		problems = append(problems, Problem{Severity: "error", Message: fmt.Sprintf("expected class is missing: %s", expectedClass), ComponentID: componentID})
	}
	if line, params := findPythonMethodSignature(content, "evaluate"); line == 0 {
		problems = append(problems, Problem{Severity: "error", Message: "evaluate method is missing", ComponentID: componentID})
	} else if !pythonMethodSignatureMatches(params, []string{"self", "inputs", "state", "params", "context"}) {
		problems = append(problems, Problem{
			Severity:    "error",
			Message:     "evaluate signature must be (self, inputs, state, params, context)",
			ComponentID: componentID,
			Line:        line,
		})
	}
	if line, params := findPythonMethodSignature(content, "initialize"); line != 0 && !pythonMethodSignatureMatches(params, []string{"self", "params", "context"}) {
		problems = append(problems, Problem{
			Severity:    "error",
			Message:     "initialize signature must be (self, params, context)",
			ComponentID: componentID,
			Line:        line,
		})
	}
	return problems
}

func generatedWrapperStepProblems(componentID string, content string) []Problem {
	line, params := findPythonFunctionSignature(content, "step")
	if line == 0 {
		return []Problem{{Severity: "error", Message: "step function is missing", ComponentID: componentID}}
	}
	if pythonMethodSignatureMatches(params, []string{"inputs", "state", "params", "context"}) {
		return []Problem{}
	}
	return []Problem{{
		Severity:    "error",
		Message:     "step signature must be (inputs, state, params, context)",
		ComponentID: componentID,
		Line:        line,
	}}
}

func findPythonFunctionSignature(content string, functionName string) (int, []string) {
	pattern := regexp.MustCompile(`(?m)^def\s+` + regexp.QuoteMeta(functionName) + `\s*\(([^)]*)\)`)
	match := pattern.FindStringSubmatchIndex(content)
	if len(match) < 4 {
		return 0, nil
	}
	return regexpLine(content, match[:2]), pythonParameterNames(content[match[2]:match[3]])
}

func findPythonMethodSignature(content string, methodName string) (int, []string) {
	pattern := regexp.MustCompile(`(?m)^\s+def\s+` + regexp.QuoteMeta(methodName) + `\s*\(([^)]*)\)`)
	match := pattern.FindStringSubmatchIndex(content)
	if len(match) < 4 {
		return 0, nil
	}
	return regexpLine(content, match[:2]), pythonParameterNames(content[match[2]:match[3]])
}

func pythonParameterNames(signature string) []string {
	parts := strings.Split(signature, ",")
	names := []string{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		name = strings.TrimLeft(name, "*")
		if index := strings.Index(name, "="); index >= 0 {
			name = name[:index]
		}
		if index := strings.Index(name, ":"); index >= 0 {
			name = name[:index]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func pythonMethodSignatureMatches(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func sourceContractReferenceProblems(component model.Component, content string, relativePath string) []Problem {
	problems := []Problem{}
	for _, node := range component.Nodes.Inputs {
		if node.IsRequired() && !sourceReferencesInput(content, node.ID) {
			problems = append(problems, Problem{
				Severity:    "warning",
				Message:     fmt.Sprintf("required input node is not referenced in source: %s", node.ID),
				ComponentID: component.ID,
				Source:      relativePath,
				Line:        sourceContractHintLine(component, content),
			})
		}
	}
	for _, node := range component.Nodes.Outputs {
		if !sourceReferencesQuotedName(content, node.ID) {
			problems = append(problems, Problem{
				Severity:    "warning",
				Message:     fmt.Sprintf("output node is not obviously returned by source: %s", node.ID),
				ComponentID: component.ID,
				NodeID:      node.ID,
				Source:      relativePath,
				Line:        sourceContractHintLine(component, content),
			})
		}
	}
	parameterNames := sourceContractParameterNames(component)
	for _, ref := range sourceNamespaceReferences(content, "params") {
		if _, ok := parameterNames[ref.Name]; ok {
			continue
		}
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("parameter reference is not in component contract: %s", ref.Name),
			ComponentID: component.ID,
			Source:      relativePath,
			Line:        ref.Line,
			Column:      ref.Column,
		})
	}
	stateNames := sourceContractStateNames(component)
	for _, ref := range sourceNamespaceReferences(content, "state") {
		if _, ok := stateNames[ref.Name]; ok {
			continue
		}
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("state reference is not in component contract: %s", ref.Name),
			ComponentID: component.ID,
			Source:      relativePath,
			Line:        ref.Line,
			Column:      ref.Column,
		})
	}
	return problems
}

func sourceContractParameterNames(component model.Component) map[string]struct{} {
	names := map[string]struct{}{}
	for name := range component.Parameters {
		names[name] = struct{}{}
	}
	for name := range component.ParameterDefinitions {
		names[name] = struct{}{}
	}
	return names
}

func sourceContractStateNames(component model.Component) map[string]struct{} {
	names := map[string]struct{}{}
	for name := range component.StateDefinitions {
		names[name] = struct{}{}
	}
	return names
}

func sourceContractHintLine(component model.Component, content string) int {
	targetLine := 0
	if component.Source.Layout == "generated_wrapper" {
		line, _ := findPythonFunctionSignature(content, "step")
		if line != 0 {
			targetLine = line
		}
	} else {
		line, _ := findPythonMethodSignature(content, "evaluate")
		if line != 0 {
			targetLine = line
		}
	}
	if line := firstPythonReturnLineAtOrAfter(content, targetLine); line != 0 {
		return line
	}
	if targetLine != 0 {
		return targetLine
	}
	if strings.TrimSpace(content) != "" {
		return 1
	}
	return 0
}

func firstPythonReturnLineAtOrAfter(content string, minLine int) int {
	if minLine <= 0 {
		minLine = 1
	}
	pattern := regexp.MustCompile(`(?m)^\s*return\b`)
	for _, match := range pattern.FindAllStringIndex(content, -1) {
		line := regexpLine(content, match)
		if line >= minLine {
			return line
		}
	}
	return 0
}

func sourceReferencesInput(content string, id string) bool {
	doubleQuoted := fmt.Sprintf(`"%s"`, id)
	singleQuoted := fmt.Sprintf(`'%s'`, id)
	return strings.Contains(content, "inputs["+doubleQuoted+"]") ||
		strings.Contains(content, "inputs["+singleQuoted+"]") ||
		strings.Contains(content, "inputs.get("+doubleQuoted) ||
		strings.Contains(content, "inputs.get("+singleQuoted)
}

type sourceNamespaceReference struct {
	Name   string
	Line   int
	Column int
}

func sourceNamespaceReferences(content string, namespace string) []sourceNamespaceReference {
	pattern := regexp.MustCompile(`(^|[^A-Za-z0-9_.])` + regexp.QuoteMeta(namespace) + `\s*(?:\[\s*"([^"]+)"\s*\]|\[\s*'([^']+)'\s*\]|\.get\(\s*"([^"]+)"|\.get\(\s*'([^']+)')`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	seen := map[string]struct{}{}
	refs := []sourceNamespaceReference{}
	for _, match := range matches {
		if len(match) < 12 {
			continue
		}
		start, end := firstMatchedGroup(match, 2, 3, 4, 5)
		if start < 0 || end < 0 {
			continue
		}
		name := content[start:end]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		refs = append(refs, sourceNamespaceReference{
			Name:   name,
			Line:   regexpLine(content, []int{start, end}),
			Column: regexpColumn(content, start),
		})
	}
	return refs
}

func firstMatchedGroup(match []int, groups ...int) (int, int) {
	for _, group := range groups {
		index := group * 2
		if index+1 >= len(match) {
			continue
		}
		if match[index] >= 0 && match[index+1] >= 0 {
			return match[index], match[index+1]
		}
	}
	return -1, -1
}

func sourceReferencesQuotedName(content string, id string) bool {
	return strings.Contains(content, fmt.Sprintf(`"%s"`, id)) || strings.Contains(content, fmt.Sprintf(`'%s'`, id))
}

func regexpLine(content string, match []int) int {
	if len(match) != 2 {
		return 0
	}
	return strings.Count(content[:match[0]], "\n") + 1
}

func regexpColumn(content string, index int) int {
	if index < 0 || index > len(content) {
		return 0
	}
	lineStart := strings.LastIndex(content[:index], "\n")
	if lineStart < 0 {
		return index + 1
	}
	return index - lineStart
}

func pythonSyntaxProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", "import sys\ncompile(sys.stdin.read(), sys.argv[1], 'exec')", relativePath)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return []Problem{{Severity: "warning", Message: "python syntax check timed out", ComponentID: componentID}}
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			return []Problem{{Severity: "warning", Message: "python syntax check unavailable: " + err.Error(), ComponentID: componentID}}
		}
		return []Problem{syntaxProblemFromStderr(componentID, stderrText)}
	}
	return []Problem{}
}

func pythonLoadProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, expectedClass string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := strings.Join([]string{
		"import sys",
		"namespace = {}",
		"source = sys.stdin.read()",
		"exec(compile(source, sys.argv[1], 'exec'), namespace)",
		"cls = namespace.get(sys.argv[2])",
		"if cls is None:",
		"    raise AttributeError('expected class is missing: ' + sys.argv[2])",
		"if not callable(cls):",
		"    raise TypeError('expected class is not callable: ' + sys.argv[2])",
	}, "\n")
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", script, relativePath, expectedClass)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return []Problem{{Severity: "warning", Message: "python load check timed out", ComponentID: componentID}}
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			return []Problem{{Severity: "warning", Message: "python load check unavailable: " + err.Error(), ComponentID: componentID}}
		}
		problem := syntaxProblemFromStderr(componentID, stderrText)
		problem.Message = "source load failed: " + problem.Message
		return []Problem{problem}
	}
	return []Problem{}
}

func pythonUndefinedNameProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := strings.Join([]string{
		"import ast, builtins, json, sys",
		"source = sys.stdin.read()",
		"tree = ast.parse(source, filename=sys.argv[1])",
		"allowed = set(dir(builtins)) | {'self', 'inputs', 'state', 'params', 'context'}",
		"assigned = set()",
		"loads = []",
		"class Visitor(ast.NodeVisitor):",
		"    def visit_FunctionDef(self, node):",
		"        assigned.add(node.name)",
		"        for arg in list(node.args.posonlyargs) + list(node.args.args) + list(node.args.kwonlyargs):",
		"            assigned.add(arg.arg)",
		"        if node.args.vararg:",
		"            assigned.add(node.args.vararg.arg)",
		"        if node.args.kwarg:",
		"            assigned.add(node.args.kwarg.arg)",
		"        self.generic_visit(node)",
		"    def visit_ClassDef(self, node):",
		"        assigned.add(node.name)",
		"        self.generic_visit(node)",
		"    def visit_Import(self, node):",
		"        for alias in node.names:",
		"            assigned.add(alias.asname or alias.name.split('.')[0])",
		"    def visit_ImportFrom(self, node):",
		"        for alias in node.names:",
		"            assigned.add(alias.asname or alias.name)",
		"    def visit_Name(self, node):",
		"        if isinstance(node.ctx, (ast.Store, ast.Param)):",
		"            assigned.add(node.id)",
		"        elif isinstance(node.ctx, ast.Load):",
		"            loads.append((node.id, node.lineno, node.col_offset + 1))",
		"        self.generic_visit(node)",
		"Visitor().visit(tree)",
		"seen = set()",
		"problems = []",
		"for name, line, column in loads:",
		"    if name in assigned or name in allowed or name in seen:",
		"        continue",
		"    seen.add(name)",
		"    problems.append({'name': name, 'line': line, 'column': column})",
		"print(json.dumps(problems))",
	}, "\n")
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", script, relativePath)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return []Problem{}
	}
	var reported []struct {
		Name   string `json:"name"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &reported); err != nil {
		return []Problem{}
	}
	problems := []Problem{}
	for _, item := range reported {
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("undefined name may fail at runtime: %s", item.Name),
			ComponentID: componentID,
			Line:        item.Line,
			Column:      item.Column,
		})
	}
	return problems
}

func syntaxProblemFromStderr(componentID string, stderrText string) Problem {
	line := 0
	linePattern := regexp.MustCompile(`(?m)File ".*", line ([0-9]+)`)
	if match := linePattern.FindStringSubmatch(stderrText); len(match) == 2 {
		fmt.Sscanf(match[1], "%d", &line)
	}
	lines := strings.Split(stderrText, "\n")
	message := strings.TrimSpace(lines[len(lines)-1])
	if message == "" {
		message = "python syntax error"
	}
	return Problem{Severity: "error", Message: message, ComponentID: componentID, Line: line}
}

func resolveStudioPython(projectRoot string, env model.EnvironmentConfig) string {
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

func hasErrorProblems(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Severity == "error" {
			return true
		}
	}
	return false
}
