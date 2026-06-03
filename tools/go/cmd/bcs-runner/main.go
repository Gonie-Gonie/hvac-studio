package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

func main() {
	if err := run(os.Args); err != nil {
		code := apperror.ExitCode(err)
		fmt.Fprintf(os.Stderr, "error[%s]: %v\n", apperror.CodeName(apperror.Code(code)), err)
		os.Exit(code)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return usage()
	}

	switch args[1] {
	case "validate":
		return validate(args[2:])
	case "run":
		return runProject(args[2:])
	case "serve":
		return serveProject(args[2:], os.Stdin, os.Stdout)
	case "schema":
		return exportSchema(args[2:])
	case "validate-data":
		return validateData(args[2:])
	default:
		return usage()
	}
}

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	projectPath := flags.String("project", "", "path to project.bcsproj")
	if err := flags.Parse(args); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *projectPath == "" {
		return apperror.Errorf(apperror.CodeValidation, "--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	for _, diagnostic := range plan.Diagnostics {
		fmt.Printf("%s: %s\n", diagnostic.Severity, diagnostic.Message)
	}
	fmt.Printf("validation ok: project=%s entry_system=%s components=%d connections=%d\n",
		loaded.Project.ProjectName,
		loaded.Project.EntrySystem,
		len(plan.System.Components),
		len(plan.System.Connections),
	)
	return nil
}

func runProject(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	projectPath := flags.String("project", "", "path to project.bcsproj")
	inputPath := flags.String("input", "", "path to input JSON")
	outputPath := flags.String("output", "", "path to output JSON")
	parameterSetPath := flags.String("parameter-set", "", "project-relative parameter set JSON")
	if err := flags.Parse(args); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *projectPath == "" {
		return apperror.Errorf(apperror.CodeValidation, "--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *parameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, *parameterSetPath); err != nil {
			return err
		}
	}

	resolvedInput := *inputPath
	if resolvedInput == "" {
		resolvedInput = loaded.Project.DefaultInput
	}
	if resolvedInput == "" {
		return apperror.Errorf(apperror.CodeInput, "--input is required when project.default_input is empty")
	}

	input, err := runtimecore.LoadInput(resolveProjectPath(loaded.Root, resolvedInput))
	if err != nil {
		return apperror.Wrap(apperror.CodeInput, err)
	}

	result, err := runtimecore.Run(context.Background(), loaded, input)
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if *parameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(*parameterSetPath)
	}

	resolvedOutput := *outputPath
	if resolvedOutput == "" {
		resolvedOutput = loaded.Project.DefaultOutput
	}
	if resolvedOutput != "" {
		resolvedOutput = resolveProjectPath(loaded.Root, resolvedOutput)
	}
	return apperror.Wrap(apperror.CodeRuntime, runtimecore.WriteResult(resolvedOutput, result))
}

type serveRequest struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Inputs  map[string]any `json:"inputs"`
	Context map[string]any `json:"context"`
}

type serveResponse struct {
	ID      string                 `json:"id,omitempty"`
	OK      bool                   `json:"ok"`
	Message string                 `json:"message,omitempty"`
	Result  *runtimecore.RunResult `json:"result,omitempty"`
	Error   *serveError            `json:"error,omitempty"`
}

type serveError struct {
	Kind    string `json:"kind"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func serveProject(args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	projectPath := flags.String("project", "", "path to project.bcsproj")
	if err := flags.Parse(args); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *projectPath == "" {
		return apperror.Errorf(apperror.CodeValidation, "--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	session, err := runtimecore.NewSession(context.Background(), loaded)
	if err != nil {
		return err
	}
	defer session.Close()

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req serveRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := encoder.Encode(serveResponse{OK: false, Error: responseError(apperror.Wrap(apperror.CodeInput, fmt.Errorf("invalid JSON request: %w", err)))}); err != nil {
				return apperror.Wrap(apperror.CodeRuntime, err)
			}
			continue
		}
		if req.Type == "shutdown" {
			if err := encoder.Encode(serveResponse{ID: req.ID, OK: true, Message: "shutdown"}); err != nil {
				return apperror.Wrap(apperror.CodeRuntime, err)
			}
			return nil
		}
		result, err := session.Evaluate(runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context})
		if err != nil {
			if err := encoder.Encode(serveResponse{ID: req.ID, OK: false, Error: responseError(err)}); err != nil {
				return apperror.Wrap(apperror.CodeRuntime, err)
			}
			continue
		}
		if err := encoder.Encode(serveResponse{ID: req.ID, OK: true, Result: result}); err != nil {
			return apperror.Wrap(apperror.CodeRuntime, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return apperror.Wrap(apperror.CodeInput, err)
	}
	return nil
}

func responseError(err error) *serveError {
	code := apperror.ErrorCode(err)
	return &serveError{
		Kind:    apperror.CodeName(code),
		Code:    int(code),
		Message: err.Error(),
	}
}

func exportSchema(args []string) error {
	flags := flag.NewFlagSet("schema", flag.ContinueOnError)
	projectPath := flags.String("project", "", "path to project.bcsproj")
	outputPath := flags.String("output", "", "path to output JSON")
	if err := flags.Parse(args); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *projectPath == "" {
		return apperror.Errorf(apperror.CodeValidation, "--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	return apperror.Wrap(apperror.CodeRuntime, schemaexport.Write(resolveProjectPath(loaded.Root, *outputPath), schema))
}

func validateData(args []string) error {
	flags := flag.NewFlagSet("validate-data", flag.ContinueOnError)
	projectPath := flags.String("project", "", "path to project.bcsproj")
	mappingPath := flags.String("mapping", "", "project-relative path to validation mapping JSON")
	outputPath := flags.String("output", "", "path to output JSON")
	parameterSetPath := flags.String("parameter-set", "", "project-relative parameter set JSON")
	highErrorRows := flags.Int("high-error-rows", 3, "number of high-error rows to keep per output")
	if err := flags.Parse(args); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *projectPath == "" {
		return apperror.Errorf(apperror.CodeValidation, "--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if *parameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, *parameterSetPath); err != nil {
			return err
		}
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, *mappingPath)
	if err != nil {
		return err
	}
	result, err := modelvalidation.Run(context.Background(), loaded, mapping, modelvalidation.Options{
		HighErrorRows: *highErrorRows,
	})
	if err != nil {
		return err
	}
	if *parameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(*parameterSetPath)
	}
	return writeJSONOutput(resolveProjectPath(loaded.Root, *outputPath), result)
}

func writeJSONOutput(outputPath string, value any) error {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if outputPath == "" {
		fmt.Println(string(output))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	return apperror.Wrap(apperror.CodeRuntime, os.WriteFile(outputPath, append(output, '\n'), 0o644))
}

func resolveProjectPath(projectRoot string, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectRoot, path)
}

func usage() error {
	return apperror.Errorf(apperror.CodeValidation, "usage: bcs-runner <validate|run|serve|schema|validate-data> --project project.bcsproj")
}
