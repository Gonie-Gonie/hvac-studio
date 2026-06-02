package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type envStatus struct {
	OK       bool          `json:"ok"`
	Root     string        `json:"root"`
	Mode     string        `json:"mode"`
	Python   toolStatus    `json:"python"`
	Checks   []checkStatus `json:"checks"`
	Problems []string      `json:"problems"`
}

type toolStatus struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type checkStatus struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
}

var versionCommand = commandVersion

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	command, commandArgs := parseCommand(args)
	if command != "status" && command != "check" {
		return usage()
	}

	flags := flag.NewFlagSet("bcs-env "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rootFlag := flags.String("root", "", "repository or package root")
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	if err := flags.Parse(commandArgs); err != nil {
		return err
	}

	root, err := findRoot(*rootFlag)
	if err != nil {
		return err
	}
	status := collectStatus(root)
	if *jsonOutput {
		if err := writeJSON(stdout, status); err != nil {
			return err
		}
	} else {
		writeHuman(stdout, status)
	}
	if command == "check" && !status.OK {
		return fmt.Errorf("environment check failed with %d problem(s)", len(status.Problems))
	}
	return nil
}

func parseCommand(args []string) (string, []string) {
	if len(args) < 2 {
		return "status", []string{}
	}
	if strings.HasPrefix(args[1], "-") {
		return "status", args[1:]
	}
	return args[1], args[2:]
}

func usage() error {
	return errors.New("usage: bcs-env [status|check] [--root PATH] [--json]")
}

func collectStatus(root string) envStatus {
	root, _ = filepath.Abs(root)
	mode := detectMode(root)
	checks := requiredChecks(root, mode)
	problems := []string{}
	for i := range checks {
		checks[i].Present = pathExists(checks[i].Path)
		if checks[i].Required && !checks[i].Present {
			problems = append(problems, fmt.Sprintf("missing %s: %s", checks[i].Label, checks[i].Path))
		}
	}

	pythonPath := resolvePython(root)
	python := toolStatus{
		Path:    pythonPath,
		Present: pythonPath != "",
	}
	if python.Present {
		python.Version, python.Error = versionCommand(pythonPath, "--version")
		if python.Error != "" {
			problems = append(problems, fmt.Sprintf("Python runtime failed: %s", python.Error))
		}
	} else {
		problems = append(problems, "missing Python runtime")
	}

	return envStatus{
		OK:       len(problems) == 0,
		Root:     root,
		Mode:     mode,
		Python:   python,
		Checks:   checks,
		Problems: problems,
	}
}

func detectMode(root string) string {
	if pathExists(filepath.Join(root, "tools", "go", "go.mod")) {
		return "repository"
	}
	if pathExists(filepath.Join(root, "HVAC Studio.exe")) && pathExists(filepath.Join(root, "bin", "studio.exe")) {
		return "portable-studio"
	}
	if pathExists(filepath.Join(root, "manifest.json")) &&
		pathExists(filepath.Join(root, "project", "project.bcsproj")) &&
		pathExists(filepath.Join(root, "bin", executableName("bcs-runner"))) {
		return "runtime-export"
	}
	if pathExists(filepath.Join(root, "bin", executableName("bcs-runner"))) {
		return "runtime-package"
	}
	return "unknown"
}

func requiredChecks(root string, mode string) []checkStatus {
	if mode == "runtime-export" {
		return []checkStatus{
			check("export_manifest", "export manifest", filepath.Join(root, "manifest.json"), true),
			check("readme", "export README", filepath.Join(root, "README.md"), true),
			check("run_script", "default run script", filepath.Join(root, "run-default.ps1"), true),
			check("project", "exported project", filepath.Join(root, "project", "project.bcsproj"), true),
			check("graph", "exported graph", filepath.Join(root, "project", "graph.json"), true),
			check("interface_schema", "public IO schema", filepath.Join(root, "schema", "public-io.json"), true),
			check("runner", "runner executable", filepath.Join(root, "bin", executableName("bcs-runner")), true),
			check("env", "environment checker", filepath.Join(root, "bin", executableName("bcs-env")), true),
			check("runtime_python", "packaged Python runtime", filepath.Join(root, "runtime", "python", executableName("python")), true),
		}
	}
	checks := []checkStatus{
		check("runtime_manifest", "runtime manifest", filepath.Join(root, "runtime", "manifest.json"), true),
		check("python_worker", "Python worker", filepath.Join(root, "python", "bcs_worker"), true),
		check("python_sdk", "Python SDK", filepath.Join(root, "python", "bcs_sdk"), true),
		check("schema", "schemas", filepath.Join(root, "schema"), true),
		check("examples", "examples", filepath.Join(root, "examples"), true),
	}
	switch mode {
	case "portable-studio":
		checks = append(checks,
			check("templates", "templates", filepath.Join(root, "templates"), true),
			check("runner", "runner executable", filepath.Join(root, "bin", executableName("bcs-runner")), true),
			check("studio_server", "Studio server executable", filepath.Join(root, "bin", executableName("studio")), true),
			check("studio_desktop", "Studio desktop executable", filepath.Join(root, "HVAC Studio.exe"), true),
		)
	case "runtime-package":
		checks = append(checks, check("runner", "runner executable", filepath.Join(root, "bin", executableName("bcs-runner")), true))
	case "repository":
		checks = append(checks,
			check("templates", "templates", filepath.Join(root, "templates"), true),
			check("go_module", "Go module", filepath.Join(root, "tools", "go", "go.mod"), true),
		)
	}
	return checks
}

