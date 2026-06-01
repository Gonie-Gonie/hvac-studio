package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
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
		return err
	}
	if *projectPath == "" {
		return fmt.Errorf("--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return err
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return err
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
		return err
	}
	if *projectPath == "" {
		return fmt.Errorf("--project is required")
	}

	loaded, err := project.Load(*projectPath)
	if err != nil {
		return err
	}

	resolvedInput := *inputPath
	if resolvedInput == "" {
		resolvedInput = loaded.Project.DefaultInput
	}
	if resolvedInput == "" {
		return fmt.Errorf("--input is required when project.default_input is empty")
	}

	input, err := runtimecore.LoadInput(resolveProjectPath(loaded.Root, resolvedInput))
	if err != nil {
		return err
	}

	result, err := runtimecore.Run(context.Background(), loaded, input)
	if err != nil {
		return err
	}

	resolvedOutput := *outputPath
	if resolvedOutput == "" {
		resolvedOutput = loaded.Project.DefaultOutput
	}
	if resolvedOutput != "" {
		resolvedOutput = resolveProjectPath(loaded.Root, resolvedOutput)
	}
	return runtimecore.WriteResult(resolvedOutput, result)
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
	return fmt.Errorf("usage: bcs-runner <validate|run> --project project.bcsproj")
}
