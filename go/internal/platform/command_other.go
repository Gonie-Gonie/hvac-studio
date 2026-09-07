//go:build !windows

package platform

import (
	"context"
	"os/exec"
)

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
