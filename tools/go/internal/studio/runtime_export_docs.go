package studio

import (
	"fmt"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func runtimeExportReadme(projectPath string, defaultInput string, lockfile string, entrypoints []runtimeExportEntrypoint) string {
	inputLine := ""
	if defaultInput != "" {
		inputLine = fmt.Sprintf("- Default input: `%s`\n", defaultInput)
	}
	lockfileLine := ""
	if lockfile != "" {
		lockfileLine = fmt.Sprintf("- Python lockfile: `%s`\n", lockfile)
	}
	commandLines := []string{}
	pythonExamples := []string{}
	for _, entrypoint := range entrypoints {
		if strings.HasSuffix(entrypoint.Rel, ".ps1") {
			commandLines = append(commandLines, fmt.Sprintf("- `powershell -ExecutionPolicy Bypass -File .\\%s`", entrypoint.Rel))
		}
		if strings.HasSuffix(entrypoint.Rel, ".py") {
			pythonExamples = append(pythonExamples, fmt.Sprintf("`%s`", entrypoint.Rel))
		}
	}
	pythonLine := ""
	if len(pythonExamples) > 0 {
		pythonLine = fmt.Sprintf("- Python SDK examples: %s\n", strings.Join(pythonExamples, ", "))
	}
	return "# Runtime Export\n\n" +
		"This folder contains a runnable Studio runtime export.\n\n" +
		fmt.Sprintf("- Project: `%s`\n", projectPath) +
		inputLine +
		lockfileLine +
		"- Public IO schema: `schema/public-io.json`\n" +
		"- CLI guide: `docs/CLI_Guide.md`\n" +
		pythonLine +
		"- Runner: `bin/bcs-runner.exe`\n\n" +
		"Run scripts write result JSON under `outputs/` and component-log diagnostic bundles under `outputs/logs/`.\n\n" +
		"Available Windows commands:\n\n" +
		strings.Join(commandLines, "\n") +
		"\n"
}

func runtimeExportCLIGuide(files []string, plan *compiler.Plan, projectPath string, defaultInput string, includeSDKExamples bool) string {
	inputs := []model.PublicNodeRef{}
	outputs := []model.PublicNodeRef{}
	components := []string{}
	if plan != nil {
		inputs = plan.System.PublicInputs
		outputs = plan.System.PublicOutputs
		components = plan.System.Components
	}
	scenarioInput := strings.ReplaceAll(defaultInput, "/", `\`)
	if scenarioInput == "" {
		scenarioInput = "project\\inputs\\input.json"
	}
	sections := []string{
		"# Runtime CLI Guide",
		"",
		fmt.Sprintf("- Project: `%s`", projectPath),
		fmt.Sprintf("- Default input: `%s`", defaultInput),
		"- Public schema: `schema/public-io.json`",
		"",
		"## Commands",
		"",
		"- `powershell -ExecutionPolicy Bypass -File .\\check-env.ps1 -Json`",
		"- `powershell -ExecutionPolicy Bypass -File .\\run-default.ps1`",
		"- `powershell -ExecutionPolicy Bypass -File .\\run-default.ps1 -LogBundle outputs\\logs\\default-logs.json`",
		fmt.Sprintf("- `powershell -ExecutionPolicy Bypass -File .\\run-scenario.ps1 -InputFile %s`", scenarioInput),
		"- `powershell -ExecutionPolicy Bypass -File .\\serve.ps1 -RequestFile requests.jsonl -Output outputs\\serve-responses.jsonl`",
	}
	if includeSDKExamples {
		sections = append(sections, "- `runtime\\python\\python.exe sdk-example.py`")
	}
	if len(exportFilesWithPrefix(files, "project/scenarios/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\run-batch.ps1`")
	}
	if seriesInput := firstProjectRelativeSeriesInputExport(files); seriesInput != "" {
		sections = append(sections, fmt.Sprintf("- `powershell -ExecutionPolicy Bypass -File .\\run-series.ps1 -InputFile %s`", strings.ReplaceAll(exportArtifactPath(seriesInput), "/", `\`)))
	}
	if len(exportFilesWithPrefix(files, "project/validation/mappings/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\validate-data.ps1`")
	}
	if len(exportFilesWithPrefix(files, "project/calibration/setups/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\calibrate.ps1`")
	}
	if len(exportFilesWithPrefix(files, "project/optimization/setups/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\optimize.ps1`")
		if includeSDKExamples {
			sections = append(sections, "- `runtime\\python\\python.exe optimize-sdk.py`")
		}
	}
	sections = append(sections,
		"",
		"## Expected Outputs",
		"",
		runtimeExportExpectedOutputs(files, includeSDKExamples),
		"",
		"## Public Inputs",
		"",
		runtimeExportPublicNodeTable(inputs),
		"",
		"## Public Outputs",
		"",
		runtimeExportPublicNodeTable(outputs),
		"",
		"## Components",
		"",
		runtimeExportBulletList(components),
		"",
		"## Included Artifacts",
		"",
		"### Parameter Sets",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/parameter_sets/")),
		"",
		"### Validation Mappings",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/validation/mappings/")),
		"",
		"### Calibration Setups",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/calibration/setups/")),
		"",
		"### Optimization Setups",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/optimization/setups/")),
		"",
		"## Troubleshooting",
		"",
		"- Run `check-env.ps1 -Json` first and inspect any reported problem.",
		"- Run scripts also write component-log diagnostic bundles to `outputs\\logs\\*-logs.json`.",
		"- Keep input paths relative to the export root unless you intentionally pass an absolute path.",
		"- Runner errors use stable exit codes and structured JSON when called with `--error-format json`.",
		"",
		"## Exit Codes",
		"",
		"| Code | Meaning |",
		"|---:|---|",
		"| 0 | Success |",
		"| 1 | Validation or usage error |",
		"| 2 | Runtime error |",
		"| 3 | Input schema or input data error |",
		"| 4 | Python worker/component execution error |",
		"| 5 | License or packaged runtime error |",
		"",
	)
	return strings.Join(sections, "\n")
}

func runtimeExportExpectedOutputs(files []string, includeSDKExamples bool) string {
	rows := [][]string{
		{"check-env.ps1 -Json", "JSON environment report on stdout."},
		{"run-default.ps1", "`outputs\\latest.json` and `outputs\\logs\\latest-logs.json` unless `-Output` or `-LogBundle` is supplied."},
		{"run-scenario.ps1", "`outputs\\scenario-result.json` and `outputs\\logs\\scenario-result-logs.json` unless `-Output` or `-LogBundle` is supplied."},
		{"serve.ps1", "`outputs\\serve-responses.jsonl` when `-Output` is supplied."},
	}
	if includeSDKExamples {
		rows = append(rows, []string{"sdk-example.py", "`outputs\\sdk-example-output.json`."})
	}
	if len(exportFilesWithPrefix(files, "project/scenarios/")) > 0 {
		rows = append(rows, []string{"run-batch.ps1", "One JSON file per scenario under `outputs\\batch\\`, plus per-case log bundles under `outputs\\logs\\`."})
	}
	if firstProjectRelativeSeriesInputExport(files) != "" {
		rows = append(rows, []string{"run-series.ps1", "`outputs\\series-result.json` with `series[]`, aggregated `outputs`, and `final_states`."})
	}
	if len(exportFilesWithPrefix(files, "project/validation/mappings/")) > 0 {
		rows = append(rows, []string{"validate-data.ps1", "`outputs\\validation-result.json`."})
	}
	if len(exportFilesWithPrefix(files, "project/calibration/setups/")) > 0 {
		rows = append(rows, []string{"calibrate.ps1", "`outputs\\calibration-result.json` and a saved parameter set when `-SaveParameterSet` is used."})
	}
	if len(exportFilesWithPrefix(files, "project/optimization/setups/")) > 0 {
		rows = append(rows, []string{"optimize.ps1", "`outputs\\optimization-result.json`, plus a scenario or parameter set when save options are used."})
		if includeSDKExamples {
			rows = append(rows, []string{"optimize-sdk.py", "`outputs\\optimization-sdk-result.json`."})
		}
	}
	lines := []string{"| Command | Output |", "|---|---|"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("| `%s` | %s |", row[0], row[1]))
	}
	return strings.Join(lines, "\n")
}

func runtimeExportPublicNodeTable(nodes []model.PublicNodeRef) string {
	if len(nodes) == 0 {
		return "_None._"
	}
	var builder strings.Builder
	builder.WriteString("| ID | Name | Type | Unit | Required |\n")
	builder.WriteString("|---|---|---|---|---|\n")
	for _, node := range nodes {
		required := "no"
		if node.IsRequired() {
			required = "yes"
		}
		builder.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | `%s` | %s |\n",
			node.ID,
			markdownTableCell(node.Name),
			node.ValueType,
			node.Unit,
			required,
		))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func runtimeExportBulletList(values []string) string {
	if len(values) == 0 {
		return "_None._"
	}
	lines := []string{}
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("- `%s`", value))
	}
	return strings.Join(lines, "\n")
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}
