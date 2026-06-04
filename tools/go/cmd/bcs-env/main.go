package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
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

type componentTemplateManifest struct {
	ClassName string          `json:"class_name"`
	Source    json.RawMessage `json:"source"`
}

type generatedWrapperSource struct {
	Layout   string `json:"layout"`
	Metadata string `json:"metadata"`
	Init     string `json:"init"`
	Step     string `json:"step"`
	Helpers  string `json:"helpers"`
	Wrapper  string `json:"wrapper"`
}

type templateSourcePath struct {
	Rel           string
	Label         string
	DeclaresClass bool
}

type projectManifest struct {
	Environment struct {
		Lockfile string `json:"lockfile"`
	} `json:"environment"`
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
	problems = append(problems, validateTemplates(root, mode)...)
	lockfileChecks, lockfileProblems := projectLockfileChecks(root, mode)
	checks = append(checks, lockfileChecks...)
	problems = append(problems, lockfileProblems...)

	pythonPath := resolvePython(root, mode)
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
	if pathExists(filepath.Join(root, "HVAC Studio.exe")) && pathExists(platform.BinExecutable(root, "studio")) {
		return "portable-studio"
	}
	if pathExists(filepath.Join(root, "manifest.json")) &&
		pathExists(filepath.Join(root, "project", "project.bcsproj")) &&
		pathExists(platform.BinExecutable(root, "bcs-runner")) {
		return "runtime-export"
	}
	if pathExists(platform.BinExecutable(root, "bcs-runner")) {
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
			check("runner", "runner executable", platform.BinExecutable(root, "bcs-runner"), true),
			check("env", "environment checker", platform.BinExecutable(root, "bcs-env"), true),
			check("runtime_python", "packaged Python runtime", platform.RuntimePythonPath(root), true),
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
			check("project_template_scalar", "scalar project template", filepath.Join(root, "templates", "projects", "scalar", "project.bcsproj"), true),
			check("component_template_scalar_manifest", "scalar component template manifest", filepath.Join(root, "templates", "components", "scalar", "manifest.json"), true),
		)
		checks = append(checks, scalarComponentTemplateSourceChecks(root)...)
		checks = append(checks,
			check("runner", "runner executable", platform.BinExecutable(root, "bcs-runner"), true),
			check("studio_server", "Studio server executable", platform.BinExecutable(root, "studio"), true),
			check("studio_desktop", "Studio desktop executable", filepath.Join(root, "HVAC Studio.exe"), true),
			check("runtime_python", "packaged Python runtime", platform.RuntimePythonPath(root), true),
		)
	case "runtime-package":
		checks = append(checks,
			check("runner", "runner executable", platform.BinExecutable(root, "bcs-runner"), true),
			check("runtime_python", "packaged Python runtime", platform.RuntimePythonPath(root), true),
		)
	case "repository":
		checks = append(checks,
			check("templates", "templates", filepath.Join(root, "templates"), true),
			check("project_template_scalar", "scalar project template", filepath.Join(root, "templates", "projects", "scalar", "project.bcsproj"), true),
			check("component_template_scalar_manifest", "scalar component template manifest", filepath.Join(root, "templates", "components", "scalar", "manifest.json"), true),
		)
		checks = append(checks, scalarComponentTemplateSourceChecks(root)...)
		checks = append(checks, check("go_module", "Go module", filepath.Join(root, "tools", "go", "go.mod"), true))
	}
	return checks
}

func scalarComponentTemplateSourceChecks(root string) []checkStatus {
	manifest, err := readScalarComponentTemplateManifest(root)
	if err != nil {
		return nil
	}
	paths, problems := componentTemplateSourcePaths(manifest)
	if len(problems) > 0 {
		return nil
	}
	checks := []checkStatus{}
	seen := map[string]bool{}
	for _, source := range paths {
		cleanSource, err := cleanTemplateSource(source.Rel)
		if err != nil || seen[cleanSource] {
			continue
		}
		seen[cleanSource] = true
		checks = append(checks, check(
			"component_template_scalar_source",
			"scalar component template "+source.Label,
			filepath.Join(root, "templates", "components", "scalar", cleanSource),
			true,
		))
	}
	return checks
}

