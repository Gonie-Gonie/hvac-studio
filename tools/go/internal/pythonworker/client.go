package pythonworker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
)

type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	stderr  *bytes.Buffer
	mu      sync.Mutex
	nextID  atomic.Int64
}

type Response struct {
	ID      string         `json:"id"`
	OK      bool           `json:"ok"`
	Message string         `json:"message"`
	Outputs map[string]any `json:"outputs"`
	State   map[string]any `json:"state"`
	Error   *WorkerError   `json:"error"`
}

type WorkerError struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Traceback string `json:"traceback"`
}

func (e *WorkerError) Error() string {
	if e == nil {
		return ""
	}
	if e.Traceback != "" {
		return fmt.Sprintf("%s: %s\n%s", e.Type, e.Message, e.Traceback)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func Start(ctx context.Context, pythonExe string, projectRoot string) (*Client, error) {
	if pythonExe == "" {
		pythonExe = "python"
	}

	workerPath, err := findWorkerPath(projectRoot)
	if err != nil {
		return nil, err
	}

	cmd := platform.CommandContext(ctx, pythonExe, "-m", "bcs_worker.worker", "--stdio")
	cmd.Dir = projectRoot
	cmd.Env = withPythonPath(os.Environ(), []string{workerPath, projectRoot})

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start python worker: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	client := &Client{
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
		stderr:  stderr,
	}
	if err := client.Ping(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Ping() error {
	var response Response
	return c.request(map[string]any{"type": "ping"}, &response)
}

func (c *Client) LoadComponent(componentID string, classPath string, projectRoot string) error {
	var response Response
	return c.request(map[string]any{
		"type":         "load_component",
		"component_id": componentID,
		"class":        classPath,
		"project_root": projectRoot,
	}, &response)
}

func (c *Client) InitializeComponent(componentID string, params map[string]any, context map[string]any) (map[string]any, error) {
	var response Response
	err := c.request(map[string]any{
		"type":         "initialize_component",
		"component_id": componentID,
		"params":       params,
		"context":      context,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.State, nil
}

func (c *Client) EvaluateComponent(componentID string, inputs map[string]any, state map[string]any, params map[string]any, context map[string]any) (map[string]any, map[string]any, error) {
	var response Response
	err := c.request(map[string]any{
		"type":         "evaluate_component",
		"component_id": componentID,
		"inputs":       inputs,
		"state":        state,
		"params":       params,
		"context":      context,
	}, &response)
	if err != nil {
		return nil, nil, err
	}
	return response.Outputs, response.State, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var response Response
	_ = c.request(map[string]any{"type": "shutdown"}, &response)
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}

func (c *Client) request(payload map[string]any, response *Response) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := strconv.FormatInt(c.nextID.Add(1), 10)
	payload["id"] = id

	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(c.stdin, string(encoded)); err != nil {
		return fmt.Errorf("write worker request: %w", err)
	}

	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return fmt.Errorf("read worker response: %w; stderr: %s", err, c.stderr.String())
		}
		return fmt.Errorf("worker exited without response; stderr: %s", c.stderr.String())
	}

	line := c.scanner.Bytes()
	if err := json.Unmarshal(line, response); err != nil {
		return fmt.Errorf("decode worker response %q: %w", string(line), err)
	}
	if response.ID != id {
		return fmt.Errorf("worker response id mismatch: got %s want %s", response.ID, id)
	}
	if !response.OK {
		if response.Error != nil {
			return response.Error
		}
		return fmt.Errorf("worker request failed")
	}
	return nil
}

func findWorkerPath(start string) (string, error) {
	candidates := []string{}
	if start != "" {
		absStart, _ := filepath.Abs(start)
		for {
			candidates = append(candidates, filepath.Join(absStart, "python", "bcs_worker"))
			parent := filepath.Dir(absStart)
			if parent == absStart {
				break
			}
			absStart = parent
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		abs := cwd
		for {
			candidates = append(candidates, filepath.Join(abs, "python", "bcs_worker"))
			parent := filepath.Dir(abs)
			if parent == abs {
				break
			}
			abs = parent
		}
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if _, err := os.Stat(filepath.Join(candidate, "bcs_worker", "worker.py")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find python/bcs_worker from %s", start)
}

func withPythonPath(env []string, paths []string) []string {
	value := strings.Join(paths, string(os.PathListSeparator))
	key := "PYTHONPATH"
	next := make([]string, 0, len(env)+1)
	found := false
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), key+"=") {
			found = true
			existing := strings.TrimPrefix(item, key+"=")
			if existing != "" {
				value = value + string(os.PathListSeparator) + existing
			}
			next = append(next, key+"="+value)
			continue
		}
		next = append(next, item)
	}
	if !found {
		next = append(next, key+"="+value)
	}
	return next
}
