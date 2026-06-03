package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
	"github.com/goniegonie/hvac-studio/tools/go/internal/studio"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5174", "address for the Studio server mode")
	repoRoot := flag.String("repo", "", "repository root")
	serverMode := flag.Bool("server", false, "run the Studio HTTP server for automation")
	noWindow := flag.Bool("no-window", false, "deprecated alias for --server")
	window := flag.Bool("window", false, "deprecated; the Wails desktop window is the default")
	flag.Parse()
	_ = window

	root := *repoRoot
	if root == "" {
		var err error
		root, err = findRepoRoot()
		if err != nil {
			log.Fatal(err)
		}
	}

	if shouldRunServer(*serverMode, *noWindow) {
		if err := runServer(root, *addr); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := runDesktop(root); err != nil {
		log.Fatal(err)
	}
}

func shouldRunServer(serverMode bool, noWindow bool) bool {
	return serverMode || noWindow
}

func runServer(root string, addr string) error {
	server, err := studio.New(root)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("HVAC Studio server listening at http://%s\n", listener.Addr().String())
	return http.Serve(listener, server.Handler())
}

func runDesktop(root string) error {
	server, err := studio.New(root)
	if err != nil {
		return err
	}
	assets, err := studio.StaticAssets()
	if err != nil {
		return err
	}

	return wails.Run(&options.App{
		Title:     "HVAC Studio",
		Width:     1440,
		Height:    920,
		MinWidth:  1180,
		MinHeight: 760,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: server.Handler(),
		},
	})
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
		pathExists(platform.BinExecutable(dir, "bcs-runner")) ||
		(pathExists(filepath.Join(dir, "release-manifest.json")) && pathExists(filepath.Join(dir, "runtime", "manifest.json")))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