func validateTemplates(root string, mode string) []string {
	if mode != "repository" && mode != "portable-studio" {
		return nil
	}
	manifest, err := readScalarComponentTemplateManifest(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return []string{fmt.Sprintf("invalid scalar component template manifest: %s", err)}
	}
	if strings.TrimSpace(manifest.ClassName) == "" {
		return []string{"invalid scalar component template manifest: class_name is required"}
	}
	sources, sourceProblems := componentTemplateSourcePaths(manifest)
	if len(sourceProblems) > 0 {
		return sourceProblems
	}
	templateRoot := filepath.Join(root, "templates", "components", "scalar")
	for _, source := range sources {
		cleanSource, err := cleanTemplateSource(source.Rel)
		if err != nil {
			return []string{fmt.Sprintf("invalid scalar component template source path: %s", source.Rel)}
		}
		sourcePath := filepath.Join(templateRoot, cleanSource)
		sourceBytes, err := os.ReadFile(sourcePath)
		if err != nil {
			return []string{fmt.Sprintf("missing scalar component template source: %s", sourcePath)}
		}
		if source.DeclaresClass && !strings.Contains(string(sourceBytes), "class "+manifest.ClassName+":") {
			return []string{fmt.Sprintf("scalar component template source does not declare %s", manifest.ClassName)}
		}
	}
	return nil
}

func readScalarComponentTemplateManifest(root string) (componentTemplateManifest, error) {
	manifestPath := filepath.Join(root, "templates", "components", "scalar", "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return componentTemplateManifest{}, err
	}
	var manifest componentTemplateManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return componentTemplateManifest{}, err
	}
	return manifest, nil
}

func componentTemplateSourcePaths(manifest componentTemplateManifest) ([]templateSourcePath, []string) {
	sourceRaw := strings.TrimSpace(string(manifest.Source))
	if sourceRaw == "" || sourceRaw == "null" {
		return nil, []string{"invalid scalar component template manifest: source is required"}
	}
	var sourceString string
	if err := json.Unmarshal(manifest.Source, &sourceString); err == nil {
		if strings.TrimSpace(sourceString) == "" {
			return nil, []string{"invalid scalar component template manifest: source is required"}
		}
		return []templateSourcePath{{
			Rel:           sourceString,
			Label:         "source",
			DeclaresClass: true,
		}}, nil
	}

	var source generatedWrapperSource
	if err := json.Unmarshal(manifest.Source, &source); err != nil {
		return nil, []string{fmt.Sprintf("invalid scalar component template manifest source: %s", err)}
	}
	if strings.TrimSpace(source.Layout) != "generated_wrapper" {
		return nil, []string{"invalid scalar component template manifest: generated_wrapper source layout is required"}
	}
	if strings.TrimSpace(source.Wrapper) == "" {
		return nil, []string{"invalid scalar component template manifest: generated_wrapper wrapper is required"}
	}
	if strings.TrimSpace(source.Step) == "" {
		return nil, []string{"invalid scalar component template manifest: generated_wrapper step is required"}
	}
	if strings.TrimSpace(source.Metadata) != "" {
		if _, err := cleanTemplateSource(source.Metadata); err != nil {
			return nil, []string{fmt.Sprintf("invalid scalar component template source path: %s", source.Metadata)}
		}
	}

	sources := []templateSourcePath{
		{Rel: source.Wrapper, Label: "wrapper", DeclaresClass: true},
		{Rel: source.Step, Label: "step"},
	}
	for _, optional := range []templateSourcePath{
		{Rel: source.Init, Label: "init"},
		{Rel: source.Helpers, Label: "helpers"},
	} {
		if strings.TrimSpace(optional.Rel) != "" {
			sources = append(sources, optional)
		}
	}
	return sources, nil
}

func cleanTemplateSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("path is required")
	}
	cleanSource := filepath.Clean(source)
	if filepath.IsAbs(cleanSource) || cleanSource == ".." || strings.HasPrefix(cleanSource, ".."+string(filepath.Separator)) {
		return "", errors.New("path must stay inside the scalar template")
	}
	return cleanSource, nil
}