func check(id string, label string, path string, required bool) checkStatus {
	return checkStatus{
		ID:       id,
		Label:    label,
		Path:     path,
		Required: required,
	}
}

func resolvePython(root string) string {
	candidates := []string{
		os.Getenv("HVAC_STUDIO_PYTHON"),
		filepath.Join(root, "runtime", "python", executableName("python")),
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
		filepath.Join(root, ".venv", "bin", "python"),
	}
	candidates = append(candidates, repoManagedPythonCandidates(root)...)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if pathExists(candidate) {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	if path, err := exec.LookPath("python"); err == nil {
		return path
	}
	return ""
}

func repoManagedPythonCandidates(root string) []string {
	installRoot := filepath.Join(root, ".repo_tools", "python")
	dirs, err := os.ReadDir(installRoot)
	if err != nil {
		return []string{}
	}
	candidates := []string{}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		candidates = append(candidates, filepath.Join(installRoot, dir.Name(), executableName("python")))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	return candidates
}

func commandVersion(path string, arg string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, arg)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if ctx.Err() != nil {
		return text, ctx.Err().Error()
	}
	if err != nil {
		return text, err.Error()
	}
	return text, ""
}

func findRoot(explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if !looksLikeRoot(root) {
			return "", fmt.Errorf("path does not look like an HVAC Studio root: %s", root)
		}
		return root, nil
	}

	starts := []string{}
	if dir, err := os.Getwd(); err == nil {
		starts = append(starts, dir)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		starts = append(starts, exeDir, filepath.Dir(exeDir))
	}

	seen := map[string]bool{}
	for _, start := range starts {
		absStart, err := filepath.Abs(start)
		if err != nil || seen[absStart] {
			continue
		}
		seen[absStart] = true
		if root, err := findRootFrom(absStart); err == nil {
			return root, nil
		}
	}
	return "", errors.New("could not find repository or package root")
}

func findRootFrom(dir string) (string, error) {
	for {
		if looksLikeRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository or package root from %s", dir)
		}
		dir = parent
	}
}

func looksLikeRoot(dir string) bool {
	if pathExists(filepath.Join(dir, "tools", "go", "go.mod")) {
		return true
	}
	if pathExists(filepath.Join(dir, "manifest.json")) &&
		pathExists(filepath.Join(dir, "project", "project.bcsproj")) &&
		pathExists(filepath.Join(dir, "bin", executableName("bcs-runner"))) {
		return true
	}
	if pathExists(filepath.Join(dir, "runtime", "manifest.json")) && pathExists(filepath.Join(dir, "python", "bcs_worker")) {
		return true
	}
	if pathExists(filepath.Join(dir, "release-manifest.json")) && pathExists(filepath.Join(dir, "bin", executableName("bcs-runner"))) {
		return true
	}
	return false
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeJSON(w io.Writer, status envStatus) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

func writeHuman(w io.Writer, status envStatus) {
	fmt.Fprintf(w, "HVAC Studio environment\n")
	fmt.Fprintf(w, "root: %s\n", status.Root)
	fmt.Fprintf(w, "mode: %s\n", status.Mode)
	if status.Python.Present {
		version := status.Python.Version
		if version == "" {
			version = "version unavailable"
		}
		fmt.Fprintf(w, "python: ok %s (%s)\n", status.Python.Path, version)
		if status.Python.Error != "" {
			fmt.Fprintf(w, "python warning: %s\n", status.Python.Error)
		}
	} else {
		fmt.Fprintf(w, "python: missing\n")
	}
	for _, item := range status.Checks {
		state := "missing"
		if item.Present {
			state = "ok"
		}
		fmt.Fprintf(w, "%s: %s %s\n", item.ID, state, item.Path)
	}
	if status.OK {
		fmt.Fprintf(w, "status: ok\n")
		return
	}
	fmt.Fprintf(w, "status: failed\n")
	for _, problem := range status.Problems {
		fmt.Fprintf(w, "- %s\n", problem)
	}
}
