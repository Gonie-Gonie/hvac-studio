package projectpath

import (
	"fmt"
	"path/filepath"
	"strings"
)

func CleanRelative(value string) (string, error) {
	original := strings.TrimSpace(value)
	if original == "" {
		return "", fmt.Errorf("path must be project-relative and stay inside project root: %s", value)
	}
	if strings.HasPrefix(original, "/") || strings.HasPrefix(original, `\`) {
		return "", fmt.Errorf("path must be project-relative and stay inside project root: %s", value)
	}
	clean := filepath.Clean(filepath.FromSlash(original))
	if clean == "." ||
		clean == ".." ||
		filepath.IsAbs(clean) ||
		filepath.VolumeName(clean) != "" ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be project-relative and stay inside project root: %s", value)
	}
	return filepath.ToSlash(clean), nil
}

func ResolveInside(root string, value string) (string, error) {
	clean, err := CleanRelative(value)
	if err != nil {
		return "", err
	}
	return resolveCandidateInside(root, filepath.FromSlash(clean), value)
}

func ResolveOwned(root string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path must stay inside project root: %s", value)
	}
	nativeValue := filepath.FromSlash(value)
	if filepath.IsAbs(nativeValue) || filepath.VolumeName(nativeValue) != "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return resolveCandidateInside(root, nativeValue, value)
	}
	clean, err := CleanRelative(value)
	if err != nil {
		return "", err
	}
	return resolveCandidateInside(root, filepath.FromSlash(clean), value)
}

func resolveCandidateInside(root string, candidate string, original string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(candidate) && filepath.VolumeName(candidate) == "" {
		candidate = filepath.Join(absRoot, candidate)
	}
	resolved, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must stay inside project root: %s", original)
	}
	return resolved, nil
}