func projectLockfileChecks(root string, mode string) ([]checkStatus, []string) {
	projectPaths := findProjectManifests(root, mode)
	checks := []checkStatus{}
	problems := []string{}
	for _, projectPath := range projectPaths {
		projectBytes, err := os.ReadFile(projectPath)
		if err != nil {
			problems = append(problems, fmt.Sprintf("read project for lockfile check: %s: %s", projectPath, err))
			continue
		}
		var manifest projectManifest
		if err := json.Unmarshal(projectBytes, &manifest); err != nil {
			problems = append(problems, fmt.Sprintf("decode project for lockfile check: %s: %s", projectPath, err))
			continue
		}
		lockfile := strings.TrimSpace(manifest.Environment.Lockfile)
		if lockfile == "" {
			continue
		}
		lockfilePath, err := projectOwnedPath(filepath.Dir(projectPath), lockfile)
		if err != nil {
			problems = append(problems, fmt.Sprintf("invalid project Python lockfile %s in %s: %s", lockfile, projectPath, err))
			continue
		}
		relProject, err := filepath.Rel(root, projectPath)
		if err != nil {
			relProject = projectPath
		}
		item := check(
			"project_lockfile",
			"project Python lockfile for "+filepath.ToSlash(relProject),
			lockfilePath,
			true,
		)
		item.Present = pathExists(lockfilePath)
		checks = append(checks, item)
		if !item.Present {
			problems = append(problems, fmt.Sprintf("missing project Python lockfile for %s: %s", filepath.ToSlash(relProject), lockfilePath))
		}
	}
	return checks, problems
}

func findProjectManifests(root string, mode string) []string {
	candidates := []string{}
	switch mode {
	case "runtime-export":
		candidates = append(candidates, filepath.Join(root, "project", "project.bcsproj"))
	case "repository":
		candidates = append(candidates,
			filepath.Join(root, "examples"),
			filepath.Join(root, "templates", "projects"),
			filepath.Join(root, "projects"),
		)
	case "portable-studio":
		candidates = append(candidates,
			filepath.Join(root, "examples"),
			filepath.Join(root, "templates", "projects"),
			filepath.Join(root, "projects"),
		)
	case "runtime-package":
		candidates = append(candidates, filepath.Join(root, "examples"))
	default:
		return []string{}
	}

	projectPaths := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if filepath.Base(candidate) == "project.bcsproj" && !seen[candidate] {
				seen[candidate] = true
				projectPaths = append(projectPaths, candidate)
			}
			continue
		}
		_ = filepath.WalkDir(candidate, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				switch entry.Name() {
				case ".git", ".repo_tools", "dist", "exports", "runs", "__pycache__":
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Name() != "project.bcsproj" || seen[path] {
				return nil
			}
			seen[path] = true
			projectPaths = append(projectPaths, path)
			return nil
		})
	}
	sort.Strings(projectPaths)
	return projectPaths
}

func projectOwnedPath(projectRoot string, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", errors.New("path must be relative to the project root")
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filepath.Join(absRoot, path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("path must stay inside the project root")
	}
	return absPath, nil
}

func check(id string, label string, path string, required bool) checkStatus {
	return checkStatus{
		ID:       id,
		Label:    label,
		Path:     path,
		Required: required,
	}
}

func resolvePython(root string, mode string) string {
	candidates := []string{}
	if mode == "portable-studio" || mode == "runtime-package" || mode == "runtime-export" {
		candidates = append(candidates, platform.RuntimePythonPath(root), os.Getenv("HVAC_STUDIO_PYTHON"))
	} else {
		candidates = append(candidates, os.Getenv("HVAC_STUDIO_PYTHON"), platform.RuntimePythonPath(root))
	}
	candidates = append(candidates, platform.VirtualEnvPythonCandidates(root)...)
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
	if path, ok := platform.LookPath("python"); ok {
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
		candidates = append(candidates, filepath.Join(installRoot, dir.Name(), platform.ExecutableName("python")))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	return candidates
}

func commandVersion(path string, arg string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := platform.CommandContext(ctx, path, arg)
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
		pathExists(platform.BinExecutable(dir, "bcs-runner")) {
		return true
	}
	if pathExists(filepath.Join(dir, "runtime", "manifest.json")) && pathExists(filepath.Join(dir, "python", "bcs_worker")) {
		return true
	}
	if pathExists(filepath.Join(dir, "release-manifest.json")) && pathExists(platform.BinExecutable(dir, "bcs-runner")) {
		return true
	}
	return false
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
