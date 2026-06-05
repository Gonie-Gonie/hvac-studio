package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func RuntimeID() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func ExecutableName(name string) string {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		return name + ".exe"
	}
	return name
}

func BinExecutable(root string, name string) string {
	return filepath.Join(root, "bin", ExecutableName(name))
}

func RuntimePythonPath(root string) string {
	return filepath.Join(root, "runtime", "python", ExecutableName("python"))
}

func RuntimePythonCandidates(root string) []string {
	candidates := []string{
		RuntimePythonPath(root),
		filepath.Join(root, "runtime", "python", "bin", "python"),
	}
	if runtime.GOOS != "windows" {
		candidates = append(candidates, filepath.Join(root, "runtime", "python", "python"))
	}
	return uniquePaths(candidates)
}

func VirtualEnvPythonCandidates(root string) []string {
	return uniquePaths([]string{
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
		filepath.Join(root, ".venv", "bin", "python"),
	})
}

func IsDefaultPythonName(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	switch name {
	case "python", "python.exe", "python3", "python3.exe":
		return true
	default:
		return false
	}
}

func FindNearestRuntimePython(start string) string {
	if start == "" {
		return ""
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		for _, candidate := range RuntimePythonCandidates(absStart) {
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

func LookPath(name string) (string, bool) {
	path, err := exec.LookPath(name)
	return path, err == nil
}

func uniquePaths(paths []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}
