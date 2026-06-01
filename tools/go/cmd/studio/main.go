package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/studio"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5174", "address for the Studio web UI")
	repoRoot := flag.String("repo", "", "repository root")
	flag.Parse()

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
	fmt.Printf("HVAC Studio listening at http://%s\n", listener.Addr().String())
	log.Fatal(http.Serve(listener, server.Handler()))
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
