package platform

import (
	"context"
	"testing"
)

func TestCommandContextHidesWindowsConsole(t *testing.T) {
	cmd := CommandContext(context.Background(), "python", "--version")
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is required on Windows")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow must be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags = %#x, want CREATE_NO_WINDOW", cmd.SysProcAttr.CreationFlags)
	}
}
