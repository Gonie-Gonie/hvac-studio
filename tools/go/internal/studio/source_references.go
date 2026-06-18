package studio

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func sourceContractReferenceProblems(component model.Component, content string, relativePath string) []Problem {
	problems := []Problem{}
	inputNames := sourceContractNodeNames(component.Nodes.Inputs)
	for _, ref := range sourceNamespaceReferences(content, "inputs") {
		if _, ok := inputNames[ref.Name]; ok {
			continue
		}
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("input node reference is not in component contract: %s", ref.Name),
			ComponentID: component.ID,
			NodeID:      ref.Name,
			Source:      relativePath,
			Line:        ref.Line,
			Column:      ref.Column,
		})
	}
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
	outputNames := sourceContractNodeNames(component.Nodes.Outputs)
	for _, ref := range sourceReturnOutputKeyReferences(content) {
		if _, ok := outputNames[ref.Name]; ok {
			continue
		}
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("output node reference is not in component contract: %s", ref.Name),
			ComponentID: component.ID,
			NodeID:      ref.Name,
			Source:      relativePath,
			Line:        ref.Line,
			Column:      ref.Column,
		})
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

func sourceContractNodeNames(nodes []model.Node) map[string]struct{} {
	names := map[string]struct{}{}
	for _, node := range nodes {
		names[node.ID] = struct{}{}
	}
	return names
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

func sourceReturnOutputKeyReferences(content string) []sourceNamespaceReference {
	returnPattern := regexp.MustCompile(`\breturn\b`)
	matches := returnPattern.FindAllStringIndex(content, -1)
	seen := map[string]struct{}{}
	refs := []sourceNamespaceReference{}
	for _, match := range matches {
		braceStart := sourceReturnOutputDictStart(content, match[1])
		if braceStart < 0 {
			continue
		}
		for _, ref := range sourceDictKeyReferences(content, braceStart) {
			if _, ok := seen[ref.Name]; ok {
				continue
			}
			seen[ref.Name] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs
}

func sourceReturnOutputDictStart(content string, index int) int {
	for index < len(content) {
		switch content[index] {
		case ' ', '\t', '\r', '\n', '(':
			index++
		case '{':
			return index
		default:
			return -1
		}
	}
	return -1
}

func sourceDictKeyReferences(content string, braceStart int) []sourceNamespaceReference {
	if braceStart < 0 || braceStart >= len(content) || content[braceStart] != '{' {
		return nil
	}
	refs := []sourceNamespaceReference{}
	depth := 1
	for index := braceStart + 1; index < len(content) && depth > 0; {
		switch content[index] {
		case '\'', '"':
			name, start, end, next, ok := sourceStringLiteralAt(content, index)
			if !ok {
				return refs
			}
			if depth == 1 && sourceStringIsDictKey(content, next) {
				refs = append(refs, sourceNamespaceReference{
					Name:   name,
					Line:   regexpLine(content, []int{start, end}),
					Column: regexpColumn(content, start),
				})
			}
			index = next
		case '{':
			depth++
			index++
		case '}':
			depth--
			index++
		default:
			index++
		}
	}
	return refs
}

func sourceStringLiteralAt(content string, quoteIndex int) (string, int, int, int, bool) {
	if quoteIndex < 0 || quoteIndex >= len(content) {
		return "", 0, 0, 0, false
	}
	quote := content[quoteIndex]
	if quote != '\'' && quote != '"' {
		return "", 0, 0, 0, false
	}
	for index := quoteIndex + 1; index < len(content); index++ {
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] == quote {
			return content[quoteIndex+1 : index], quoteIndex + 1, index, index + 1, true
		}
	}
	return "", 0, 0, 0, false
}

func sourceStringIsDictKey(content string, index int) bool {
	for index < len(content) {
		switch content[index] {
		case ' ', '\t', '\r', '\n':
			index++
		default:
			return content[index] == ':'
		}
	}
	return false
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
