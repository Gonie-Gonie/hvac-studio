package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/studio"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5174", "address for the Studio web UI")
	repoRoot := flag.String("repo", "", "repository root")
	window := flag.Bool("window", true, "open Studio in an app-style desktop window")
	noWindow := flag.Bool("no-window", false, "run only the local Studio server")
	flag.Parse()
	if *noWindow {
		*window = false
	}

	root := *repoRoot
	if root == "" {
		var err error
		root, err = findRepoRoot()
		if err != nil {
			log.Fatal(err)
		}
	}

	server, err := studio.New(root)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + listener.Addr().String()
	fmt.Printf("HVAC Studio listening at %s\n", url)
	if *window {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := openStudioWindow(url); err != nil {
				log.Printf("open Studio window: %v", err)
			}
		}()
	}
	log.Fatal(http.Serve(listener, server.Handler()))
}

func openStudioWindow(url string) error {
	commands := browserWindowCommands(url)
	var lastErr error
	for _, command := range commands {
		if err := command.Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no browser command available")
}

func browserWindowCommands(url string) []*exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		commands := []*exec.Cmd{}
		for _, browser := range windowsBrowserCandidates() {
			commands = append(commands, exec.Command(browser, "--app="+url, "--new-window"))
		}
		commands = append(commands, exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url))
		return commands
	case "darwin":
		return []*exec.Cmd{exec.Command("open", url)}
	default:
		return []*exec.Cmd{exec.Command("xdg-open", url)}
	}
}

func windowsBrowserCandidates() []string {
	candidates := []string{}
	for _, name := range []string{"msedge.exe", "chrome.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, path)
		}
	}
	for _, path := range []string{
		joinEnvPath("ProgramFiles", "Microsoft", "Edge", "Application", "msedge.exe"),
		joinEnvPath("ProgramFiles(x86)", "Microsoft", "Edge", "Application", "msedge.exe"),
		joinEnvPath("LocalAppData", "Microsoft", "Edge", "Application", "msedge.exe"),
		joinEnvPath("ProgramFiles", "Google", "Chrome", "Application", "chrome.exe"),
		joinEnvPath("ProgramFiles(x86)", "Google", "Chrome", "Application", "chrome.exe"),
		joinEnvPath("LocalAppData", "Google", "Chrome", "Application", "chrome.exe"),
	} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			candidates = append(candidates, path)
		}
	}
	return uniqueStrings(candidates)
}

func joinEnvPath(name string, parts ...string) string {
	root := os.Getenv(name)
	if root == "" {
		return ""
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func findRepoRoot() (string, error) {
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
		root, err := findRootFrom(absStart)
		if err == nil {
			return root, nil
		}
	}
	return "", fmt.Errorf("could not find repository or portable package root")
}

func findRootFrom(dir string) (string, error) {
	for {
		if looksLikeRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository or portable package root from %s", dir)
		}
		dir = parent
	}
}

func looksLikeRoot(dir string) bool {
	if !pathExists(filepath.Join(dir, "examples")) {
		return false
	}
	return pathExists(filepath.Join(dir, "tools", "go", "go.mod")) ||
		pathExists(filepath.Join(dir, "bin", "bcs-runner.exe")) ||
		pathExists(filepath.Join(dir, "bin", "bcs-runner")) ||
		(pathExists(filepath.Join(dir, "release-manifest.json")) && pathExists(filepath.Join(dir, "runtime", "manifest.json")))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
