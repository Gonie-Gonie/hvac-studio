package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
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

	resolvedOutput := *outputPath
	if resolvedOutput == "" {
		resolvedOutput = loaded.Project.DefaultOutput
	}
	if resolvedOutput != "" {
		resolvedOutput = resolveProjectPath(loaded.Root, resolvedOutput)
	}
	return apperror.Wrap(apperror.CodeRuntime, runtimecore.WriteResult(resolvedOutput, result))
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
	return apperror.Errorf(apperror.CodeValidation, "usage: bcs-runner <validate|run> --project project.bcsproj")
}
