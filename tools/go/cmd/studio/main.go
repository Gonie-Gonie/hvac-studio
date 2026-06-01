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
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "examples")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "tools", "go", "go.mod")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root from %s", dir)
		}
		dir = parent
	}
}
