package projectpath

import (
	"path/filepath"
	"testing"
)

func TestCleanRelativeRejectsUnsafePaths(t *testing.T) {
	tests := []string{
		"",
		".",
		"..",
		"../model.json",
		"assets/../../model.json",
		`C:\models\model.json`,
		"/models/model.json",
		`\models\model.json`,
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := CleanRelative(value); err == nil {
				t.Fatalf("expected error for %q", value)
			}
		})
	}
}

func TestCleanRelativeNormalizesSafePaths(t *testing.T) {
	got, err := CleanRelative(`assets/models/../model.json`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "assets/model.json" {
		t.Fatalf("clean path = %q", got)
	}
}

func TestResolveInsideReturnsAbsoluteProjectPath(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveInside(root, "assets/model.json")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "assets", "model.json")
	if got != want {
		t.Fatalf("resolved = %q want %q", got, want)
	}
}
